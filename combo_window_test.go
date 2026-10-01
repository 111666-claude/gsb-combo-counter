package combo

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestBucketExpiryStepwise 逐桶滑出：只靠查询推进时刻，不依赖新命中。
func TestBucketExpiryStepwise(t *testing.T) {
	tracker := NewTracker()
	for index := 0; index < 9; index++ {
		tracker.Hit(HitEvent{AttackID: fmt.Sprintf("a%d", index), Target: "dummy", AtMs: int64(index * 100)})
	}
	cases := []struct {
		now        int64
		count      int
		buckets    int
		multiplier float64
	}{
		{800, 9, 9, 1.5},  // 桶起始 >= 0，全部有效
		{801, 8, 8, 1.5},  // 桶 0（起始 0 < 1）整桶过期
		{900, 8, 8, 1.5},  // 桶 1 起始 100 不早于 100，仍有效
		{901, 7, 7, 1.5},  // 桶 1 过期
		{1000, 7, 7, 1.5}, // 桶 2 起始 200 不早于 200
		{1001, 6, 6, 1.5}, // 桶 2 过期
		{1200, 5, 5, 1.5}, // 桶 4 起始 400 不早于 400
		{1201, 4, 4, 1.2}, // 桶 4 过期，掉到 4
		{1301, 3, 3, 1.2}, // 桶 5 过期，掉到 3
		{1401, 2, 2, 1.0}, // 桶 6 过期，掉到 2
		{1501, 1, 1, 1.0}, // 桶 7 过期，掉到 1
		{1600, 1, 1, 1.0}, // 桶 8 起始 800 不早于 800
		{1601, 0, 0, 1.0}, // 全部滑出
	}
	for _, tc := range cases {
		if got := tracker.Count(tc.now); got != tc.count {
			t.Fatalf("now=%d 的 Count 应为 %d，实际 %d", tc.now, tc.count, got)
		}
		if got := tracker.Buckets(tc.now); got != tc.buckets {
			t.Fatalf("now=%d 的 Buckets 应为 %d，实际 %d", tc.now, tc.buckets, got)
		}
		if got := tracker.Multiplier(tc.now); got != tc.multiplier {
			t.Fatalf("now=%d 的 Multiplier 应为 %v，实际 %v", tc.now, tc.multiplier, got)
		}
	}
	if got := tracker.ActiveTargets(1601); got != 0 {
		t.Fatalf("全部滑出后 ActiveTargets 应为 0，实际 %d", got)
	}
}

// TestDedupeScope 去重范围：按（挥击, 目标）。
func TestDedupeScope(t *testing.T) {
	tracker := NewTracker()
	// 同一次横扫扫到三个目标：每个目标各算一次。
	for _, target := range []string{"left", "mid", "right"} {
		tracker.Hit(HitEvent{AttackID: "cleave", Target: target, AtMs: 0})
	}
	if got := tracker.Count(0); got != 3 {
		t.Fatalf("一次挥击三个目标应计 3，实际 %d", got)
	}
	// 同一目标重复上报同一段挥击（哪怕跨桶）只算一次。
	tracker.Hit(HitEvent{AttackID: "cleave", Target: "left", AtMs: 150})
	if got := tracker.Count(150); got != 3 {
		t.Fatalf("重复上报不应重复计数，实际 %d", got)
	}
	// 不同挥击打同一目标各算一次。
	tracker.Hit(HitEvent{AttackID: "stab", Target: "left", AtMs: 200})
	if got := tracker.Count(200); got != 4 {
		t.Fatalf("新挥击应再计一次，实际 %d", got)
	}
	if got := tracker.ActiveTargets(200); got != 3 {
		t.Fatalf("活跃目标应为 3，实际 %d", got)
	}
}

// TestHitReturnValue Hit 返回该目标当前窗口内的命中数。
func TestHitReturnValue(t *testing.T) {
	tracker := NewTracker()
	if got := tracker.Hit(HitEvent{AttackID: "a", Target: "dummy", AtMs: 0}); got != 1 {
		t.Fatalf("首次命中应返回 1，实际 %d", got)
	}
	if got := tracker.Hit(HitEvent{AttackID: "b", Target: "dummy", AtMs: 100}); got != 2 {
		t.Fatalf("第二次命中应返回 2，实际 %d", got)
	}
	if got := tracker.Hit(HitEvent{AttackID: "a", Target: "dummy", AtMs: 120}); got != 2 {
		t.Fatalf("重复上报应返回当前值 2，实际 %d", got)
	}
	// 时钟推进到 1000 后，0/100 两桶已滑出，只剩新命中这一桶。
	if got := tracker.Hit(HitEvent{AttackID: "c", Target: "dummy", AtMs: 1000}); got != 1 {
		t.Fatalf("旧桶滑出后应返回 1，实际 %d", got)
	}
}

