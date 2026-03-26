package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// SecurityMiddleware - middleware для безопасности
func SecurityMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        
        // Проверка блокировки IP
        var blockedUntil time.Time
        err := database.Pool.QueryRow(context.Background(), `
            SELECT blocked_until FROM blocked_ips WHERE ip = $1 AND blocked_until > NOW()
        `, ip).Scan(&blockedUntil)
        
        if err == nil {
            c.JSON(http.StatusForbidden, gin.H{
                "error": fmt.Sprintf("IP заблокирован до %s", blockedUntil.Format("2006-01-02 15:04:05")),
            })
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// LogSecurityEvent - логирование событий безопасности
func LogSecurityEvent(userID uuid.UUID, action string, ip string, userAgent string, details map[string]interface{}, status string) {
    detailsJSON, _ := json.Marshal(details)
    
    _, err := database.Pool.Exec(context.Background(), `
        INSERT INTO security_logs (user_id, action, ip, user_agent, details, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
    `, userID, action, ip, userAgent, detailsJSON, status)
    
    if err != nil {
        log.Printf("❌ Ошибка логирования безопасности: %v", err)
    }
}

// GetSecurityLogs - получить логи безопасности пользователя
func GetSecurityLogs(c *gin.Context) {
    userID := getUserID(c)
    
    limit := c.DefaultQuery("limit", "50")
    offset := c.DefaultQuery("offset", "0")
    action := c.Query("action")
    
    query := `
        SELECT id, action, ip, user_agent, details, status, created_at
        FROM security_logs
        WHERE user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if action != "" {
        query += fmt.Sprintf(" AND action = $%d", argIndex)
        args = append(args, action)
        argIndex++
    }
    
    query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
    args = append(args, limit, offset)
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var logs []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var action, ip, userAgent, status string
        var detailsJSON []byte
        var createdAt time.Time
        
        rows.Scan(&id, &action, &ip, &userAgent, &detailsJSON, &status, &createdAt)
        
        var details map[string]interface{}
        json.Unmarshal(detailsJSON, &details)
        
        logs = append(logs, map[string]interface{}{
            "id":         id,
            "action":     action,
            "ip":         ip,
            "user_agent": userAgent,
            "details":    details,
            "status":     status,
            "created_at": createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "logs":    logs,
    })
}

// RecordFailedAttempt - запись неудачной попытки
func RecordFailedAttempt(ip string, userID *uuid.UUID, action string) {
    ctx := context.Background()
    
    // Обновляем или вставляем счетчик
    _, err := database.Pool.Exec(ctx, `
        INSERT INTO failed_attempts (ip, user_id, action, attempt_count, last_attempt)
        VALUES ($1, $2, $3, 1, NOW())
        ON CONFLICT (ip, user_id, action) DO UPDATE SET
            attempt_count = failed_attempts.attempt_count + 1,
            last_attempt = NOW()
    `, ip, userID, action)
    
    if err != nil {
        log.Printf("❌ Ошибка записи неудачной попытки: %v", err)
        return
    }
    
    // Проверяем, нужно ли блокировать IP
    var attemptCount int
    database.Pool.QueryRow(ctx, `
        SELECT attempt_count FROM failed_attempts
        WHERE ip = $1 AND user_id IS NOT DISTINCT FROM $2 AND action = $3
    `, ip, userID, action).Scan(&attemptCount)
    
    // Блокируем после 5 неудачных попыток
    if attemptCount >= 5 {
        blockUntil := time.Now().Add(15 * time.Minute)
        _, err = database.Pool.Exec(ctx, `
            INSERT INTO blocked_ips (ip, reason, blocked_until)
            VALUES ($1, $2, $3)
            ON CONFLICT (ip) DO UPDATE SET
                blocked_until = EXCLUDED.blocked_until,
                reason = EXCLUDED.reason
        `, ip, fmt.Sprintf("Слишком много неудачных попыток (%d)", attemptCount), blockUntil)
        
        if err == nil {
            log.Printf("🚫 IP %s заблокирован до %s (причина: %d неудачных попыток)", 
                ip, blockUntil.Format("2006-01-02 15:04:05"), attemptCount)
        }
    }
}

// ResetFailedAttempts - сброс счетчика неудачных попыток
func ResetFailedAttempts(ip string, userID *uuid.UUID, action string) {
    _, err := database.Pool.Exec(context.Background(), `
        DELETE FROM failed_attempts
        WHERE ip = $1 AND user_id IS NOT DISTINCT FROM $2 AND action = $3
    `, ip, userID, action)
    
    if err != nil {
        log.Printf("❌ Ошибка сброса счетчика: %v", err)
    }
}

// GetBlockedIPs - получить список заблокированных IP
func GetBlockedIPs(c *gin.Context) {
    userID := getUserID(c)
    
    // Проверяем права администратора
    var role string
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT role FROM users WHERE id = $1
    `, userID).Scan(&role)
    
    if role != "admin" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен"})
        return
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT ip, reason, blocked_until, created_at
        FROM blocked_ips
        WHERE blocked_until > NOW()
        ORDER BY blocked_until DESC
    `)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var ips []map[string]interface{}
    for rows.Next() {
        var ip, reason string
        var blockedUntil, createdAt time.Time
        
        rows.Scan(&ip, &reason, &blockedUntil, &createdAt)
        
        ips = append(ips, map[string]interface{}{
            "ip":            ip,
            "reason":        reason,
            "blocked_until": blockedUntil,
            "created_at":    createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "ips":     ips,
    })
}

// UnblockIP - разблокировать IP
func UnblockIP(c *gin.Context) {
    userID := getUserID(c)
    
    // Проверяем права администратора
    var role string
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT role FROM users WHERE id = $1
    `, userID).Scan(&role)
    
    if role != "admin" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен"})
        return
    }
    
    ip := c.Param("ip")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM blocked_ips WHERE ip = $1
    `, ip)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Очищаем счетчики неудачных попыток
    database.Pool.Exec(c.Request.Context(), `
        DELETE FROM failed_attempts WHERE ip = $1
    `, ip)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "IP разблокирован",
    })
}

// GetSecurityStats - статистика безопасности
func GetSecurityStats(c *gin.Context) {
    userID := getUserID(c)
    
    var role string
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT role FROM users WHERE id = $1
    `, userID).Scan(&role)
    
    if role != "admin" {
        c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен"})
        return
    }
    
    var stats struct {
        TotalLogs      int     `json:"total_logs"`
        FailedLogins   int     `json:"failed_logins"`
        BlockedIPs     int     `json:"blocked_ips"`
        SuspiciousActivity int `json:"suspicious_activity"`
    }
    
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT 
            COUNT(*) as total_logs,
            COUNT(CASE WHEN action = 'login' AND status = 'failed' THEN 1 END) as failed_logins,
            (SELECT COUNT(*) FROM blocked_ips WHERE blocked_until > NOW()) as blocked_ips,
            COUNT(CASE WHEN status = 'failed' AND created_at > NOW() - INTERVAL '1 hour' THEN 1 END) as suspicious_activity
        FROM security_logs
        WHERE created_at > NOW() - INTERVAL '7 days'
    `).Scan(&stats.TotalLogs, &stats.FailedLogins, &stats.BlockedIPs, &stats.SuspiciousActivity)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "stats":   stats,
    })
}