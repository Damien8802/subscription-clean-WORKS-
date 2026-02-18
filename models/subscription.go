package models

import (
"context"
"time"
"subscription-system/database"
)

type Subscription struct {
ID                 string    `json:"id" db:"id"`
UserID             string    `json:"user_id" db:"user_id"`
PlanID             int       `json:"plan_id" db:"plan_id"`
Status             string    `json:"status" db:"status"`
CurrentPeriodStart time.Time `json:"current_period_start" db:"current_period_start"`
CurrentPeriodEnd   time.Time `json:"current_period_end" db:"current_period_end"`
CancelAtPeriodEnd  bool      `json:"cancel_at_period_end" db:"cancel_at_period_end"`
TrialEnd           *time.Time `json:"trial_end" db:"trial_end"`
CreatedAt          time.Time `json:"created_at" db:"created_at"`
UpdatedAt          time.Time `json:"updated_at" db:"updated_at"`
// Присоединённые данные плана (опционально)
PlanName        string  `json:"plan_name,omitempty" db:"plan_name"`
PlanPriceMonthly float64 `json:"plan_price_monthly,omitempty" db:"plan_price_monthly"`
PlanCurrency    string  `json:"plan_currency,omitempty" db:"plan_currency"`
}

// CreateSubscription создаёт новую подписку для пользователя
func CreateSubscription(userID string, planID int, periodMonths int) (*Subscription, error) {
var sub Subscription
now := time.Now()
periodEnd := now.AddDate(0, periodMonths, 0) // добавляем месяцы

query := `
INSERT INTO user_subscriptions (user_id, plan_id, current_period_start, current_period_end)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, plan_id, status, current_period_start, current_period_end, cancel_at_period_end, trial_end, created_at, updated_at
`
err := database.Pool.QueryRow(context.Background(), query, userID, planID, now, periodEnd).Scan(
&sub.ID, &sub.UserID, &sub.PlanID, &sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
&sub.CancelAtPeriodEnd, &sub.TrialEnd, &sub.CreatedAt, &sub.UpdatedAt,
)
if err != nil {
return nil, err
}
return &sub, nil
}

// GetUserSubscriptions возвращает все подписки пользователя с информацией о плане
func GetUserSubscriptions(userID string) ([]Subscription, error) {
rows, err := database.Pool.Query(context.Background(), `
SELECT 
s.id, s.user_id, s.plan_id, s.status, s.current_period_start, s.current_period_end,
s.cancel_at_period_end, s.trial_end, s.created_at, s.updated_at,
p.name as plan_name, p.price_monthly as plan_price_monthly, p.currency as plan_currency
FROM user_subscriptions s
JOIN subscription_plans p ON s.plan_id = p.id
WHERE s.user_id = $1
ORDER BY s.created_at DESC
`, userID)
if err != nil {
return nil, err
}
defer rows.Close()

var subs []Subscription
for rows.Next() {
var s Subscription
err := rows.Scan(
&s.ID, &s.UserID, &s.PlanID, &s.Status, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
&s.CancelAtPeriodEnd, &s.TrialEnd, &s.CreatedAt, &s.UpdatedAt,
&s.PlanName, &s.PlanPriceMonthly, &s.PlanCurrency,
)
if err != nil {
return nil, err
}
subs = append(subs, s)
}
return subs, nil
}
