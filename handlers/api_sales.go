package handlers

import (
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
)

// Тарифы
type APIPlan struct {
    ID            int     `json:"id"`
    Name          string  `json:"name"`
    RequestsLimit int     `json:"requests_limit"`
    Price         float64 `json:"price"`
    Description   string  `json:"description"`
}

var apiPlans = []APIPlan{
    {
        ID:            1,
        Name:          "Бесплатный",
        RequestsLimit: 10,
        Price:         0,
        Description:   "10 запросов в день для тестирования",
    },
    {
        ID:            2,
        Name:          "Старт",
        RequestsLimit: 500,
        Price:         500,
        Description:   "500 запросов в день для небольших проектов",
    },
    {
        ID:            3,
        Name:          "Бизнес",
        RequestsLimit: 5000,
        Price:         3000,
        Description:   "5000 запросов в день + расширенная аналитика",
    },
    {
        ID:            4,
        Name:          "Корпоративный",
        RequestsLimit: 50000,
        Price:         15000,
        Description:   "50 000 запросов в день + поддержка 24/7",
    },
}

// Временная структура для хранения ключей в памяти (пока нет БД)
var tempKeys = make(map[string]struct {
    UserID        string
    PlanID        int
    RequestsToday int
    LastReset     time.Time
})

// ВРЕМЕННАЯ функция для получения тестового user_id
func getTestUserID(c *gin.Context) (string, bool) {
    // Используем существующего пользователя из логов
    testID := "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    fmt.Println("⚠️ Используем тестового пользователя:", testID)
    return testID, true
}

// Получить текущий план пользователя
func GetUserPlan(c *gin.Context) {
    userID, ok := getTestUserID(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    
    // Ищем ключ в памяти
    if keyData, exists := tempKeys[userID]; exists {
        var plan APIPlan
        for _, p := range apiPlans {
            if p.ID == keyData.PlanID {
                plan = p
                break
            }
        }
        
        // Сбрасываем счетчик если новый день
        if time.Since(keyData.LastReset).Hours() >= 24 {
            keyData.RequestsToday = 0
            keyData.LastReset = time.Now()
            tempKeys[userID] = keyData
        }
        
        c.JSON(http.StatusOK, gin.H{
            "has_key":        true,
            "key":            userID, // временно используем userID как ключ
            "plan":           plan,
            "requests_today": keyData.RequestsToday,
            "requests_left":  plan.RequestsLimit - keyData.RequestsToday,
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "has_key": false,
        "message": "У вас нет API ключа. Получите бесплатный для тестирования!",
    })
}

// Создать новый API ключ
func CreateAPIKey(c *gin.Context) {
    userID, ok := getTestUserID(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    
    var req struct {
        PlanID int `json:"plan_id"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    // Проверяем существование плана
    var selectedPlan APIPlan
    found := false
    for _, p := range apiPlans {
        if p.ID == req.PlanID {
            selectedPlan = p
            found = true
            break
        }
    }
    if !found {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Plan not found"})
        return
    }
    
    // Проверяем, нет ли уже ключа
    if _, exists := tempKeys[userID]; exists {
        c.JSON(http.StatusConflict, gin.H{
            "error":   "У вас уже есть активный ключ",
            "key":     userID,
            "message": "Хотите обновить тариф?",
        })
        return
    }
    
    // Сохраняем ключ в памяти
    tempKeys[userID] = struct {
        UserID        string
        PlanID        int
        RequestsToday int
        LastReset     time.Time
    }{
        UserID:        userID,
        PlanID:        selectedPlan.ID,
        RequestsToday: 0,
        LastReset:     time.Now(),
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message": "API ключ успешно создан!",
        "key":     userID,
        "plan":    selectedPlan,
    })
}

// Обновить тариф
func UpgradeAPIKey(c *gin.Context) {
    userID, ok := getTestUserID(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    
    var req struct {
        NewPlanID int `json:"new_plan_id"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    // Проверяем новый план
    var newPlan APIPlan
    found := false
    for _, p := range apiPlans {
        if p.ID == req.NewPlanID {
            newPlan = p
            found = true
            break
        }
    }
    if !found {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Plan not found"})
        return
    }
    
    // Находим текущий ключ
    keyData, exists := tempKeys[userID]
    if !exists {
        c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
        return
    }
    
    // Обновляем план
    keyData.PlanID = newPlan.ID
    tempKeys[userID] = keyData
    
    c.JSON(http.StatusOK, gin.H{
        "message":  fmt.Sprintf("Тариф обновлен на %s", newPlan.Name),
        "new_plan": newPlan,
    })
}

// Основной API эндпоинт для клиентов
func PublicSearchAPI(c *gin.Context) {
    // Получаем API ключ из заголовка
    apiKey := c.GetHeader("X-API-Key")
    if apiKey == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required. Get it at /api-sales"})
        return
    }
    
    // Проверяем ключ
    keyData, exists := tempKeys[apiKey]
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
        return
    }
    
    // Получаем план
    var plan APIPlan
    for _, p := range apiPlans {
        if p.ID == keyData.PlanID {
            plan = p
            break
        }
    }
    
    // Сбрасываем счетчик если новый день
    if time.Since(keyData.LastReset).Hours() >= 24 {
        keyData.RequestsToday = 0
        keyData.LastReset = time.Now()
        tempKeys[apiKey] = keyData
    }
    
    // Проверяем лимиты
    if keyData.RequestsToday >= plan.RequestsLimit {
        c.JSON(http.StatusTooManyRequests, gin.H{
            "error":       "Daily limit exceeded",
            "limit":       plan.RequestsLimit,
            "reset_in":    "24 hours",
            "upgrade_url": "/api-sales",
        })
        return
    }
    
    // Получаем запрос
    query := c.Query("q")
    if query == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
        return
    }
    
    // Ищем в интернете (используем ваши функции из ai.go)
    price := getAveragePrice(query)
    advice := getAdvice(query)
    
    // Увеличиваем счетчик
    keyData.RequestsToday++
    tempKeys[apiKey] = keyData
    
    // Формируем ответ
    response := gin.H{
        "success": true,
        "query":   query,
        "data": gin.H{
            "price":           price,
            "price_formatted": formatPrice(price),
            "advice":          advice,
        },
        "usage": gin.H{
            "today":     keyData.RequestsToday,
            "limit":     plan.RequestsLimit,
            "remaining": plan.RequestsLimit - keyData.RequestsToday,
        },
    }
    
    c.JSON(http.StatusOK, response)
}

// Получить статистику использования
func GetAPIUsage(c *gin.Context) {
    userID, ok := getTestUserID(c)
    if !ok {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }
    
    keyData, exists := tempKeys[userID]
    if !exists {
        c.JSON(http.StatusNotFound, gin.H{"error": "No API key found"})
        return
    }
    
    var plan APIPlan
    for _, p := range apiPlans {
        if p.ID == keyData.PlanID {
            plan = p
            break
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "api_key":        userID,
        "plan_id":        keyData.PlanID,
        "plan_name":      plan.Name,
        "requests_today": keyData.RequestsToday,
        "limit":          plan.RequestsLimit,
        "remaining":      plan.RequestsLimit - keyData.RequestsToday,
    })
}

// Страница продажи API
func APISalesPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "api-sales.html", gin.H{
        "plans": apiPlans,
    })
}