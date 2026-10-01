// Package combo 是战斗连击：按目标分链、按 100ms 桶做滑动窗口计数，并维护倍率档位与结算快照。
//
// 实现口径：
//   - 桶号 = AtMs / BucketMs；桶起始时刻 = 桶号 * BucketMs。
//   - 桶起始时刻早于 now-WindowMs 的桶整桶过期，过期在 Hit 与每次查询时按当前时刻懒触发，
//     不依赖之后是否还有新命中。
//   - 去重键是 (AttackID, Target)，去重表随桶一起回收。
//   - 全局 count/buckets 增量维护；桶号最小堆保证每次只弹出真的过期的桶，摊还 O(1)。
package combo

import "container/heap"

// WindowMs 是连击窗口：落在 [now-WindowMs, now] 内的命中才算数。
const WindowMs = 800

// BucketMs 是计数桶的粒度，窗口按桶对齐。
const BucketMs = 100

// MaxBuckets 是每个目标在窗口内最多保留的桶数。
const MaxBuckets = WindowMs/BucketMs + 1

// HitEvent 是一次命中上报。
type HitEvent struct {
	AttackID string
	Target   string
	AtMs     int64
	Damage   int
}

// chain 是单个目标的命中链：只保留窗口内的桶与去重表。
type chain struct {
	buckets map[int64]int    // 桶号 -> 桶内有效命中数
	seen    map[string]int64 // AttackID -> 首次命中的桶号
	total   int              // 窗口内有效命中数
}

// bucketSlot 记录某个桶里有哪些目标，用于整桶过期时定位受影响的链。
type bucketSlot struct {
	targets map[string]struct{}
}

// keyHeap 是桶号最小堆，堆顶始终是最老的桶。
type keyHeap []int64

func (h keyHeap) Len() int           { return len(h) }
func (h keyHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h keyHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *keyHeap) Push(value any) { *h = append(*h, value.(int64)) }

func (h *keyHeap) Pop() any {
	old := *h
	top := old[len(old)-1]
	*h = old[:len(old)-1]
	return top
}

// Tracker 记录连击状态。chains 按目标分链，slots/keys 是全局桶索引，
// count 与 bucketCount 是窗口内的增量统计。
type Tracker struct {
	chains      map[string]*chain
	slots       map[int64]*bucketSlot
	keys        keyHeap
	clock       int64
	count       int
	bucketCount int
}

// NewTracker 建一个连击计数器。
func NewTracker() *Tracker {
	return &Tracker{
		chains: map[string]*chain{},
		slots:  map[int64]*bucketSlot{},
	}
}

// advance 把查询时刻推进到 nowMs（只进不退），并按当前时刻回收过期桶。
// 桶起始时刻早于 clock-WindowMs 的桶整桶消失：计数、档位、内存观测同步更新。
func (t *Tracker) advance(nowMs int64) {
	if nowMs > t.clock {
		t.clock = nowMs
	}
	cutoff := t.clock - WindowMs
	for len(t.keys) > 0 && t.keys[0]*BucketMs < cutoff {
		key := heap.Pop(&t.keys).(int64)
		slot := t.slots[key]
		delete(t.slots, key)
		for target := range slot.targets {
			entry := t.chains[target]
			hits := entry.buckets[key]
			entry.total -= hits
			t.count -= hits
			t.bucketCount--
			delete(entry.buckets, key)
			for attackID, bucket := range entry.seen {
				if bucket == key {
					delete(entry.seen, attackID)
				}
			}
			if len(entry.buckets) == 0 {
				delete(t.chains, target)
			}
		}
	}
}

// Hit 记一次命中，返回该目标当前窗口内的命中数。
// 同一段挥击对同一个目标只算一次，扫到多个目标各自计数。
func (t *Tracker) Hit(ev HitEvent) int {
	t.advance(ev.AtMs)
	key := ev.AtMs / BucketMs
	entry := t.chains[ev.Target]
	// 迟到且整桶已滑出窗口的上报不计数，也不留任何痕迹。
	if key*BucketMs < t.clock-WindowMs {
		if entry == nil {
			return 0
		}
		return entry.total
	}
	if entry == nil {
		entry = &chain{buckets: map[int64]int{}, seen: map[string]int64{}}
		t.chains[ev.Target] = entry
	}
	if _, duplicate := entry.seen[ev.AttackID]; duplicate {
		return entry.total
	}
	entry.seen[ev.AttackID] = key
	if _, exists := entry.buckets[key]; !exists {
		t.bucketCount++
	}
	entry.buckets[key]++
	entry.total++
	t.count++
	slot := t.slots[key]
	if slot == nil {
		slot = &bucketSlot{targets: map[string]struct{}{}}
		t.slots[key] = slot
		heap.Push(&t.keys, key)
	}
	slot.targets[ev.Target] = struct{}{}
	return entry.total
}

// Count 是 nowMs 时刻窗口内的连击总数。
func (t *Tracker) Count(nowMs int64) int {
	t.advance(nowMs)
	return t.count
}

// Multiplier 是 nowMs 时刻的倍率档位，只由当前 Count 决定。
func (t *Tracker) Multiplier(nowMs int64) float64 {
	return multiplierFor(t.Count(nowMs))
}

// Settle 是 nowMs 时刻的结算快照，始终等于同一时刻的 (Count, Multiplier)。
func (t *Tracker) Settle(nowMs int64) (int, float64) {
	count := t.Count(nowMs)
	return count, multiplierFor(count)
}

// ActiveTargets 是 nowMs 时刻窗口内还有有效命中的目标数。
func (t *Tracker) ActiveTargets(nowMs int64) int {
	t.advance(nowMs)
	return len(t.chains)
}

// Buckets 是 nowMs 时刻保留的（目标, 桶）条目总数。
func (t *Tracker) Buckets(nowMs int64) int {
	t.advance(nowMs)
	return t.bucketCount
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
