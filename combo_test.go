package combo

import "testing"

func TestCountsDistinctSwings(t *testing.T) {
	tracker := NewTracker()
	for index, at := range []int64{0, 100, 200} {
		tracker.Hit(HitEvent{AttackID: string(rune('a' + index)), Target: "dummy", AtMs: at, Damage: 10})
	}
	if got := tracker.Count(200); got != 3 {
		t.Fatalf("三次挥击的连击数不对：%d", got)
	}
	if got := tracker.Multiplier(200); got != 1.2 {
		t.Fatalf("三连击的倍率应该是 1.2：%v", got)
	}
	if count, multiplier := tracker.Settle(200); count != 3 || multiplier != 1.2 {
		t.Fatalf("同一时刻的结算快照应该等于实时值：%d %v", count, multiplier)
	}
}

func TestDedupeSameSwingSameTarget(t *testing.T) {
	tracker := NewTracker()
	for index := 0; index < 3; index++ {
		tracker.Hit(HitEvent{AttackID: "swing-1", Target: "dummy", AtMs: int64(index * 20), Damage: 10})
	}
	if got := tracker.Count(60); got != 1 {
		t.Fatalf("同一段挥击对同一目标只算一次：%d", got)
	}
}

func TestMultiplierTiers(t *testing.T) {
	tracker := NewTracker()
	for index := 0; index < 5; index++ {
		tracker.Hit(HitEvent{AttackID: string(rune('a' + index)), Target: "dummy", AtMs: int64(index * 50)})
	}
	if got := tracker.Multiplier(200); got != 1.5 {
		t.Fatalf("五连击的倍率应该是 1.5：%v", got)
	}
}

func TestActiveTargetsCountsFreshHits(t *testing.T) {
	tracker := NewTracker()
	tracker.Hit(HitEvent{AttackID: "a", Target: "left", AtMs: 0})
	tracker.Hit(HitEvent{AttackID: "b", Target: "right", AtMs: 0})
	if got := tracker.ActiveTargets(0); got != 2 {
		t.Fatalf("同一时刻两个目标都该算活跃：%d", got)
	}
}

func TestConstants(t *testing.T) {
	if WindowMs != 800 || BucketMs != 100 || MaxBuckets != 9 {
		t.Fatal("连击窗口常量被改了")
	}
}
