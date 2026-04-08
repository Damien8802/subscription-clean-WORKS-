package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"subscription-system/database"
)

// APIKeyAuthMiddleware - проверка API ключа
func APIKeyAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Получаем ключ из заголовка
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			c.Abort()
			return
		}

		// Поддержка "Bearer sk_xxx" и просто "sk_xxx"
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		if !strings.HasPrefix(apiKey, "sk_") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key format"})
			c.Abort()
			return
		}

		// Ищем ключ в БД
		var keyID, userID, keyHash string
		var isActive bool

		err := database.Pool.QueryRow(context.Background(), `
			SELECT id, user_id, key_hash, is_active
			FROM api_keys 
			WHERE is_active = true
		`).Scan(&keyID, &userID, &keyHash, &isActive)

		if err == pgx.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			c.Abort()
			return
		}

		// Проверяем хеш
		if err := bcrypt.CompareHashAndPassword([]byte(keyHash), []byte(apiKey)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		// Сохраняем в контекст
		c.Set("api_key_id", keyID)
		c.Set("user_id", userID)

		c.Next()

		// После запроса - обновляем статистику
		UpdateAPIKeyStats(keyID, c.Writer.Status())
	}
}

// UpdateAPIKeyStats - обновление статистики использования API ключа
func UpdateAPIKeyStats(keyID string, statusCode int) {
	go func() {
		ctx := context.Background()
		database.Pool.Exec(ctx, `
			UPDATE api_keys 
			SET daily_used = daily_used + 1,
			    monthly_used = monthly_used + 1,
			    last_used_at = NOW()
			WHERE id = $1
		`, keyID)

		database.Pool.Exec(ctx, `
			INSERT INTO api_request_logs (api_key_id, status_code, created_at)
			VALUES ($1, $2, NOW())
		`, keyID, statusCode)
	}()
}