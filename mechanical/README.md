# FreeCAD 外壳

运行 `kindle_case.FCMacro` 会读取 `parameters.json` 并生成：

```text
mechanical/generated/kindle-ha-dashboard-case.FCStd
```

模型包含 Kindle 腔体、屏幕开口、底部 micro 走线孔、背面 ESP32 凹槽、pogo pin 通道和盖板安装柱。参数默认值以 Kindle 10 / 2019 基础款的公开机身尺寸为基准，KT2 Basic 实物、ESP32 板和 pogo pin 装配尺寸必须在加工前复测后再调整 `parameters.json`。

FreeCAD GUI：`菜单 → 宏 → 宏… → 运行 kindle_case.FCMacro`。

命令行生成：

```bash
/Applications/FreeCAD.app/Contents/Resources/bin/freecadcmd mechanical/kindle_case.FCMacro
```
