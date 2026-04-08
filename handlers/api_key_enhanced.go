package handlers

import (
    "context"
    "crypto/rand"
    "encoding/base64" 
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
    "subscription-system/database"
)

// GenerateAPIKey - создание нового API ключа
func GenerateAPIKey(c *gin.Context) {
    // Получаем userID из контекста (именно userID, а не user_id)
    userIDInterface, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found - please login first"})
        return
    }
    
    // Преобразуем в UUID
    userIDStr := userIDInterface.(string)
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
        return
    }

    var req struct {
        Name     string `json:"name" binding:"required"`
        PlanType string `json:"plan_type"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.PlanType == "" {
        req.PlanType = "free"
    }

    // Генерация ключа
    keyBytes := make([]byte, 32)
    rand.Read(keyBytes)
    rawKey := "sk_" + base64.URLEncoding.EncodeToString(keyBytes)

    // Хеширование
    hash, _ := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)

    // Лимиты по плану
    dailyLimit, monthlyLimit := getLimitsByPlan(req.PlanType)

    var keyID uuid.UUID
    err = database.Pool.QueryRow(context.Background(), `
        INSERT INTO api_keys (user_id, name, key_hash, plan_type, daily_limit, monthly_limit)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `, userID, req.Name, string(hash), req.PlanType, dailyLimit, monthlyLimit).Scan(&keyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, gin.H{
        "id":            keyID.String(),
        "name":          req.Name,
        "key":           rawKey,
        "plan_type":     req.PlanType,
        "daily_limit":   dailyLimit,
        "monthly_limit": monthlyLimit,
        "message":       "Save this key! It won't be shown again.",
    })
}// RevokeAPIKey - отзыв ключа
func RevokeAPIKey(c *gin.Context) {
    userID, _ := c.Get("user_id")
    keyID := c.Param("id")

    result, err := database.Pool.Exec(context.Background(), `
        UPDATE api_keys SET is_active = false 
        WHERE id = $1 AND user_id = $2
    `, keyID, userID)

    if err != nil || result.RowsAffected() == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "API key revoked"})
}

// GetAPIKeyStats - статистика ключа
func GetAPIKeyStats(c *gin.Context) {
    keyID := c.Param("id")

    var stats struct {
        TotalRequests int     `json:"total_requests"`
        SuccessRate   float64 `json:"success_rate"`
        AvgResponse   float64 `json:"avg_response_time"`
    }

    database.Pool.QueryRow(context.Background(), `
        SELECT 
            COUNT(*) as total_requests,
            COALESCE(AVG(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) * 100, 0) as success_rate,
            COALESCE(AVG(response_time), 0) as avg_response_time
        FROM api_request_logs
        WHERE api_key_id = $1
    `, keyID).Scan(&stats.TotalRequests, &stats.SuccessRate, &stats.AvgResponse)

    c.JSON(http.StatusOK, stats)
}

// GetAPIKeyDailyStats - дневная статистика
func GetAPIKeyDailyStats(c *gin.Context) {
    keyID := c.Param("id")

    rows, err := database.Pool.Query(context.Background(), `
        SELECT date, total_requests, success_requests, error_requests, avg_response_time
        FROM api_daily_stats
        WHERE api_key_id = $1
        ORDER BY date DESC
        LIMIT 30
    `, keyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var stats []gin.H
    for rows.Next() {
        var date time.Time
        var total, success, errors int
        var avgTime float64
        rows.Scan(&date, &total, &success, &errors, &avgTime)

        stats = append(stats, gin.H{
            "date":               date.Format("2006-01-02"),
            "total_requests":     total,
            "success_requests":   success,
            "error_requests":     errors,
            "avg_response_time":  avgTime,
        })
    }

    c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// getLimitsByPlan - лимиты по тарифу
func getLimitsByPlan(plan string) (daily, monthly int) {
    switch strings.ToLower(plan) {
    case "basic":
        return 1000, 10000
    case "pro":
        return 5000, 50000
    case "enterprise":
        return 50000, 500000
    default:
        return 100, 1000
    }
}
// GetAPIKeys - список ключей пользователя
func GetAPIKeys(c *gin.Context) {
    // Получаем userID из контекста
    userIDInterface, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
        return
    }
    
    userIDStr := userIDInterface.(string)
    userID, err := uuid.Parse(userIDStr)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
        return
    }

    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, name, plan_type, daily_limit, monthly_limit, daily_used, monthly_used,
               is_active, expires_at, last_used_at, created_at
        FROM api_keys 
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var keys []gin.H
    for rows.Next() {
        var id, name, planType string
        var dailyLimit, monthlyLimit, dailyUsed, monthlyUsed int
        var isActive bool
        var expiresAt, lastUsedAt, createdAt time.Time

        rows.Scan(&id, &name, &planType, &dailyLimit, &monthlyLimit, &dailyUsed, &monthlyUsed,
            &isActive, &expiresAt, &lastUsedAt, &createdAt)

        keys = append(keys, gin.H{
            "id":             id,
            "name":           name,
            "plan_type":      planType,
            "daily_limit":    dailyLimit,
            "monthly_limit":  monthlyLimit,
            "daily_used":     dailyUsed,
            "monthly_used":   monthlyUsed,
            "is_active":      isActive,
            "expires_at":     expiresAt,
            "last_used_at":   lastUsedAt,
            "created_at":     createdAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{"keys": keys})
}