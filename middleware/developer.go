package middleware

import (
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// DevAccessMiddleware - пропускает разработчиков везде
func DevAccessMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            c.Next()
            return
        }

        var isDeveloper bool
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT is_developer FROM users WHERE id = $1
        `, userID).Scan(&isDeveloper)

        if err == nil && isDeveloper {
            c.Set("is_developer", true)
            c.Set("has_full_access", true)
            c.Header("X-Developer-Mode", "true")
        }

        c.Next()
    }
}

// RequireServiceAccess - проверяет доступ к услуге (разработчики пропускаются)
func RequireServiceAccess(serviceCode string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Разработчики имеют полный доступ
        if isDev, exists := c.Get("is_developer"); exists && isDev.(bool) {
            c.Next()
            return
        }

        // Режим разработки (SKIP_AUTH)
        if c.GetBool("skip_auth") {
            c.Next()
            return
        }

        userID := c.GetString("user_id")
        if userID == "" {
            c.Next()
            return
        }

        // Проверяем активную подписку
        var count int
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT COUNT(*) FROM user_service_subscriptions 
            WHERE user_id = $1 AND service_code = $2 AND status = 'active'
            AND (expires_at IS NULL OR expires_at > NOW())
        `, userID, serviceCode).Scan(&count)

        if count == 0 {
            c.HTML(402, "service_locked.html", gin.H{
                "service_code": serviceCode,
                "service_name": getServiceName(serviceCode),
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

func getServiceName(serviceCode string) string {
    names := map[string]string{
        "crm": "CRM система", "1c": "1С Интеграция", "teamsphere": "TeamSphere",
        "inventory": "Складской учет", "hr": "HR модуль", "vpn": "VPN сервис",
        "cloud": "Облачное хранилище", "logistics": "Логистика",
        "whatsapp": "WhatsApp интеграция", "ai": "AI ассистент",
    }
    if name, ok := names[serviceCode]; ok {
        return name
    }
    return serviceCode
}