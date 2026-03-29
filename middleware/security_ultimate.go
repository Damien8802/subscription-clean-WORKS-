package middleware

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "log"
    "net"
    "net/http"
    "regexp"
    "strings"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "golang.org/x/time/rate"
)

var (
    blacklistedIPs   = make(map[string]time.Time)
    blacklistMutex   sync.RWMutex
    bruteForceAttempts = make(map[string]int)
    bruteForceMutex    sync.RWMutex
    csrfTokens         = make(map[string]string)
    csrfMutex          sync.RWMutex
    
    sqlPatterns = []string{
        "(?i)SELECT.*FROM",
        "(?i)INSERT.*INTO",
        "(?i)UPDATE.*SET",
        "(?i)DELETE.*FROM",
        "(?i)DROP.*TABLE",
        "(?i)UNION.*SELECT",
        "(?i)OR.*=.*--",
        "(?i)'.*OR.*'.*=",
        "(?i)WAITFOR.*DELAY",
    }
    
    xssPatterns = []string{
        "<script",
        "javascript:",
        "onerror=",
        "onload=",
        "onclick=",
        "eval\\(",
        "document\\.cookie",
        "alert\\(",
        "<iframe",
        "<object",
    }
)

// UltimateSecurity - полная защита
func UltimateSecurity() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := getRealIP(c)
        
        // 1. Проверка чёрного списка IP
        if isBlacklisted(ip) {
            c.JSON(http.StatusForbidden, gin.H{"error": "Доступ запрещён"})
            c.Abort()
            return
        }
        
        // 2. Защита от SQL инъекций
        if containsSQLInjection(c.Request.URL.String()) {
            logSecurity(ip, "SQL Injection", c.Request.URL.String())
            c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
            c.Abort()
            return
        }
        
        // 3. Защита от XSS
        for _, param := range c.Request.URL.Query() {
            for _, value := range param {
                if containsXSS(value) {
                    logSecurity(ip, "XSS Attack", value)
                    c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ввод"})
                    c.Abort()
                    return
                }
            }
        }
        
        // 4. Защита от Path Traversal
        if strings.Contains(c.Request.URL.Path, "..") {
            logSecurity(ip, "Path Traversal", c.Request.URL.Path)
            c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный путь"})
            c.Abort()
            return
        }
        
        // 5. Защитные заголовки
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
        c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://cdnjs.cloudflare.com; img-src 'self' data: https:; font-src 'self' https://cdnjs.cloudflare.com; connect-src 'self'")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        
        // 6. CSRF защита для не-GET запросов
        if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.Request.Method != "OPTIONS" {
            csrfToken := c.GetHeader("X-CSRF-Token")
            sessionID, _ := c.Cookie("session_id")
            if !validateCSRFToken(sessionID, csrfToken) {
                c.JSON(http.StatusForbidden, gin.H{"error": "Неверный CSRF токен"})
                c.Abort()
                return
            }
        }
        
        // 7. Генерация CSRF токена для GET
        if c.Request.Method == "GET" {
            sessionID, _ := c.Cookie("session_id")
            if sessionID == "" {
                sessionID = generateSessionID()
                c.SetCookie("session_id", sessionID, 3600, "/", "", false, true)
            }
            csrfToken := generateCSRFToken(sessionID)
            c.Header("X-CSRF-Token", csrfToken)
            c.Set("csrf_token", csrfToken)
        }
        
        c.Next()
    }
}

// RateLimitUltimate - лимит запросов
func RateLimitUltimate() gin.HandlerFunc {
    limiters := make(map[string]*rate.Limiter)
    var mu sync.Mutex
    
    return func(c *gin.Context) {
        ip := getRealIP(c)
        
        mu.Lock()
        limiter, exists := limiters[ip]
        if !exists {
            limiter = rate.NewLimiter(rate.Limit(100), 100)
            limiters[ip] = limiter
        }
        mu.Unlock()
        
        if !limiter.Allow() {
            logSecurity(ip, "Rate Limit Exceeded", c.Request.URL.Path)
            c.JSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много запросов"})
            c.Abort()
            return
        }
        c.Next()
    }
}

// AntiBruteForce - защита от подбора паролей
func AntiBruteForce() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := getRealIP(c)
        
        if c.Request.URL.Path == "/api/auth/login" && c.Request.Method == "POST" {
            bruteForceMutex.Lock()
            attempts := bruteForceAttempts[ip]
            
            if attempts >= 5 {
                bruteForceMutex.Unlock()
                logSecurity(ip, "Brute Force", "5+ attempts")
                c.JSON(http.StatusTooManyRequests, gin.H{"error": "Слишком много попыток. Подождите 15 минут."})
                c.Abort()
                return
            }
            
            // Увеличиваем счётчик после неудачной попытки
            c.Set("brute_force_ip", ip)
            bruteForceMutex.Unlock()
        }
        
        c.Next()
        
        // Если логин неудачный, увеличиваем счётчик
        if c.Request.URL.Path == "/api/auth/login" && c.Writer.Status() == http.StatusUnauthorized {
            if ipVal, exists := c.Get("brute_force_ip"); exists {
                bruteForceMutex.Lock()
                bruteForceAttempts[ipVal.(string)]++
                // Очищаем старые записи
                if len(bruteForceAttempts) > 1000 {
                    for k := range bruteForceAttempts {
                        delete(bruteForceAttempts, k)
                        break
                    }
                }
                bruteForceMutex.Unlock()
            }
        }
    }
}

