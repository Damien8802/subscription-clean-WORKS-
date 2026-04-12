package handlers

import (
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// GetUserServices - получить доступные услуги пользователя
func GetUserServices(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT s.service_code, s.name, s.description, s.price_month, 
               COALESCE(uss.status, 'inactive') as status, 
               uss.expires_at
        FROM services s
        LEFT JOIN user_service_subscriptions uss ON s.service_code = uss.service_code AND uss.user_id = $1
        WHERE s.is_active = true
        ORDER BY s.sort_order
    `, userID)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"services": []interface{}{}})
        return
    }
    defer rows.Close()

    var services []gin.H
    for rows.Next() {
        var code, name, description, status string
        var priceMonth float64
        var expiresAt *time.Time
        rows.Scan(&code, &name, &description, &priceMonth, &status, &expiresAt)
        
        isActive := status == "active"
        var expiresStr string
        if expiresAt != nil {
            expiresStr = expiresAt.Format("2006-01-02")
        }
        
        services = append(services, gin.H{
            "code":        code,
            "name":        name,
            "description": description,
            "price":       priceMonth,
            "is_active":   isActive,
            "expires_at":  expiresStr,
        })
    }

    c.JSON(http.StatusOK, gin.H{"services": services})
}

// SubscribeToService - подписка на услугу
func SubscribeToService(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        ServiceCode string `json:"service_code" binding:"required"`
        Period      string `json:"period"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.Period == "" {
        req.Period = "month"
    }

    // Получаем цену услуги
    var price float64
    var serviceName string
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT price_month, name FROM services WHERE service_code = $1
    `, req.ServiceCode).Scan(&price, &serviceName)

    expiresAt := time.Now().AddDate(0, 1, 0)
    if req.Period == "year" {
        price *= 12
        expiresAt = time.Now().AddDate(1, 0, 0)
    }

    // Создаем или обновляем подписку
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO user_service_subscriptions (user_id, service_code, status, started_at, expires_at)
        VALUES ($1, $2, 'active', NOW(), $3)
        ON CONFLICT (user_id, service_code) 
        DO UPDATE SET status = 'active', expires_at = $3, updated_at = NOW()
    `, userID, req.ServiceCode, expiresAt)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "message":    "✅ Подписка оформлена!",
        "service":    serviceName,
        "expires_at": expiresAt.Format("2006-01-02"),
    })
}
