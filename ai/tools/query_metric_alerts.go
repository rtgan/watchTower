package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
	"watchTower/common/config"
	myutils "watchTower/common/utils"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// PrometheusAlert 对应 Prometheus HTTP API GET /api/v1/alerts 返回的 data.alerts[] 中单个告警。
//
// 示例（节选其中的单个）：
//
//	{
//	  "labels": {
//	    "alertname": "HighCPUUsage",
//	    "instance": "10.0.0.5:9100",
//	    "job": "node-exporter",
//	    "severity": "warning"
//	  },
//	  "annotations": {
//	    "summary": "实例 CPU 使用率过高",
//	    "description": "10.0.0.5:9100 上 CPU 使用率已超过 80% 持续 5 分钟"
//	  },
//	  "state": "firing",
//	  "activeAt": "2026-04-14T08:30:00.123456789Z",
//	  "value": "1e+00"
//	}
type PrometheusAlert struct {
	// Labels 告警标签：用于唯一标识与路由告警的键值对，常见键有 alertname、instance、job、severity 等。
	Labels map[string]string `json:"labels"`
	// Annotations 注解：给人看的说明文字，常见键有 summary、description（本工具里 description 会映射到简化输出）。
	Annotations map[string]string `json:"annotations"`
	// State 告警状态：firing（已触发并持续满足规则）、pending（满足条件但未到 for 时长）、inactive（未激活）。
	State string `json:"state"`
	// ActiveAt 告警进入当前状态的时间，一般为 RFC3339 / RFC3339Nano 字符串，例如 "2026-04-14T08:30:00.123456789Z"。
	ActiveAt string `json:"activeAt"`
	// Value 告警表达式当前采样值，Prometheus 常以科学计数法字符串形式给出，例如 "1e+00" 表示 1；部分环境可能为空。
	Value string `json:"value"`
}

// PrometheusAlertsResult 对应 GET /api/v1/alerts 的完整 JSON 响应体。
//
// 成功示例：
//
//	{ "status": "success", "data": { "alerts": [ /* PrometheusAlert ... */ ] } }
//
// 失败示例：
//
//	{ "status": "error", "error": "bad_data: ...", "errorType": "bad_data" }
type PrometheusAlertsResult struct {
	// API 整体状态：success 表示 data 有效；error 时表示请求失败，应看 Error / ErrorType。
	Status string `json:"status"`
	Data   struct {
		Alerts []PrometheusAlert `json:"alerts"` // 返回的告警列表
	} `json:"data"`
	// 当status为error时的错误说明文本。
	Error string `json:"error,omitempty"`
	// 错误类型，例如 bad_data、timeout 等，具体以 Prometheus 版本为准。
	ErrorType string `json:"errorType,omitempty"`
}

// SimplifiedAlert 简化的告警信息
type SimplifiedAlert struct {
	AlertName   string `json:"alert_name" jsonschema:"description=告警名称，从 Prometheus 告警的 labels.alertname 字段提取"`
	Description string `json:"description" jsonschema:"description=告警描述信息，从 Prometheus 告警的 annotations.description 字段提取"`
	State       string `json:"state" jsonschema:"description=告警状态，通常为 'firing'（触发中）或 'pending'（待触发）"`
	ActiveAt    string `json:"active_at" jsonschema:"description=告警激活时间，RFC3339 格式的时间戳，例如 '2025-10-29T08:48:42.496134755Z'"`
	Duration    string `json:"duration" jsonschema:"description=告警持续时间，从激活时间到当前时间的时长，格式如 '2h30m15s'、'30m15s' 或 '15s'"`
}

// PrometheusAlertsOutput 告警查询输出
type PrometheusAlertsOutput struct {
	Success bool              `json:"success" jsonschema:"description=查询是否成功"`
	Alerts  []SimplifiedAlert `json:"alerts,omitempty" jsonschema:"description=活动告警列表，每个告警包含名称、描述、状态、激活时间和持续时间。相同 alertname 的告警只保留第一个"`
	Message string            `json:"message,omitempty" jsonschema:"description=操作结果的状态消息"`
	Error   string            `json:"error,omitempty" jsonschema:"description=如果查询失败，包含错误信息"`
}

