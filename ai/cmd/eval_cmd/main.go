package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
	_ "watchTower/ai/agent/supervisor" // 触发 init() 注册 eval 的 supervisor 钩子，使 --strategy supervisor 可用
	"watchTower/common/config"
	"watchTower/eval"
)

// eval_cmd: 离线评估 AIOps agent。
//
// 用法：
//   go run ./ai/cmd/eval_cmd                         # 跑内置 3 场景
//   go run ./ai/cmd/eval_cmd --dir eval/scenarios    # 跑自定义场景目录
//   go run ./ai/cmd/eval_cmd --concurrency 3 --no-judge
//   go run ./ai/cmd/eval_cmd --strategy supervisor   # Phase 4 接入后可用
//
// 依赖：需在 .env 配置 ARK_API_KEY（LLM 是被评估对象，不 mock）；工具走 mock，无需 Prometheus/CLS/Milvus。
func main() {
	dir := flag.String("dir", "", "场景目录（空则用内置 DefaultScenarios）")
	concurrency := flag.Int("concurrency", 1, "并发场景数（<=1 串行）")
	noJudge := flag.Bool("no-judge", false, "跳过 LLM-as-judge（仅确定性检查）")
	strategy := flag.String("strategy", "plan_execute", "执行策略：plan_execute | supervisor")
	perRunTimeout := flag.Duration("timeout", 120*time.Second, "单场景超时")
	strict := flag.Bool("strict", false, "任一场景不通过则退出码 1（用于 CI 回归）")
	flag.Parse()

	if _, err := config.InitConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "init config: %v\n", err)
		os.Exit(2)
	}

	var scenarios []*eval.Scenario
	if *dir != "" {
		s, err := eval.LoadScenarios(*dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load scenarios: %v\n", err)
			os.Exit(2)
		}
		scenarios = s
	} else {
		scenarios = eval.DefaultScenarios()
	}
	if len(scenarios) == 0 {
		fmt.Fprintln(os.Stderr, "no scenarios to run")
		os.Exit(2)
	}
	fmt.Printf("Loaded %d scenario(s). strategy=%s judge=%v concurrency=%d\n\n", len(scenarios), *strategy, !*noJudge, *concurrency)

	runner := eval.NewRunner(eval.WithJudge(!*noJudge), eval.WithStrategy(*strategy))
	results := make([]*eval.RunResult, len(scenarios))
	cards := make([]*eval.Scorecard, len(scenarios))

	if *concurrency <= 1 {
		for i, s := range scenarios {
			runOne(runner, s, *perRunTimeout, &results[i], &cards[i])
		}
	} else {
		// 并发分支直接用 RunAll（内部已做并发控制）
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(len(scenarios))*(*perRunTimeout))
		defer cancel()
		all := runner.RunAll(ctx, scenarios, *concurrency)
		for i, c := range all {
			cards[i] = c
		}
	}

	// 打印每个 scorecard
	for i, c := range cards {
		if c == nil {
			continue
		}
		printScorecard(c, results[i])
	}

	summary := eval.Summarize(nonNil(cards))
	fmt.Println("==================== Summary ====================")
	fmt.Printf("Total=%d  Passed=%d  PassRate=%.1f%%  AvgScore=%.1f\n",
		summary.Total, summary.Passed, summary.PassRate, summary.AvgScore)

	// 落盘
	if err := writeResults(summary, cards, results); err != nil {
		fmt.Fprintf(os.Stderr, "write results: %v\n", err)
	}

	if *strict && summary.Passed != summary.Total {
		os.Exit(1)
	}
}

func runOne(runner *eval.Runner, s *eval.Scenario, perRunTimeout time.Duration, rr **eval.RunResult, sc **eval.Scorecard) {
	ctx, cancel := context.WithTimeout(context.Background(), perRunTimeout)
	defer cancel()
	r, card := runner.Run(ctx, s)
	*rr = r
	*sc = card
}

func printScorecard(c *eval.Scorecard, rr *eval.RunResult) {
	passMark := "FAIL"
	if c.Pass {
		passMark = "PASS"
	}
	fmt.Printf("-------------------- %s [%s] [%s] --------------------\n", c.ScenarioID, c.Strategy, passMark)
	fmt.Printf("Total=%.1f  Trajectory=%.0f  Steps=%d\n", c.Total, c.TrajectoryScore, stepsOf(rr))
	fmt.Printf("  ToolCheck      : %s\n", c.ToolCheck.Detail)
	fmt.Printf("  KeywordCheck   : %s\n", c.KeywordCheck.Detail)
	fmt.Printf("  ErrorCodeCheck : %s\n", c.ErrorCodeCheck.Detail)
	fmt.Printf("  RootCauseCheck : %s\n", c.RootCauseCheck.Detail)
	if c.JudgeScore.Error != "" {
		fmt.Printf("  Judge          : skipped (%s)\n", c.JudgeScore.Error)
	} else {
		fmt.Printf("  Judge          : %.1f (rc=%.0f comp=%.0f remed=%.0f halluc=%.0f) %s\n",
			c.JudgeScore.Score, c.JudgeScore.RootCause, c.JudgeScore.Completeness,
			c.JudgeScore.Remediation, c.JudgeScore.HallucinationFreedom, c.JudgeScore.Reason)
	}
	for _, n := range c.Notes {
		fmt.Printf("  note: %s\n", n)
	}
	fmt.Println()
}

func stepsOf(rr *eval.RunResult) int {
	if rr == nil {
		return 0
	}
	return rr.Steps
}

func nonNil(cards []*eval.Scorecard) []*eval.Scorecard {
	out := make([]*eval.Scorecard, 0, len(cards))
	for _, c := range cards {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}

func writeResults(summary eval.Summary, cards []*eval.Scorecard, results []*eval.RunResult) error {
	root := moduleRoot()
	dir := filepath.Join(root, "eval", "results")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ts := time.Now().Format("20060102-150405")
	name := ts
	if summary.Strategy != "" { // 文件名带策略，便于对比 plan_execute vs supervisor
		name += "-" + summary.Strategy
	}
	path := filepath.Join(dir, name+".json")
	type record struct {
		Summary eval.Summary         `json:"summary"`
		Cards   []*eval.Scorecard    `json:"cards"`
		Results []*eval.RunResult    `json:"results"`
	}
	b, err := json.MarshalIndent(record{Summary: summary, Cards: cards, Results: results}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("Results written to %s\n", path)
	return nil
}

func moduleRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
