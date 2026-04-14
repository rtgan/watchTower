package chat_workflow

import (
	"context"
	"fmt"
	"watchTower/common/config"
	m "watchTower/model"

	"github.com/cloudwego/eino/components/model"
)

func newChatModel(ctx context.Context) (cm model.ToolCallingChatModel, err error) {
	creator := m.GetGlobalFactory().GetModelCreator(m.DsThinkChatModelType)
	if creator == nil {
		return nil, fmt.Errorf("chat model creator not found")
	}
	cm = creator(ctx, config.Conf).(*m.DsThinkChatModel).Model
	return cm, nil
}
