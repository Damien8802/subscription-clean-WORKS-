package models

import (
"context"
"encoding/json"
"fmt"
"subscription-system/database"
)

type Plan struct {
ID            int             `json:"id" db:"id"`
Name          string          `json:"name" db:"name"`
Code          string          `json:"code" db:"code"`
Description   string          `json:"description" db:"description"`
PriceMonthly  float64         `json:"price_monthly" db:"price_monthly"`
PriceYearly   float64         `json:"price_yearly" db:"price_yearly"`
Currency      string          `json:"currency" db:"currency"`
Features      json.RawMessage `json:"features" db:"features"`
MaxUsers      int             `json:"max_users" db:"max_users"`
IsActive      bool            `json:"is_active" db:"is_active"`
SortOrder     int             `json:"sort_order" db:"sort_order"`
AIQuota       int64           `json:"ai_quota" db:"ai_quota"`
AIModels      json.RawMessage `json:"ai_models" db:"ai_models"`
AITokensUsed  int64           `json:"ai_tokens_used" db:"ai_tokens_used"` // добавлено
}

// GetAllActivePlans возвращает все активные планы, отсортированные по порядку
func GetAllActivePlans() ([]Plan, error) {
rows, err := database.Pool.Query(context.Background(), `
SELECT id, name, code, description, price_monthly, price_yearly, currency, features, max_users, is_active, sort_order, ai_quota, ai_models
FROM subscription_plans
WHERE is_active = true
ORDER BY sort_order ASC
`)
if err != nil {
return nil, err
}
defer rows.Close()

var plans []Plan
for rows.Next() {
var p Plan
err := rows.Scan(&p.ID, &p.Name, &p.Code, &p.Description, &p.PriceMonthly, &p.PriceYearly,
&p.Currency, &p.Features, &p.MaxUsers, &p.IsActive, &p.SortOrder, &p.AIQuota, &p.AIModels)
if err != nil {
return nil, err
}
plans = append(plans, p)
}
return plans, nil
}

// GetPlanByID получает план по ID
func GetPlanByID(id int) (*Plan, error) {
var p Plan
err := database.Pool.QueryRow(context.Background(), `
SELECT id, name, code, description, price_monthly, price_yearly, currency, features, max_users, is_active, sort_order, ai_quota, ai_models
FROM subscription_plans
WHERE id = $1 AND is_active = true
`, id).Scan(&p.ID, &p.Name, &p.Code, &p.Description, &p.PriceMonthly, &p.PriceYearly,
&p.Currency, &p.Features, &p.MaxUsers, &p.IsActive, &p.SortOrder, &p.AIQuota, &p.AIModels)
if err != nil {
return nil, err
}
return &p, nil
}

// GetAllPlans возвращает ВСЕ планы (не только активные) – для админки
func GetAllPlans() ([]Plan, error) {
rows, err := database.Pool.Query(context.Background(), `
SELECT id, name, code, description, price_monthly, price_yearly, currency, features, max_users, is_active, sort_order, ai_quota, ai_models
FROM subscription_plans
ORDER BY sort_order ASC, id ASC
`)
if err != nil {
return nil, err
}
defer rows.Close()

var plans []Plan
for rows.Next() {
var p Plan
err := rows.Scan(&p.ID, &p.Name, &p.Code, &p.Description, &p.PriceMonthly, &p.PriceYearly,
&p.Currency, &p.Features, &p.MaxUsers, &p.IsActive, &p.SortOrder, &p.AIQuota, &p.AIModels)
if err != nil {
return nil, err
}
plans = append(plans, p)
}
return plans, nil
}

// CreatePlan создаёт новый план
func CreatePlan(name, code, description string, priceMonthly, priceYearly float64, currency string,
features []string, maxUsers int, isActive bool, sortOrder int, aiQuota int64, aiModels []string) (*Plan, error) {
featuresJSON, err := json.Marshal(features)
if err != nil {
return nil, err
}
aiModelsJSON, err := json.Marshal(aiModels)
if err != nil {
return nil, err
}
var p Plan
query := `
INSERT INTO subscription_plans (name, code, description, price_monthly, price_yearly, currency, features, max_users, is_active, sort_order, ai_quota, ai_models)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, name, code, description, price_monthly, price_yearly, currency, features, max_users, is_active, sort_order, ai_quota, ai_models
`
err = database.Pool.QueryRow(context.Background(), query,
name, code, description, priceMonthly, priceYearly, currency, featuresJSON, maxUsers, isActive, sortOrder, aiQuota, aiModelsJSON,
).Scan(&p.ID, &p.Name, &p.Code, &p.Description, &p.PriceMonthly, &p.PriceYearly,
&p.Currency, &p.Features, &p.MaxUsers, &p.IsActive, &p.SortOrder, &p.AIQuota, &p.AIModels)
if err != nil {
return nil, err
}
return &p, nil
}

// UpdatePlan обновляет план
func UpdatePlan(id int, name, code, description string, priceMonthly, priceYearly float64, currency string,
features []string, maxUsers int, isActive bool, sortOrder int, aiQuota int64, aiModels []string) error {
featuresJSON, err := json.Marshal(features)
if err != nil {
return err
}
aiModelsJSON, err := json.Marshal(aiModels)
if err != nil {
return err
}
_, err = database.Pool.Exec(context.Background(), `
UPDATE subscription_plans SET
name = $1, code = $2, description = $3,
price_monthly = $4, price_yearly = $5, currency = $6,
features = $7, max_users = $8, is_active = $9, sort_order = $10,
ai_quota = $11, ai_models = $12,
updated_at = NOW()
WHERE id = $13
`, name, code, description, priceMonthly, priceYearly, currency, featuresJSON, maxUsers, isActive, sortOrder,
aiQuota, aiModelsJSON, id)
return err
}

// DeletePlan удаляет план, если на него нет активных подписок
func DeletePlan(id int) error {
var count int64
err := database.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM user_subscriptions
WHERE plan_id = $1 AND status = 'active'
`, id).Scan(&count)
if err != nil {
return err
}
if count > 0 {
return fmt.Errorf("cannot delete plan with active subscriptions")
}
_, err = database.Pool.Exec(context.Background(), `DELETE FROM subscription_plans WHERE id = $1`, id)
return err
}

// GetUserActivePlan возвращает активный план пользователя и использованные токены AI
func GetUserActivePlan(userID string) (*Plan, error) {
var plan Plan
query := `
SELECT p.id, p.name, p.code, p.description, p.price_monthly, p.price_yearly, p.currency,
       p.features, p.max_users, p.is_active, p.sort_order, p.ai_quota, p.ai_models,
       COALESCE(s.ai_tokens_used, 0) as ai_tokens_used
FROM user_subscriptions s
JOIN subscription_plans p ON s.plan_id = p.id
WHERE s.user_id = $1 AND s.status = 'active' AND s.current_period_end > NOW()
LIMIT 1
`
err := database.Pool.QueryRow(context.Background(), query, userID).Scan(
&plan.ID, &plan.Name, &plan.Code, &plan.Description,
&plan.PriceMonthly, &plan.PriceYearly, &plan.Currency,
&plan.Features, &plan.MaxUsers, &plan.IsActive, &plan.SortOrder,
&plan.AIQuota, &plan.AIModels, &plan.AITokensUsed,
)
if err != nil {
return nil, err
}
return &plan, nil
}
