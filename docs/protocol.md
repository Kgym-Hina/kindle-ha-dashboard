# `kindle-dashboard/v1` 协议

## 界面文档

界面 JSON 的顶层字段如下：

```json
{
  "schema": "kindle-dashboard/v1",
  "revision": 1,
  "updated_at": "2026-09-05T00:00:00Z",
  "target": {
    "mode": "portable",
    "zone_id": null,
    "width": 600,
    "height": 800,
    "background": "#ffffff"
  },
  "pages": []
}
```

每个页面包含 `id`、`name`、`background` 和 `elements`。组件的公共字段为 `id`、`type`、`frame`、`style` 和可选 `action`：

- `text`：使用 `text` 显示文本。
- `button`：显示文本按钮，可触摸。
- `image_button`：显示图片按钮，可触摸。
- `image`：显示 `image.src`，支持 HTTP(S)、本地文件和 data URL。
- `rect`：矩形/卡片。
- `line`：线段。

## 动作

```json
{
  "type": "call_service",
  "domain": "light",
  "service": "turn_on",
  "service_data": {"entity_id": "light.living_room"}
}
```

`action.type` 可取：

- `navigate_page`：`page_id` 为本地页面 ID。
- `call_service`：`domain`、`service`、`service_data` 直接映射到 HA WebSocket `call_service`。
- `show_message`：`title`、`message` 在 Kindle 上弹出对话框。

图片推荐使用编辑器上传产生的 PNG/JPEG/GIF data URL，或使用 Kindle 可访问的 PNG/JPEG/GIF HTTP(S) 地址。Kindle 只会把认证头发送给与 `ha_url` 同源的图片地址。

## Home Assistant 实体

### 界面实体

```text
sensor.kindle_dashboard_portable
sensor.kindle_dashboard_zone_<zone_id>
```

实体的 `state` 是 revision 字符串，属性中的 `document` 是完整界面 JSON。

### 位置实体

```text
sensor.kindle_<device_id>_location
```

`state` 是 zone ID，属性包含 `resistance_ohms`、`raw_mv` 和 `source`。

### 电池实体

```text
sensor.kindle_<device_id>_battery
```

`state` 是 0–100 的整数，属性包含 `unit_of_measurement: "%"`、`device_class: "battery"` 和 `source`。

### 推送事件

事件类型：`kindle_dashboard_message`

```json
{
  "target_device_id": "kindle-bedroom",
  "title": "提醒",
  "message": "请关闭窗户",
  "timeout_ms": 10000
}
```

`target_device_id` 省略或为空时视为广播。

在 Home Assistant 自动化中可直接使用 `event: kindle_dashboard_message`，参考 [`docs/home-assistant-examples.yaml`](home-assistant-examples.yaml)。编辑器的“推送消息”按钮使用同一个事件接口。
