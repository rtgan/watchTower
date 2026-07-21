package plan_execute_replan

import (
	"context"
	"fmt"
	"watchTower/ai/skills"
	"watchTower/ai/tools"
	"watchTower/common/trace"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

// BuildPlanExecuteReplanAgent 真实工具链路（保持原行为）。内部转调 WithProvider。
func BuildPlanExecuteReplanAgent(ctx context.Context, query string) (string, []string, error) {
	return BuildPlanExecuteReplanAgentWithProvider(ctx, query, tools.RealProvider{})
}

// BuildPlanExecuteReplanAgentWithProvider 可注入工具来源。
//   - 真实链路：传 tools.RealProvider{}
//   - eval：传 tools.MockProvider（确定性 mock 工具）
//   - supervisor 复用同一套 plan-execute-replan 骨架时可注入裁剪后的工具子集
func BuildPlanExecuteReplanAgentWithProvider(ctx context.Context, query string, provider tools.ToolProvider) (string, []string, error) {
	if skillBlock := skills.RouteForPrompt(ctx, query); skillBlock != "" {
		query = skillBlock + "\n" + query // 向量路由 top-K skill 注入（embedder 不可用时 RouteForPrompt 降级全量）
	}
	planAgent, err := NewPlannerAgent(ctx)
	if err != nil {
		return "", []string{}, err
	}

	executeAgent, err := NewExecutorAgent(ctx, provider)
	if err != nil {
		return "", []string{}, err
	}

	replanAgent, err := NewReplannerAgent(ctx)
	if err != nil {
		return "", []string{}, err
	}

	planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{ //实际是：先跑`planner`，再loop跑`executor+replanner`
		Planner:       planAgent,    //负责生成计划
		Executor:      executeAgent, //负责执行计划（利用工具）----与下边agent构成循环
		Replanner:     replanAgent,  //负责评估当前执行结果，决定：生成新的计划再重新执行 or 直接输出最终结果
		MaxIterations: 20,
	})
	if err != nil {
		return "", []string{}, fmt.Errorf("build PlanExecuteAgent Error: %v", err)
	}
	r := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: planExecuteAgent,
	})
	rec := trace.FromContext(ctx) // 可选：若调用方注入 Recorder，捕获 adk 事件流到 trace
	iter := r.Query(ctx, query)
	var lastMessage adk.Message
	var detail []string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		fmt.Println("------------- Event -------------")
		prints.Event(event) //把这一步的event打印到控制台上
		if event.Output != nil {
			lastMessage, _, err = adk.GetMessage(event)
			detail = append(detail, lastMessage.String())
			if rec != nil {
				rec.Event(ctx, "adk-event", "planexecute", map[string]string{
					"msg": truncStr(lastMessage.String(), 500),
				})
			}
		}
	}
	if lastMessage == nil {
		return "", []string{}, fmt.Errorf("get lastMessage Error")
	}
	return lastMessage.Content, detail, nil
}

// truncStr 截断字符串，避免单个 adk 事件撑爆 trace。
func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
