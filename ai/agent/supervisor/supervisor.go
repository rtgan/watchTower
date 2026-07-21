package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
	"watchTower/ai/tools"
	"watchTower/common/trace"
	"watchTower/eval"

	"github.com/cloudwego/eino/components/tool"
)

// defaultConcurrency 子 agent 并发上限。
const defaultConcurrency = 4

// execute supervisor 主流程：triage（取告警清单）-> fan-out 并行子 agent -> synthesize（汇总报告）。
// 记录到传入的 rec（不 finalize，由调用方决定何时落盘），与 plan_execute_replan 的"调用方持 recorder"模式一致。
func execute(ctx context.Context, query string, provider tools.ToolProvider, rec *trace.Recorder) (string, []*Finding, error) {
	if provider == nil {
		provider = tools.RealProvider{}
	}
	if rec == nil {
		// 防御：无 recorder 时临时建一个（仅本函数内有效）
		rec = trace.NewRecorder(query, time.Now().Format(time.RFC3339Nano))
	}
	ctx = trace.WithRecorder(ctx, rec)

	// ---- triage ----
	ts, perr := provider.Provide(ctx)
	if perr != nil {
		return "", nil, perr
	}
	if len(ts.Alerts) == 0 {
		return "", nil, fmt.Errorf("no alerts tool available")
	}
	alertsTool, ok := ts.Alerts[0].(tool.InvokableTool)
	if !ok {
		return "", nil, fmt.Errorf("alerts tool not invokable")
	}
	s, ctx2 := rec.StartSpan(ctx, "triage:query_prometheus_alerts", "Tool", "{}")
	alertsJSON, e := alertsTool.InvokableRun(ctx2, "{}")
	rec.EndSpan(ctx2, s, alertsJSON, e)
	if e != nil {
		return "", nil, e
	}
	alerts, err := parseAlerts(alertsJSON)
	if err != nil {
		return "", nil, err
	}
	if len(alerts) == 0 {
		return "未发现活跃告警", nil, nil
	}

	// ---- fan-out：每条告警一个子 agent，并发上限 defaultConcurrency ----
	findings := make([]*Finding, len(alerts))
	sem := make(chan struct{}, defaultConcurrency)
	var wg sync.WaitGroup
	for i, a := range alerts {
		wg.Add(1)
		go func(idx int, alert AlertInfo) {
			defer wg.Done()
			sem <- struct{}{} //控制并发上限
			defer func() { <-sem }()
			findings[idx] = runSubAgent(ctx, rec, alert, provider)
		}(i, a)
	}
	wg.Wait()

	// ---- synthesize ----
	report, err := synthesize(ctx, findings, query)
	return report, findings, err
}

// Run 供 eval 钩子 / 直接调用。ctx 无 recorder 时自建并 finalize+落盘；有则复用、不重复 finalize。
func Run(ctx context.Context, query string, provider tools.ToolProvider) (report string, findings []*Finding, run *trace.Run, err error) {
	rec := trace.FromContext(ctx)
	owned := rec == nil
	if owned {
		rec = trace.NewRecorder(query, time.Now().Format(time.RFC3339Nano))
	}
	report, findings, err = execute(ctx, query, provider, rec)
	status := "ok"
	if err != nil {
		status = "error"
	}
	if owned {
		run = rec.Finalize(status, err)
	} else {
		run = rec.Run()
	}
	return report, findings, run, err
}

// BuildSupervisorAgent 供 controller / cmd 调用，签名与 plan_execute_replan 对齐（返回 detail=findings JSON）。
// ctx 有 recorder（controller 注入）则复用、不 finalize；无则自管。
func BuildSupervisorAgent(ctx context.Context, query string) (string, []string, error) {
	rec := trace.FromContext(ctx)
	owned := rec == nil
	if owned {
		rec = trace.NewRecorder(query, time.Now().Format(time.RFC3339Nano))
	}
	report, findings, err := execute(ctx, query, tools.RealProvider{}, rec)
	if owned {
		status := "ok"
		if err != nil {
			status = "error"
		}
		rec.Finalize(status, err)
	}
	if err != nil && report == "" {
		return "", nil, err
	}
	return report, findingsToDetail(findings), err
}

func findingsToDetail(findings []*Finding) []string {
	detail := make([]string, 0, len(findings))
	for _, f := range findings {
		b, _ := json.Marshal(f)
		detail = append(detail, string(b))
	}
	return detail
}

// 注入 eval 的 supervisor 钩子，使 `go run ./ai/cmd/eval_cmd --strategy supervisor` 可评估。
func init() {
	eval.SetSupervisorRunFn(func(ctx context.Context, query string, provider tools.ToolProvider) (string, []string, error) {
		report, findings, _, err := Run(ctx, query, provider)
		if err != nil && report == "" {
			return "", nil, err
		}
		return report, findingsToDetail(findings), err
	})
}
