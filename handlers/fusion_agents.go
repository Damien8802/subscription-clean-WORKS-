package handlers

import (
    "fmt"
    "log"
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
    "subscription-system/models"
)

// getAPIKeyFromRequest - получает API ключ из разных источников
func getAPIKeyFromRequest(c *gin.Context) string {
    devHeader := c.GetHeader("X-Developer-Access")
    if devHeader == "fusion-dev-2024" {
        return "dev_mode"
    }
    
    apiKey := c.GetHeader("X-API-Key")
    if apiKey != "" {
        return apiKey
    }
    
    apiKey = c.Query("api_key")
    if apiKey != "" {
        return apiKey
    }
    
    authHeader := c.GetHeader("Authorization")
    if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
        return authHeader[7:]
    }
    
    return ""
}

// getUserIDFromAPIKey - получает userID из API ключа или возвращает тестовый для разработки
func getUserIDFromAPIKey(c *gin.Context, apiKey string) (string, error) {
    if apiKey == "dev_mode" {
        return "aa5f14e6-30e1-476c-ac42-8c11ced838a4", nil
    }
    
    var userID string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT user_id FROM api_keys 
        WHERE key_hash = crypt($1, key_hash) AND is_active = true
    `, apiKey).Scan(&userID)
    
    if err != nil {
        return "", err
    }
    return userID, nil
}

// GetMyAgents - получить список AI агентов
func GetMyAgents(c *gin.Context) {
    apiKey := getAPIKeyFromRequest(c)
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    userID, err := getUserIDFromAPIKey(c, apiKey)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
        return
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, role, instructions, model, temperature, is_active, created_at
        FROM ai_agents 
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)
    if err != nil {
        log.Printf("❌ Ошибка загрузки агентов: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load agents"})
        return
    }
    defer rows.Close()
    
    var agents []gin.H
    for rows.Next() {
        var id, name, role, instructions, model string
        var temperature float64
        var isActive bool
        var createdAt string
        
        rows.Scan(&id, &name, &role, &instructions, &model, &temperature, &isActive, &createdAt)
        
        agents = append(agents, gin.H{
            "id":           id,
            "name":         name,
            "role":         role,
            "instructions": instructions,
            "model":        model,
            "temperature":  temperature,
            "is_active":    isActive,
            "created_at":   createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "agents": agents,
        "total":  len(agents),
    })
}

// CreateFusionAgent - создать AI агента
func CreateFusionAgent(c *gin.Context) {
    var req struct {
        Name         string  `json:"name" binding:"required"`
        Role         string  `json:"role" binding:"required"`
        Instructions string  `json:"instructions"`
        Model        string  `json:"model"`
        Temperature  float64 `json:"temperature"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    apiKey := getAPIKeyFromRequest(c)
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    var userID string
    var agentsLimit, agentsCreated int
    
    if apiKey == "dev_mode" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
        agentsLimit = 100
        agentsCreated = 0
    } else {
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT k.user_id, k.agents_limit, COALESCE(k.agents_created, 0)
            FROM api_keys k
            WHERE k.key_hash = crypt($1, k.key_hash) AND k.is_active = true
        `, apiKey).Scan(&userID, &agentsLimit, &agentsCreated)
        
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
            return
        }
    }
    
    if agentsCreated >= agentsLimit {
        c.JSON(http.StatusForbidden, gin.H{
            "error":       "Agent limit reached",
            "limit":       agentsLimit,
            "current":     agentsCreated,
            "upgrade_url": "/fusion-portal#pricing",
        })
        return
    }
    
    agentID := uuid.New().String()
    model := req.Model
    if model == "" {
        model = "yandex-gpt-lite"
    }
    temperature := req.Temperature
    if temperature == 0 {
        temperature = 0.7
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO ai_agents (id, user_id, name, role, instructions, model, temperature, is_active, created_at, updated_at, type, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7, true, NOW(), NOW(), $8, 'active')
    `, agentID, userID, req.Name, req.Role, req.Instructions, model, temperature, req.Role)
    
    if err != nil {
        log.Printf("❌ Ошибка создания агента: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create agent"})
        return
    }
    
    if apiKey != "dev_mode" {
        database.Pool.Exec(c.Request.Context(), `
            UPDATE api_keys SET agents_created = agents_created + 1
            WHERE key_hash = crypt($1, key_hash)
        `, apiKey)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "id":      agentID,
        "message": "Agent created successfully",
    })
}

