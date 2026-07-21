package router

import (
	"net/http"
	"strconv"
	"watchTower/common/metrics"
	"watchTower/common/trace"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) *gin.Engine {
	// 健康检查端点（用于 K8s liveness/readiness probes，GET 请求）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Prometheus 抓取端点（agent 自身指标：latency/tool/token/deflection）
	r.GET("/metrics", func(c *gin.Context) {
		metrics.Handler().ServeHTTP(c.Writer, c.Request)
	})

	globalRouter := r.Group("/api")
	globalRouter.Use(metrics.Middleware()) // 记录每个 /api 路由的 latency + run 计数
	{
		ChatRouter(globalRouter)
		TraceRouter(globalRouter)
	}
	return r
}

// TraceRouter 暴露 trace 查询端点（debug agent 失败用）。
//
//	GET /api/traces         -> 最近 N 个 run 摘要（?limit=50）
//	GET /api/traces/:id     -> 单个 run 完整 span 树
func TraceRouter(r *gin.RouterGroup) {
	r.GET("/traces", func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		c.JSON(http.StatusOK, trace.DefaultStore.List(limit))
	})
	r.GET("/traces/:id", func(c *gin.Context) {
		run, err := trace.DefaultStore.Get(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, run)
	})
}
