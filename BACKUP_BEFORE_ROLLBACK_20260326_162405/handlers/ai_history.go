package handlers

import (
    "net/http"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

type AIRequest struct {
    ID        int    `json:"id"`
    Question  string `json:"question"`
    Answer    string `json:"answer"`
    CreatedAt string `json:"created_at"`
}

func GetUserAIHistoryHandler(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(),
        `SELECT id, question, answer, created_at 
         FROM ai_requests 
         WHERE user_id = $1 
         ORDER BY created_at DESC 
         LIMIT 10`, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    defer rows.Close()

    var history []AIRequest
    for rows.Next() {
        var req AIRequest
        if err := rows.Scan(&req.ID, &req.Question, &req.Answer, &req.CreatedAt); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "scan error"})
            return
        }
        history = append(history, req)
    }

    c.JSON(http.StatusOK, gin.H{"history": history})
}