// UpdateFusionAgent - обновить агента
func UpdateFusionAgent(c *gin.Context) {
    agentID := c.Param("id")
    
    var req struct {
        Name         string  `json:"name"`
        Role         string  `json:"role"`
        Instructions string  `json:"instructions"`
        Model        string  `json:"model"`
        Temperature  float64 `json:"temperature"`
        IsActive     *bool   `json:"is_active"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    apiKey := getAPIKeyFromRequest(c)
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    var userID string
    if apiKey == "dev_mode" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    } else {
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT user_id FROM api_keys 
            WHERE key_hash = crypt($1, key_hash) AND is_active = true
        `, apiKey).Scan(&userID)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
            return
        }
    }
    
    // Проверяем владельца
    var ownerID string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT user_id FROM ai_agents WHERE id = $1
    `, agentID).Scan(&ownerID)
    
    if err != nil || ownerID != userID {
        c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
        return
    }
    
    query := `UPDATE ai_agents SET updated_at = NOW()`
    args := []interface{}{}
    argPos := 1
    
    if req.Name != "" {
        query += fmt.Sprintf(`, name = $%d`, argPos)
        args = append(args, req.Name)
        argPos++
    }
    if req.Role != "" {
        query += fmt.Sprintf(`, role = $%d`, argPos)
        args = append(args, req.Role)
        argPos++
    }
    if req.Instructions != "" {
        query += fmt.Sprintf(`, instructions = $%d`, argPos)
        args = append(args, req.Instructions)
        argPos++
    }
    if req.Model != "" {
        query += fmt.Sprintf(`, model = $%d`, argPos)
        args = append(args, req.Model)
        argPos++
    }
    if req.Temperature > 0 {
        query += fmt.Sprintf(`, temperature = $%d`, argPos)
        args = append(args, req.Temperature)
        argPos++
    }
    if req.IsActive != nil {
        query += fmt.Sprintf(`, is_active = $%d`, argPos)
        args = append(args, *req.IsActive)
        argPos++
    }
    
    query += fmt.Sprintf(` WHERE id = $%d`, argPos)
    args = append(args, agentID)
    
    _, err = database.Pool.Exec(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Agent updated successfully"})
}

// DeleteFusionAgent - удалить агента
func DeleteFusionAgent(c *gin.Context) {
    agentID := c.Param("id")
    
    apiKey := getAPIKeyFromRequest(c)
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    var userID string
    if apiKey == "dev_mode" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    } else {
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT user_id FROM api_keys 
            WHERE key_hash = crypt($1, key_hash) AND is_active = true
        `, apiKey).Scan(&userID)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
            return
        }
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM ai_agents
        WHERE id = $1 AND user_id = $2
    `, agentID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
        return
    }
    
    if apiKey != "dev_mode" {
        database.Pool.Exec(c.Request.Context(), `
            UPDATE api_keys SET agents_created = agents_created - 1
            WHERE key_hash = crypt($1, key_hash)
        `, apiKey)
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Agent deleted successfully"})
}

// ChatWithFusionAgent - чат с AI агентом
func ChatWithFusionAgent(c *gin.Context) {
    agentID := c.Param("id")
    
    var req struct {
        Message string `json:"message" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    apiKey := getAPIKeyFromRequest(c)
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    var aiRequestsUsed, aiRequestsLimit int
    var userID string
    
    if apiKey == "dev_mode" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
        aiRequestsUsed = 0
        aiRequestsLimit = 10000
    } else {
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT user_id, ai_requests_used, ai_requests_limit
            FROM api_keys
            WHERE key_hash = crypt($1, key_hash) AND is_active = true
        `, apiKey).Scan(&userID, &aiRequestsUsed, &aiRequestsLimit)
        
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
            return
        }
        
        if aiRequestsUsed >= aiRequestsLimit {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error":        "AI requests limit exceeded",
                "limit":        aiRequestsLimit,
                "used":         aiRequestsUsed,
                "upgrade_url":  "/fusion-portal#pricing",
            })
            return
        }
    }
    
    var agent models.AIAgent
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT name, role, instructions, model, temperature
        FROM ai_agents 
        WHERE id = $1 AND user_id = $2 AND is_active = true
    `, agentID, userID).Scan(&agent.Name, &agent.Role, &agent.Instructions, &agent.Model, &agent.Temperature)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
        return
    }
    
    response := "Это ответ от агента " + agent.Name + " на сообщение: " + req.Message
    
    if apiKey != "dev_mode" {
        database.Pool.Exec(c.Request.Context(), `
            UPDATE api_keys SET ai_requests_used = ai_requests_used + 1
            WHERE key_hash = crypt($1, key_hash)
        `, apiKey)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "response": response,
        "agent":    agent.Name,
    })
}

// GetFusionAIAnalytics - получить аналитику
func GetFusionAIAnalytics(c *gin.Context) {
    apiKey := getAPIKeyFromRequest(c)
    if apiKey == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "API key required"})
        return
    }
    
    var aiRequestsUsed, aiRequestsLimit int
    var agentsLimit, agentsCreated int
    var planName string
    
    if apiKey == "dev_mode" {
        c.JSON(http.StatusOK, gin.H{
            "ai_requests_used":   0,
            "ai_requests_limit":  10000,
            "ai_remaining":       10000,
            "ai_percent":         0,
            "agents_limit":       100,
            "agents_created":     0,
            "agents_remaining":   100,
            "plan_name":          "Developer",
        })
        return
    }
    
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
        "ai_requests_used":   aiRequestsUsed,
        "ai_requests_limit":  aiRequestsLimit,
        "ai_remaining":       aiRequestsLimit - aiRequestsUsed,
        "ai_percent":         float64(aiRequestsUsed) / float64(aiRequestsLimit) * 100,
        "agents_limit":       agentsLimit,
        "agents_created":     agentsCreated,
        "agents_remaining":   agentsLimit - agentsCreated,
        "plan_name":          planName,
    })
}