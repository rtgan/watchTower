package mem

import (
	"context"
	"fmt"
	"strings"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/schema"
)

// summarizeEnabled 是否启用摘要压缩（config.memory.summarize，默认 false）。
func summarizeEnabled() bool {
	return config.Conf != nil && config.Conf.Memory.Summarize
}

// compactHistory 把较早的历史消息压缩成一条摘要，保留最近 keep 条 + 摘要。
//
// 输入 msgs 超过阈值；保留最后 keep 条作为"近期窗口"，其余压缩成摘要。
func compactHistory(ctx context.Context, msgs []*schema.Message, keep int) ([]*schema.Message, error) {
	if keep > len(msgs) {
		keep = len(msgs)
	}
	old := msgs[:len(msgs)-keep]
	recent := msgs[len(msgs)-keep:]

	summary, err := summarizeMessages(ctx, old)
	if err != nil {
		return nil, err
	}
	// 近期窗口仍可能超过 maxWindow，再做一次成对淘汰
	if len(recent) > keep {
		excess := len(recent) - keep
		if excess%2 != 0 {
			excess++
		}
		if excess > len(recent) {
			excess = len(recent)
		}
		recent = recent[excess:]
	}
	return append([]*schema.Message{summary}, recent...), nil
}

// summarizeMessages 用 DsQuick 把一组历史消息压缩成一条 system 摘要消息。
func summarizeMessages(ctx context.Context, old []*schema.Message) (*schema.Message, error) {
	creator := model.GetGlobalFactory().GetModelCreator(model.DsQuickChatModelType)
	if creator == nil {
		return nil, fmt.Errorf("quick model creator not found")
	}
	cm := creator(ctx, config.Conf).(*model.DsQuickChatModel).Model

	var sb strings.Builder
	for _, m := range old {
		sb.WriteString(string(m.Role))
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	msgs := []*schema.Message{
		schema.SystemMessage("你是会话摘要器。把以下历史对话压缩成简洁中文摘要，保留关键事实、用户意图与已做决定，不要丢失重要细节。"),
		schema.UserMessage(sb.String()),
	}
	out, err := cm.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}
	return schema.SystemMessage("【历史会话摘要】" + out.Content), nil
}
