# Spec: 暴露 watchTower 为 MCP Server

> 对应维度 #5。已用 MCP 作 CLS 客户端（回退路径）；本项目把自身的告警/日志/文档查询能力**反向暴露**成 MCP server，供别的 agent / IDE（Cursor/Claude）复用。2026 企业落地标配姿态。

## 目标
用 `mark3labs/mcp-go` 的 server 端，把 watchTower 的三个核心工具打包为 MCP tools：
- `query_prometheus_alerts`
- `query_log`（CLS 直连）
- `query_internal_docs`（RAG）

并提供 stdio + SSE 两种 transport 启动。

## 契约

### `ai/mcp_server/server.go`
```go
func NewServer(ctx) (*server.MCPServer, error)   // 注册三个工具
func RunStdio(ctx) error                          // stdio transport（本地/IDE 接入）
func RunSSE(ctx, addr string) error               // SSE transport（远程接入）
```

### 工具注册
每个工具：
```go
mcp.NewToolWithRawSchema(name, description, schema json.RawMessage)
server.AddTool(tool, func(ctx, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 解析 req.Params.Arguments -> 调对应 eino tool 的 InvokableRun -> 包成 mcp.CallToolResult
})
```
- schema：直接复用各工具的 input struct（用 `encoding/json` 或 jsonschema 生成；为稳，手工写 JSON schema 常量，与工具 jsonschema tag 对齐）。
- 结果：`mcp.NewToolResultText(output)`；失败时 `mcp.NewToolResultError(err)`。

### `ai/cmd/mcp_server_cmd/main.go`
- 读 config（复用 CLS/Prometheus/Milvus 配置）。
- 默认 stdio；`--sse :9100` 走 SSE。
- 不依赖 LLM（纯工具透传），轻量。

## 复用
- 工具实现直接复用 `ai/tools/*` 的构造函数（`NewCLSQueryLogTool` 等）或直接调底层函数，保证 MCP 暴露的工具与 agent 内部用的是同一份。
- query_log：若 CLS 未配置，该 MCP tool 仍注册但在调用时返回明确错误（不 crash server）。

## 验收
- [ ] `go run ./ai/cmd/mcp_server_cmd` stdio 启动，用 mcp-go client `ListTools` 能看到三个工具。
- [ ] 用 client `CallTool("query_prometheus_alerts", {})` 能拿到与 agent 内部一致的告警 JSON。
- [ ] `--sse :9100` 启动后，SSE endpoint 可被外部 MCP client 连接并调用。
- [ ] README 写明如何在 Cursor/Claude Desktop 配置接入。

## 文件
- 新增：`ai/mcp_server/server.go`、`ai/cmd/mcp_server_cmd/main.go`
- 修改：无（不改现有 agent 链路）
