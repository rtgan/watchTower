package router

import (
	"watchTower/controller/chat"

	"github.com/gin-gonic/gin"
)

func ChatRouter(r *gin.RouterGroup) {
	r.POST("/chat", chat.Chat)
	r.POST("/chat-stream", chat.ChatStream)
	r.POST("/ai-ops", chat.AIOps)
	r.POST("/upload", chat.FileUpload)
}
