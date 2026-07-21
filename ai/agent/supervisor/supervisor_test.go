package supervisor

import (
	"strings"
	"testing"
)

func TestParseAlerts(t *testing.T) {
	alertsJSON := `{"status":"success","data":{"alerts":[
		{"labels":{"alertname":"ServiceDown","severity":"critical","instance":"10.0.0.10:8080"},"annotations":{"description":"服务不可用"},"state":"firing"},
		{"labels":{"alertname":"ServiceDown","severity":"critical","instance":"10.0.0.10:8081"},"annotations":{"description":"重复告警"},"state":"firing"},
		{"labels":{"alertname":"HighCPUUsage","severity":"warning","instance":"10.0.0.10:9100"},"annotations":{"description":"CPU 高"},"state":"firing"}
	]}}`
	alerts, err := parseAlerts(alertsJSON)
	if err != nil {
		t.Fatalf("parseAlerts: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expect 2 alerts after dedup, got %d", len(alerts))
	}
	if alerts[0].Name != "ServiceDown" || alerts[0].Description != "服务不可用" {
		t.Fatalf("first alert mismatch: %+v", alerts[0])
	}
	if alerts[1].Name != "HighCPUUsage" {
		t.Fatalf("second alert mismatch: %+v", alerts[1])
	}
}

func TestParseAlertsError(t *testing.T) {
	_, err := parseAlerts(`{"status":"error","error":"bad_data"}`)
	if err == nil {
		t.Fatal("expect error for non-success status")
	}
}

// TestParseAlertsToolFormat 验证真实 query_prometheus_alerts 工具的简化输出格式（非原始 Prometheus）。
// 这是真实 /api/ai-ops 链路 triage 拿到的格式，曾因 parseAlerts 只认原始格式而 500。
func TestParseAlertsToolFormat(t *testing.T) {
	// 工具简化输出：success + alerts[].alert_name/description
	simplified := `{"success":true,"alerts":[
		{"alert_name":"ServiceDown","description":"服务不可用","state":"firing","active_at":"x","duration":"1m"},
		{"alert_name":"HighCPUUsage","description":"CPU 高","state":"firing","active_at":"y","duration":"2m"}
	],"message":"Successfully retrieved 2 active alerts"}`
	alerts, err := parseAlerts(simplified)
	if err != nil {
		t.Fatalf("parseAlerts tool format: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("expect 2 alerts, got %d", len(alerts))
	}
	if alerts[0].Name != "ServiceDown" || alerts[0].Description != "服务不可用" {
		t.Fatalf("first alert mismatch: %+v", alerts[0])
	}
}

// TestParseAlertsToolError 验证工具失败（success=false）时返回真实错误信息。
func TestParseAlertsToolError(t *testing.T) {
	_, err := parseAlerts(`{"success":false,"error":"connection refused","message":"Failed"}`)
	if err == nil {
		t.Fatal("expect error for tool failure")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("error should contain real cause, got: %v", err)
	}
}

func TestParseFindingJSON(t *testing.T) {
	raw := `一些前缀文本 {"root_cause":"order/service.go:128","evidence":["panic","code=12003"],"remediation":"回滚 v1.8.1"} 后缀`
	f := parseFinding(raw, "ServiceDown")
	if f.RootCause != "order/service.go:128" {
		t.Fatalf("root cause mismatch: %s", f.RootCause)
	}
	if len(f.Evidence) != 2 {
		t.Fatalf("evidence count: %d", len(f.Evidence))
	}
	if f.AlertName != "ServiceDown" {
		t.Fatalf("alert name: %s", f.AlertName)
	}
}

func TestParseFindingFallback(t *testing.T) {
	// 无 JSON：原文作为 root_cause
	raw := "无法定位根因，建议人工排查"
	f := parseFinding(raw, "X")
	if !strings.Contains(f.RootCause, "无法定位") {
		t.Fatalf("fallback root cause: %s", f.RootCause)
	}
}

func TestWidenTimeWindow(t *testing.T) {
	args := `{"query":"panic","start_time":"2026-06-20 06:00:00","end_time":"2026-06-20 07:00:00","limit":50}`
	out := widenTimeWindow(args)
	if !strings.Contains(out, "panic") {
		t.Fatalf("widened should keep query: %s", out)
	}
	if strings.Contains(out, "2026-06-20 06:00:00") {
		t.Fatalf("widened should replace original start_time: %s", out)
	}
}

func TestIsEmptyOrFailed(t *testing.T) {
	if !isEmptyOrFailed(`{"success":false,"total":0}`) {
		t.Fatal("expect failed result detected")
	}
	if !isEmptyOrFailed(`{"success":true,"total":0}`) {
		t.Fatal("expect empty result detected")
	}
	if isEmptyOrFailed(`{"success":true,"total":5}`) {
		t.Fatal("expect non-empty success not detected as empty")
	}
	if isEmptyOrFailed("not json") {
		t.Fatal("non-json should not be treated as empty")
	}
}