// TestMemoryReclaim 窗口外的目标、桶与去重表必须回收。
func TestMemoryReclaim(t *testing.T) {
	tracker := NewTracker()
	for index := 0; index < 2000; index++ {
		tracker.Hit(HitEvent{AttackID: fmt.Sprintf("a%d", index), Target: fmt.Sprintf("mob-%d", index), AtMs: 0})
	}
	if got := tracker.ActiveTargets(0); got != 2000 {
		t.Fatalf("命中后活跃目标应为 2000，实际 %d", got)
	}
	// 其中 10 个目标在 4900 再挨一下。
	for index := 0; index < 10; index++ {
		tracker.Hit(HitEvent{AttackID: fmt.Sprintf("b%d", index), Target: fmt.Sprintf("mob-%d", index), AtMs: 4900})
	}
	// 五秒时刻：只剩 10 个目标、10 个桶，1990 个目标连同去重表一起回收。
	if got := tracker.ActiveTargets(5000); got != 10 {
		t.Fatalf("now=5000 活跃目标应为 10，实际 %d", got)
	}
	if got := tracker.Buckets(5000); got != 10 {
		t.Fatalf("now=5000 桶数应为 10，实际 %d", got)
	}
	if len(tracker.chains) != 10 || len(tracker.slots) != 1 || len(tracker.keys) != 1 {
		t.Fatalf("内部结构未同步回收：chains=%d slots=%d keys=%d",
			len(tracker.chains), len(tracker.slots), len(tracker.keys))
	}
	if got := len(tracker.chains["mob-0"].seen); got != 1 {
		t.Fatalf("过期桶的去重条目应一并回收，mob-0 还剩 %d 条", got)
	}
	// 彻底空窗后全部归零。
	if got := tracker.ActiveTargets(9900); got != 0 {
		t.Fatalf("空窗后活跃目标应为 0，实际 %d", got)
	}
	if got := tracker.Buckets(9900); got != 0 {
		t.Fatalf("空窗后桶数应为 0，实际 %d", got)
	}
	if len(tracker.chains) != 0 || len(tracker.slots) != 0 || len(tracker.keys) != 0 {
		t.Fatalf("空窗后内部结构应为空：chains=%d slots=%d keys=%d",
			len(tracker.chains), len(tracker.slots), len(tracker.keys))
	}
}

// TestSettleAlwaysConsistent Settle 在任意时刻都等于同一时刻的 (Count, Multiplier)。
func TestSettleAlwaysConsistent(t *testing.T) {
	tracker := NewTracker()
	for _, now := range []int64{0, 100, 450, 800, 801, 5000, 5001} {
		if now < 800 {
			tracker.Hit(HitEvent{AttackID: fmt.Sprintf("a%d", now), Target: "dummy", AtMs: now})
		}
		count, multiplier := tracker.Settle(now)
		if count != tracker.Count(now) || multiplier != tracker.Multiplier(now) {
			t.Fatalf("now=%d 结算 (%d,%v) 与实时 (%d,%v) 不一致",
				now, count, multiplier, tracker.Count(now), tracker.Multiplier(now))
		}
	}
}

// reference 是「全量保留 + 逐桶过滤」的参考实现：正确但慢，用于对照。
type reference struct {
	events []HitEvent
	seen   map[[2]string]struct{}
}

func (r *reference) hit(ev HitEvent) {
	pair := [2]string{ev.AttackID, ev.Target}
	if _, ok := r.seen[pair]; ok {
		return
	}
	r.seen[pair] = struct{}{}
	r.events = append(r.events, ev)
}

// snapshot 返回 nowMs 时刻的 (连击数, 活跃目标数, 桶数)。
func (r *reference) snapshot(nowMs int64) (count, active, buckets int) {
	cutoff := nowMs - WindowMs
	perTarget := map[string]map[int64]struct{}{}
	for _, ev := range r.events {
		if ev.AtMs > nowMs {
			continue
		}
		key := ev.AtMs / BucketMs
		if key*BucketMs < cutoff {
			continue
		}
		count++
		keys := perTarget[ev.Target]
		if keys == nil {
			keys = map[int64]struct{}{}
			perTarget[ev.Target] = keys
		}
		keys[key] = struct{}{}
	}
	active = len(perTarget)
	for _, keys := range perTarget {
		buckets += len(keys)
	}
	return count, active, buckets
}

type streamOp struct {
	hit   *HitEvent
	query int64
}

