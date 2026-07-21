package chat

import (
	"context"
	"log"
	"net/http"
	"time"
	"watchTower/ai/agent/chat_workflow"
	logcallback "watchTower/common/log_callback"
	"watchTower/common/trace"
	"watchTower/mem"
	"watchTower/model/vo"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

func Chat(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()
	var req vo.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 加载历史：若长期记忆启用且带 UserId，注入语义召回记忆
	var history []*schema.Message
	lt := mem.GetDefaultLongTermMemory()
	if lt != nil && req.UserId != "" {
		sm := mem.GetSimpleMemoryWithUser(req.Id, req.UserId, lt)
		history = sm.GetMessagesWithContext(ctx, req.Question)
	} else {
		history = mem.GetSimpleMemory(req.Id).GetMessages()
	}

	userMessage := &chat_workflow.UserMessage{
		ID:      req.Id,
		Query:   req.Question,
		History: history,
	}
	// 请求级 trace：eino callback 适配器记录每个组件 span，落 traces/<runID>.json
	rec := trace.NewRecorder(req.Question, time.Now().Format(time.RFC3339Nano))
	ctx = trace.WithRecorder(ctx, rec)
	runner, err := chat_workflow.BuildChatAgent(ctx)
	if err != nil {
		log.Printf("ERROR: BuildChatAgent failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out, err := runner.Invoke(ctx, userMessage, compose.WithCallbacks(logcallback.LogCallback(nil), trace.EinoHandler(rec)))
	status := "ok"
	if err != nil {
		status = "error"
		log.Printf("ERROR: Chat Invoke failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		rec.Finalize(status, err)
		return
	}
	rec.Finalize(status, err)
	res := &vo.ChatRes{
		Answer: out.Content,
	}
	mem.GetSimpleMemory(req.Id).SetMessages(ctx, schema.UserMessage(req.Question))
	mem.GetSimpleMemory(req.Id).SetMessages(ctx, schema.AssistantMessage(out.Content, nil))

	// 写入全量历史（MySQL 持久化）
	if hs := mem.GetDefaultHistoryStore(); hs != nil {
		_ = hs.AppendMessages(req.Id, req.UserId, []*mem.Message{
			schema.UserMessage(req.Question),
			schema.AssistantMessage(out.Content, nil),
		})
	}

	c.JSON(http.StatusOK, res)
}
