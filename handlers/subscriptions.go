package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// MySubscriptionsPageHandler отображает страницу с подписками пользователя
func MySubscriptionsPageHandler(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        userID = "test-user-123"
    }

    c.HTML(http.StatusOK, "my-subscriptions.html", gin.H{
        "Title":  "Мои подписки",
        "UserID": userID,
    })
}

// CreateSubscriptionHandler - создание новой подписки (заглушка)
func CreateSubscriptionHandler(c *gin.Context) {
    var req struct {
        PlanCode string `json:"plan_code" binding:"required"`
        UserID   string `json:"user_id"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Подписка создана",
    })
}

// GetUserSubscriptionsHandler - список подписок пользователя (заглушка)
func GetUserSubscriptionsHandler(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "subscriptions": []interface{}{},
    })
}