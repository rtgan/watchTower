package plan_execute_replan

import (
	"context"
	"fmt"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

// 负责生成计划
func NewPlannerAgent(ctx context.Context) (adk.Agent, error) {
	creator := model.GetGlobalFactory().GetModelCreator(model.DsThinkChatModelType)
	if creator == nil {
		return nil, fmt.Errorf("chat model creator not found")
	}
	planModel := creator(ctx, config.Conf).(*model.DsThinkChatModel).Model

	return planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{ //会默认带上计划生成工具(PlanToolInfo)
		ToolCallingChatModel: planModel,
	})
}
