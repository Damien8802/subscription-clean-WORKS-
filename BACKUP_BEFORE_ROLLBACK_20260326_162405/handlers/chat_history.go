package handlers

import (
    "database/sql"
    "log"
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

type ChatMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type SaveMessageRequest struct {
    UserID  string `json:"user_id" binding:"required"`
    Role    string `json:"role" binding:"required"`
    Content string `json:"content" binding:"required"`
}

func SaveChatMessage(c *gin.Context) {
    var req SaveMessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(),
        `INSERT INTO chat_history (user_id, role, content) VALUES ($1, $2, $3)`,
        req.UserID, req.Role, req.Content)
    if err != nil {
        log.Printf("SaveChatMessage error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

func GetChatHistory(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(),
        `SELECT role, content, created_at FROM chat_history WHERE user_id = $1 ORDER BY created_at ASC`,
        userID)
    if err != nil {
        log.Printf("GetChatHistory query error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    defer rows.Close()

    var messages []ChatMessage
    for rows.Next() {
        var msg ChatMessage
        var createdAt sql.NullString // используем NullString для created_at
        if err := rows.Scan(&msg.Role, &msg.Content, &createdAt); err != nil {
            log.Printf("GetChatHistory scan error: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "scan error"})
            return
        }
        messages = append(messages, msg)
    }
    if err := rows.Err(); err != nil {
        log.Printf("GetChatHistory rows error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "rows error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"history": messages})
}
