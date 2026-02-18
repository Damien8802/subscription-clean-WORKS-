package middleware

import (
        "log"
        "net/http"
        "strings"

        "subscription-system/models"

        "github.com/gin-gonic/gin"
)

// APIKeyAuthMiddleware проверяет API-ключ в заголовке Authorization
func APIKeyAuthMiddleware() gin.HandlerFunc {
        return func(c *gin.Context) {
                authHeader := c.GetHeader("Authorization")
                if authHeader == "" {
                        log.Println("❌ Authorization header missing")
                        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
                        return
                }

                parts := strings.SplitN(authHeader, " ", 2)
                if !(len(parts) == 2 && strings.ToLower(parts[0]) == "bearer") {
                        log.Println("❌ Invalid authorization header format")
                        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
                        return
                }

                rawKey := parts[1]

                apiKey, err := models.VerifyAPIKey(rawKey)
                if err != nil {
                        log.Printf("❌ VerifyAPIKey failed: %v", err)
                        c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
                        return
                }

                // Подробное логирование состояния ключа
                log.Printf("🔍 APIKey: id=%s, userID=%s, isActive=%v, quotaLimit=%d, quotaUsed=%d",
                        apiKey.ID, apiKey.UserID, apiKey.IsActive, apiKey.QuotaLimit, apiKey.QuotaUsed)

                if !apiKey.IsActive {
                        log.Printf("⛔ API key is disabled (isActive=false)")
                        c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "API key is disabled"})
                        return
                }

                // Проверяем лимит, если он не безлимитный
                if apiKey.QuotaLimit != -1 && apiKey.QuotaUsed >= apiKey.QuotaLimit {
                        log.Printf("⛔ Quota exceeded: limit=%d, used=%d", apiKey.QuotaLimit, apiKey.QuotaUsed)
                        c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"error": "quota exceeded"})
                        return
                }

                // Логируем значения квоты (уже есть выше, но оставим для совместимости)
                log.Printf("✅ APIKey проверен, пропускаем запрос")

                // Сохраняем информацию о ключе в контекст
                c.Set("apiKeyID", apiKey.ID)
                c.Set("apiKeyUserID", apiKey.UserID)
                c.Set("providerCredentials", []byte(apiKey.ProviderCredentials))
                c.Set("quotaLimit", apiKey.QuotaLimit)
                c.Set("quotaUsed", apiKey.QuotaUsed)

                c.Next()
        }
}