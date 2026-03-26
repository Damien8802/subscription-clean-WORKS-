package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

func AIChatPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "ai-chat.html", gin.H{
        "Title":   "AI Чат - ServerAgent",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}