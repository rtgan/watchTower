package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) *gin.Engine {
	// 健康检查端点（用于 K8s liveness/readiness probes，GET 请求）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	globalRouter := r.Group("/api")
	{
		ChatRouter(globalRouter)
	}
	return r
}
