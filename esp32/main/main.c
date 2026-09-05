#include <math.h>
#include <stdbool.h>
#include <stdio.h>
#include <string.h>

#include "esp_adc/adc_cali.h"
#include "esp_adc/adc_cali_scheme.h"
#include "esp_adc/adc_oneshot.h"
#include "esp_event.h"
#include "esp_http_client.h"
#include "esp_log.h"
#include "esp_netif.h"
#include "esp_system.h"
#include "esp_wifi.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include "freertos/task.h"
#include "nvs_flash.h"

#define TAG "kindle-location"
#define WIFI_CONNECTED_BIT BIT0

typedef struct {
    const char *id;
    int min_ohms;
    int max_ohms;
} zone_profile_t;

static const zone_profile_t zone_profiles[] = {
    {CONFIG_KD_ZONE_1_ID, CONFIG_KD_ZONE_1_MIN_OHMS, CONFIG_KD_ZONE_1_MAX_OHMS},
    {CONFIG_KD_ZONE_2_ID, CONFIG_KD_ZONE_2_MIN_OHMS, CONFIG_KD_ZONE_2_MAX_OHMS},
    {CONFIG_KD_ZONE_3_ID, CONFIG_KD_ZONE_3_MIN_OHMS, CONFIG_KD_ZONE_3_MAX_OHMS},
};

static EventGroupHandle_t wifi_event_group;
static adc_oneshot_unit_handle_t adc_handle;
static adc_cali_handle_t adc_calibration;
static bool adc_is_calibrated;

static void wifi_event_handler(void *arg, esp_event_base_t event_base, int32_t event_id, void *event_data) {
    if (event_base == WIFI_EVENT && event_id == WIFI_EVENT_STA_START) {
        esp_wifi_connect();
    } else if (event_base == WIFI_EVENT && event_id == WIFI_EVENT_STA_DISCONNECTED) {
        ESP_LOGW(TAG, "Wi-Fi disconnected; retrying");
        esp_wifi_connect();
        xEventGroupClearBits(wifi_event_group, WIFI_CONNECTED_BIT);
    } else if (event_base == IP_EVENT && event_id == IP_EVENT_STA_GOT_IP) {
        xEventGroupSetBits(wifi_event_group, WIFI_CONNECTED_BIT);
    }
}

static void wifi_init(void) {
    wifi_event_group = xEventGroupCreate();
    ESP_ERROR_CHECK(esp_netif_init());
    ESP_ERROR_CHECK(esp_event_loop_create_default());
    esp_netif_create_default_wifi_sta();
    wifi_init_config_t wifi_config = WIFI_INIT_CONFIG_DEFAULT();
    ESP_ERROR_CHECK(esp_wifi_init(&wifi_config));
    ESP_ERROR_CHECK(esp_event_handler_register(WIFI_EVENT, ESP_EVENT_ANY_ID, &wifi_event_handler, NULL));
    ESP_ERROR_CHECK(esp_event_handler_register(IP_EVENT, IP_EVENT_STA_GOT_IP, &wifi_event_handler, NULL));
    wifi_config_t station_config = {0};
    strncpy((char *)station_config.sta.ssid, CONFIG_KD_WIFI_SSID, sizeof(station_config.sta.ssid));
    strncpy((char *)station_config.sta.password, CONFIG_KD_WIFI_PASSWORD, sizeof(station_config.sta.password));
    station_config.sta.threshold.authmode = WIFI_AUTH_WPA2_PSK;
    ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));
    ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &station_config));
    ESP_ERROR_CHECK(esp_wifi_start());
    ESP_LOGI(TAG, "Wi-Fi started");
}

static void adc_init(void) {
    adc_oneshot_unit_init_cfg_t unit_config = {
        .unit_id = ADC_UNIT_1,
        .ulp_mode = ADC_ULP_MODE_DISABLE,
    };
    ESP_ERROR_CHECK(adc_oneshot_new_unit(&unit_config, &adc_handle));
    adc_oneshot_chan_cfg_t channel_config = {
        .atten = ADC_ATTEN_DB_12,
        .bitwidth = ADC_BITWIDTH_DEFAULT,
    };
    adc_channel_t channel = (adc_channel_t)CONFIG_KD_ADC_CHANNEL;
    ESP_ERROR_CHECK(adc_oneshot_config_channel(adc_handle, channel, &channel_config));
    ESP_LOGI(TAG, "ADC GPIO=%d channel=%d", CONFIG_KD_ADC_GPIO, CONFIG_KD_ADC_CHANNEL);

#if ADC_CALI_SCHEME_CURVE_FITTING_SUPPORTED
    adc_cali_curve_fitting_config_t calibration_config = {
        .unit_id = ADC_UNIT_1,
        .atten = ADC_ATTEN_DB_12,
        .bitwidth = ADC_BITWIDTH_DEFAULT,
    };
    if (adc_cali_create_scheme_curve_fitting(&calibration_config, &adc_calibration) == ESP_OK) {
        adc_is_calibrated = true;
    }
#elif ADC_CALI_SCHEME_LINE_FITTING_SUPPORTED
    adc_cali_line_fitting_config_t calibration_config = {
        .unit_id = ADC_UNIT_1,
        .atten = ADC_ATTEN_DB_12,
        .bitwidth = ADC_BITWIDTH_DEFAULT,
    };
    if (adc_cali_create_scheme_line_fitting(&calibration_config, &adc_calibration) == ESP_OK) {
        adc_is_calibrated = true;
    }
#endif
}

