package middleware

import (
	"log"
	"net/http"
	"strings"
	"subscription-system/auth"
	"subscription-system/config"
	"subscription-system/database"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware проверяет JWT, но пропускает всё, если cfg.SkipAuth == true
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Публичные маршруты – всегда пропускаем
		publicRoutes := map[string]bool{
			"/":                true,
			"/about":           true,
			"/contact":         true,
			"/info":            true,
			"/pricing":         true,
			"/partner":         true,
			"/referral":        true,
			"/login":           true,
			"/register":        true,
			"/forgot-password": true,
			"/api/health":      true,
			"/api/crm/health":  true,
			"/api/test":        true,
			"/api/auth/login":  true,
			"/api/auth/register": true,
			"/api/auth/refresh": true,
		}
		if publicRoutes[c.Request.URL.Path] {
			c.Next()
			return
		}

		// ========== РЕЖИМ РАЗРАБОТКИ ==========
		if cfg.SkipAuth {
			// Подставляем первого пользователя (обычно админ) для всех запросов
			var id string
			err := database.Pool.QueryRow(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1").Scan(&id)
			if err != nil {
				log.Printf("⚠️ Не удалось получить первого пользователя: %v", err)
				c.Next()
				return
			}
			c.Set("userID", id)
			c.Set("userRole", "admin")
			log.Printf("🔓 SkipAuth: установлен userID=%s, role=admin", id)
			c.Next()
			return
		}

		// ========== РЕАЛЬНАЯ ПРОВЕРКА JWT ==========
		if c.GetHeader("X-Skip-Auth") == "true" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && strings.ToLower(parts[0]) == "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		tokenString := parts[1]
		claims, err := auth.ValidateAccessToken(cfg, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired access token"})
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Set("userRole", claims.Role)
		c.Next()
	}
}

// AdminMiddleware проверяет роль admin
func AdminMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.SkipAuth {
			c.Next()
			return
		}
		role, exists := c.Get("userRole")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			return
		}
		c.Next()
	}
}
