package router

import (
	"watchTower/controller/chat"

	"github.com/gin-gonic/gin"
)

func ChatRouter(r *gin.RouterGroup) {
	r.POST("/chat", chat.Chat)
}
