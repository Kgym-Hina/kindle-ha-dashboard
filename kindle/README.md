# Kindle 端

Kindle 端只有 Go 可执行程序，运行在 KUAL 扩展目录中。程序会：

- 使用配置文件中的 Long-Lived Access Token 连接 Home Assistant REST/WebSocket API；
- 读取 portable/zone 界面实体中的 `document` 属性并渲染灰度 PNG；
- 监听 evdev 触摸，按 JSON 的 `action` 调用 HA 服务、切换页面或弹出消息；
- 订阅 `kindle_dashboard_message` 事件；
- 定时把 Kindle 电量写入 `sensor.<device_id>_battery`。

安装 `dist/kindle-ha-dashboard.zip` 后，编辑扩展根目录的 `config`，再在 KUAL 中启动。安装包内置 Noto Sans CJK SC 字体，支持中文并按 JSON 中的 `font_size` 和 `font_weight` 渲染；如需替换字体，可在 `font_path` 和 `font_bold_path` 中填写 Kindle 上的 TTF/OTF 路径。
