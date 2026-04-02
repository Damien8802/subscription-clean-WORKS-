package middleware

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

var (
    globalLimiter = rate.NewLimiter(100, 200)
    ipLimiters    = sync.Map{}
)

func getIPLimiter(ip string) *rate.Limiter {
    if limiter, ok := ipLimiters.Load(ip); ok {
        return limiter.(*rate.Limiter)
    }
    limiter := rate.NewLimiter(20, 50)
    ipLimiters.Store(ip, limiter)
    return limiter
}

func MegaSecurityMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        ip := c.ClientIP()
        ipLimiter := getIPLimiter(ip)
        if !ipLimiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "Too many requests",
                "code":  "RATE_LIMIT",
            })
            c.Abort()
            return
        }
        
        if !globalLimiter.Allow() {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "error": "Server overloaded",
                "code":  "GLOBAL_LIMIT",
            })
            c.Abort()
            return
        }
        
        for _, param := range c.Params {
            if containsInjection(param.Value) {
                c.JSON(http.StatusBadRequest, gin.H{
                    "error": "Invalid input",
                    "code":  "INVALID",
                })
                c.Abort()
                return
            }
        }
        
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        
        requestID := make([]byte, 16)
        rand.Read(requestID)
        c.Header("X-Request-ID", hex.EncodeToString(requestID))
        
        c.Next()
        
        duration := time.Since(start)
        fmt.Printf("[%s] %s %s - %d (%v)\n", 
            ip, c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)
    }
}

func containsInjection(s string) bool {
    dangerous := []string{"--", ";", "DROP", "DELETE", "INSERT", "UPDATE", "SELECT", "UNION"}
    s = strings.ToUpper(s)
    for _, d := range dangerous {
        if strings.Contains(s, d) {
            return true
        }
    }
    return false
}
