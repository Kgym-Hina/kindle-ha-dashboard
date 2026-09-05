# Home Assistant Add-on

该目录是可被 Home Assistant Supervisor 构建的 Add-on。Docker 镜像包含两层构建：Vite 生成 React 编辑器静态文件，TypeScript backend 提供 API 和 Home Assistant REST bridge，最终由同一个 Node 进程服务。

backend 通过 `/data/documents/` 保存每个 portable/zone 文档，通过 HA REST API 更新 entity attributes；Kindle 端只需要配置 HA 地址和 Long-Lived Access Token，不需要暴露 Add-on 端口给 Kindle。

独立 Docker 运行时可以通过环境变量设置 `PORT`、`KINDLE_DATA_DIR`、`HA_URL` 和 `HA_TOKEN`，也可以在编辑器 Settings 中配置 HA 地址和令牌。Supervisor 场景会自动读取 `SUPERVISOR_TOKEN`，默认访问 `http://supervisor/core`。
