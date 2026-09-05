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

当 `target.mode` 为 `zone` 时，`zone_id` 保存 Home Assistant Area 的 ID。协议字段名保留为 `zone_id`，用于兼容已有文档；编辑器中的发布目标显示为“房间”，指的是 HA Area（房间），不是 `zone.*` 地理位置实体。

每个页面包含 `id`、`name`、可选的 `parent_id`、`background` 和 `elements`。`parent_id` 用于组成可从页面跳转进入的子页。组件的公共字段为 `id`、`type`、`frame`、`style` 和可选 `action`：

- `text`：使用 `text` 显示文本。
- `button`：显示文本按钮，可触摸。
- `image_button`：图片本身就是点击区域，不额外绘制文字、填充或边框。
- `image`：显示 `image.src`，支持 HTTP(S)、本地文件和 data URL。
- `rect`：矩形/卡片。
- `line`：线段。
- `switch`：绑定开关实体，点击后调用 `switch.toggle`。
- `climate`：绑定温控实体，内置目标温度和 HVAC 模式控制。

文本和按钮可以使用 `binding` 显示实体状态或属性。编辑器会从 Home Assistant 读取实体列表和属性名，并提供“一键插入值”：

```json
{
  "text": "室温：{value} ℃",
  "binding": {"entity_id": "sensor.room_temperature", "field": "state", "decimals": 1}
}
```

编辑器画布和 Kindle 渲染器都会保留底部 72 像素作为返回、主页、设置导航栏；导航栏不属于页面元素。

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
- `refresh_config`：Kindle 内置等待页动作，重新从 Home Assistant 拉取界面配置。
- `exit`：Kindle 内置等待页动作，退出程序并恢复原生 Kindle 框架。

Kindle 每 10 秒从 Home Assistant 校准一次文档和绑定实体状态。只有实际画面发生变化时才会刷新电子墨水屏；连续 30 分钟没有变化时强制全屏刷新一次。两个间隔都可以在 Kindle `config` 中调整。

图片推荐使用编辑器上传产生的 PNG/JPEG/GIF data URL，或使用 Kindle 可访问的 PNG/JPEG/GIF HTTP(S) 地址。Kindle 只会把认证头发送给与 `ha_url` 同源的图片地址。

## Home Assistant 实体

### 界面实体

```text
sensor.kindle_dashboard_portable
sensor.kindle_dashboard_zone_<zone_id>
```

实体的 `state` 是 revision 字符串，属性中的 `document` 是完整界面 JSON。

### Kindle 位置实体

```text
sensor.kindle_<device_id>_location
```

`state` 是 Kindle 当前所在房间的 ID，属性包含 `resistance_ohms`、`raw_mv` 和 `source`。

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
