// Package combo 是战斗连击：按目标分链、按 100ms 桶做滑动窗口计数，并维护倍率档位与结算快照。
// 缺陷：过期只在相邻命中间隔超过窗口时整链清空（逐桶滑出没做）、全局计数只加不减、
// 去重键漏了目标、档位用的是峰值、结算快照定型后不再刷新、离线目标与旧桶都不回收。
package combo

// WindowMs 是连击窗口：落在 [now-WindowMs, now] 内的命中才算数。
const WindowMs = 800

// BucketMs 是计数桶的粒度，窗口按桶对齐。
const BucketMs = 100

// MaxBuckets 是每个目标最多保留的桶数。
const MaxBuckets = WindowMs/BucketMs + 1

// HitEvent 是一次命中上报。
type HitEvent struct {
	AttackID string
	Target   string
	AtMs     int64
	Damage   int
}

type chain struct {
	buckets map[int64]int
	newest  int64
}

func (c *chain) total() int {
	sum := 0
	for _, hits := range c.buckets {
		sum += hits
	}
	return sum
}

// Tracker 记录连击状态。chains 按目标分链，seen 是挥击去重表。
type Tracker struct {
	chains map[string]*chain
	seen   map[string]int64
	count  int
	tip    int

	settledCount int
	settledMul   float64
}

// NewTracker 建一个连击计数器。
func NewTracker() *Tracker {
	return &Tracker{chains: map[string]*chain{}, seen: map[string]int64{}}
}

// Hit 记一次命中，返回该目标当前的窗口计数。
// 缺陷：去重表只按 AttackID 记，同一次挥击打多个目标只算一次；
// 缺陷：只在相邻两次命中间隔超过窗口时整链清空，逐桶滑出没实现；
// 缺陷：全局计数只加不减，离线目标与过期桶都不回收。
func (t *Tracker) Hit(ev HitEvent) int {
	target := t.chains[ev.Target]
	if target == nil {
		target = &chain{buckets: map[int64]int{}}
		t.chains[ev.Target] = target
	}
	if _, duplicate := t.seen[ev.AttackID]; duplicate {
		return target.total()
	}
	t.seen[ev.AttackID] = ev.AtMs
	if target.newest != 0 && ev.AtMs-target.newest > WindowMs {
		target.buckets = map[int64]int{}
	}
	target.buckets[ev.AtMs/BucketMs]++
	if ev.AtMs > target.newest {
		target.newest = ev.AtMs
	}
	t.count++
	if target.total() > t.tip {
		t.tip = target.total()
	}
	return target.total()
}

// Count 是 nowMs 时刻窗口内的连击总数。
// 缺陷：直接返回只增不减的累计值，不看 nowMs。
func (t *Tracker) Count(nowMs int64) int { return t.count }

// Multiplier 是 nowMs 时刻的倍率档位。
// 缺陷：用的是峰值而不是当前值，滑出窗口后不会掉档。
func (t *Tracker) Multiplier(nowMs int64) float64 { return multiplierFor(t.tip) }

// Settle 是 nowMs 时刻的结算快照。
// 缺陷：第一次调用定型之后再也不刷新，与实时值脱节。
func (t *Tracker) Settle(nowMs int64) (int, float64) {
	if t.settledCount == 0 {
		t.settledCount = t.count
		t.settledMul = multiplierFor(t.tip)
	}
	return t.settledCount, t.settledMul
}

// ActiveTargets 是 nowMs 时刻窗口内还有命中的目标数。
// 缺陷：从不回收离线目标。
func (t *Tracker) ActiveTargets(nowMs int64) int { return len(t.chains) }

// Buckets 是 nowMs 时刻保留的桶总数。
// 缺陷：从不回收过期桶。
func (t *Tracker) Buckets(nowMs int64) int {
	total := 0
	for _, entry := range t.chains {
		total += len(entry.buckets)
	}
	return total
}

func multiplierFor(count int) float64 {
	switch {
	case count >= 10:
		return 2.0
	case count >= 5:
		return 1.5
	case count >= 3:
		return 1.2
	default:
		return 1.0
	}
}
