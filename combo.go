// Package combo 是战斗连击计数：同一次挥击去重、断连判定、峰值档位结算。
// 缺陷：重复命中没有去重，断连时峰值档位（Tip）没有跟着清。
package combo

// WindowMs 是连击窗口：距上一次有效命中超过这个毫秒数就断连。
const WindowMs = 800

// HitEvent 是一次命中事件。
type HitEvent struct {
	AttackID string // 同一次挥击（同一段攻击动作）的编号
	Target   string // 被命中的目标
	AtMs     int64  // 命中时刻，毫秒
	Damage   int
}

// Tracker 记录当前连击的状态。
type Tracker struct {
	count        int
	tip          int
	lastAtMs     int64
	lastAttackID string
	hits         int
}

// Hit 记一次命中，返回命中后的连击数。
// 缺陷：没有按 AttackID 去重，同一次挥击的重复命中也会涨连击；
// 缺陷：断连时只把 Count 清零，Tip 留着不降。
func (t *Tracker) Hit(ev HitEvent) int {
	if ev.AtMs-t.lastAtMs > WindowMs {
		t.count = 0
	}
	t.count++
	t.hits++
	t.lastAtMs = ev.AtMs
	t.lastAttackID = ev.AttackID
	if t.count > t.tip {
		t.tip = t.count
	}
	return t.count
}

// Count 是当前连击数。
func (t *Tracker) Count() int { return t.count }

// Tip 是本段连击的峰值连击数，结算档位按它算。
func (t *Tracker) Tip() int { return t.tip }

// Hits 是累计接受的命中事件数（含被忽略的重复事件）。
func (t *Tracker) Hits() int { return t.hits }

// Multiplier 按峰值档位返回连击倍率。
func (t *Tracker) Multiplier() float64 {
	switch {
	case t.tip >= 10:
		return 2.0
	case t.tip >= 5:
		return 1.5
	case t.tip >= 3:
		return 1.2
	default:
		return 1.0
	}
}
