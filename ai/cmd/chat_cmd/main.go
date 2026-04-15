package main

import (
	"context"
	"fmt"
	"watchTower/ai/agent/chat_workflow"
	"watchTower/common/config"
	"watchTower/mem"

	"github.com/cloudwego/eino/schema"
)

func main() {
	config.InitConfig()
	ctx := context.Background()
	id := "111"
	userMessage := &chat_workflow.UserMessage{
		ID:      id,
		Query:   "你好",
		History: mem.GetSimpleMemory(id).GetMessages(),
	}
	runner, err := chat_workflow.BuildChatAgent(ctx)
	if err != nil {
		panic(err)
	}

	// 第一次对话
	out, err := runner.Invoke(ctx, userMessage)
	if err != nil {
		panic(err)
	}
	answer := out.Content
	fmt.Println("Q: 你好")
	fmt.Println("A:", answer)
	// 保存对话历史
	mem.GetSimpleMemory(id).SetMessages(schema.UserMessage("你好"))
	mem.GetSimpleMemory(id).SetMessages(schema.SystemMessage(out.Content))

	// 第二次对话
	userMessage = &chat_workflow.UserMessage{
		ID:      id,
		Query:   "服务下线是什么原因",
		History: mem.GetSimpleMemory(id).GetMessages(),
	}
	out, err = runner.Invoke(ctx, userMessage)
	if err != nil {
		panic(err)
	}
	answer = out.Content
	fmt.Println("----------------")
	fmt.Println("Q: 服务下线是什么原因")
	fmt.Println("A:", answer)
}
