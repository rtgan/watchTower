package eval

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
)

// runTool 调用 BaseTool 的 InvokableRun（断言为 InvokableTool）。
func runTool(t *testing.T, bt tool.BaseTool, args string) string {
	t.Helper()
	it, ok := bt.(tool.InvokableTool)
	if !ok {
		t.Fatalf("tool not InvokableTool: %T", bt)
	}
	out, err := it.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	return out
}

// TestDefaultScenarios 内置场景结构合法。
func TestDefaultScenarios(t *testing.T) {
	ss := DefaultScenarios()
	if len(ss) != 3 {
		t.Fatalf("expect 3 scenarios, got %d", len(ss))
	}
	for _, s := range ss {
		if s.ID == "" || s.AlertsJSON == "" {
			t.Fatalf("scenario missing id or alerts: %+v", s)
		}
		var probe map[string]any
		if err := json.Unmarshal([]byte(s.AlertsJSON), &probe); err != nil {
			t.Fatalf("scenario %s alerts not json: %v", s.ID, err)
		}
	}
}

// TestMockLogTool 验证 mock query_log 按 keyword 命中 fixture。
func TestMockLogTool(t *testing.T) {
	s := scenarioSingleServiceDown()
	prov := MockProvider{S: s}
	ts, err := prov.Provide(context.Background())
	if err != nil {
		t.Fatalf("provide: %v", err)
	}
	if len(ts.Log) != 1 {
		t.Fatalf("expect 1 log tool, got %d", len(ts.Log))
	}
	out := runTool(t, ts.Log[0], `{"query":"panic"}`)
	var res mockLogOut
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("parse mock log out: %v\nraw: %s", err, out)
	}
	if !res.Success || res.Total == 0 {
		t.Fatalf("expect non-empty logs for query=panic, got %+v", res)
	}
	joined := ""
	for _, l := range res.Logs {
		joined += l.Msg + "\n"
	}
	if !strings.Contains(joined, "order/service.go:128") {
		t.Fatalf("panic logs missing root cause line: %s", joined)
	}

	out2 := runTool(t, ts.Log[0], `{"query":"nonexistent"}`)
	var res2 mockLogOut
	json.Unmarshal([]byte(out2), &res2)
	if res2.Total != 0 {
		t.Fatalf("expect empty for unmatched keyword, got %d", res2.Total)
	}
}

// TestMockDocsTool 验证 mock query_internal_docs 按 alertname 命中。
func TestMockDocsTool(t *testing.T) {
	s := scenarioSingleServiceDown()
	prov := MockProvider{S: s}
	ts, _ := prov.Provide(context.Background())
	out := runTool(t, ts.Docs[0], `{"query":"ServiceDown"}`)
	if !strings.Contains(out, "panic") {
		t.Fatalf("ServiceDown doc should mention panic keyword, got: %s", out)
	}
}

// TestMockAlertsTool 验证 mock query_prometheus_alerts 返回 scenario 告警快照。
func TestMockAlertsTool(t *testing.T) {
	s := scenarioSingleServiceDown()
	prov := MockProvider{S: s}
	ts, _ := prov.Provide(context.Background())
	out := runTool(t, ts.Alerts[0], `{}`)
	if !strings.Contains(out, "ServiceDown") {
		t.Fatalf("alerts mock should contain ServiceDown, got: %s", out)
	}
}

// TestCheckers 验证确定性检查器。
func TestCheckers(t *testing.T) {
	detail := []string{"call query_log with keyword=panic", "call query_internal_docs ServiceDown"}
	report := "根因是 order/service.go:128 空指针，错误码 12003"

	if !CheckTools(detail, []string{"query_log", "query_internal_docs"}).Pass {
		t.Fatal("CheckTools should pass")
	}
	if !CheckKeywords(detail, report, []string{"panic"}).Pass {
		t.Fatal("CheckKeywords should pass")
	}
	if !CheckErrorCodes(report, []string{"12003"}).Pass {
		t.Fatal("CheckErrorCodes should pass")
	}
	if !CheckRootCause(report, "order/service.go:128").Pass {
		t.Fatal("CheckRootCause should pass")
	}
	if CheckRootCause(report, "nonexistent").Pass {
		t.Fatal("CheckRootCause should fail for nonexistent")
	}
}

// TestTrajectoryScore 验证步数效率分单调性。
func TestTrajectoryScore(t *testing.T) {
	baseline := trajectoryScore(trajectoryBaseline)
	if baseline != 100 {
		t.Fatalf("baseline steps should score 100, got %.0f", baseline)
	}
	high := trajectoryScore(trajectoryBaseline + 10)
	if high >= baseline {
		t.Fatalf("more steps should score lower: %.0f >= %.0f", high, baseline)
	}
}

// TestParseJudge 验证 judge JSON 解析（含代码围栏剥离）。
func TestParseJudge(t *testing.T) {
	raw := "```json\n{\"root_cause\":25,\"completeness\":20,\"remediation\":15,\"hallucination_freedom\":18,\"reason\":\"ok\"}\n```"
	r := parseJudge(raw)
	if r.Error != "" {
		t.Fatalf("parse error: %s", r.Error)
	}
	if r.Score != 78 {
		t.Fatalf("expect score 78, got %.0f", r.Score)
	}
}

// TestScorecardStrategyField 验证 strategy 字段从 Runner 流到 Scorecard/Summary。
// 离线（WithJudge(false) 不调 LLM judge；score 只做确定性检查）。
func TestScorecardStrategyField(t *testing.T) {
	s := scenarioSingleServiceDown()
	rr := &RunResult{FinalReport: "根因 order/service.go:128", Detail: []string{"query_log keyword=panic"}}

	// supervisor
	r := NewRunner(WithJudge(false), WithStrategy("supervisor"))
	sc := r.score(context.Background(), s, rr)
	if sc.Strategy != "supervisor" {
		t.Fatalf("scorecard strategy = %q, want supervisor", sc.Strategy)
	}
	sum := Summarize([]*Scorecard{sc})
	if sum.Strategy != "supervisor" {
		t.Fatalf("summary strategy = %q, want supervisor", sum.Strategy)
	}

	// 默认 plan_execute
	r2 := NewRunner(WithJudge(false))
	sc2 := r2.score(context.Background(), s, rr)
	if sc2.Strategy != "plan_execute" {
		t.Fatalf("default strategy = %q, want plan_execute", sc2.Strategy)
	}
}
