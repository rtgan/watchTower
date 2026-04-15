package chat

import (
	"context"
	"log"
	"net/http"
	"time"
	"watchTower/ai/agent/chat_workflow"
	logcallback "watchTower/common/log_callback"
	"watchTower/mem"
	"watchTower/model/vo"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

func Chat(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Minute)
	defer cancel()
	var req vo.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userMessage := &chat_workflow.UserMessage{
		ID:      req.Id,
		Query:   req.Question,
		History: mem.GetSimpleMemory(req.Id).GetMessages(),
	}
	runner, err := chat_workflow.BuildChatAgent(ctx)
	if err != nil {
		log.Printf("ERROR: BuildChatAgent failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(logcallback.LogCallback(nil)))
	if err != nil {
		log.Printf("ERROR: Chat Invoke failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	res := &vo.ChatRes{
		Answer: out.Content,
	}
	mem.GetSimpleMemory(req.Id).SetMessages(schema.UserMessage(req.Question))
	mem.GetSimpleMemory(req.Id).SetMessages(schema.AssistantMessage(out.Content, nil))

	c.JSON(http.StatusOK, res)
}
