package tools

import (
	"context"
	"testing"
	"watchTower/common/config"

	"github.com/bytedance/sonic"
)

func TestGetCurrentTime(t *testing.T) {
	timeTool := NewGetCurrentTimeTool()
	result, err := timeTool.InvokableRun(context.Background(), "{}")
	if err != nil {
		t.Fatalf("get current time: %v", err)
	}
	t.Logf("current time: %v", result)
}

func TestMysqlCrud(t *testing.T) {
	mysqlTool := NewMysqlCrudTool()
	input := &MysqlCrudInput{
		DSN:         "root:IfuckQQ77!@tcp(127.0.0.1:3306)/mscoin?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai",
		SQL:         "SELECT * FROM member",
		OperateType: "query",
	}
	json, err := sonic.MarshalString(input)
	if err != nil {
		t.Fatalf("mysql crud: %v", err)
	}
	result, err := mysqlTool.InvokableRun(context.Background(), json)
	if err != nil {
		t.Fatalf("mysql crud: %v", err)
	}
	t.Logf("mysql crud: %v", result)
}

func TestQueryInternalDocs(t *testing.T) {
	config.InitConfig() // 先加载配置
	internalDocsTool := NewQueryInternalDocsTool()
	input := &QueryInternalDocsInput{
		Query: "12001是什么错误码",
	}
	json, err := sonic.MarshalString(input)
	if err != nil {
		t.Fatalf("query internal docs: %v", err)
	}
	result, err := internalDocsTool.InvokableRun(context.Background(), json)
	if err != nil {
		t.Fatalf("query internal docs: %v", err)
	}
	t.Logf("query internal docs: %v", result)
}

func TestGetLogMcpTool(t *testing.T) {
	config.InitConfig() // 先加载配置
	logMcpTools, err := GetLogMcpTool()
	if err != nil {
		t.Fatalf("get log mcp tool: %v", err)
	}
	for _, tool := range logMcpTools {
		t.Logf("log mcp tool: %v", tool)
	}
}

func TestQueryPrometheusAlerts(t *testing.T) {
	config.InitConfig() // 先加载配置
	prometheusAlertsTool := NewPrometheusAlertsQueryTool()
	result, err := prometheusAlertsTool.InvokableRun(context.Background(), "{}")
	if err != nil {
		t.Fatalf("query prometheus alerts: %v", err)
	}
	t.Logf("query prometheus alerts: %v", result)
}
