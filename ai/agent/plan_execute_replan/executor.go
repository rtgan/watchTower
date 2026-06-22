package plan_execute_replan

import (
	"context"
	"fmt"
	"log"
	"watchTower/ai/tools"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// 负责执行计划（利用工具）
func NewExecutorAgent(ctx context.Context) (adk.Agent, error) {
	// log：走腾讯 CLS MCP。本地无 CLS / token 失效时降级跳过日志工具，不阻塞整个 Agent。
	// （告警 + 文档链路仍可独立验证；真实 CLS 配好后日志链路自动恢复）
	toolList := []tool.BaseTool{}
	mcpTool, err := tools.GetLogMcpTool()
	if err != nil {
		log.Printf("[executor] CLS MCP 日志工具不可用，已降级跳过 query_log: %v", err)
	} else {
		toolList = append(toolList, mcpTool...)
	}
	// alerts
	toolList = append(toolList, tools.NewPrometheusAlertsQueryTool())
	// file
	toolList = append(toolList, tools.NewQueryInternalDocsTool())
	// time
	toolList = append(toolList, tools.NewGetCurrentTimeTool())

	// 创建模型
	creator := model.GetGlobalFactory().GetModelCreator(model.DsQuickChatModelType)
	if creator == nil {
		return nil, fmt.Errorf("chat model creator not found")
	}
	execModel := creator(ctx, config.Conf).(*model.DsQuickChatModel).Model

	return planexecute.NewExecutor(ctx, &planexecute.ExecutorConfig{
		Model: execModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: toolList,
			},
		},
		MaxIterations: 999999,
	})
}
