package middleware

import (
    "log"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

type RateLimiter struct {
    mu       sync.Mutex
    attempts map[string][]time.Time
    limit    int
    window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
    return &RateLimiter{
        attempts: make(map[string][]time.Time),
        limit:    limit,
        window:   window,
    }
}

func (rl *RateLimiter) Limit(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    // Очищаем старые попытки
    var valid []time.Time
    for _, t := range rl.attempts[key] {
        if now.Sub(t) < rl.window {
            valid = append(valid, t)
        }
    }

    if len(valid) >= rl.limit {
        rl.attempts[key] = valid
        return true // превышен лимит
    }

    rl.attempts[key] = append(valid, now)
    return false
}

// SecurityMonitor middleware для отслеживания подозрительной активности
func SecurityMonitor() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        
        // Логируем подозрительные статусы
        status := c.Writer.Status()
        if status == 401 || status == 403 {
            log.Printf("⚠️ Неавторизованный доступ: %s %s с IP %s", 
                c.Request.Method, c.Request.URL.Path, c.ClientIP())
        }
        
        // Логируем слишком быстрые запросы (потенциальные атаки)
        duration := time.Since(c.GetTime("startTime"))
        if duration < 10*time.Millisecond && c.Request.URL.Path != "/api/health" {
            log.Printf("🚨 Подозрительно быстрый запрос: %s %s (%v) с IP %s",
                c.Request.Method, c.Request.URL.Path, duration, c.ClientIP())
        }
    }
}

// Helper function to get start time (добавляем в Logger middleware)
func init() {
    // Эта функция будет вызвана при инициализации
    // Убедитесь, что в Logger middleware установлено startTime
}