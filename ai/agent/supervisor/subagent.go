package supervisor

import (
	"context"
	"fmt"
	"time"
	"watchTower/ai/skills"
	"watchTower/ai/tools"
	"watchTower/common/config"
	"watchTower/common/trace"
	"watchTower/model"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// subAgentMaxStep 单条告警子 agent 的最大步数（有界，避免失控烧 token）。
const subAgentMaxStep = 12

// runSubAgent 对单条告警跑一个 ReAct 子 agent，返回 Finding。
//
//   - 子 agent 自带一个子 recorder（独立 trace，落 traces/<id>.json），父 recorder 记一个指向事件。
//   - 工具子集：log（带恢复 + 埋点）、docs、time；不给 alerts（避免子 agent 重复抓全量告警）。
//   - 工具被 InstrumentedTool 包裹，调用进子 trace + 触发 tool_call_total 指标。
func runSubAgent(ctx context.Context, parent *trace.Recorder, alert AlertInfo, provider tools.ToolProvider) *Finding {
	subRec := trace.NewRecorder("alert:"+alert.Name, time.Now().Format(time.RFC3339Nano))
	subCtx := trace.WithRecorder(ctx, subRec)
	if parent != nil {
		parent.Event(ctx, "subagent:"+alert.Name, "supervisor", map[string]string{
			"severity": alert.Severity, "instance": alert.Instance,
		})
	}

	ts, err := provider.Provide(ctx)
	if err != nil {
		return failFinding(alert, "provide tools: "+err.Error(), subRec)
	}

	var toolList []tool.BaseTool
	// log：先恢复包装，再埋点包装
	for _, t := range ts.Log {
		if it, ok := t.(tool.InvokableTool); ok {
			toolList = append(toolList, NewInstrumentedTool(NewRecoverableLogTool(it), subRec))
		} else {
			toolList = append(toolList, t)
		}
	}
	toolList = append(toolList, instrument(ts.Docs, subRec)...)
	toolList = append(toolList, instrument(ts.Time, subRec)...)
	if len(toolList) == 0 {
		return failFinding(alert, "no tools available", subRec)
	}

	creator := model.GetGlobalFactory().GetModelCreator(model.DsQuickChatModelType)
	if creator == nil {
		return failFinding(alert, "model creator not found", subRec)
	}
	cm := creator(ctx, config.Conf).(*model.DsQuickChatModel).Model

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: cm,
		ToolsConfig:      compose.ToolsNodeConfig{Tools: toolList},
		MaxStep:          subAgentMaxStep,
	})
	if err != nil {
		return failFinding(alert, "new react agent: "+err.Error(), subRec)
	}

	msgs := []*schema.Message{
		schema.SystemMessage(subAgentSystemPrompt(ctx, alert)),
		schema.UserMessage(subAgentUserPrompt(alert)),
	}
	s, subCtx2 := subRec.StartSpan(subCtx, "react-agent", "Agent", alert.Name)
	out, err := agent.Generate(subCtx2, msgs)
	subRec.EndSpan(subCtx2, s, contentOrErr(out, err), err)
	if err != nil {
		run := subRec.Finalize("error", err)
		return &Finding{AlertName: alert.Name, Error: "subagent generate: " + err.Error(), TraceID: run.ID}
	}
	run := subRec.Finalize("ok", nil)
	f := parseFinding(out.Content, alert.Name)
	f.TraceID = run.ID
	if f.RootCause == "" && len(f.Evidence) == 0 {
		f.Error = "无证据（日志/文档未命中），建议人工排查或扩大检索范围"
	}
	return &f
}

func contentOrErr(m *schema.Message, err error) string {
	if err != nil {
		return err.Error()
	}
	if m == nil {
		return ""
	}
	return m.Content
}

func failFinding(alert AlertInfo, msg string, rec *trace.Recorder) *Finding {
	run := rec.Finalize("error", fmt.Errorf("%s", msg))
	return &Finding{AlertName: alert.Name, Error: msg, TraceID: run.ID}
}

func subAgentSystemPrompt(ctx context.Context, alert AlertInfo) string {
	skillBlock := skills.RouteForPrompt(ctx, alert.Name+" "+alert.Description)
	return fmt.Sprintf(`你是告警排查子 agent，负责排查单条告警：%s。
严格遵循内部文档与日志证据，不得编造。步骤：
1. 调用 get_current_time 获取当前时间。
2. 调用 query_internal_docs 查询告警 "%s" 的处理方案与应搜关键字。
3. 按文档规定的关键字调用 query_log 搜日志。
4. 若首次无日志，工具会自动扩大时间窗重试一次。
5. 仅基于文档与日志输出结论，最后输出 JSON：{"root_cause":"...","evidence":["..."],"remediation":"..."}。

%s`, alert.Name, alert.Name, skillBlock)
}

func subAgentUserPrompt(alert AlertInfo) string {
	return fmt.Sprintf(`告警：%s（severity=%s, instance=%s）
描述：%s
请排查并输出 JSON finding。`, alert.Name, alert.Severity, alert.Instance, alert.Description)
}
