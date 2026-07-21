package plan_execute_replan

import (
	"context"
	"fmt"
	"watchTower/ai/tools"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// maxExecutorIterations 单个 Executor 步内部 ReAct 循环的迭代上限。
//
// 原值为 999999（实际无界），存在生产风险：模型陷入"反复调用工具但不收敛"时会一直烧 token
// 直到外层 planexecute 的 MaxIterations 或 context 超时。改为有界上界后，单步内不会失控；
// 外层 planexecute.New 的 MaxIterations(20) 仍兜底整个 plan-execute-replan 循环。
const maxExecutorIterations = 10

// 负责执行计划（利用工具）
//
// provider 决定工具来源：真实链路传 tools.RealProvider{}，eval 注入 tools.MockProvider。
func NewExecutorAgent(ctx context.Context, provider tools.ToolProvider) (adk.Agent, error) {
	var toolList []tool.BaseTool
	if provider == nil {
		provider = tools.RealProvider{}
	}
	ts, err := provider.Provide(ctx)
	if err != nil {
		return nil, fmt.Errorf("executor provide tools: %w", err)
	}
	toolList = append(toolList, ts.All()...)

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
		MaxIterations: maxExecutorIterations,
	})
}