// generateStream 生成一条随机命中流：同挥击多目标、重复上报、乱序时间戳、
// 跨桶边界、严重迟到与长空窗；查询时刻单调不减且不早于任何已见命中。
func generateStream(rng *rand.Rand, seed int64) []streamOp {
	targets := make([]string, 0, 64)
	for index := 0; index < 64; index++ {
		targets = append(targets, fmt.Sprintf("mob-%d", index))
	}
	var ops []streamOp
	var clock int64
	var maxAt int64
	var queryNow int64
	steps := 200 + rng.Intn(400)
	for step := 0; step < steps; step++ {
		if rng.Intn(20) == 0 {
			clock += 2000 + int64(rng.Intn(5000)) // 长空窗
		} else {
			clock += int64(rng.Intn(120))
		}
		attackID := fmt.Sprintf("atk-%d-%d", seed, step)
		swept := map[string]struct{}{}
		for n := 1 + rng.Intn(3); n > 0; n-- {
			swept[targets[rng.Intn(len(targets))]] = struct{}{}
		}
		for target := range swept {
			late := int64(0)
			if rng.Intn(50) == 0 {
				late = 1000 + int64(rng.Intn(2000)) // 严重迟到：整桶已过期
			}
			reports := 1
			if rng.Intn(3) == 0 {
				reports += rng.Intn(2) // 同一目标重复上报
			}
			for report := 0; report < reports; report++ {
				at := clock + int64(rng.Intn(40)) - late
				if rng.Intn(10) == 0 {
					at -= int64(rng.Intn(150)) // 乱序：小幅回退
				}
				if rng.Intn(4) == 0 {
					at = (at/BucketMs)*BucketMs + BucketMs - 1 // 贴着桶边界
				}
				if at < 0 {
					at = 0
				}
				if at > maxAt {
					maxAt = at
				}
				ev := HitEvent{AttackID: attackID, Target: target, AtMs: at, Damage: 10}
				ops = append(ops, streamOp{hit: &ev})
			}
		}
		if rng.Intn(3) == 0 {
			// 查询时刻单调不减，且不早于任何已见命中。
			if candidate := maxAt + 1 + int64(rng.Intn(60)); candidate > queryNow {
				queryNow = candidate
			}
			ops = append(ops, streamOp{query: queryNow})
		}
	}
	return ops
}

// observe 在 nowMs 时刻采样六个接口，拼成可比较的字符串。
func observe(tracker *Tracker, nowMs int64) string {
	settledCount, settledMultiplier := tracker.Settle(nowMs)
	return fmt.Sprintf("count=%d mul=%v settle=%d/%v targets=%d buckets=%d",
		tracker.Count(nowMs), tracker.Multiplier(nowMs),
		settledCount, settledMultiplier,
		tracker.ActiveTargets(nowMs), tracker.Buckets(nowMs))
}

// TestRandomizedAgainstReference 随机 200 组命中流与参考实现比对，差异必须为 0；
// 同一批事件重复回放，结果必须相同。
func TestRandomizedAgainstReference(t *testing.T) {
	for seed := int64(1); seed <= 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		ops := generateStream(rng, seed)

		tracker := NewTracker()
		ref := &reference{seen: map[[2]string]struct{}{}}
		var first []string
		for _, op := range ops {
			if op.hit != nil {
				tracker.Hit(*op.hit)
				ref.hit(*op.hit)
				continue
			}
			wantCount, wantActive, wantBuckets := ref.snapshot(op.query)
			if got := tracker.Count(op.query); got != wantCount {
				t.Fatalf("seed=%d now=%d Count=%d，参考值 %d", seed, op.query, got, wantCount)
			}
			if got := tracker.Multiplier(op.query); got != multiplierFor(wantCount) {
				t.Fatalf("seed=%d now=%d Multiplier=%v，参考值 %v", seed, op.query, got, multiplierFor(wantCount))
			}
			if count, multiplier := tracker.Settle(op.query); count != wantCount || multiplier != multiplierFor(wantCount) {
				t.Fatalf("seed=%d now=%d Settle=(%d,%v)，参考值 (%d,%v)",
					seed, op.query, count, multiplier, wantCount, multiplierFor(wantCount))
			}
			if got := tracker.ActiveTargets(op.query); got != wantActive {
				t.Fatalf("seed=%d now=%d ActiveTargets=%d，参考值 %d", seed, op.query, got, wantActive)
			}
			if got := tracker.Buckets(op.query); got != wantBuckets {
				t.Fatalf("seed=%d now=%d Buckets=%d，参考值 %d", seed, op.query, got, wantBuckets)
			}
			first = append(first, observe(tracker, op.query))
		}

		// 重复回放：结果必须逐位相同。
		replay := NewTracker()
		var second []string
		for _, op := range ops {
			if op.hit != nil {
				replay.Hit(*op.hit)
				continue
			}
			second = append(second, observe(replay, op.query))
		}
		if len(first) != len(second) {
			t.Fatalf("seed=%d 回放观测数不一致", seed)
		}
		for index := range first {
			if first[index] != second[index] {
				t.Fatalf("seed=%d 第 %d 次观测回放不一致：%s != %s", seed, index, first[index], second[index])
			}
		}
	}
}
