package main

import (
	"context"
	"fmt"
	"watchTower/ai/tools"
	conf "watchTower/common/config"
	logcallback "watchTower/common/log_callback"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

func main() {
	conf.InitConfig()
	ctx := context.Background()
	// 创建 ChatModel（密钥从配置读取，避免硬编码）
	cfg := &openai.ChatModelConfig{
		APIKey:  conf.Conf.DsThinkChatModel.ApiKey,
		Model:   conf.Conf.DsThinkChatModel.Model,
		BaseURL: conf.Conf.DsThinkChatModel.BaseUrl,
	}
	chatModel, err := openai.NewChatModel(ctx, cfg)
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
	resp, err := runnable.Invoke(
		ctx,
		[]*schema.Message{
			{
				Role:    schema.User,
				Content: "告诉我你有哪些工具可以使用",
			},
		},
		compose.WithCallbacks(logcallback.LogCallback(&conf.Conf.LogCallback)),
	)
	if err != nil {
		panic(err)
	}
	// 输出结果
	fmt.Println(resp.Content)
}
