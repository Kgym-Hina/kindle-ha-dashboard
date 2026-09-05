# 系统架构

```text
┌────────────────────────── Home Assistant ──────────────────────────┐
│                                                                    │
│  sensor.kindle_dashboard_*  ◄── Add-on backend ◄── React editor    │
│          ▲                         │                               │
│          │ state_changed            └── REST API / event bus        │
│          │                                                         │
│  sensor.kindle_*_battery ◄──── Kindle Go ────► call_service        │
│  sensor.kindle_*_location ◄── ESP-IDF        kindle_dashboard_msg  │
└───────────────────────────┬────────────────────────────────────────┘
                            │ WebSocket + REST
                            ▼
                    Kindle e-ink device
              JSON renderer + evdev touch handler
```

## 数据流

1. React 编辑器通过 Add-on backend 读取界面文档和 HA zones。
2. 发布时 backend 将文档保存到 `/data/documents/`，并以 `POST /api/states/...` 写入 Home Assistant 实体属性 `document`。
3. Kindle 启动后通过 Long-Lived Access Token 连接 `/api/websocket`，订阅 `state_changed` 和 `kindle_dashboard_message`。
4. Kindle 首次启动通过 REST 获取 portable 文档；收到 location 状态后切换到对应 zone 文档；收到 zone 文档更新时立即重绘。
5. 点击 JSON 中定义的 `call_service` 动作时，Kindle 直接通过 HA WebSocket `call_service` 执行服务。
6. ESP32 通过 ADC 读取底座电阻值，区间映射成 zone ID，并向 HA 写入位置实体；电阻变化才上报，避免制造事件风暴。
7. Kindle 周期性从 `/sys/class/power_supply/` 读取电量，使用 HA REST 更新电池实体。

## 模块边界

- `kindle/internal/model`：协议类型和校验，不依赖具体显示/网络实现。
- `kindle/internal/render`：纯 JSON 到灰度 PNG 的渲染，以及 Kindle 显示命令适配。
- `kindle/internal/ha`：HA WebSocket/REST 客户端、断线重连、事件解析。
- `kindle/internal/input`：Linux evdev 触摸坐标读取和事件归一化。
- `kindle/internal/power`：电池 sysfs 读取。
- `ha-addon/server`：编辑器 API、持久化和 HA 状态/事件桥接。
- `web/src`：编辑器界面，不直接保存 HA token。
- `esp32/main`：Wi-Fi、ADC、阻值映射和 HA REST 上报。
- `mechanical`：只负责参数化几何，不与电子/软件目录耦合。
