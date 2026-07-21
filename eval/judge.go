package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"watchTower/common/config"
	"watchTower/model"

	"github.com/cloudwego/eino/schema"
)

// JudgeResult LLM-as-judge 打分结果。总分 0-100。
type JudgeResult struct {
	Score                float64 `json:"score"`                  // 总分（各分项之和）
	RootCause            float64 `json:"root_cause"`             // 根因正确性 0-30
	Completeness         float64 `json:"completeness"`           // 完整性 0-30
	Remediation          float64 `json:"remediation"`            // 处置可执行性 0-20
	HallucinationFreedom float64 `json:"hallucination_freedom"`  // 是否未使用文档外信息 0-20（20=无幻觉）
	Reason               string  `json:"reason"`
	Error                string  `json:"error,omitempty"`
}

const judgeSystemPrompt = `你是一个严格的 AIOps 告警分析报告评审员。

被评审的 agent 在系统提示中被明确要求："完全遵循内部文档的内容进行查询和分析，不允许使用文档外的任何信息。"
因此你需要特别检查报告是否使用了文档之外的信息（幻觉）。

评分维度（各项满分见括号）：
- root_cause (0-30)：是否正确定位根因。
- completeness (0-30)：是否覆盖所有活跃告警并给出证据链。
- remediation (0-20)：处置策略是否具体可执行。
- hallucination_freedom (0-20)：是否仅基于提供的内部文档作答。报告出现文档未支撑的断言时扣分。

只输出一个 JSON 对象，不要任何额外文字或 markdown 代码块：
{"root_cause":0,"completeness":0,"remediation":0,"hallucination_freedom":0,"reason":"简短中文说明"}`

// Judge 用 DsThink 模型对报告打分。
//
// 失败不致命：若模型不可用或解析失败，返回带 Error 的 JudgeResult（Score=0），不中断评估。
// 但注意：runner 的 Total 公式按 30*(Score/100) 计入该项，失败/关闭时该项贡献 0（不做排除重归一化），
// Total 上限降到 70 < 75，会使 Pass 恒为 false。后续如需"失败时该项不参与"，需在 runner 侧重归一化。
func Judge(ctx context.Context, report string, s *Scenario) JudgeResult {
	creator := model.GetGlobalFactory().GetModelCreator(model.DsThinkChatModelType)
	if creator == nil {
		return JudgeResult{Error: "judge model creator not found"}
	}
	cm := creator(ctx, config.Conf).(*model.DsThinkChatModel).Model
	if cm == nil {
		return JudgeResult{Error: "judge model nil"}
	}

	// 把内部文档内容拼给 judge，用于幻觉比对
	var docsSB strings.Builder
	for i, d := range s.Docs {
		docsSB.WriteString(fmt.Sprintf("--- 文档 %d (%s) ---\n%s\n\n", i+1, strings.Join(d.AlertNames, "/"), d.Text))
	}

	userMsg := fmt.Sprintf(`【内部文档（agent 应仅基于此作答）】
%s

【期望报告要点（golden，非逐字比对）】
%s

【agent 实际产出的报告】
%s

请按评分维度输出 JSON。`,
		docsSB.String(), s.Expected.GoldenReport, report)

	msgs := []*schema.Message{
		schema.SystemMessage(judgeSystemPrompt),
		schema.UserMessage(userMsg),
	}
	out, err := cm.Generate(ctx, msgs)
	if err != nil {
		return JudgeResult{Error: "judge generate: " + err.Error()}
	}
	return parseJudge(out.Content)
}

func parseJudge(raw string) JudgeResult {
	res := JudgeResult{}
	cleaned := stripCodeFence(raw)
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start < 0 || end <= start {
		return JudgeResult{Error: "judge output not json: " + truncate(raw, 200)}
	}
	var payload struct {
		RootCause            *float64 `json:"root_cause"`
		Completeness         *float64 `json:"completeness"`
		Remediation          *float64 `json:"remediation"`
		HallucinationFreedom *float64 `json:"hallucination_freedom"`
		Reason               string   `json:"reason"`
	}
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &payload); err != nil {
		return JudgeResult{Error: "judge parse: " + err.Error()}
	}
	if payload.RootCause != nil {
		res.RootCause = clamp(*payload.RootCause, 0, 30)
	}
	if payload.Completeness != nil {
		res.Completeness = clamp(*payload.Completeness, 0, 30)
	}
	if payload.Remediation != nil {
		res.Remediation = clamp(*payload.Remediation, 0, 20)
	}
	if payload.HallucinationFreedom != nil {
		res.HallucinationFreedom = clamp(*payload.HallucinationFreedom, 0, 20)
	}
	res.Score = res.RootCause + res.Completeness + res.Remediation + res.HallucinationFreedom
	res.Reason = payload.Reason
	return res
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
