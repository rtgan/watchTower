package router

import "github.com/gin-gonic/gin"

func InitRouter(r *gin.Engine) *gin.Engine {

	globalRouter := r.Group("/api")
	{
		ChatRouter(globalRouter)
	}
	return r
}
