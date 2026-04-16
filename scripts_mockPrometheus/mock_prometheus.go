// mock_prometheus.go — 模拟 Prometheus /api/v1/alerts 接口，用于测试 query_prometheus_alerts 工具。
// 启动: go run scripts/mock_prometheus.go
// 默认监听 :9090，与 conf.yml 中 prometheus.base_url 保持一致。
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
				{
					"labels": map[string]string{
						"alertname": "HighCPUUsage",
						"instance":  "10.0.0.5:9100",
						"job":       "node-exporter",
						"severity":  "warning",
					},
					"annotations": map[string]string{
						"summary":     "实例 CPU 使用率过高",
						"description": "10.0.0.5:9100 上 CPU 使用率已超过 80%，持续 5 分钟",
					},
					"state":    "firing",
					"activeAt": now.Add(-15 * time.Minute).Format(time.RFC3339Nano),
					"value":    "8.5e+01",
				},
				{
					"labels": map[string]string{
						"alertname": "HighMemoryUsage",
						"instance":  "10.0.0.6:9100",
						"job":       "node-exporter",
						"severity":  "critical",
					},
					"annotations": map[string]string{
						"summary":     "实例内存使用率过高",
						"description": "10.0.0.6:9100 上内存使用率已超过 90%，可能导致 OOM",
					},
					"state":    "firing",
					"activeAt": now.Add(-32 * time.Minute).Format(time.RFC3339Nano),
					"value":    "9.3e+01",
				},
				{
					"labels": map[string]string{
						"alertname":  "DiskSpaceRunningLow",
						"instance":   "10.0.0.7:9100",
						"job":        "node-exporter",
						"severity":   "warning",
						"mountpoint": "/data",
					},
					"annotations": map[string]string{
						"summary":     "磁盘空间不足",
						"description": "10.0.0.7:9100 的 /data 分区剩余空间不足 10%",
					},
					"state":    "firing",
					"activeAt": now.Add(-2 * time.Hour).Format(time.RFC3339Nano),
					"value":    "9.1e+01",
				},
				{
					"labels": map[string]string{
						"alertname": "APIHighErrorRate",
						"instance":  "10.0.0.10:8080",
						"job":       "api-server",
						"severity":  "critical",
						"endpoint":  "/api/order/create",
					},
					"annotations": map[string]string{
						"summary":     "接口失败率过高",
						"description": "/api/order/create 接口在过去 5 分钟内错误率达到 15%，超过阈值 5%",
					},
					"state":    "firing",
					"activeAt": now.Add(-8 * time.Minute).Format(time.RFC3339Nano),
					"value":    "1.5e+01",
				},
				{
					"labels": map[string]string{
						"alertname": "ServiceDown",
						"instance":  "10.0.0.12:3306",
						"job":       "mysql-exporter",
						"severity":  "critical",
					},
					"annotations": map[string]string{
						"summary":     "MySQL 服务不可用",
						"description": "10.0.0.12:3306 的 MySQL 实例连续 3 次健康检查失败，服务可能已宕机",
					},
					"state":    "firing",
					"activeAt": now.Add(-3 * time.Minute).Format(time.RFC3339Nano),
					"value":    "0e+00",
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
