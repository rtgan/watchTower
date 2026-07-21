package tools

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/tool"
)

// ToolSet 把 Agent 可用的工具按用途分组，供 executor / supervisor / eval 复用。
//
// 设计动机：原先每个执行器（plan_execute_replan/executor.go）把工具列表硬编码在内部，
// eval 无法注入 mock 工具做确定性评估，supervisor 也无法按子 agent 裁剪工具子集。
// 抽象出 ToolProvider 后，真实链路用 RealProvider，eval 用 MockProvider，互不侵入。
type ToolSet struct {
	Log    []tool.BaseTool // query_log（CLS 直连 / MCP 回退）；可能为空（降级跳过）
	Alerts []tool.BaseTool // query_prometheus_alerts
	Docs   []tool.BaseTool // query_internal_docs（RAG/Milvus）
	Time   []tool.BaseTool // get_current_time
}

// All 展平为单一切片（顺序：Log -> Alerts -> Docs -> Time）。
func (ts ToolSet) All() []tool.BaseTool {
	out := make([]tool.BaseTool, 0, len(ts.Log)+len(ts.Alerts)+len(ts.Docs)+len(ts.Time))
	out = append(out, ts.Log...)
	out = append(out, ts.Alerts...)
	out = append(out, ts.Docs...)
	out = append(out, ts.Time...)
	return out
}

// ToolProvider 抽象工具来源。默认实现 RealProvider 走真实工具；eval 注入 MockProvider。
type ToolProvider interface {
	Provide(ctx context.Context) (ToolSet, error)
}

// RealProvider 真实工具（保持与原 executor 完全一致的行为）。
type RealProvider struct{}

func (RealProvider) Provide(ctx context.Context) (ToolSet, error) {
	ts := ToolSet{
		Alerts: []tool.BaseTool{NewPrometheusAlertsQueryTool()},
		Docs:   []tool.BaseTool{NewQueryInternalDocsTool()},
		Time:   []tool.BaseTool{NewGetCurrentTimeTool()},
	}
	// log：走腾讯 CLS（直连优先 / MCP 回退）。本地无 CLS 或 token 失效时降级跳过，不阻塞 Agent。
	if mcpTools, err := GetLogMcpTool(); err != nil {
		log.Printf("[provider] CLS 日志工具不可用，已降级跳过 query_log: %v", err)
	} else {
		ts.Log = mcpTools
	}
	return ts, nil
}
