// Command combo-counter 跑连击样例。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/combo"
)

func swing(tracker *combo.Tracker, index int, target string, atMs int64) {
	tracker.Hit(combo.HitEvent{AttackID: fmt.Sprintf("swing-%d", index), Target: target, AtMs: atMs, Damage: 10})
}

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("combo-counter", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sample := flags.String("sample", "aoe", "aoe / slide / settle / churn")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	tracker := combo.NewTracker()
	switch *sample {
	case "aoe":
		// 一次横扫挥击同时扫到三个目标。
		for _, target := range []string{"left", "mid", "right"} {
			tracker.Hit(combo.HitEvent{AttackID: "cleave-1", Target: target, AtMs: 0, Damage: 10})
		}
		fmt.Fprintf(stdout, "count=%d multiplier=%.1f\n", tracker.Count(0), tracker.Multiplier(0))
	case "slide":
		// 五连击之后停 600 毫秒，再补一下。
		for index, at := range []int64{0, 100, 200, 300, 400} {
			swing(tracker, index, "dummy", at)
		}
		swing(tracker, 5, "dummy", 1000)
		fmt.Fprintf(stdout, "count=%d multiplier=%.1f\n", tracker.Count(1000), tracker.Multiplier(1000))
	case "settle":
		// 五连击先结算一次，再补一下看快照跟不跟得上。
		for index, at := range []int64{0, 100, 200, 300, 400} {
			swing(tracker, index, "dummy", at)
		}
		settledCount, settledMultiplier := tracker.Settle(400)
		swing(tracker, 5, "dummy", 500)
		fmt.Fprintf(stdout, "live=%d/%.1f settled=%d/%.1f\n",
			tracker.Count(500), tracker.Multiplier(500), settledCount, settledMultiplier)
	case "churn":
		// 二十万个目标各挨一下，五秒后再看内存观测。
		for index := 0; index < 200000; index++ {
			swing(tracker, index, fmt.Sprintf("mob-%d", index), 0)
		}
		fmt.Fprintf(stdout, "targets=%d buckets=%d\n", tracker.ActiveTargets(5000), tracker.Buckets(5000))
	default:
		fmt.Fprintln(stderr, "需要 --sample aoe|slide|settle|churn")
		return 2
	}
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
