# 硬件与尺寸说明

## Kindle

软件默认显示区域为 `600 × 800`，与老项目中的 `599 × 799` 原始触摸坐标保持兼容。实际机器需要用 `evtest` 确认 `/dev/input/event*` 和 ABS X/Y 最大值，再写入 Kindle `config`。默认 `input_device` 为触摸屏 `/dev/input/event1`，`key_device` 为物理键 `/dev/input/event0`；dashboard 运行时会独占物理键并禁用自动锁屏。

FreeCAD 默认按 Kindle 10 / 2019 基础款（常被称为 KT2 Basic/Kindle Basic 3）的约 `113 × 160 × 8.7 mm` 外形建立腔体；这只是公开规格基准，最终打印前仍要以手上的 KT2 实物为准。

## ESP32 电阻识别

推荐电路为：ESP32 ADC 引脚接固定上拉电阻 `R_pullup`，磁吸底座的编码电阻接到地。ADC 测得电压 `V` 后：

```text
R_unknown = R_pullup × V / (Vcc − V)
```

实际板上应增加 1% 精度电阻、TVS/ESD 保护和 ADC 输入限流。固件使用多个区间匹配：

```text
zone_id, min_ohms, max_ohms
```

建议在每个底座装配后记录 20 次测量值，再设置区间而不是直接复用示例值。

## pogo pin / micro 走线

外壳脚本默认预留：

- 底部走线槽和 micro 线出口；
- 背面 ESP32 腔体；
- 腔体与 Kindle 背部之间的 pogo pin 通道；
- 盖板螺柱和可调公差。

所有尺寸在 `mechanical/parameters.json`，单位为 mm。KT2 Basic 的精确外壳尺寸、板卡型号、连接器中心距和 pogo pin 直径必须在加工前用实物/卡尺复核。
