package middleware

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// CheckPasswordAge - проверяет возраст пароля
func CheckPasswordAge(userID string) (bool, error) {
    var passwordChangedAt time.Time
    err := database.Pool.QueryRow(context.Background(), `
        SELECT password_changed_at FROM users WHERE id = $1
    `, userID).Scan(&passwordChangedAt)
    
    if err != nil {
        return false, err
    }
    
    // Если пароль старше 90 дней
    return time.Since(passwordChangedAt) > 90*24*time.Hour, nil
}

// ForcePasswordChangeMiddleware - middleware для принудительной смены пароля
func ForcePasswordChangeMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, exists := c.Get("user_id")
        if !exists {
            c.Next()
            return
        }
        
        needsChange, err := CheckPasswordAge(userID.(string))
        if err == nil && needsChange {
            // Проверяем, не пытается ли пользователь сменить пароль
            if c.Request.URL.Path != "/api/user/change-password" && c.Request.URL.Path != "/api/user/profile" {
                c.JSON(http.StatusUnauthorized, gin.H{
                    "error": "Password expired. Please change your password.",
                    "code":  "PASSWORD_EXPIRED",
                })
                c.Abort()
                return
            }
        }
        c.Next()
    }
}