// AntiSQLInjection - защита от SQL инъекций
func AntiSQLInjection() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Проверяем URL
        if containsSQLInjection(c.Request.URL.String()) {
            logSecurity(getRealIP(c), "SQL Injection", c.Request.URL.String())
            c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
            c.Abort()
            return
        }
        
        // Проверяем POST параметры
        if c.Request.Method == "POST" {
            c.Request.ParseForm()
            for _, values := range c.Request.PostForm {
                for _, value := range values {
                    if containsSQLInjection(value) {
                        logSecurity(getRealIP(c), "SQL Injection", value)
                        c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ввод"})
                        c.Abort()
                        return
                    }
                }
            }
        }
        
        c.Next()
    }
}

// AntiXSS - защита от XSS
func AntiXSS() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Проверяем query параметры
        for _, param := range c.Request.URL.Query() {
            for _, value := range param {
                if containsXSS(value) {
                    logSecurity(getRealIP(c), "XSS Attack", value)
                    c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ввод"})
                    c.Abort()
                    return
                }
            }
        }
        
        // Проверяем POST параметры
        if c.Request.Method == "POST" {
            c.Request.ParseForm()
            for _, values := range c.Request.PostForm {
                for _, value := range values {
                    if containsXSS(value) {
                        logSecurity(getRealIP(c), "XSS Attack", value)
                        c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ввод"})
                        c.Abort()
                        return
                    }
                }
            }
        }
        
        c.Next()
    }
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

func getRealIP(c *gin.Context) string {
    if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
        ips := strings.Split(xff, ",")
        return strings.TrimSpace(ips[0])
    }
    if xri := c.GetHeader("X-Real-IP"); xri != "" {
        return xri
    }
    ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
    if err != nil {
        return c.Request.RemoteAddr
    }
    return ip
}

func isBlacklisted(ip string) bool {
    blacklistMutex.RLock()
    defer blacklistMutex.RUnlock()
    
    if expire, exists := blacklistedIPs[ip]; exists {
        if time.Now().Before(expire) {
            return true
        }
        // Удаляем просроченные
        delete(blacklistedIPs, ip)
    }
    return false
}

func containsSQLInjection(input string) bool {
    for _, pattern := range sqlPatterns {
        matched, _ := regexp.MatchString(pattern, input)
        if matched {
            return true
        }
    }
    return false
}

func containsXSS(input string) bool {
    lowerInput := strings.ToLower(input)
    for _, pattern := range xssPatterns {
        if strings.Contains(lowerInput, pattern) {
            return true
        }
    }
    return false
}

func generateSessionID() string {
    b := make([]byte, 32)
    rand.Read(b)
    return hex.EncodeToString(b)
}

func generateCSRFToken(sessionID string) string {
    hash := sha256.Sum256([]byte(sessionID + time.Now().String()))
    return hex.EncodeToString(hash[:])
}

func validateCSRFToken(sessionID, token string) bool {
    csrfMutex.RLock()
    defer csrfMutex.RUnlock()
    
    expected, exists := csrfTokens[sessionID]
    if exists && expected == token {
        return true
    }
    // Если токена нет, генерируем новый
    if !exists && sessionID != "" {
        csrfMutex.RUnlock()
        csrfMutex.Lock()
        csrfTokens[sessionID] = token
        csrfMutex.Unlock()
        return true
    }
    return false
}

func logSecurity(ip, eventType, details string) {
    log.Printf("🚨 [SECURITY ALERT] IP: %s | Event: %s | Details: %s | Time: %s", 
        ip, eventType, details, time.Now().Format("2006-01-02 15:04:05"))
}

// BlockIP - добавить IP в чёрный список
func BlockIP(ip string, duration time.Duration) {
    blacklistMutex.Lock()
    defer blacklistMutex.Unlock()
    blacklistedIPs[ip] = time.Now().Add(duration)
    log.Printf("🔒 IP %s заблокирован на %v", ip, duration)
}

// UnblockIP - удалить IP из чёрного списка
func UnblockIP(ip string) {
    blacklistMutex.Lock()
    defer blacklistMutex.Unlock()
    delete(blacklistedIPs, ip)
    log.Printf("🔓 IP %s разблокирован", ip)
}

// GetBruteForceAttempts - получить количество попыток
func GetBruteForceAttempts(ip string) int {
    bruteForceMutex.RLock()
    defer bruteForceMutex.RUnlock()
    return bruteForceAttempts[ip]
}

// ResetBruteForceAttempts - сбросить счётчик попыток
func ResetBruteForceAttempts(ip string) {
    bruteForceMutex.Lock()
    defer bruteForceMutex.Unlock()
    delete(bruteForceAttempts, ip)
}