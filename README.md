# combo-counter

战斗连击计数：同一次挥击去重、断连判定、峰值档位结算，只用 Go 标准库。

```
go test ./...
go run ./cmd/combo-counter --sample repeat
go run ./cmd/combo-counter --sample break
```

## 口径（README 为准）

- **同一次挥击只算一次**：`AttackID` 相同的命中事件是重复上报，必须整条忽略（不涨连击、
  不刷新窗口、不改倍率）。`--sample repeat`（同一个 `swing-1` 上报 5 次）按口径是
  `count=1 tip=1 multiplier=1.0`，现在是 `count=5 tip=5 multiplier=1.5`。
- **断连要连档位一起清**：距上一次有效命中超过 `WindowMs`（严格大于）就断连，
  `Count` 与 `Tip` 都回到 0，下一击从 1 开始。`--sample break`（连打 5 下，断了再打 2 下）
  按口径是 `count=2 tip=2 multiplier=1.0`，现在是 `count=2 tip=5 multiplier=1.5`。
- 倍率档位按本段连击的峰值 `Tip`：`1-2` 档 `1.0`、`3-4` 档 `1.2`、`5-9` 档 `1.5`、`>=10` 档 `2.0`。
- 不变量：同一串事件重复回放结果相同；`Tip >= Count` 恒成立；断连后 `Tip` 必须等于 `Count`；
  连击跨目标延续（换目标不算断连）。
- 规模：单场 100 万次命中，摊还 O(1)、内存 O(1)；不许把所有命中事件留下来再排序或重放。

## 现在的行为

- `Hit` 不看 `AttackID`，来一条算一条，重复上报会把连击数刷上去。
- 断连时只把 `count` 清零，`tip` 留在峰值，倍率不会掉档。

## 输出

```
count=1 tip=1 multiplier=1.0
count=2 tip=2 multiplier=1.0
```

## 目录

```
combo.go                连击计数
cmd/combo-counter       命令行入口
combo_test.go           go test 用例
```
