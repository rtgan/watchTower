package chat

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"watchTower/ai/agent/chat_workflow"
	logcallback "watchTower/common/log_callback"
	"watchTower/mem"
	"watchTower/model/vo"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
)

func ChatStream(c *gin.Context) {
	var req vo.ChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Failed to bind JSON: %v", err)})
		return
	}
	baseCtx, cancel := context.WithTimeout(c.Copy().Request.Context(), 3*time.Minute)
	defer cancel()
	ctx := context.WithValue(baseCtx, "client_id", req.Id)

	userMessage := &chat_workflow.UserMessage{
		ID:      req.Id,
		Query:   req.Question,
		History: mem.GetSimpleMemory(req.Id).GetMessages(),
	}
	runner, err := chat_workflow.BuildChatAgent(ctx)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stream, err := runner.Stream(ctx, userMessage, compose.WithCallbacks(logcallback.LogCallback(nil)))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("Failed to start stream: %v", err)})
		return
	}
	defer stream.Close()

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		log.Println("[SSE]ChatStream: streaming unsupported")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"message": "streaming unsupported"})
		return
	}

	// 仅在确定进入流式响应后再写 SSE 头，避免与 JSON 错误响应混用 Content-Type
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no") // 禁止代理缓存

	cb := func(msg string) {
		log.Printf("[SSE] Sending chunk: %s (len=%d)\n", msg, len(msg))
		// SSE multi-line: content 中的每个 \n 都需要拆成独立的 "data: " 行，
		// 最后用一个空行(\n\n) 表示事件结束。
		parts := strings.Split(msg, "\n")
		for _, part := range parts {
			if _, err := fmt.Fprintf(c.Writer, "data: %s\n", part); err != nil {
				log.Println("[SSE] Write error:", err)
				return
			}
		}
		if _, err := c.Writer.Write([]byte("\n")); err != nil {
			log.Println("[SSE] Write error:", err)
			return
		}
		flusher.Flush()
		log.Println("[SSE] Flushed")
	}

	var fullResp strings.Builder

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// 发送 [DONE] 标志，通知前端/浏览器流式响应结束
			_, err = c.Writer.Write([]byte("data: [DONE]\n\n"))
			if err != nil {
				log.Println("ChatStream write DONE error:", err)
				c.SSEvent("error", gin.H{"message": fmt.Sprintf("Failed to send [DONE]: %v", err)})
				return
			}
			flusher.Flush()
			log.Println("ChatStream sent [DONE]")
			break
		}
		if err != nil {
			c.SSEvent("error", gin.H{"message": fmt.Sprintf("Failed to send message: %v", err)}) //相当于writer.write并自动flusher.flush
			return
		}
		if len(chunk.Content) > 0 {
			fullResp.WriteString(chunk.Content) // 聚合

			cb(chunk.Content) // 实时调用cb函数，方便主动发送给前端
		}
	}

	mem.GetSimpleMemory(req.Id).SetMessages(schema.UserMessage(req.Question))
	mem.GetSimpleMemory(req.Id).SetMessages(schema.AssistantMessage(fullResp.String(), nil))

	// 正文已在流中通过 cb 推送，并以 data: [DONE] 结束；不要再 c.JSON，否则会在 SSE 体后拼接 JSON，客户端解析会坏掉
}
