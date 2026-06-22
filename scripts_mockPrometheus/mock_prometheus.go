// mock_prometheus.go — 模拟 Prometheus /api/v1/alerts 接口，用于测试 query_prometheus_alerts 工具。
// 启动: go run scripts_mockPrometheus/mock_prometheus.go
// 默认监听 :9090，与 conf.yml 中 prometheus.base_url 保持一致。
//
// 本 mock 的 6 条告警围绕同一个 incident-20260620 order-service 雪崩事件，构成一条根因链：
//   发布引入 nil pointer bug → panic 重启(ServiceDown) → 内存泄漏 OOM(HighMemoryUsage)
//   → 重启风暴 CPU 飙升(HighCPUUsage) → panic 日志刷爆 /data(DiskSpaceRunningLow)
//   → 下游 payment-service 调用失败(APIHighErrorRate, 错误码 12003) → 失败订单未同步(ReconciliationDiff)
// 与 mock_data/ 下的日志、知识库文档《告警处理手册》《服务错误类型》一一对应。
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/api/v1/alerts", handleAlerts)
	http.HandleFunc("/api/v1/query", handleQuery)

	addr := ":9090"
	fmt.Println("=== Mock Prometheus 已启动 ===")
	fmt.Printf("监听地址: http://localhost%s\n", addr)
	fmt.Printf("告警接口: http://localhost%s/api/v1/alerts\n", addr)
	fmt.Println("按 Ctrl+C 停止")
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleAlerts(w http.ResponseWriter, r *http.Request) {
	log.Printf("[mock] %s %s", r.Method, r.URL.Path)

	now := time.Now().UTC()

	alerts := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"alerts": []map[string]interface{}{
				// —— order-service 雪崩事件：6 条告警构成同一根因链 ——
				{
					"labels": map[string]string{
						"alertname": "ServiceDown",
						"instance":  "10.0.0.10:8080",
						"job":       "order-service",
						"severity":  "critical",
					},
					"annotations": map[string]string{
						"summary":     "order-service 服务不可用",
						"description": "10.0.0.10:8080 的 order-service 连续 3 次健康检查失败，疑似 panic 重启，服务可能已宕机",
					},
					"state":    "firing",
					"activeAt": now.Add(-12 * time.Minute).Format(time.RFC3339Nano),
					"value":    "0e+00",
				},
				{
					"labels": map[string]string{
						"alertname": "APIHighErrorRate",
						"instance":  "10.0.0.10:8080",
						"job":       "order-service",
						"severity":  "critical",
						"endpoint":  "/api/order/create",
					},
					"annotations": map[string]string{
						"summary":     "接口失败率过高",
						"description": "/api/order/create 接口在过去 5 分钟内错误率达到 15%，超过阈值 5%，疑似下游 payment-service 不可用",
					},
					"state":    "firing",
					"activeAt": now.Add(-8 * time.Minute).Format(time.RFC3339Nano),
					"value":    "1.5e+01",
				},
				{
					"labels": map[string]string{
						"alertname": "ReconciliationDiff",
						"instance":  "10.0.0.11:8081",
						"job":       "reconciliation-service",
						"severity":  "warning",
					},
					"annotations": map[string]string{
						"summary":     "与下游对账发现差异",
						"description": "reconciliation-service 发现上游 order 与下游 payment 对账差异 23 笔，疑似失败订单未同步",
					},
					"state":    "firing",
					"activeAt": now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
					"value":    "2.3e+01",
				},
				{
					"labels": map[string]string{
						"alertname": "HighMemoryUsage",
						"instance":  "10.0.0.10:9100",
						"job":       "node-exporter",
						"severity":  "critical",
					},
					"annotations": map[string]string{
						"summary":     "实例内存使用率过高",
						"description": "10.0.0.10:9100 上内存使用率已超过 93%，疑似 order-service 内存泄漏，可能导致 OOM",
					},
					"state":    "firing",
					"activeAt": now.Add(-32 * time.Minute).Format(time.RFC3339Nano),
					"value":    "9.3e+01",
				},
				{
					"labels": map[string]string{
						"alertname": "HighCPUUsage",
						"instance":  "10.0.0.10:9100",
						"job":       "node-exporter",
						"severity":  "warning",
					},
					"annotations": map[string]string{
						"summary":     "实例 CPU 使用率过高",
						"description": "10.0.0.10:9100 上 CPU 使用率已超过 85%，持续 5 分钟，疑似 order-service 重启风暴",
					},
					"state":    "firing",
					"activeAt": now.Add(-15 * time.Minute).Format(time.RFC3339Nano),
					"value":    "8.5e+01",
				},
				{
					"labels": map[string]string{
						"alertname":  "DiskSpaceRunningLow",
						"instance":   "10.0.0.10:9100",
						"job":        "node-exporter",
						"severity":   "warning",
						"mountpoint": "/data",
					},
					"annotations": map[string]string{
						"summary":     "磁盘空间不足",
						"description": "10.0.0.10:9100 的 /data 分区剩余空间不足 10%，疑似 panic 堆栈日志暴涨刷爆磁盘",
					},
					"state":    "firing",
					"activeAt": now.Add(-2 * time.Hour).Format(time.RFC3339Nano),
					"value":    "9.1e+01",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

// handleQuery 提供一个简单的 /api/v1/query 兜底，防止 Agent 探测时 404
func handleQuery(w http.ResponseWriter, r *http.Request) {
	log.Printf("[mock] %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"resultType": "vector",
			"result":     []interface{}{},
		},
	})
}
