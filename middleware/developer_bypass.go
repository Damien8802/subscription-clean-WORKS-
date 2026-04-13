package middleware

import (
    "github.com/gin-gonic/gin"
)

// DevFullAccessMiddleware - полный доступ для разработчика
func DevFullAccessMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Проверяем роль из контекста
        role, exists := c.Get("user_role")
        
        // Если разработчик - пропускаем без проверок
        if exists && (role == "developer" || role == "admin") {
            c.Set("has_full_access", true)
            c.Next()
            return
        }
        
        c.Next()
    }
}