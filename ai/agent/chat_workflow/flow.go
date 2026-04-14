package chat_workflow

import (
	"context"
	"watchTower/ai/tools"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
)

// newReactAgentLambda component initialization function of node 'ReactAgent' in graph 'EinoAgent'
func newReactAgentLambda(ctx context.Context) (lba *compose.Lambda, err error) {
	// TODO Modify component configuration here.
	config := &react.AgentConfig{
		MaxStep:            25,
		ToolReturnDirectly: map[string]struct{}{}}
	chatModelIns11, err := newChatModel(ctx)
	if err != nil {
		return nil, err
	}
	config.ToolCallingModel = chatModelIns11
	toolIns21, err := newTool(ctx) //duckduckgo工具
	if err != nil {
		return nil, err
	}
	toolIns22, err := newTool1(ctx)
	if err != nil {
		return nil, err
	}
	toolIns23, err := newTool2(ctx)
	if err != nil {
		return nil, err
	}
	toolIns24, err := newTool3(ctx) //googlesearch工具
	if err != nil {
		return nil, err
	}
	config.ToolsConfig.Tools = []tool.BaseTool{toolIns21, toolIns22, toolIns23, toolIns24}

	//除了上述工具，再额外增加下所需的：CLS日志查询工具、Prometheus告警查询工具、Mysql-Crud工具、当前时间工具、内部文档查询工具
	mcpTool, err := tools.GetLogMcpTool()
	if err != nil {
		return nil, err
	}
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, mcpTool...)
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewPrometheusAlertsQueryTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewMysqlCrudTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewGetCurrentTimeTool())
	config.ToolsConfig.Tools = append(config.ToolsConfig.Tools, tools.NewQueryInternalDocsTool())

	ins, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, err
	}
	lba, err = compose.AnyLambda(ins.Generate, ins.Stream, nil, nil) //Agent接了「普通输出」和「流式输出」两种能力
	if err != nil {
		return nil, err
	}
	return lba, nil
}
