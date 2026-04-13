package middleware

import (
    "log"
    "net/http"
    "strings"
    "subscription-system/config"
    "subscription-system/utils"

    "github.com/gin-gonic/gin"
)

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

      
        // Получаем заголовок разработчика
        devHeader := c.GetHeader("X-Developer-Access")

        // ========== РЕЖИМ РАЗРАБОТЧИКА (ЗАГОЛОВОК) ==========
        if devHeader == "fusion-dev-2024" {
            userID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
            c.Set("user_id", userID)
            c.Set("user_name", "Разработчик")
            c.Set("role", "admin")
            c.Set("tenant_id", "11111111-1111-1111-1111-111111111111")
            log.Printf("[DEV] 🔧 Режим разработчика: %s %s (заголовок принят)", method, path)
            c.Next()
            return
        }

        // Пропускаем маршруты архива
        if strings.HasPrefix(path, "/archive/") {
            c.Next()
            return
        }

        // ========== ПУБЛИЧНЫЕ МАРШРУТЫ ==========
        publicRoutes := map[string]bool{
            "/":                         true,
            "/about":                    true,
            "/contact":                  true,
            "/info":                     true,
            "/pricing":                  true,
            "/partner":                  true,
            "/login":                    true,
            "/register":                 true,
            "/forgot-password":          true,
            "/api/health":               true,
            "/api/crm/health":           true,
            "/api/test":                 true,
            "/api/auth/login":           true,
            "/api/auth/register":        true,
            "/api/auth/refresh":         true,
            "/api/auth/logout":          true,
            "/api/crm/ai/ask":           true,
            "/api/ai/ask":               true,
            "/fusion-portal":            true,
            "/dev/login":                true,
        }

        if publicRoutes[path] {
            c.Next()
            return
        }

        // ========== ПРОВЕРКА JWT ТОКЕНА ==========
        authHeader := c.GetHeader("Authorization")
        tokenString := ""

        if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
            tokenString = strings.TrimPrefix(authHeader, "Bearer ")
        }

        if tokenString == "" {
            cookie, err := c.Cookie("token")
            if err == nil && cookie != "" {
                tokenString = cookie
            }
        }

        if tokenString == "" {
            log.Printf("[AUTH] ❌ Неавторизованный доступ: %s %s с IP %s", method, path, c.ClientIP())
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "authorization header required",
                "code":  "UNAUTHORIZED",
            })
            c.Abort()
            return
        }

        // Верифицируем JWT токен
        claims, err := utils.ValidateToken(tokenString)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "invalid or expired token",
                "code":  "INVALID_TOKEN",
            })
            c.Abort()
            return
        }

        // Устанавливаем данные пользователя
        c.Set("user_id", claims.UserID)
        c.Set("user_name", claims.UserName)
        c.Set("user_email", claims.Email)
        c.Set("role", claims.Role)
        c.Set("tenant_id", claims.TenantID)

        // ========== ПРОВЕРКА НА РАЗРАБОТЧИКА dev@saaspro.ru ==========
        if claims.Email == "dev@saaspro.ru" {
            c.Set("role", "admin")
            c.Set("is_developer", true)
            log.Printf("[AUTH] 👑 РАЗРАБОТЧИК %s получил полный доступ", claims.Email)
        }

        log.Printf("[AUTH] ✅ Авторизован: %s (%s), путь: %s %s", claims.UserName, claims.Email, method, path)

        c.Next()
    }
}

func AdminMiddleware(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        method := c.Request.Method

        role, exists := c.Get("role")
        if !exists {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
                "error": "unauthorized - role not found",
                "code":  "ROLE_NOT_FOUND",
            })
            return
        }

        if role != "admin" && role != "developer" {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "admin access required",
                "code":  "ADMIN_REQUIRED",
            })
            return
        }

        log.Printf("[ADMIN] 🟢 Доступ разрешен для %s на %s %s", role, method, path)
        c.Next()
    }
}
