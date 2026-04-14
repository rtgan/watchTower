package router

import "github.com/gin-gonic/gin"

func InitRouter() *gin.Engine {
	r := gin.Default()
	globalRouter := r.Group("/api")
	{
		ChatRouter(globalRouter.Group("/chat"))
	}
	return r
}
