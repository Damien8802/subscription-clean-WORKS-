package models

import (
"context"
"fmt"
"subscription-system/database"
"time"
)

// AdminSubscription – подписка с данными пользователя и тарифа для админ-панели
type AdminSubscription struct {
ID                 string    `json:"id"`
UserID             string    `json:"user_id"`
UserEmail          string    `json:"user_email"`
UserName           string    `json:"user_name"`
PlanID             int       `json:"plan_id"`
PlanName           string    `json:"plan_name"`
PlanPrice          float64   `json:"plan_price"`
Status             string    `json:"status"`
CurrentPeriodStart time.Time `json:"current_period_start"`
CurrentPeriodEnd   time.Time `json:"current_period_end"`
CancelAtPeriodEnd  bool      `json:"cancel_at_period_end"`
CreatedAt          time.Time `json:"created_at"`
}

// GetAdminSubscriptions возвращает список подписок с пагинацией, поиском и фильтром по статусу
func GetAdminSubscriptions(search, status string, limit, offset int) ([]AdminSubscription, int64, error) {
var total int64
var subs []AdminSubscription

// Базовый запрос для подсчёта
countQuery := `
SELECT COUNT(DISTINCT s.id)
FROM user_subscriptions s
JOIN users u ON s.user_id = u.id
JOIN subscription_plans p ON s.plan_id = p.id
WHERE 1=1
`
var countArgs []interface{}
if search != "" {
countQuery += ` AND (u.email ILIKE $1 OR u.name ILIKE $1 OR p.name ILIKE $1)`
countArgs = append(countArgs, "%"+search+"%")
}
if status != "" {
countQuery += ` AND s.status = $` + fmt.Sprint(len(countArgs)+1)
countArgs = append(countArgs, status)
}
err := database.Pool.QueryRow(context.Background(), countQuery, countArgs...).Scan(&total)
if err != nil {
return nil, 0, err
}

// Запрос данных
query := `
SELECT 
s.id, s.user_id, u.email, u.name,
s.plan_id, p.name, p.price_monthly,
s.status, s.current_period_start, s.current_period_end,
s.cancel_at_period_end, s.created_at
FROM user_subscriptions s
JOIN users u ON s.user_id = u.id
JOIN subscription_plans p ON s.plan_id = p.id
WHERE 1=1
`
var args []interface{}
if search != "" {
query += ` AND (u.email ILIKE $1 OR u.name ILIKE $1 OR p.name ILIKE $1)`
args = append(args, "%"+search+"%")
}
if status != "" {
query += ` AND s.status = $` + fmt.Sprint(len(args)+1)
args = append(args, status)
}
query += ` ORDER BY s.created_at DESC`
query += ` OFFSET $` + fmt.Sprint(len(args)+1)
args = append(args, offset)
query += ` LIMIT $` + fmt.Sprint(len(args)+1)
args = append(args, limit)

rows, err := database.Pool.Query(context.Background(), query, args...)
if err != nil {
return nil, 0, err
}
defer rows.Close()

for rows.Next() {
var sub AdminSubscription
err := rows.Scan(
&sub.ID, &sub.UserID, &sub.UserEmail, &sub.UserName,
&sub.PlanID, &sub.PlanName, &sub.PlanPrice,
&sub.Status, &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
&sub.CancelAtPeriodEnd, &sub.CreatedAt,
)
if err != nil {
return nil, 0, err
}
subs = append(subs, sub)
}
return subs, total, nil
}

// AdminCancelSubscription отменяет подписку (устанавливает статус canceled или cancel_at_period_end = true)
func AdminCancelSubscription(subID string, immediate bool) error {
var query string
if immediate {
query = `UPDATE user_subscriptions SET status = 'canceled', updated_at = NOW() WHERE id = $1`
} else {
query = `UPDATE user_subscriptions SET cancel_at_period_end = true, updated_at = NOW() WHERE id = $1`
}
_, err := database.Pool.Exec(context.Background(), query, subID)
return err
}

// AdminReactivateSubscription отменяет запрос на отмену (если cancel_at_period_end = true)
func AdminReactivateSubscription(subID string) error {
_, err := database.Pool.Exec(context.Background(), `
UPDATE user_subscriptions 
SET cancel_at_period_end = false, updated_at = NOW() 
WHERE id = $1 AND cancel_at_period_end = true
`, subID)
return err
}

// GetSubscriptionStats возвращает статистику по статусам подписок
func GetSubscriptionStats() (map[string]int64, error) {
stats := make(map[string]int64)
rows, err := database.Pool.Query(context.Background(), `
SELECT status, COUNT(*) FROM user_subscriptions GROUP BY status
`)
if err != nil {
return nil, err
}
defer rows.Close()
for rows.Next() {
var status string
var count int64
err = rows.Scan(&status, &count)
if err != nil {
return nil, err
}
stats[status] = count
}
return stats, nil
}
