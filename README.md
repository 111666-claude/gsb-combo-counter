# combo-counter

战斗连击：按目标分链、按 100ms 桶做滑动窗口计数，并维护倍率档位与结算快照，只用 Go 标准库。

```
go test ./...
go run ./cmd/combo-counter --sample aoe
go run ./cmd/combo-counter --sample slide
go run ./cmd/combo-counter --sample settle
go run ./cmd/combo-counter --sample churn
```

## 口径（README 为准）

- **窗口**：命中落在 `[now-800, now]` 内才算数；窗口按 100ms 的桶对齐，桶起始时刻早于
  `now-800` 的桶整桶过期。过期按"当前查询时刻"判断，不依赖后面还有没有新命中。
- **去重**：同一段挥击（`AttackID`）对**同一个目标**只算一次；同一次挥击扫到多个目标时，
  每个目标各算一次。
- **倍率**：`Multiplier(now)` 只由 `Count(now)` 决定 —— `1-2` 档 `1.0`、`3-4` 档 `1.2`、
  `5-9` 档 `1.5`、`>=10` 档 `2.0`。
- **结算**：`Settle(now)` 必须等于同一时刻的 `(Count(now), Multiplier(now))`。
- **内存观测**：`ActiveTargets(now)` 只统计窗口内还有有效命中的目标，`Buckets(now)` 只统计
  窗口内的桶；窗口外的目标与桶必须回收。每个目标最多保留 `MaxBuckets = 9` 个桶。
- **规模**：单场 2000 万次命中、100 万目标，`Hit` 与 `Count`/`Multiplier`/`Settle`/
  `ActiveTargets`/`Buckets` 都摊还 O(1)，内存 O(活跃目标数 × 9)。

## 输出契约（不改格式）

```
count=3 multiplier=1.2
count=4 multiplier=1.2
live=6/1.5 settled=6/1.5
targets=0 buckets=0
```

## 常量

```
WindowMs   = 800
BucketMs   = 100
MaxBuckets = 9
```

## 目录

```
combo.go                连击计数
cmd/combo-counter       命令行入口
combo_test.go           go test 用例
```
