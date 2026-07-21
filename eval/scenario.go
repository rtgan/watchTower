package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Scenario 一个评估场景：固定的 mock 输入 + 期望的判定。
//
// 设计为离线 + 确定性：所有工具返回都来自本结构，agent 的 LLM 调用是真实的（被评估对象）。
// 同一 Scenario 多次跑，确定性检查分数应一致；LLM-as-judge 因模型有随机性，允许 ±5 抖动。
type Scenario struct {
	ID          string `json:"id"`
	Description string `json:"description"`

	// ---- mock 数据源（注入给 agent 的工具）----
	// query_prometheus_alerts 的返回体（完整 Prometheus /api/v1/alerts 响应）
	AlertsJSON string `json:"alerts_json"`
	// query_log 的返回：按 keyword 匹配 fixture。多个 fixture 命中则拼接。
	Logs []LogFixture `json:"logs"`
	// query_internal_docs 的返回：按 alertname 匹配 fixture。
	Docs []DocFixture `json:"docs"`
	// get_current_time 固定返回，保证时间相关参数确定性
	FixedNow string `json:"fixed_now"`

	// ---- 期望（用于打分）----
	Expected Expected `json:"expected"`
}

// LogFixture 一份日志样例。agent 用 query_log 检索时，若入参 query 命中任一 keyword，
// 则返回本 fixture 的 Text。
type LogFixture struct {
	Keywords []string `json:"keywords"` // 命中关键字，如 ["panic"]
	Text     string   `json:"text"`     // 返回的日志文本
}

// DocFixture 一份内部文档样例。agent 用 query_internal_docs 检索时，若入参 query 命中任一 AlertName，
// 则返回本 fixture 的 Text。
type DocFixture struct {
	AlertNames []string `json:"alert_names"` // 命中告警名，如 ["ServiceDown","服务下线"]
	Text       string   `json:"text"`        // 返回的文档小节
}

// Expected 期望判定。用于确定性检查器 + LLM-as-judge。
type Expected struct {
	// 期望命中的根因关键词（出现在报告里即算命中），如 "order/service.go:128"
	RootCause string `json:"root_cause"`
	// 期望 query_log 使用过的关键字（出现在轨迹里即算命中）
	Keywords []string `json:"keywords"`
	// 期望解读的错误码（出现在报告里即算命中）
	ErrorCodes []string `json:"error_codes"`
	// 期望必须调用的工具（出现在轨迹里即算命中）
	MustCallTools []string `json:"must_call_tools"`
	// 期望报告要点（judge 对照用，非逐字比对）
	GoldenReport string `json:"golden_report"`
}

// LoadScenarios 从目录读所有 *.json 场景。
func LoadScenarios(dir string) ([]*Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []*Scenario
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var s Scenario
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		out = append(out, &s)
	}
	return out, nil
}
