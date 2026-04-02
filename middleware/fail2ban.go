package middleware

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// Fail2BanMiddleware - блокировка подозрительных IP
func Fail2BanMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        
        // Проверяем, не заблокирован ли IP
        var blockedUntil time.Time
        err := database.Pool.QueryRow(context.Background(), `
            SELECT blocked_until FROM blocked_ips 
            WHERE ip = $1 AND blocked_until > NOW()
        `, ip).Scan(&blockedUntil)
        
        if err == nil {
            c.JSON(http.StatusForbidden, gin.H{
                "error": "IP blocked",
                "until": blockedUntil,
                "code":  "IP_BLOCKED",
            })
            c.Abort()
            return
        }
        
        c.Next()
        
        // Если запрос вернул 401 (неудачная авторизация)
        if c.Writer.Status() == http.StatusUnauthorized {
            recordFailedAttempt(ip)
        }
    }
}

func recordFailedAttempt(ip string) {
    // Увеличиваем счетчик неудачных попыток
    var attempts int
    var blockedUntil time.Time
    
    err := database.Pool.QueryRow(context.Background(), `
        SELECT attempts_count, blocked_until FROM blocked_ips 
        WHERE ip = $1 AND blocked_until > NOW()
    `, ip).Scan(&attempts, &blockedUntil)
    
    if err == nil {
        // IP уже есть, увеличиваем счетчик
        attempts++
        if attempts >= 5 {
            // Блокируем на 15 минут
            blockedUntil = time.Now().Add(15 * time.Minute)
        }
        
        database.Pool.Exec(context.Background(), `
            UPDATE blocked_ips 
            SET attempts_count = $1, blocked_until = $2
            WHERE ip = $3
        `, attempts, blockedUntil, ip)
    } else {
        // Новый IP
        blockedUntil = time.Now().Add(15 * time.Minute)
        database.Pool.Exec(context.Background(), `
            INSERT INTO blocked_ips (ip, attempts_count, blocked_until)
            VALUES ($1, 1, $2)
        `, ip, blockedUntil)
    }
}
