package combo

import "testing"

func TestCountsDistinctSwings(t *testing.T) {
	tracker := &Tracker{}
	for index, at := range []int64{0, 100, 200} {
		if got := tracker.Hit(HitEvent{AttackID: string(rune('a' + index)), AtMs: at, Damage: 10}); got != index+1 {
			t.Fatalf("第 %d 次挥击的连击数不对：%d", index+1, got)
		}
	}
	if tracker.Tip() != 3 {
		t.Fatalf("峰值应该是 3：%d", tracker.Tip())
	}
}

func TestWindowBoundaryKeepsChain(t *testing.T) {
	tracker := &Tracker{}
	tracker.Hit(HitEvent{AttackID: "a", AtMs: 0})
	if got := tracker.Hit(HitEvent{AttackID: "b", AtMs: WindowMs}); got != 2 {
		t.Fatalf("间隔正好等于窗口时应该继续连击：%d", got)
	}
}

func TestBreakRestartsCount(t *testing.T) {
	tracker := &Tracker{}
	tracker.Hit(HitEvent{AttackID: "a", AtMs: 0})
	tracker.Hit(HitEvent{AttackID: "b", AtMs: 100})
	if got := tracker.Hit(HitEvent{AttackID: "c", AtMs: 100 + WindowMs + 1}); got != 1 {
		t.Fatalf("断连之后应该从 1 重新数：%d", got)
	}
}

func TestMultiplierTiers(t *testing.T) {
	tracker := &Tracker{}
	if tracker.Multiplier() != 1.0 {
		t.Fatalf("没有连击时倍率应该是 1.0：%v", tracker.Multiplier())
	}
	for index := 0; index < 5; index++ {
		tracker.Hit(HitEvent{AttackID: string(rune('a' + index)), AtMs: int64(index * 100)})
	}
	if tracker.Multiplier() != 1.5 {
		t.Fatalf("五连击的倍率应该是 1.5：%v", tracker.Multiplier())
	}
}

func TestWindowConstant(t *testing.T) {
	if WindowMs != 800 {
		t.Fatal("连击窗口被改了")
	}
}
