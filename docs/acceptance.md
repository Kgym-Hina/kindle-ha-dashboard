# 验收清单

## 软件链路

- [ ] Add-on 启动后 `/api/health` 返回 `ok: true`。
- [ ] 编辑器读取 Home Assistant Area Registry 房间，选择 portable 或房间后能加载对应文档。
- [ ] 实体字段使用带域图标的对话框，可按名称/实体 ID 搜索，并按类型和房间筛选、分组。
- [ ] 发布后 HA 中对应 `sensor.kindle_dashboard_*` 的 `attributes.document` 是完整 JSON。
- [ ] Kindle 启动日志显示 WebSocket 鉴权成功，并能在文档更新后重绘。
- [ ] Kindle 触摸按钮可以调用指定 HA 服务或切换本地页面。
- [ ] Add-on 推送的 `kindle_dashboard_message` 能在 Kindle 上弹窗，并可触摸关闭。
- [ ] Kindle 电量实体能更新到 0–100。

## 硬件链路

- [ ] 用 `evtest` 确认 Kindle 触摸设备和坐标上限。
- [ ] 用万用表/串口日志采集每个底座电阻样本，更新 ESP32 区间。
- [ ] 确认 ESP32 ADC 通道与实际 GPIO 映射，并加入 ESD/限流。
- [ ] 用实物复测 Kindle、ESP32、pogo pin 和 micro 线尺寸后更新 FreeCAD 参数。
- [ ] 打印外壳低层样件，验证屏幕不受压、底座能插拔、走线不折伤。
