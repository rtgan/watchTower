package supervisor

import (
	"context"
	"fmt"
	"strings"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/schema"
)

// synthesize 用 DsThink 把所有子 agent 的 finding 汇总成一份告警分析报告。
//
// 不编造 finding 之外的信息；finding 标注 error 的告警在结论里建议人工介入。
func synthesize(ctx context.Context, findings []*Finding, query string) (string, error) {
	creator := model.GetGlobalFactory().GetModelCreator(model.DsThinkChatModelType)
	if creator == nil {
		return "", fmt.Errorf("think model creator not found")
	}
	cm := creator(ctx, config.Conf).(*model.DsThinkChatModel).Model

	var sb strings.Builder
	for i, f := range findings {
		sb.WriteString(fmt.Sprintf("### 告警 %d: %s\n", i+1, f.AlertName))
		sb.WriteString(fmt.Sprintf("- 根因: %s\n", f.RootCause))
		sb.WriteString(fmt.Sprintf("- 证据: %s\n", strings.Join(f.Evidence, "; ")))
		sb.WriteString(fmt.Sprintf("- 处置: %s\n", f.Remediation))
		if f.Error != "" {
			sb.WriteString(fmt.Sprintf("- 子 agent 状态: %s\n", f.Error))
		}
		sb.WriteString("\n")
	}
	msgs := []*schema.Message{
		schema.SystemMessage("你是告警运维分析报告汇总器。基于各子 agent 的 finding 汇总成一份报告，包含：活跃告警清单、每个告警根因分析、处置方案执行、结论。不编造 finding 之外的信息；子 agent 标注错误的告警在结论里建议人工介入。"),
		schema.UserMessage(fmt.Sprintf("原始任务:\n%s\n\n子 agent findings:\n%s", query, sb.String())),
	}
	out, err := cm.Generate(ctx, msgs)
	if err != nil {
		return "", err
	}
	return out.Content, nil
}
