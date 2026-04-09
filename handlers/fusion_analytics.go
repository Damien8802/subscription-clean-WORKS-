package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// GetAIAnalytics - получить аналитику по AI использованию для текущего API ключа
func GetAIAnalytics(c *gin.Context) {
    // Получаем API ключ из заголовка
    apiKey := c.GetHeader("X-API-Key")
    if apiKey == "" {
        apiKey = c.Query("api_key")
    }
    
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    var aiRequestsUsed, aiRequestsLimit int
    var agentsLimit, agentsCreated int
    var planName string
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT 
            COALESCE(k.ai_requests_used, 0),
            COALESCE(k.ai_requests_limit, 100),
            COALESCE(k.agents_limit, 1),
            COALESCE(k.agents_created, 0),
            COALESCE(p.name, 'Free')
        FROM api_keys k
        LEFT JOIN api_plans p ON k.plan_id = p.id
        WHERE k.key_hash = crypt($1, k.key_hash) AND k.is_active = true
    `, apiKey).Scan(&aiRequestsUsed, &aiRequestsLimit, &agentsLimit, &agentsCreated, &planName)
    
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "ai_requests_used":  aiRequestsUsed,
        "ai_requests_limit": aiRequestsLimit,
        "ai_remaining":      aiRequestsLimit - aiRequestsUsed,
        "ai_percent":        float64(aiRequestsUsed) / float64(aiRequestsLimit) * 100,
        "agents_limit":      agentsLimit,
        "agents_created":    agentsCreated,
        "agents_remaining":  agentsLimit - agentsCreated,
        "plan_name":         planName,
    })
}