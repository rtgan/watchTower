package supervisor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AlertInfo 从告警快照解析出的单条告警信息（用于 fan-out）。
type AlertInfo struct {
	Name        string // labels.alertname
	Description string // annotations.description
	Severity    string // labels.severity
	Instance    string // labels.instance
}

// parseAlerts 从 query_prometheus_alerts 的返回体解析告警清单。
// 兼容两种格式：
//   - 工具简化格式（真实链路）：{"success":true,"alerts":[{"alert_name","description"}]}
//   - 原始 Prometheus 格式（eval mock）：{"status":"success","data":{"alerts":[{"labels":{"alertname"}}]}}
//
// 去重（按 alertname），与 ai/tools/query_metric_alerts.go 的简化逻辑一致。
func parseAlerts(alertsJSON string) ([]AlertInfo, error) {
	// 1) 先尝试工具简化格式
	var simp struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Alerts  []struct {
			AlertName   string `json:"alert_name"`
			Description string `json:"description"`
		} `json:"alerts"`
	}
	if err := json.Unmarshal([]byte(alertsJSON), &simp); err == nil && simp.Success {
		seen := map[string]bool{}
		var out []AlertInfo
		for _, a := range simp.Alerts {
			if a.AlertName == "" || seen[a.AlertName] {
				continue
			}
			seen[a.AlertName] = true
			out = append(out, AlertInfo{Name: a.AlertName, Description: a.Description})
		}
		return out, nil
	}

	// 2) 回退原始 Prometheus 格式
	var resp struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			Alerts []struct {
				Labels struct {
					AlertName string `json:"alertname"`
					Severity  string `json:"severity"`
					Instance  string `json:"instance"`
				} `json:"labels"`
				Annotations struct {
					Description string `json:"description"`
				} `json:"annotations"`
				State string `json:"state"`
			} `json:"alerts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(alertsJSON), &resp); err != nil {
		return nil, fmt.Errorf("parse alerts: %w", err)
	}
	if resp.Status != "success" {
		if resp.Error != "" {
			return nil, fmt.Errorf("prometheus error: %s", resp.Error)
		}
		return nil, fmt.Errorf("prometheus error: %s", resp.Status)
	}
	seen := map[string]bool{}
	var out []AlertInfo
	for _, a := range resp.Data.Alerts {
		if a.Labels.AlertName == "" || seen[a.Labels.AlertName] {
			continue
		}
		seen[a.Labels.AlertName] = true
		out = append(out, AlertInfo{
			Name:        a.Labels.AlertName,
			Description: a.Annotations.Description,
			Severity:    a.Labels.Severity,
			Instance:    a.Labels.Instance,
		})
	}
	return out, nil
}

// Finding 子 agent 对单条告警的排查结论。
type Finding struct {
	AlertName   string   `json:"alert_name"`
	RootCause   string   `json:"root_cause"`
	Evidence    []string `json:"evidence"`
	Remediation string   `json:"remediation"`
	Error       string   `json:"error,omitempty"` // 子 agent 失败 / 无证据时填
	TraceID     string   `json:"trace_id"`
}

// parseFinding 从子 agent 的最终输出里尽力解析 JSON finding；解析失败则把原文当 root_cause。
func parseFinding(raw, alertName string) Finding {
	f := Finding{AlertName: alertName}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		var payload struct {
			RootCause   string   `json:"root_cause"`
			Evidence    []string `json:"evidence"`
			Remediation string   `json:"remediation"`
		}
		if err := json.Unmarshal([]byte(raw[start:end+1]), &payload); err == nil {
			f.RootCause = payload.RootCause
			f.Evidence = payload.Evidence
			f.Remediation = payload.Remediation
			return f
		}
	}
	// 降级：原文作为根因
	f.RootCause = strings.TrimSpace(raw)
	return f
}
