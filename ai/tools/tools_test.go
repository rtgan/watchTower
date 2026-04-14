package tools

import (
	"context"
	"testing"

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
