package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// SecurityPageHandler отображает страницу безопасности
func SecurityPageHandler(c *gin.Context) {
    // Получаем user_id из контекста или используем тестовый
    userID, exists := c.Get("userID")
    if !exists {
        userID = c.Query("user_id")
        if userID == "" {
            userID = "test-user-123"
        }
    }

    // Используем новый шаблон security_new.html
    c.HTML(http.StatusOK, "security_new.html", gin.H{
        "title":   "Безопасность аккаунта",
        "user_id": userID,
    })
}