static int read_adc_mv(void) {
    int raw = 0;
    adc_channel_t channel = (adc_channel_t)CONFIG_KD_ADC_CHANNEL;
    if (adc_oneshot_read(adc_handle, channel, &raw) != ESP_OK) {
        return -1;
    }
    int millivolts = 0;
    if (adc_is_calibrated && adc_cali_raw_to_voltage(adc_calibration, raw, &millivolts) == ESP_OK) {
        return millivolts;
    }
    return raw * CONFIG_KD_VCC_MV / 4095;
}

static int read_smoothed_mv(void) {
    int total = 0;
    int valid = 0;
    for (int i = 0; i < 9; i++) {
        int value = read_adc_mv();
        if (value >= 0) {
            total += value;
            valid++;
        }
        vTaskDelay(pdMS_TO_TICKS(15));
    }
    return valid == 0 ? -1 : total / valid;
}

static float resistance_from_mv(int millivolts) {
    if (millivolts <= 0 || millivolts >= CONFIG_KD_VCC_MV) {
        return INFINITY;
    }
    float voltage = (float)millivolts;
    return ((float)CONFIG_KD_PULLUP_OHMS * voltage) / ((float)CONFIG_KD_VCC_MV - voltage);
}

static const char *zone_for_resistance(float resistance) {
    for (size_t i = 0; i < sizeof(zone_profiles) / sizeof(zone_profiles[0]); i++) {
        if (resistance >= zone_profiles[i].min_ohms && resistance <= zone_profiles[i].max_ohms) {
            return zone_profiles[i].id;
        }
    }
    return "unknown";
}

static esp_err_t publish_location(const char *zone, int millivolts, float resistance) {
    char endpoint[256];
    snprintf(endpoint, sizeof(endpoint), "%s/api/states/sensor.%s_location", CONFIG_KD_HA_URL, CONFIG_KD_DEVICE_ID);
    char body[640];
    if (isfinite(resistance)) {
        snprintf(body, sizeof(body),
                 "{\"state\":\"%s\",\"attributes\":{\"friendly_name\":\"Kindle location\",\"source\":\"esp32-resistance\",\"raw_mv\":%d,\"resistance_ohms\":%.1f}}",
                 zone, millivolts, resistance);
    } else {
        snprintf(body, sizeof(body),
                 "{\"state\":\"%s\",\"attributes\":{\"friendly_name\":\"Kindle location\",\"source\":\"esp32-resistance\",\"raw_mv\":%d,\"resistance_ohms\":null}}",
                 zone, millivolts);
    }
    esp_http_client_config_t config = {.url = endpoint, .method = HTTP_METHOD_POST, .timeout_ms = 10000};
    esp_http_client_handle_t client = esp_http_client_init(&config);
    if (client == NULL) return ESP_FAIL;
    char auth_header[384];
    snprintf(auth_header, sizeof(auth_header), "Bearer %s", CONFIG_KD_HA_TOKEN);
    esp_http_client_set_header(client, "Authorization", auth_header);
    esp_http_client_set_header(client, "Content-Type", "application/json");
    esp_http_client_set_post_field(client, body, strlen(body));
    esp_err_t result = esp_http_client_perform(client);
    int status_code = result == ESP_OK ? esp_http_client_get_status_code(client) : 0;
    if (result == ESP_OK && status_code >= 200 && status_code < 300) {
        ESP_LOGI(TAG, "location=%s resistance=%.1fΩ HTTP=%d", zone, resistance, status_code);
    } else if (result == ESP_OK) {
        ESP_LOGW(TAG, "HA location update rejected: HTTP=%d", status_code);
        result = ESP_FAIL;
    } else {
        ESP_LOGW(TAG, "HA location update failed: %s", esp_err_to_name(result));
    }
    esp_http_client_cleanup(client);
    return result;
}

void app_main(void) {
    esp_err_t nvs_result = nvs_flash_init();
    if (nvs_result == ESP_ERR_NVS_NO_FREE_PAGES || nvs_result == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        ESP_ERROR_CHECK(nvs_flash_erase());
        nvs_result = nvs_flash_init();
    }
    ESP_ERROR_CHECK(nvs_result);
    wifi_init();
    adc_init();
    xEventGroupWaitBits(wifi_event_group, WIFI_CONNECTED_BIT, pdFALSE, pdFALSE, portMAX_DELAY);
    const char *last_zone = "";
    TickType_t last_report = 0;
    while (true) {
        int millivolts = read_smoothed_mv();
        float resistance = millivolts < 0 ? INFINITY : resistance_from_mv(millivolts);
        const char *zone = millivolts < 0 ? "unknown" : zone_for_resistance(resistance);
        TickType_t now = xTaskGetTickCount();
        bool interval_elapsed = now - last_report >= pdMS_TO_TICKS(CONFIG_KD_REPORT_INTERVAL_SEC * 1000);
        bool zone_changed = strcmp(last_zone, zone) != 0;
        bool retry_window_open = last_report == 0 || now - last_report >= pdMS_TO_TICKS(5000);
        if ((zone_changed && retry_window_open) || interval_elapsed) {
            if (publish_location(zone, millivolts < 0 ? 0 : millivolts, resistance) == ESP_OK) {
                last_zone = zone;
            }
            last_report = now;
        }
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}
