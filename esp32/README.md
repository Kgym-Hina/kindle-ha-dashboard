# ESP32 Location Bridge

这是一个 ESP-IDF 5.x 项目。ESP32 通过 ADC 读取磁吸底座的编码电阻，按 Kconfig 中的区间映射出 zone，并写入：

```text
sensor.<device_id>_location
```

## 配置与烧录

```bash
cd esp32
idf.py set-target esp32
idf.py menuconfig
idf.py build
idf.py -p /dev/cu.usbserial-XXXX flash monitor
```

在 `Kindle Location Bridge` 菜单中配置 Wi-Fi、HA 地址、Long-Lived Token、ADC GPIO、上拉电阻和 3 个区域的阻值区间。示例区间只用于开发验证，装配前必须用实测数据替换。

## 接线约定

ADC GPIO ── `R_pullup` ── 3.3V

ADC GPIO ── 底座编码电阻 ── GND

请在 ADC 输入附近增加限流和 ESD 防护。若使用的 ESP32 变体 ADC 通道与 GPIO 号不一致，需要把 `main.c` 的 GPIO 到通道映射改成该芯片的数据手册对应值。

