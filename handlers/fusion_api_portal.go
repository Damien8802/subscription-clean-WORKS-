package handlers

import (
    "log" 
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
    "subscription-system/database"
)

// FusionAPIPortalHandler - главная страница API портала
func FusionAPIPortalHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "fusion_api_portal.html", gin.H{
        "title":       "FusionAPI — Портал разработчика",
        "brand":       "FusionAPI",
        "description": "Единый API для всех сервисов вашего бизнеса",
    })
}

func GetMyAPIKey(c *gin.Context) {
    userID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    
    var keyID string
    var keyHash string
    var planID *string
    var requestsLimit, requestsUsed int
    var lastShownAt *time.Time
    var rawKey string
    
    // Ищем существующий активный ключ
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, key_hash, plan_id, COALESCE(requests_limit, 1000), COALESCE(requests_used, 0), last_shown_at
        FROM api_keys WHERE user_id = $1 AND is_active = true LIMIT 1
    `, userID).Scan(&keyID, &keyHash, &planID, &requestsLimit, &requestsUsed, &lastShownAt)

    if err != nil {
        // Ключа нет - создаём новый
        newKeyID := uuid.New().String()
        rawKey = "fus_" + uuid.New().String()
        hashBytes, _ := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)

        var freePlanID string
        database.Pool.QueryRow(c.Request.Context(), `SELECT id FROM api_plans WHERE code = 'free'`).Scan(&freePlanID)

        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO api_keys (id, user_id, name, key_hash, is_active, created_at, updated_at, plan_id, requests_limit, requests_used, last_shown_at)
            VALUES ($1, $2, $3, $4, true, NOW(), NOW(), $5, 1000, 0, NOW())
        `, newKeyID, userID, "FusionAPI Key", string(hashBytes), freePlanID)

        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать ключ"})
            return
        }

        // Показываем ключ ПОЛНОСТЬЮ (первый и единственный раз)
        maskedKey := rawKey
        
        var planName, planCode string
        database.Pool.QueryRow(c.Request.Context(), `SELECT name, code FROM api_plans WHERE id = $1`, freePlanID).Scan(&planName, &planCode)
        
        c.JSON(http.StatusOK, gin.H{
            "key": maskedKey,
            "full_key": rawKey,
            "plan_name": planName,
            "plan_code": planCode,
            "requests_used": 0,
            "requests_limit": 1000,
            "is_first_time": true,  // Флаг что показан полный ключ
        })
        return
    }

    // Ключ существует - проверяем, показывали ли его уже
    var planName, planCode string
    if planID != nil {
        database.Pool.QueryRow(c.Request.Context(), `SELECT name, code FROM api_plans WHERE id = $1`, *planID).Scan(&planName, &planCode)
    } else {
        planName, planCode = "Free", "free"
    }

    var displayKey string
    var isFirstTime bool
    
    if lastShownAt == nil {
        // Ключ ещё не показывали - показываем ПОЛНОСТЬЮ (но такого не должно быть, т.к. при создании ставим NOW())
        // Восстановить полный ключ из хеша невозможно, поэтому предлагаем создать новый
        displayKey = "🔑 Нажмите 'Новый ключ' чтобы получить ключ"
        isFirstTime = false
    } else {
        // Ключ уже показывали - показываем ТОЛЬКО начало и конец
        // Генерируем маскированный ключ на основе ID
        displayKey = "fus_" + keyID[:4] + "********" + keyID[len(keyID)-4:]
        isFirstTime = false
    }
    
    c.JSON(http.StatusOK, gin.H{
        "key": displayKey,
        "plan_name": planName,
        "plan_code": planCode,
        "requests_used": requestsUsed,
        "requests_limit": requestsLimit,
        "is_first_time": isFirstTime,
    })
}
// GetAPIUsageStats - статистика использования API
func GetAPIUsageStats(c *gin.Context) {
    userID := c.GetString("user_id")
    
    var requestsUsed, requestsLimit int
    var planName string
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(k.requests_used, 0), COALESCE(k.requests_limit, 1000), COALESCE(p.name, 'Free')
        FROM api_keys k
        LEFT JOIN api_plans p ON k.plan_id = p.id
        WHERE k.user_id = $1 AND k.is_active = true
        LIMIT 1
    `, userID).Scan(&requestsUsed, &requestsLimit, &planName)
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "requests_used":  0,
            "requests_limit": 1000,
            "plan_name":      "Free",
            "remaining":      1000,
            "remaining_percent": 100.0,
            "reset_days":     30,
        })
        return
    }
    
    remaining := requestsLimit - requestsUsed
    remainingPercent := float64(remaining) / float64(requestsLimit) * 100
    
    now := time.Now()
    nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
    resetDays := int(nextMonth.Sub(now).Hours() / 24)
    
    c.JSON(http.StatusOK, gin.H{
        "requests_used":     requestsUsed,
        "requests_limit":    requestsLimit,
        "plan_name":         planName,
        "remaining":         remaining,
        "remaining_percent": remainingPercent,
        "reset_days":        resetDays,
    })
}

func RegenerateAPIKey(c *gin.Context) {
    userID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    
    log.Printf("🔄 RegenerateAPIKey для userID: %s", userID)
    
    // Деактивируем старые ключи
    database.Pool.Exec(c.Request.Context(), 
        `UPDATE api_keys SET is_active = false WHERE user_id = $1`, userID)

    // Получаем план пользователя
    var planID string
    var requestsLimit int
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(plan_id, (SELECT id FROM api_plans WHERE code = 'free')), COALESCE(requests_limit, 1000)
        FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1
    `, userID).Scan(&planID, &requestsLimit)
    
    if err != nil {
        database.Pool.QueryRow(c.Request.Context(), `SELECT id FROM api_plans WHERE code = 'free'`).Scan(&planID)
        requestsLimit = 1000
    }

    // Создаём новый ключ
    newKeyID := uuid.New().String()
    rawKey := "fus_" + uuid.New().String()
    hashBytes, _ := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)

    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO api_keys (id, user_id, name, key_hash, is_active, created_at, updated_at, plan_id, requests_limit, requests_used, last_shown_at)
        VALUES ($1, $2, $3, $4, true, NOW(), NOW(), $5, $6, 0, NOW())
    `, newKeyID, userID, "FusionAPI Key", string(hashBytes), planID, requestsLimit)

    if err != nil {
        log.Printf("❌ Ошибка создания ключа: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create new key"})
        return
    }

    log.Printf("✅ Новый API ключ создан: %s", rawKey)
    
    // Показываем ПОЛНЫЙ ключ (первый и единственный раз)
    c.JSON(http.StatusOK, gin.H{
        "key": rawKey,
        "full_key": rawKey,
        "message": "API key regenerated successfully",
        "is_first_time": true,
    })
}// GetAPIPlans - получение списка тарифов
func GetAPIPlans(c *gin.Context) {
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, code, COALESCE(description, ''), requests_limit, price_monthly, price_yearly,
               COALESCE(has_support, false), COALESCE(has_sla, false), COALESCE(has_webhooks, false), 
               COALESCE(rate_limit, 60), COALESCE(badge_color, '#667eea'), sort_order
        FROM api_plans WHERE is_active = true ORDER BY sort_order
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load plans"})
        return
    }
    defer rows.Close()
    
    var plans []gin.H
    for rows.Next() {
        var id, name, code, description, badgeColor string
        var requestsLimit, rateLimit, sortOrder int
        var priceMonthly, priceYearly float64
        var hasSupport, hasSLA, hasWebhooks bool
        
        rows.Scan(&id, &name, &code, &description, &requestsLimit, &priceMonthly, &priceYearly,
            &hasSupport, &hasSLA, &hasWebhooks, &rateLimit, &badgeColor, &sortOrder)
        
        plans = append(plans, gin.H{
            "id":             id,
            "name":           name,
            "code":           code,
            "description":    description,
            "requests_limit": requestsLimit,
            "price_monthly":  priceMonthly,
            "price_yearly":   priceYearly,
            "has_support":    hasSupport,
            "has_sla":        hasSLA,
            "has_webhooks":   hasWebhooks,
            "rate_limit":     rateLimit,
            "badge_color":    badgeColor,
        })
    }
    
    c.JSON(http.StatusOK, plans)
}

// APIPlanUpgradeRequest - запрос на апгрейд тарифа
func APIPlanUpgradeRequest(c *gin.Context) {
    var req struct {
        PlanCode string `json:"plan_code" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "plan_code required"})
        return
    }
    
    userID := c.GetString("user_id")
    
    var planID, planName string
    var requestsLimit int
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, name, requests_limit FROM api_plans WHERE code = $1 AND is_active = true
    `, req.PlanCode).Scan(&planID, &planName, &requestsLimit)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
        return
    }
    
    _, err = database.Pool.Exec(c.Request.Context(), `
        UPDATE api_keys SET plan_id = $1, requests_limit = $2, updated_at = NOW()
        WHERE user_id = $3 AND is_active = true
    `, planID, requestsLimit, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade plan"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":   "Plan upgraded successfully",
        "plan_name": planName,
        "plan_code": req.PlanCode,
        "new_limit": requestsLimit,
    })
}

// GetAPIDocumentation - получение документации API
func GetAPIDocumentation(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "info": gin.H{
            "name":        "FusionAPI",
            "version":     "v1",
            "description": "Unified API for CRM, FinCore, TeamSphere and Cloud services",
        },
        "authentication": gin.H{
            "method":  "X-API-Key header",
            "example": `curl -X GET /api/v1/crm/customers -H "X-API-Key: your_key"`,
        },
        "endpoints": []gin.H{
            {"method": "GET", "path": "/api/v1/crm/customers", "description": "Get customers"},
            {"method": "POST", "path": "/api/v1/crm/deals", "description": "Create deal"},
            {"method": "GET", "path": "/api/v1/fincore/entries", "description": "Get accounting entries"},
        },
    })
}
