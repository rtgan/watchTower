package main

import (
	"context"
	"fmt"
	"watchTower/ai/tools"
	"watchTower/common/config"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func main() {
	config.InitConfig()
	ctx := context.Background()
	// 创建 ChatModel
	config := &openai.ChatModelConfig{
		APIKey:  "4c0b5deb-a081-4a1d-9b34-3ec485e96b40",
		Model:   "deepseek-v3-2-251201",
		BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
	}
	chatModel, err := openai.NewChatModel(ctx, config)
	if err != nil {
		panic(err)
	}
	// 获取工具信息, 用于绑定到 ChatModel
	toolList, _ := tools.GetLogMcpTool()
	toolList = append(toolList, tools.NewGetCurrentTimeTool())
	toolInfos := make([]*schema.ToolInfo, 0)
	var info *schema.ToolInfo
	for _, todoTool := range toolList {
		info, err = todoTool.Info(ctx)
		if err != nil {
			panic(err)
		}
		toolInfos = append(toolInfos, info)
	}

	// 将 tools 绑定到 ChatModel
	err = chatModel.BindTools(toolInfos)
	if err != nil {
		panic(err)
	}

	// 创建一个完整的处理链
	chain := compose.NewChain[[]*schema.Message, *schema.Message]()
	chain.AppendChatModel(chatModel, compose.WithNodeName("chat_model"))

	// 编译为 compose.Runnable，可 Invoke/Stream
	runnable, err := chain.Compile(ctx)
	if err != nil {
		panic(err)
	}
	// 运行示例
	resp, err := runnable.Invoke(ctx, []*schema.Message{
		{
			Role:    schema.User,
			Content: "告诉我你有哪些工具可以使用",
		},
	})
	if err != nil {
		panic(err)
	}
	// 输出结果
	fmt.Println(resp.Content)
}
