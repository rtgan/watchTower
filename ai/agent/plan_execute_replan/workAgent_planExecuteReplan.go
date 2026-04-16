package plan_execute_replan

import (
	"context"
	"fmt"
	"watchTower/ai/skills"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
)

func BuildPlanExecuteReplanAgent(ctx context.Context, query string) (string, []string, error) {
	if skillBlock := skills.FormatForPrompt(); skillBlock != "" {
		query = skillBlock + "\n" + query //(这里未来可以考虑优化成向量相似度匹配对应的skill)
	}
	planAgent, err := NewPlannerAgent(ctx)
	if err != nil {
		return "", []string{}, err
	}

	executeAgent, err := NewExecutorAgent(ctx)
	if err != nil {
		return "", []string{}, err
	}

	replanAgent, err := NewReplannerAgent(ctx)
	if err != nil {
		return "", []string{}, err
	}

	planExecuteAgent, err := planexecute.New(ctx, &planexecute.Config{ //实际是：先跑`planner`，再loop跑`executor+replanner`
		Planner:       planAgent,    //负责生成计划
		Executor:      executeAgent, //负责执行计划（利用工具）————与下边agent构成循环
		Replanner:     replanAgent,  //负责评估当前执行结果，决定：生成新的计划再重新执行 or 直接输出最终结果
		MaxIterations: 20,
	})
	if err != nil {
		return "", []string{}, fmt.Errorf("build PlanExecuteAgent Error: %v", err)
	}
	r := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent: planExecuteAgent,
	})
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
		}
	}
	if lastMessage == nil {
		return "", []string{}, fmt.Errorf("get lastMessage Error")
	}
	return lastMessage.Content, detail, nil
}
