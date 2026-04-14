package plan_execute_replan

import (
	"context"
	"fmt"
	"watchTower/ai/tools"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/compose"
)

// 负责执行计划（利用工具）
func NewExecutorAgent(ctx context.Context) (adk.Agent, error) {
	// log
	mcpTool, err := tools.GetLogMcpTool()
	if err != nil {
		return nil, err
	}
	toolList := mcpTool
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
