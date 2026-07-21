package eval

import (
	"context"
	"encoding/json"
	"strings"
	"watchTower/ai/tools"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// 工具名 + 描述必须与 ai/tools/* 中的真实工具完全一致，否则模型在 eval 与真实链路下的工具选择行为会分叉。
// 修改真实工具描述时务必同步此处（后续可改为从 tools 包导出常量收敛）。
const (
	mockAlertsDesc = "Query active alerts from Prometheus alerting system. This tool retrieves all currently active/firing alerts including their labels, annotations, state, and values. Use this tool when you need to check what alerts are currently firing, investigate alert conditions, or monitor alert status."
	mockLogDesc    = "Query logs from Tencent CLS by keyword. Use this to search service logs when investigating alerts. Pass the keyword the alert-handling doc prescribes (e.g. 'panic' for ServiceDown, 'response' for APIHighErrorRate, 'reconciliation' for ReconciliationDiff). Default time range is the last 1 hour."
	mockDocsDesc   = "Use this tool to search internal documentation and knowledge base for relevant information. It performs RAG (Retrieval-Augmented Generation) to find similar documents and extract processing steps. This is useful when you need to understand internal procedures, best practices, or step-by-step guides stored in the company's documentation."
	mockTimeDesc   = "Get current system time in multiple formats. Returns the current time in seconds (Unix timestamp), milliseconds, and microseconds. Use this tool when you need to retrieve current system time for logging, timing operations, or timestamping events."
)

// MockProvider 把 Scenario 的固定数据注入为 mock 工具，实现离线确定性评估。
// 与 tools.RealProvider 同实现 tools.ToolProvider，agent 侧无感知。
type MockProvider struct {
	S *Scenario
}

func (m MockProvider) Provide(ctx context.Context) (tools.ToolSet, error) {
	alerts, err := mockAlertsTool(m.S)
	if err != nil {
		return tools.ToolSet{}, err
	}
	logT, err := mockLogTool(m.S)
	if err != nil {
		return tools.ToolSet{}, err
	}
	docs, err := mockDocsTool(m.S)
	if err != nil {
		return tools.ToolSet{}, err
	}
	timeT, err := mockTimeTool(m.S)
	if err != nil {
		return tools.ToolSet{}, err
	}
	return tools.ToolSet{
		Alerts: []tool.BaseTool{alerts},
		Log:    []tool.BaseTool{logT},
		Docs:   []tool.BaseTool{docs},
		Time:   []tool.BaseTool{timeT},
	}, nil
}

// mockAlertsTool 直接返回 scenario 的告警快照（忽略入参）。
func mockAlertsTool(s *Scenario) (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_prometheus_alerts", mockAlertsDesc,
		func(ctx context.Context, input *struct{}, opts ...tool.Option) (string, error) {
			return s.AlertsJSON, nil
		})
}

type mockLogItem struct {
	Msg string `json:"msg"`
}
type mockLogOut struct {
	Success bool          `json:"success"`
	Total   int           `json:"total"`
	Logs    []mockLogItem `json:"logs"`
	Query   string        `json:"query"`
}

// mockLogTool 模拟 CLS 关键字检索：入参 query 命中任一 fixture 的 keyword 即返回该 fixture 文本。
// query 为空视为查询全部（返回所有 fixture 拼接）。
func mockLogTool(s *Scenario) (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_log", mockLogDesc,
		func(ctx context.Context, input *tools.QueryLogInput, opts ...tool.Option) (string, error) {
			q := strings.ToLower(input.Query)
			var matched []string
			for _, f := range s.Logs {
				if q == "" {
					matched = append(matched, f.Text)
					continue
				}
				for _, kw := range f.Keywords {
					if strings.Contains(q, strings.ToLower(kw)) {
						matched = append(matched, f.Text)
						break
					}
				}
			}
			var items []mockLogItem
			for _, text := range matched {
				for _, line := range strings.Split(text, "\n") {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					items = append(items, mockLogItem{Msg: line})
				}
			}
			out := mockLogOut{Success: true, Total: len(items), Logs: items, Query: input.Query}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		})
}

type mockDocItem struct {
	Content string `json:"content"`
}

// mockDocsTool 模拟 RAG 检索：入参 query 命中任一 alertname 即返回该文档小节。
func mockDocsTool(s *Scenario) (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"query_internal_docs", mockDocsDesc,
		func(ctx context.Context, input *tools.QueryInternalDocsInput, opts ...tool.Option) (string, error) {
			q := strings.ToLower(input.Query)
			var items []mockDocItem
			for _, d := range s.Docs {
				for _, name := range d.AlertNames {
					if strings.Contains(q, strings.ToLower(name)) {
						items = append(items, mockDocItem{Content: d.Text})
						break
					}
				}
			}
			b, _ := json.MarshalIndent(items, "", "  ")
			return string(b), nil
		})
}

type mockTimeOut struct {
	Success   bool   `json:"success"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// mockTimeTool 固定返回 scenario 的 FixedNow，保证时间相关参数确定性。
func mockTimeTool(s *Scenario) (tool.InvokableTool, error) {
	return utils.InferOptionableTool(
		"get_current_time", mockTimeDesc,
		func(ctx context.Context, input *tools.GetCurrentTimeInput, opts ...tool.Option) (string, error) {
			out := mockTimeOut{Success: true, Timestamp: s.FixedNow, Message: "Current time retrieved successfully (eval fixed)"}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), nil
		})
}
