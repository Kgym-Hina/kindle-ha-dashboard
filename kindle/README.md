# Kindle 端

Kindle 端只有 Go 可执行程序，运行在 KUAL 扩展目录中。程序会：

- 使用配置文件中的 Long-Lived Access Token 连接 Home Assistant REST/WebSocket API；
- 读取 portable/zone 界面实体中的 `document` 属性并渲染灰度 PNG；
- 监听 evdev 触摸，按 JSON 的 `action` 调用 HA 服务、切换页面或弹出消息；
- 支持多级页面、底部返回/主页/设置导航，以及设置页中的刷新配置和退出程序；
- 支持文本实体绑定、开关控制和温控组件；
- 订阅 `kindle_dashboard_message` 事件；
- 定时把 Kindle 电量写入 `sensor.<device_id>_battery`。

安装 `dist/kindle-ha-dashboard.zip` 后，编辑扩展根目录的 `config`，再在 KUAL 中启动。安装包内置 Noto Sans CJK SC 字体，支持中文并按 JSON 中的 `font_size` 和 `font_weight` 渲染；如需替换字体，可在 `font_path` 和 `font_bold_path` 中填写 Kindle 上的 TTF/OTF 路径。

`dashboard_refresh_interval_seconds` 控制从 Home Assistant 定期校准界面的间隔，默认 10 秒；内容没有变化时不会刷屏。`force_refresh_interval_seconds` 控制最长刷屏间隔，默认 1800 秒（30 分钟），用于定期清除电子墨水残影。
