package middleware

import (
    "log"
    "net/http"
    "strings"
    "subscription-system/config"
    "subscription-system/utils"

    "github.com/gin-gonic/gin"
)

// AuthMiddleware проверяет JWT, учитывая публичные маршруты и режим разработки
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

        // Пропускаем маршруты архива (публичный доступ)
        if strings.HasPrefix(path, "/archive/") {
            if cfg.SkipAuth {
                log.Printf("[AUTH] Архив: публичный доступ %s %s", method, path)
            }
            c.Next()
            return
        }

        // ========== ПУБЛИЧНЫЕ МАРШРУТЫ ==========
        publicRoutes := map[string]bool{
            "/":                        true,
            "/about":                   true,
            "/contact":                 true,
            "/info":                    true,
            "/pricing":                 true,
            "/partner":                 true,
            "/login":                   true,
            "/register":                true,
            "/forgot-password":         true,
            "/api/health":              true,
            "/api/crm/health":          true,
            "/api/test":                true,
            "/api/auth/login":          true,
            "/api/auth/register":       true,
            "/api/auth/refresh":        true,
            "/api/auth/logout":         true,
            "/api/crm/ai/ask":          true,
            "/api/ai/ask":              true,
        }

        if publicRoutes[path] {
            c.Next()
            return
        }

        // ========== РЕЖИМ РАЗРАБОТКИ ==========
        if cfg.SkipAuth {
            userID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
            c.Set("userID", userID)
            c.Set("role", "admin")
            c.Set("tenant_id", "11111111-1111-1111-1111-111111111111")
            log.Printf("[AUTH] 🟢 Режим разработки: %s %s, user=%s", method, path, userID)
            c.Next()
            return
        }

        // ========== ПРОДАКШЕН РЕЖИМ ==========
        if c.GetHeader("X-Skip-Auth") == "true" {
            c.Next()
            return
        }

        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "authorization header required",
                "code":  "UNAUTHORIZED",
            })
            return
        }

        parts := strings.SplitN(authHeader, " ", 2)
        if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "invalid authorization header format. Use 'Bearer <token>'",
                "code":  "INVALID_AUTH_FORMAT",
            })
            return
        }

        tokenString := parts[1]
        claims, err := utils.ValidateToken(tokenString)
        if err != nil {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "invalid or expired access token",
                "code":  "INVALID_TOKEN",
            })
            return
        }

        c.Set("userID", claims.UserID)
        c.Set("role", claims.Role)
        c.Set("tenant_id", claims.TenantID)
        log.Printf("[AUTH] 🟢 Успешная авторизация: %s %s, user=%s, role=%s", method, path, claims.UserID, claims.Role)
        c.Next()
    }
}

// AdminMiddleware проверяет роль admin
func AdminMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

        if cfg.SkipAuth {
            log.Printf("[ADMIN] 🟢 Режим разработки: доступ разрешен для %s %s", method, path)
            c.Next()
            return
        }

        role, exists := c.Get("role")
        if !exists {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "unauthorized - role not found",
                "code":  "ROLE_NOT_FOUND",
            })
            return
        }

        if role != "admin" {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "admin access required",
                "code":  "ADMIN_REQUIRED",
            })
            return
        }

        log.Printf("[ADMIN] 🟢 Доступ разрешен для admin на %s %s", method, path)
        c.Next()
    }
}