// prometheusBaseURL 从配置 prometheus.base_url 读取，缺省 http://127.0.0.1:9090（不含末尾 /）
func prometheusBaseURL() string {
	v := config.Conf.Prometheus.BaseUrl
	if v == "" {
		return "http://127.0.0.1:9090"
	}
	return strings.TrimRight(v, "/")
}

// queryPrometheusAlerts 查询Prometheus告警（GET {base}/api/v1/alerts）
func queryPrometheusAlerts() (PrometheusAlertsResult, error) {
	baseURL := prometheusBaseURL()
	apiURL := fmt.Sprintf("%s/api/v1/alerts", baseURL)
	log.Printf("Querying Prometheus alerts: %s", apiURL)

	// 查询Prometheus告警
	var result PrometheusAlertsResult
	body, err := myutils.GetWithHeader(apiURL, nil, "")
	if err != nil {
		return result, fmt.Errorf("failed to query Prometheus alerts: %v", err)
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("failed to parse response: %v", err)
	}

	return result, nil
}

// calculateDuration 计算从 activeAt 到现在的持续时间
func calculateDuration(activeAtStr string) string {
	// 解析 RFC3339 格式的时间
	activeAt, err := time.Parse(time.RFC3339Nano, activeAtStr)
	if err != nil {
		return "unknown"
	}

	// 计算持续时间
	duration := time.Since(activeAt)

	// 格式化持续时间
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// NewPrometheusAlertsQueryTool 创建Prometheus告警查询工具
func NewPrometheusAlertsQueryTool() tool.InvokableTool {
	t, err := utils.InferOptionableTool(
		"query_prometheus_alerts",
		"Query active alerts from Prometheus alerting system. This tool retrieves all currently active/firing alerts including their labels, annotations, state, and values. Use this tool when you need to check what alerts are currently firing, investigate alert conditions, or monitor alert status.",
		func(ctx context.Context, input *struct{}, opts ...tool.Option) (output string, err error) {
			log.Printf("Querying Prometheus active alerts")

			// 调用Prometheus Alerts API
			result, err := queryPrometheusAlerts()
			if err != nil {
				alertsOut := PrometheusAlertsOutput{
					Success: false,
					Error:   err.Error(),
					Message: "Failed to query Prometheus alerts",
				}
				jsonBytes, _ := json.MarshalIndent(alertsOut, "", "  ") //表示生成的json有缩进，易读性更好
				return string(jsonBytes), err
			}

			// 转换为简化格式，对于相同的 alertname，只保留第一个
			seenAlertNames := make(map[string]bool)
			simplifiedAlerts := make([]SimplifiedAlert, 0)
			for _, alert := range result.Data.Alerts {
				alertName := alert.Labels["alertname"]

				// 如果这个 alertname 已经存在，跳过
				if seenAlertNames[alertName] {
					continue
				}

				// 标记为已见过
				seenAlertNames[alertName] = true

				simplified := SimplifiedAlert{
					AlertName:   alertName,
					Description: alert.Annotations["description"],
					State:       alert.State,
					ActiveAt:    alert.ActiveAt,
					Duration:    calculateDuration(alert.ActiveAt),
				}
				simplifiedAlerts = append(simplifiedAlerts, simplified)
			}

			// 构建成功响应
			alertsOut := PrometheusAlertsOutput{
				Success: true,
				Alerts:  simplifiedAlerts,
				Message: fmt.Sprintf("Successfully retrieved %d active alerts", len(simplifiedAlerts)),
			}

			// 转换为JSON
			jsonBytes, err := json.MarshalIndent(alertsOut, "", "  ")
			if err != nil {
				log.Printf("Error marshaling alerts result to JSON: %v", err)
				return "", err
			}

			log.Printf("Prometheus alerts query completed: %d alerts found", len(simplifiedAlerts))
			return string(jsonBytes), nil
		})
	if err != nil {
		log.Fatal(err)
	}
	return t
}
