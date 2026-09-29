// Command combo-counter 跑连击样例。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/combo"
)

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("combo-counter", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sample := flags.String("sample", "repeat", "repeat / break")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	tracker := &combo.Tracker{}
	switch *sample {
	case "repeat":
		// 同一次挥击的回放被重复上报了 5 次。
		for index := 0; index < 5; index++ {
			tracker.Hit(combo.HitEvent{AttackID: "swing-1", Target: "dummy", AtMs: int64(index * 100), Damage: 10})
		}
	case "break":
		// 先连打 5 下（不同挥击），断了以后再打 2 下。
		for index := 0; index < 5; index++ {
			tracker.Hit(combo.HitEvent{AttackID: fmt.Sprintf("swing-%d", index), AtMs: int64(index * 100)})
		}
		for index, at := range []int64{3000, 3100} {
			tracker.Hit(combo.HitEvent{AttackID: fmt.Sprintf("after-%d", index), AtMs: at})
		}
	default:
		fmt.Fprintln(stderr, "需要 --sample repeat|break")
		return 2
	}
	fmt.Fprintf(stdout, "count=%d tip=%d multiplier=%.1f\n", tracker.Count(), tracker.Tip(), tracker.Multiplier())
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
