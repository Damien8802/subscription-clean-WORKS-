package models

import (
    "context"
    "encoding/json"	
    "subscription-system/database"
    "time"
)

type Plan struct {
    ID              int             `json:"id" db:"id"`
    Name            string          `json:"name" db:"name"`
    Code            string          `json:"code" db:"code"`
    Description     string          `json:"description" db:"description"`
    PriceMonthly    float64         `json:"price_monthly" db:"price_monthly"`
    PriceYearly     float64         `json:"price_yearly" db:"price_yearly"`
    Currency        string          `json:"currency" db:"currency"`
    Features        json.RawMessage `json:"features" db:"features"`
    AICapabilities  json.RawMessage `json:"ai_capabilities" db:"ai_capabilities"`
    MaxUsers        int             `json:"max_users" db:"max_users"`
    IsActive        bool            `json:"is_active" db:"is_active"`
    SortOrder       int             `json:"sort_order" db:"sort_order"`
    CreatedAt       time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

// TableName возвращает имя таблицы
func (Plan) TableName() string {
    return "subscription_plans"
}

// GetAllPlans возвращает все активные тарифы
func GetAllPlans() ([]Plan, error) {
    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, name, code, description, price_monthly, price_yearly, 
               currency, features, ai_capabilities, max_users, is_active, 
               sort_order, created_at, updated_at
        FROM subscription_plans
        WHERE is_active = true
        ORDER BY sort_order
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var plans []Plan
    for rows.Next() {
        var p Plan
        err := rows.Scan(
            &p.ID, &p.Name, &p.Code, &p.Description, 
            &p.PriceMonthly, &p.PriceYearly, &p.Currency,
            &p.Features, &p.AICapabilities, &p.MaxUsers,
            &p.IsActive, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        plans = append(plans, p)
    }
    return plans, nil
}

// GetPlanByCode возвращает тариф по коду
func GetPlanByCode(code string) (*Plan, error) {
    var p Plan
    err := database.Pool.QueryRow(context.Background(), `
        SELECT id, name, code, description, price_monthly, price_yearly, 
               currency, features, ai_capabilities, max_users, is_active, 
               sort_order, created_at, updated_at
        FROM subscription_plans
        WHERE code = $1 AND is_active = true
    `, code).Scan(
        &p.ID, &p.Name, &p.Code, &p.Description, 
        &p.PriceMonthly, &p.PriceYearly, &p.Currency,
        &p.Features, &p.AICapabilities, &p.MaxUsers,
        &p.IsActive, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt,
    )
    if err != nil {
        return nil, err
    }
    return &p, nil
}

// GetAICapabilities возвращает AI-возможности тарифа как map
func (p *Plan) GetAICapabilities() map[string]interface{} {
    var caps map[string]interface{}
    if err := json.Unmarshal(p.AICapabilities, &caps); err != nil {
        // Возвращаем значения по умолчанию если не удалось распарсить
        return map[string]interface{}{
            "model":        "basic",
            "max_requests": float64(50),
            "file_upload":  false,
            "voice_input":  false,
        }
    }
    return caps
}

// CanUseFeature проверяет, доступна ли конкретная AI-функция
func (p *Plan) CanUseFeature(feature string) bool {
    caps := p.GetAICapabilities()
    if val, ok := caps[feature]; ok {
        if boolVal, ok := val.(bool); ok {
            return boolVal
        }
    }
    return false
}

// GetMaxRequests возвращает максимальное количество запросов
func (p *Plan) GetMaxRequests() int64 {
    caps := p.GetAICapabilities()
    if val, ok := caps["max_requests"]; ok {
        switch v := val.(type) {
        case float64:
            return int64(v)
        case int64:
            return v
        }
    }
    return 50 // значение по умолчанию
}

// GetModel возвращает название модели AI
func (p *Plan) GetModel() string {
    caps := p.GetAICapabilities()
    if val, ok := caps["model"]; ok {
        if str, ok := val.(string); ok {
            return str
        }
    }
    return "basic"
}