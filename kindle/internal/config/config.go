package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	HAURL                       string `json:"ha_url"`
	LongLivedToken              string `json:"long_lived_token"`
	DeviceID                    string `json:"device_id"`
	PortableEntity              string `json:"portable_entity,omitempty"`
	ZoneEntityPrefix            string `json:"zone_entity_prefix,omitempty"`
	LocationEntity              string `json:"location_entity,omitempty"`
	BatteryEntity               string `json:"battery_entity,omitempty"`
	InputDevice                 string `json:"input_device"`
	TouchWidth                  int    `json:"touch_width"`
	TouchHeight                 int    `json:"touch_height"`
	DisplayWidth                int    `json:"display_width"`
	DisplayHeight               int    `json:"display_height"`
	BatteryIntervalSec          int    `json:"battery_interval_seconds"`
	DashboardRefreshIntervalSec int    `json:"dashboard_refresh_interval_seconds"`
	ForceRefreshIntervalSec     int    `json:"force_refresh_interval_seconds"`
	TempDir                     string `json:"temp_dir"`
	FontPath                    string `json:"font_path,omitempty"`
	FontBoldPath                string `json:"font_bold_path,omitempty"`
}

const (
	Version             = "0.1.5"
	DefaultFontPath     = "/mnt/us/extensions/kindle-ha-dashboard/fonts/NotoSansCJKsc-Regular.otf"
	DefaultFontBoldPath = "/mnt/us/extensions/kindle-ha-dashboard/fonts/NotoSansCJKsc-Bold.otf"
)

func Defaults() Config {
	return Config{
		HAURL:                       "http://homeassistant.local:8123",
		DeviceID:                    "kindle-01",
		InputDevice:                 "/dev/input/event1",
		TouchWidth:                  599,
		TouchHeight:                 799,
		DisplayWidth:                600,
		DisplayHeight:               800,
		BatteryIntervalSec:          300,
		DashboardRefreshIntervalSec: 10,
		ForceRefreshIntervalSec:     1800,
		TempDir:                     "/var/tmp/kindle-ha-dashboard",
		FontPath:                    DefaultFontPath,
		FontBoldPath:                DefaultFontBoldPath,
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("config file %q does not exist", path)
		}
		return cfg, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	cfg.applyDerivedDefaults()
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c *Config) applyDerivedDefaults() {
	c.HAURL = strings.TrimRight(strings.TrimSpace(c.HAURL), "/")
	c.DeviceID = slug(c.DeviceID)
	if c.PortableEntity == "" {
		c.PortableEntity = "sensor.kindle_dashboard_portable"
	}
	if c.ZoneEntityPrefix == "" {
		c.ZoneEntityPrefix = "sensor.kindle_dashboard_zone_"
	}
	if c.LocationEntity == "" {
		c.LocationEntity = "sensor." + c.DeviceID + "_location"
	}
	if c.BatteryEntity == "" {
		c.BatteryEntity = "sensor." + c.DeviceID + "_battery"
	}
	if c.TempDir == "" {
		c.TempDir = "/var/tmp/kindle-ha-dashboard"
	}
	if c.FontPath == "" {
		c.FontPath = DefaultFontPath
	}
	if c.FontBoldPath == "" {
		c.FontBoldPath = DefaultFontBoldPath
	}
	if c.DashboardRefreshIntervalSec <= 0 {
		c.DashboardRefreshIntervalSec = 10
	}
	if c.ForceRefreshIntervalSec <= 0 {
		c.ForceRefreshIntervalSec = 1800
	}
}

func (c Config) Validate() error {
	if c.HAURL == "" {
		return errors.New("ha_url is required")
	}
	if c.LongLivedToken == "" {
		return errors.New("long_lived_token is required")
	}
	if c.DeviceID == "" {
		return errors.New("device_id is required")
	}
	if c.InputDevice == "" || c.TouchWidth <= 0 || c.TouchHeight <= 0 {
		return errors.New("touch device and dimensions must be configured")
	}
	if c.DisplayWidth <= 0 || c.DisplayHeight <= 0 {
		return errors.New("display dimensions must be positive")
	}
	if c.BatteryIntervalSec <= 0 {
		return errors.New("battery_interval_seconds must be positive")
	}
	if c.DashboardRefreshIntervalSec <= 0 {
		return errors.New("dashboard_refresh_interval_seconds must be positive")
	}
	if c.ForceRefreshIntervalSec <= 0 {
		return errors.New("force_refresh_interval_seconds must be positive")
	}
	return nil
}

func (c Config) ZoneEntity(zoneID string) string {
	return c.ZoneEntityPrefix + slug(zoneID)
}

func PathFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("KINDLE_DASHBOARD_CONFIG")); value != "" {
		return value
	}
	return filepath.Join("/mnt/us/extensions/kindle-ha-dashboard", "config")
}

var slugPattern = regexp.MustCompile(`[^a-z0-9_-]+`)

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = slugPattern.ReplaceAllString(value, "-")
	return strings.Trim(value, "-")
}
