package tools

import (
	"context"
	"watchTower/common/config"

	eino_mcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// 获取CLS提供的MCP日志相关工具(只是工具.日志数据需接入待观测服务或提前上传一批日志进入CLS)
func GetLogMcpTool() ([]tool.BaseTool, error) {

	ctx := context.Background()
	// 1. 创建MCP客户端(SSE模式。streamable模式见chaoyang_AI的方法CreateMcpClient)
	cli, err := client.NewSSEMCPClient(config.Conf.McpUrl)
	if err != nil {
		return []tool.BaseTool{}, err
	}
	if err = cli.Start(ctx); err != nil {
		return []tool.BaseTool{}, err
	}
	// 2. 初始化MCP客户端
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "cls-mcp-server",
		Version: "1.0.0",
	}
	if _, err = cli.Initialize(ctx, initRequest); err != nil {
		return []tool.BaseTool{}, err
	}
	// 3. 获取CLS提供的MCP日志相关工具(日志数据需要上传到CLS或mock)
	mcpTools, err := eino_mcp.GetTools(ctx, &eino_mcp.Config{Cli: cli})
	if err != nil {
		return []tool.BaseTool{}, err
	}
	return mcpTools, nil
}
