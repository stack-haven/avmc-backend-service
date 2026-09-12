# 性能基准（v1.1）

## 方法

- 命令：`go test -bench=. -benchmem -benchtime=1s -run='^$' ./...`
- 基线快照存于 `testdata/bench-baseline-v1.1.txt`（每轮变更后用同命令重测对比，
  变动超过 ±20% 需在 CHANGELOG 说明或修复）。
- 对照预算（docs/19 T25）：100 次 Normalize 平均 **< 5ms/次**；
  当前单次处理器开销在 µs 量级，余量 > 3 个数量级。
- 注意机器负载对绝对值影响明显（同代码两次运行差异可达 ±30%），
  对比时应在同一环境同批次运行，或以 3 轮中位数判定。

## v1.1 基线（testdata/bench-baseline-v1.1.txt）

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Alias | 1188 | 1200 | 16 |
| Context | 324 | 274 | 3 |
| Deterministic | 1180 | 1224 | 16 |
| Disfluency | 3718 | 1390 | 40 |
| Fuzzy | 2396 | 2069 | 26 |
| Normalize | 2931 | 4174 | 36 |
| Pinyin | 2241 | 2896 | 30 |

## 与 v1.0（规范化前）对比的说明

- **Normalize**：变快（约 −20%~−60% 视负载）——跳过 identity 替换，不再为
  "空格→空格" 生成替换记录。
- **Alias / Deterministic**：变快——重复 variant 去重后 AC 模式集更小。
- **Disfluency**：为新增能力支付了成本（边界守卫 + 重复词/短语检测），
  在满载环境实测最高 +52%（6.4µs/op）；相对 5ms 预算仍可忽略。
  `WithRepeatedWordFolding(0)` + `WithRepeatedPhraseFolding(0, 0)` 可关闭检测。
- **Pinyin**：整词 homophone 匹配路径新增，开销基本持平。
