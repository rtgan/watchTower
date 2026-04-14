package plan_execute_replan

import (
	"context"
	"fmt"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

// 负责评估当前执行结果，决定：生成新的计划再重新执行 or 直接输出最终结果
func NewReplannerAgent(ctx context.Context) (adk.Agent, error) {
	creator := model.GetGlobalFactory().GetModelCreator(model.DsThinkChatModelType)
	if creator == nil {
		return nil, fmt.Errorf("chat model creator not found")
	}
	replanModel := creator(ctx, config.Conf).(*model.DsThinkChatModel).Model
	return planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel: replanModel,
	})
}
