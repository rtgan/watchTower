package chat

import (
	"net/http"
	"watchTower/model/vo"

	"github.com/gin-gonic/gin"
)

func Chat(c *gin.Context) {
	var userMessage vo.UserMessage
	if err := c.ShouldBindJSON(&userMessage); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
}
