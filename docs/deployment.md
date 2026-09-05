# 部署

## Add-on 本地仓库

将仓库路径添加到 Home Assistant 的本地 Add-on 仓库，然后安装 `Kindle HA Dashboard`。Add-on 默认监听 `8099`，启用 ingress 后可从 HA 侧边栏打开。

`ha-addon/config.yaml` 中的 `options` 支持：

- `ha_url`：独立 Docker 场景下的 HA API 地址；Supervisor 场景可留空。
- `ha_token`：独立 Docker 场景使用的 Long-Lived Access Token；Supervisor 场景优先自动使用 `SUPERVISOR_TOKEN`。
- `port`：backend 监听端口。

编辑器 Settings 中的设置会写入 Add-on `/data/settings.json`，token 不会通过 GET API 返回。

Kindle 安装包内置 Noto Sans CJK SC 字体，因此中文不依赖 Kindle 的系统字体。Kindle `config` 中的 `font_path`、`font_bold_path` 分别指向安装包内的常规和粗体字体；`font_size` 使用与编辑器一致的画布像素单位。

## Kindle 发布包

在具备 Go 工具链的开发机上运行：

```bash
cd kindle
./build_kual_package.sh
```

构建产物为 `kindle/dist/kindle-ha-dashboard.zip`。此过程需要可用的 Go 编译器和 zip 工具，本仓库不提交架构相关的伪二进制。
