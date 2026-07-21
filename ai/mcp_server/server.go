package mcp_server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"watchTower/ai/tools"

	einotool "github.com/cloudwego/eino/components/tool"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/mark3labs/mcp-go/mcp"
)

// NewServer 创建 watchTower MCP server，把三个核心工具暴露给外部 agent / IDE：
//   - query_prometheus_alerts
//   - query_log（CLS 直连优先 / MCP 回退；不可用时调用返回明确错误，不 crash server）
//   - query_internal_docs（RAG）
//
// 工具实现直接复用 ai/tools/* 的构造函数，保证 MCP 暴露的与 agent 内部用的是同一份逻辑。
func NewServer() (*mcpserver.MCPServer, error) {
	s := mcpserver.NewMCPServer("watchTower", "1.0.0")

	if err := addTool(s, "query_prometheus_alerts", alertsDesc, alertsSchema, tools.NewPrometheusAlertsQueryTool()); err != nil {
		return nil, err
	}
	if err := addTool(s, "query_internal_docs", docsDesc, docsSchema, tools.NewQueryInternalDocsTool()); err != nil {
		return nil, err
	}
	if err := addLogTool(s); err != nil {
		log.Printf("[mcp_server] query_log 注册受限: %v", err)
	}
	return s, nil
}

// addTool 注册一个工具：handler 把 MCP 入参序列化为 JSON 交给 eino 工具的 InvokableRun。
func addTool(s *mcpserver.MCPServer, name, desc, schema string, bt einotool.BaseTool) error {
	t := mcp.NewToolWithRawSchema(name, desc, json.RawMessage(schema))
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := runTool(ctx, bt, argsToJSON(req))
		if err != nil {
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("%s failed: %v", name, err), err), nil
		}
		return mcp.NewToolResultText(out), nil
	})
	return nil
}

// addLogTool 注册 query_log：每次调用 lazy 取（CLS 直连 / MCP 回退），
// 不可用时返回明确错误（工具仍可被 ListTools 发现）。
func addLogTool(s *mcpserver.MCPServer) error {
	t := mcp.NewToolWithRawSchema("query_log", logDesc, json.RawMessage(logSchema))
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logTools, err := tools.GetLogMcpTool()
		if err != nil || len(logTools) == 0 {
			msg := "query_log unavailable"
			if err != nil {
				msg += ": " + err.Error()
			}
			return mcp.NewToolResultError(msg), nil
		}
		out, err := runTool(ctx, logTools[0], argsToJSON(req))
		if err != nil {
			return mcp.NewToolResultErrorFromErr("query_log failed: "+err.Error(), err), nil
		}
		return mcp.NewToolResultText(out), nil
	})
	return nil
}

func runTool(ctx context.Context, bt einotool.BaseTool, argsJSON string) (string, error) {
	it, ok := bt.(einotool.InvokableTool)
	if !ok {
		return "", fmt.Errorf("tool not invokable")
	}
	return it.InvokableRun(ctx, argsJSON)
}

func argsToJSON(req mcp.CallToolRequest) string {
	args := req.GetArguments()
	if len(args) == 0 {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// RunStdio 以 stdio transport 启动（本地 / IDE 接入）。
func RunStdio(ctx context.Context) error {
	s, err := NewServer()
	if err != nil {
		return err
	}
	return mcpserver.NewStdioServer(s).Listen(ctx, os.Stdin, os.Stdout)
}

// RunSSE 以 SSE transport 启动（远程接入，addr 如 ":9100"）。
func RunSSE(ctx context.Context, addr string) error {
	s, err := NewServer()
	if err != nil {
		return err
	}
	return mcpserver.NewSSEServer(s).Start(addr)
}
