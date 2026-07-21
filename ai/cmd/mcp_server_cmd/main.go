package main

import (
	"context"
	"flag"
	"log"
	"watchTower/ai/mcp_server"
	"watchTower/common/config"
)

// mcp_server_cmd: 把 watchTower 的告警/日志/文档查询能力暴露为 MCP server，
// 供别的 agent / IDE（Cursor、Claude Desktop 等）复用。
//
// 用法：
//   go run ./ai/cmd/mcp_server_cmd              # stdio（本地/IDE 接入）
//   go run ./ai/cmd/mcp_server_cmd --sse :9100   # SSE（远程接入）
//
// 依赖：config 复用 etc/conf.yml 的 CLS / Prometheus / Milvus 配置。
// query_log 在 CLS 未配置时仍注册，调用时返回明确错误（不 crash server）。
func main() {
	if _, err := config.InitConfig(); err != nil {
		log.Fatalf("init config: %v", err)
	}
	sse := flag.String("sse", "", "SSE 监听地址，如 :9100；空则走 stdio")
	flag.Parse()

	ctx := context.Background()
	if *sse != "" {
		log.Printf("[mcp_server] SSE transport on %s", *sse)
		if err := mcp_server.RunSSE(ctx, *sse); err != nil {
			log.Fatalf("sse: %v", err)
		}
	} else {
		log.Println("[mcp_server] stdio transport")
		if err := mcp_server.RunStdio(ctx); err != nil {
			log.Fatalf("stdio: %v", err)
		}
	}
}
