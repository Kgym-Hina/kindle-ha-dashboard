# Kindle HA Dashboard

一个面向 Kindle KT2 Basic 的 Home Assistant 电子墨水控制面板。Kindle 只负责从 Home Assistant 订阅界面 JSON、在本地渲染和处理触摸；界面编辑、数据保存和 HA 管理集中在 Home Assistant Add-on 中。设计器支持实体绑定、开关/温控组件、嵌套页面、可缩放布局、对齐和图层排序。

项目参考了 [`Kgym-Hina/kindle_renderer`](https://github.com/Kgym-Hina/kindle_renderer) 的 KUAL 启动方式和 Kindle 端 Go 实现，但把图片轮播升级成了可交互的 JSON 界面协议。

## 交付内容

| 目录 | 内容 |
| --- | --- |
| `kindle/` | Kindle 端 Go 程序、KUAL 扩展文件和 ARMv7 打包脚本 |
| `ha-addon/` | Home Assistant Add-on Docker 配置和 Node 后端 |
| `web/` | React + Vite 界面编辑器 |
| `esp32/` | ESP-IDF 固件：ADC 读取磁吸底座电阻并上报 HA |
| `mechanical/` | FreeCAD 参数化外壳生成脚本和尺寸参数 |
| `schema/` | Kindle 与编辑器共用的 JSON Schema 和示例 |
| `docs/` | 架构、协议、接线和部署说明 |

已生成的交付产物：

- [Kindle ARMv7 可执行文件](kindle/dist/kindle-ha-dashboard/bin/kindle-dashboard)
- [KUAL 安装包](kindle/dist/kindle-ha-dashboard.zip)
- [FreeCAD 外壳文件](mechanical/generated/kindle-ha-dashboard-case.FCStd)

## 快速开始

### Home Assistant Add-on

1. 将本目录作为本地 Add-on 仓库，或把 `ha-addon/` 复制到 Home Assistant 的 Add-on 仓库。
2. 在 Add-on 配置中填写 `ha_url`；如果 Add-on 运行在 Home Assistant Supervisor 内，优先使用自动注入的 `SUPERVISOR_TOKEN`。
3. 安装并启动后，从 Add-on 页面打开编辑器。
4. 在编辑器中选择 `portable` 或 Home Assistant 的某个房间（Area），拖拽组件并发布。

房间列表读取 Home Assistant 的 Area Registry；这里的“房间”不是 `zone.*` 地理位置实体，也不是 HA 实例区分。

Add-on 会把文档写入以下 HA 实体，并触发标准 `state_changed` 事件：

```text
sensor.kindle_dashboard_portable
sensor.kindle_dashboard_zone_<zone_id>
```

本地预览编辑器时可在 `web/` 运行 `npm install` 后执行 `npm run dev`；生产部署推荐直接使用 `ha-addon/` 的 Docker 构建，使 React 静态文件和 backend 一起进入 Add-on 镜像。

### Kindle

复制 `kindle/config.example` 为 Kindle 扩展目录中的 `config`，填写 Home Assistant 地址、Long-Lived Access Token、设备 ID 和屏幕参数。随后在 KUAL 中启动扩展。

```text
/mnt/us/extensions/kindle-ha-dashboard/config
/mnt/us/extensions/kindle-ha-dashboard/bin/kindle-dashboard
```

ARMv7 构建和 KUAL 打包命令写在 `kindle/build_kual_package.sh` 中。根据当前工作区约束，本次不会在服务器上自动执行构建。

### ESP32

固件使用 ESP-IDF 5.x。进入 `esp32/` 后用 `idf.py menuconfig` 配置 Wi-Fi、HA URL、Long-Lived Token、ADC GPIO 和电阻区间，再烧录到贴在 Kindle 背后的 ESP32 板。

### 外壳

`mechanical/kindle_case.FCMacro` 会生成带参数的 FreeCAD 文档：Kindle 外壳腔体、底部 micro 走线孔、背面 ESP32 凹槽和 pogo pin 接口开口均由 `mechanical/parameters.json` 控制。

## 重要假设

- Kindle 逻辑显示尺寸默认 `600 × 800`，触摸坐标默认 `599 × 799`，均可在配置中修改。
- KT2 Basic、ESP32 开发板和 pogo pin 底座的实测尺寸尚未由本项目自动获得；外壳脚本采用可修改参数，不把未经测量的尺寸伪装成最终加工尺寸。
- 位置上报采用 ESP32 Wi-Fi + HA REST 状态实体；电阻区间映射由固件的 Kconfig 配置完成。
- Kindle 使用 HA WebSocket API 的 Long-Lived Access Token。界面 JSON 保存在 HA entity attributes，按钮调用使用同一条 WebSocket 连接。

## 协议入口

完整协议见 [`docs/protocol.md`](docs/protocol.md)，可校验的 Schema 在 [`schema/dashboard.schema.json`](schema/dashboard.schema.json)，可直接导入编辑器的示例在 [`schema/examples/portable.json`](schema/examples/portable.json)。
