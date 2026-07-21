package metrics

import (
	"net/http"
	"strconv"
	"time"
	"watchTower/common/trace"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// agent 自身的可观测指标（区别于作为数据源的 Prometheus）。
//   - latency / run 总数：HTTP 中间件可靠采集
//   - 工具调用 / token：订阅 trace 的 span 结束钩子采集（best-effort，依赖 eino 回调 Component 值）
//   - deflection：eval / 人工标注显式调用 RecordDeflection
var (
	RunDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agent_run_duration_seconds",
		Help:    "Agent 单次运行耗时（按端点）",
		Buckets: prometheus.DefBuckets,
	}, []string{"endpoint"})

	AgentRunTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "agent_run_total",
		Help: "Agent 运行次数（按端点 + 状态）",
	}, []string{"endpoint", "status"})

	ToolCallTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tool_call_total",
		Help: "工具调用次数（按工具 + 状态 success/fail）",
	}, []string{"tool", "status"})

	LLMTokensTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_tokens_total",
		Help: "LLM token 用量（按模型 + 类型 prompt/completion）",
	}, []string{"model", "type"})

	DeflectionObserved = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "deflection_observed",
		Help: "告警自动解决(deflection)观测：resolved=自动闭环 / escalated=需人工介入",
	}, []string{"result"})
)

// Register 注册所有指标 + 订阅 trace span 钩子（main 启动期调用一次）。
func Register() {
	prometheus.MustRegister(RunDuration, AgentRunTotal, ToolCallTotal, LLMTokensTotal, DeflectionObserved)
	trace.RegisterSpanHook(spanHook)
}

// spanHook 订阅 trace：工具/模型 span -> 指标。
func spanHook(s *trace.Span) {
	if s == nil {
		return
	}
	switch s.Component {
	case "Tool":
		status := "success"
		if s.Error != "" {
			status = "fail"
		}
		ToolCallTotal.WithLabelValues(orDefault(s.Name, "unknown"), status).Inc()
	case "ChatModel":
		if s.Attrs == nil {
			return
		}
		m := orDefault(s.Attrs["model"], "default")
		addTokens(m, "prompt", s.Attrs["prompt_tokens"])
		addTokens(m, "completion", s.Attrs["completion_tokens"])
	}
}

func addTokens(model, typ, val string) {
	if val == "" {
		return
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return
	}
	LLMTokensTotal.WithLabelValues(model, typ).Add(float64(n))
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// Middleware gin 中间件：按路由模式记 latency + run 计数。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}
		RunDuration.WithLabelValues(endpoint).Observe(time.Since(start).Seconds())
		status := "ok"
		if c.Writer.Status() >= 500 {
			status = "error"
		}
		AgentRunTotal.WithLabelValues(endpoint, status).Inc()
	}
}

// Handler Prometheus 抓取端点（/metrics）。
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordDeflection 记录一次 deflection 观测（resolved/escalated）。
// eval 或人工标注闭环时调用。
func RecordDeflection(result string) {
	DeflectionObserved.WithLabelValues(orDefault(result, "unknown")).Inc()
}
