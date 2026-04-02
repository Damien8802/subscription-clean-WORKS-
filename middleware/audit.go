package middleware

import (
    "bytes"
    "context"
    "io"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// AuditMiddleware - логирование всех действий
func AuditMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // Пропускаем статические файлы
        if isStaticFile(c.Request.URL.Path) {
            c.Next()
            return
        }
        
        // Читаем тело запроса
        var bodyBytes []byte
        if c.Request.Body != nil {
            bodyBytes, _ = io.ReadAll(c.Request.Body)
            c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
        }
        
        c.Next()
        
        duration := time.Since(start)
        
        // Получаем user_id из контекста (если есть)
        userID, _ := c.Get("user_id")
        userEmail, _ := c.Get("user_email")
        
        // Сохраняем в базу
        go saveAuditLog(
            userID,
            userEmail,
            c.Request.Method,
            c.Request.URL.Path,
            c.ClientIP(),
            c.Request.UserAgent(),
            c.Writer.Status(),
            duration,
        )
    }
}

func saveAuditLog(userID interface{}, userEmail interface{}, method, path, ip, ua string, status int, duration time.Duration) {
    ctx := context.Background()
    query := `
        INSERT INTO audit_log (user_id, user_email, action, ip, user_agent, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
    `
    database.Pool.Exec(ctx, query, userID, userEmail, method+" "+path, ip, ua, status)
    _ = duration // используем duration чтобы не было ошибки неиспользованной переменной
}

func isStaticFile(path string) bool {
    staticPaths := []string{"/static/", "/frontend/", "/favicon.ico", "/manifest.json", "/service-worker.js"}
    for _, p := range staticPaths {
        if len(path) >= len(p) && path[:len(p)] == p {
            return true
        }
    }
    return false
}
