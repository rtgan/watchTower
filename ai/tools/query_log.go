package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"watchTower/common/config"

	eino_mcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	tccommon "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cls "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cls/v20201016"
)

// mcpClient 创建腾讯 CLS MCP 的 SSE 客户端（回退路径，需有效 mcp_url token）
func mcpClient(ctx context.Context) (*client.Client, error) {
	cli, err := client.NewSSEMCPClient(config.Conf.McpUrl)
	if err != nil {
		return nil, err
	}
	if err = cli.Start(ctx); err != nil {
		return nil, err
	}
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "cls-mcp-server", Version: "1.0.0"}
	if _, err = cli.Initialize(ctx, initRequest); err != nil {
		return nil, err
	}
	return cli, nil
}

// GetLogMcpTool 返回日志查询工具。
//
// 优先级：
//  1. 若配置了 cls.*（直连），返回基于 CLS SearchLog API 的 query_log 工具——稳定，不依赖 MCP token。
//  2. 否则回退到腾讯 CLS 的 MCP（SSE，需有效 mcp_url token）。
//
// 两路均不可用时返回空切片 + error，调用方（executor）可降级跳过日志工具。
func GetLogMcpTool() ([]einotool.BaseTool, error) {
	// 1. CLS 直连
	if clsConfigured() {
		t, err := NewCLSQueryLogTool()
		if err != nil {
			return nil, err
		}
		return []einotool.BaseTool{t}, nil
	}

	// 2. 回退 MCP
	ctx := context.Background()
	cli, err := mcpClient(ctx)
	if err != nil {
		return []einotool.BaseTool{}, err
	}
	mcpTools, err := eino_mcp.GetTools(ctx, &eino_mcp.Config{Cli: cli})
	if err != nil {
		return []einotool.BaseTool{}, err
	}
	return mcpTools, nil
}

func clsConfigured() bool {
	c := config.Conf.CLS
	return c.SecretID != "" && c.SecretKey != "" && c.TopicID != "" && c.Region != ""
}

// QueryLogInput 日志查询输入
type QueryLogInput struct {
	Query     string `json:"query" jsonschema:"description=日志检索关键字或 CQL 检索语句，例如 panic、response、reconciliation。多个关键字用空格分隔表示 AND。留空表示查询全部日志。"`
	StartTime string `json:"start_time,omitempty" jsonschema:"description=查询起始时间，格式 2006-01-02 15:04:05。不填默认最近1小时"`
	EndTime   string `json:"end_time,omitempty" jsonschema:"description=查询结束时间，格式 2006-01-02 15:04:05。不填默认当前时间"`
	Limit     int64  `json:"limit,omitempty" jsonschema:"description=返回日志条数，默认50，最大1000"`
}

// QueryLogOutput 日志查询输出
type QueryLogOutput struct {
	Success bool       `json:"success" jsonschema:"description=查询是否成功"`
	Total   int        `json:"total" jsonschema:"description=返回的日志条数"`
	Logs    []CLSLogItem `json:"logs,omitempty" jsonschema:"description=命中的日志列表"`
	Query   string     `json:"query,omitempty" jsonschema:"description=实际使用的检索语句"`
	TimeRange string   `json:"time_range,omitempty" jsonschema:"description=实际查询的时间范围"`
	Message string     `json:"message,omitempty" jsonschema:"description=状态说明"`
	Error   string     `json:"error,omitempty" jsonschema:"description=错误信息"`
}

// CLSLogItem 单条日志（解析后的字段）
type CLSLogItem struct {
	Time    string `json:"time"`
	Service string `json:"service,omitempty"`
	Level   string `json:"level,omitempty"`
	Msg     string `json:"msg"`
}

// NewCLSQueryLogTool 创建基于 CLS SearchLog API 的日志查询工具
func NewCLSQueryLogTool() (einotool.BaseTool, error) {
	c := config.Conf.CLS
	cred := tccommon.NewCredential(c.SecretID, c.SecretKey)
	cpf := profile.NewClientProfile()
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = "cls.tencentcloudapi.com"
	}
	cpf.HttpProfile.Endpoint = endpoint
	cli, err := cls.NewClient(cred, c.Region, cpf)
	if err != nil {
		return nil, fmt.Errorf("new cls client: %w", err)
	}

	t, err := utils.InferOptionableTool(
		"query_log",
		"Query logs from Tencent CLS by keyword. Use this to search service logs when investigating alerts. "+
			"Pass the keyword the alert-handling doc prescribes (e.g. 'panic' for ServiceDown, 'response' for APIHighErrorRate, "+
			"'reconciliation' for ReconciliationDiff). Default time range is the last 1 hour.",
		func(ctx context.Context, input *QueryLogInput, opts ...einotool.Option) (string, error) {
			now := time.Now()
			end := now
			start := now.Add(-1 * time.Hour)
			if input.StartTime != "" {
				if pt, e := time.ParseInLocation("2006-01-02 15:04:05", input.StartTime, time.Local); e == nil {
					start = pt
				}
			}
			if input.EndTime != "" {
				if pt, e := time.ParseInLocation("2006-01-02 15:04:05", input.EndTime, time.Local); e == nil {
					end = pt
				}
			}
			limit := input.Limit
			if limit <= 0 || limit > 1000 {
				limit = 50
			}

			req := cls.NewSearchLogRequest()
			req.TopicId = tccommon.StringPtr(c.TopicID)
			req.From = tccommon.Int64Ptr(start.UnixMilli())
			req.To = tccommon.Int64Ptr(end.UnixMilli())
			req.Query = tccommon.StringPtr(input.Query)
			req.Limit = tccommon.Int64Ptr(limit)
			req.Sort = tccommon.StringPtr("desc")

			resp, err := cli.SearchLog(req)
			if err != nil {
				out := QueryLogOutput{Success: false, Query: input.Query, Error: err.Error(),
					Message: "CLS SearchLog failed"}
				b, _ := json.MarshalIndent(out, "", "  ")
				return string(b), nil
			}

			logs := make([]CLSLogItem, 0, len(resp.Response.Results))
			for _, r := range resp.Response.Results {
				if r.LogJson == nil {
					continue
				}
				var m map[string]string
				if e := json.Unmarshal([]byte(*r.LogJson), &m); e == nil {
					msg := m["msg"]
					if msg == "" {
						msg = *r.LogJson
					}
					logs = append(logs, CLSLogItem{
						Time:    m["time"],
						Service: m["service"],
						Level:   m["level"],
						Msg:     msg,
					})
				} else {
					logs = append(logs, CLSLogItem{Msg: *r.LogJson})
				}
			}

			out := QueryLogOutput{
				Success:    true,
				Total:      len(logs),
				Logs:       logs,
				Query:      input.Query,
				TimeRange:  fmt.Sprintf("%s ~ %s", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05")),
				Message:    fmt.Sprintf("retrieved %d logs", len(logs)),
			}
			b, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return "", err
			}
			log.Printf("[query_log] query=%q total=%d", input.Query, len(logs))
			return string(b), nil
		})
	if err != nil {
		return nil, err
	}
	return t, nil
}
