package handlers

import (
    "encoding/json"
    "net/http"
    "strconv"
    "subscription-system/database"
    "subscription-system/models"
    "github.com/gin-gonic/gin"
)

// AdminGetPlansHandler возвращает все планы (для админки)
func AdminGetPlansHandler(c *gin.Context) {
    plans, err := models.GetAllPlans()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"plans": plans})
}

// AdminCreatePlanHandler создаёт новый план
func AdminCreatePlanHandler(c *gin.Context) {
    var req struct {
        Name         string   `json:"name" binding:"required"`
        Code         string   `json:"code" binding:"required"`
        Description  string   `json:"description"`
        PriceMonthly float64  `json:"price_monthly" binding:"required"`
        PriceYearly  float64  `json:"price_yearly" binding:"required"`
        Currency     string   `json:"currency" binding:"required"`
        Features     []string `json:"features"`
        MaxUsers     int      `json:"max_users"`
        AIQuota      int64    `json:"ai_quota"`
        AIModels     []string `json:"ai_models"`
        IsActive     bool     `json:"is_active"`
        SortOrder    int      `json:"sort_order"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    featuresJSON, _ := json.Marshal(req.Features)
    aiModelsJSON, _ := json.Marshal(req.AIModels)

    var id int
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO subscription_plans 
            (name, code, description, price_monthly, price_yearly, currency, features, max_users, ai_quota, ai_models, is_active, sort_order)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        RETURNING id
    `, req.Name, req.Code, req.Description, req.PriceMonthly, req.PriceYearly, req.Currency,
        featuresJSON, req.MaxUsers, req.AIQuota, aiModelsJSON, req.IsActive, req.SortOrder).Scan(&id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    c.JSON(http.StatusCreated, gin.H{"id": id})
}

// AdminUpdatePlanHandler обновляет существующий план
func AdminUpdatePlanHandler(c *gin.Context) {
    idParam := c.Param("id")
    planID, err := strconv.Atoi(idParam)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
        return
    }

    var req struct {
        Name         string   `json:"name"`
        Code         string   `json:"code"`
        Description  string   `json:"description"`
        PriceMonthly float64  `json:"price_monthly"`
        PriceYearly  float64  `json:"price_yearly"`
        Currency     string   `json:"currency"`
        Features     []string `json:"features"`
        MaxUsers     int      `json:"max_users"`
        AIQuota      int64    `json:"ai_quota"`
        AIModels     []string `json:"ai_models"`
        IsActive     *bool    `json:"is_active"`
        SortOrder    int      `json:"sort_order"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    featuresJSON, _ := json.Marshal(req.Features)
    aiModelsJSON, _ := json.Marshal(req.AIModels)

    query := `UPDATE subscription_plans SET updated_at = NOW()`
    args := []interface{}{}
    argPos := 1

    if req.Name != "" {
        query += `, name = $` + strconv.Itoa(argPos)
        args = append(args, req.Name)
        argPos++
    }
    if req.Code != "" {
        query += `, code = $` + strconv.Itoa(argPos)
        args = append(args, req.Code)
        argPos++
    }
    if req.Description != "" {
        query += `, description = $` + strconv.Itoa(argPos)
        args = append(args, req.Description)
        argPos++
    }
    if req.PriceMonthly != 0 {
        query += `, price_monthly = $` + strconv.Itoa(argPos)
        args = append(args, req.PriceMonthly)
        argPos++
    }
    if req.PriceYearly != 0 {
        query += `, price_yearly = $` + strconv.Itoa(argPos)
        args = append(args, req.PriceYearly)
        argPos++
    }
    if req.Currency != "" {
        query += `, currency = $` + strconv.Itoa(argPos)
        args = append(args, req.Currency)
        argPos++
    }
    if req.Features != nil {
        query += `, features = $` + strconv.Itoa(argPos)
        args = append(args, featuresJSON)
        argPos++
    }
    if req.MaxUsers != 0 {
        query += `, max_users = $` + strconv.Itoa(argPos)
        args = append(args, req.MaxUsers)
        argPos++
    }
    if req.AIQuota != 0 {
        query += `, ai_quota = $` + strconv.Itoa(argPos)
        args = append(args, req.AIQuota)
        argPos++
    }
    if req.AIModels != nil {
        query += `, ai_models = $` + strconv.Itoa(argPos)
        args = append(args, aiModelsJSON)
        argPos++
    }
    if req.IsActive != nil {
        query += `, is_active = $` + strconv.Itoa(argPos)
        args = append(args, *req.IsActive)
        argPos++
    }
    if req.SortOrder != 0 {
        query += `, sort_order = $` + strconv.Itoa(argPos)
        args = append(args, req.SortOrder)
        argPos++
    }

    query += ` WHERE id = $` + strconv.Itoa(argPos)
    args = append(args, planID)

    _, err = database.Pool.Exec(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "plan updated"})
}

// AdminDeletePlanHandler удаляет план, если нет активных подписок
func AdminDeletePlanHandler(c *gin.Context) {
    idParam := c.Param("id")
    planID, err := strconv.Atoi(idParam)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
        return
    }

    // Проверяем, есть ли активные подписки на этот план
    var count int64
    err = database.Pool.QueryRow(c.Request.Context(),
        `SELECT COUNT(*) FROM user_subscriptions WHERE plan_id = $1 AND status = 'active'`, planID).Scan(&count)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    if count > 0 {
        c.JSON(http.StatusConflict, gin.H{"error": "cannot delete plan with active subscriptions"})
        return
    }

    _, err = database.Pool.Exec(c.Request.Context(), `DELETE FROM subscription_plans WHERE id = $1`, planID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "plan deleted"})
}
