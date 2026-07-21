package mcp_server

// 工具名 + 描述 + schema 必须与 ai/tools/* 中的真实工具一致（与 eval/mock_tools.go 同源同步）。
// 后续可改为从 eino tool.Info(ctx) 派生收敛。
const (
	alertsDesc = "Query active alerts from Prometheus alerting system. This tool retrieves all currently active/firing alerts including their labels, annotations, state, and values. Use this tool when you need to check what alerts are currently firing, investigate alert conditions, or monitor alert status."
	docsDesc   = "Use this tool to search internal documentation and knowledge base for relevant information. It performs RAG (Retrieval-Augmented Generation) to find similar documents and extract processing steps. This is useful when you need to understand internal procedures, best practices, or step-by-step guides stored in the company's documentation."
	logDesc    = "Query logs from Tencent CLS by keyword. Use this to search service logs when investigating alerts. Pass the keyword the alert-handling doc prescribes (e.g. 'panic' for ServiceDown, 'response' for APIHighErrorRate, 'reconciliation' for ReconciliationDiff). Default time range is the last 1 hour."
)

// JSON schema（与各工具 input struct 的 jsonschema tag 对齐）。
const (
	alertsSchema = `{"type":"object","properties":{},"description":"无入参"}`

	docsSchema = `{"type":"object","properties":{"query":{"type":"string","description":"The query string to search in internal documentation"}},"required":["query"]}`

	logSchema = `{"type":"object","properties":{"query":{"type":"string","description":"日志检索关键字或 CQL，如 panic、response。留空查全部"},"start_time":{"type":"string","description":"起始时间 2006-01-02 15:04:05，默认最近1小时"},"end_time":{"type":"string","description":"结束时间 2006-01-02 15:04:05"},"limit":{"type":"integer","description":"返回条数，默认50，最大1000"}}}`
)
