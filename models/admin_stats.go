package models

import (
"context"
"subscription-system/database"
)

type AdminStats struct {
TotalUsers         int64   `json:"total_users"`
NewUsersToday      int64   `json:"new_users_today"`
ActiveSubscriptions int64  `json:"active_subscriptions"`
TotalRevenue       float64 `json:"total_revenue"`
MRR                float64 `json:"mrr"` // Monthly Recurring Revenue
PopularPlan        string  `json:"popular_plan"`
PlanCounts         []PlanCount `json:"plan_counts"`
}

type PlanCount struct {
PlanName string `json:"plan_name"`
Count    int64  `json:"count"`
}

// GetAdminStats возвращает статистику для дашборда администратора
func GetAdminStats() (*AdminStats, error) {
stats := &AdminStats{}

// Общее количество пользователей
err := database.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM users
`).Scan(&stats.TotalUsers)
if err != nil {
return nil, err
}

// Новые пользователи за сегодня
err = database.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM users 
WHERE created_at::date = CURRENT_DATE
`).Scan(&stats.NewUsersToday)
if err != nil {
return nil, err
}

// Активные подписки
err = database.Pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM user_subscriptions 
WHERE status = 'active' AND current_period_end > NOW()
`).Scan(&stats.ActiveSubscriptions)
if err != nil {
// Если таблица подписок не создана – просто 0
stats.ActiveSubscriptions = 0
}

// Общая выручка (сумма price_monthly активных подписок)
err = database.Pool.QueryRow(context.Background(), `
SELECT COALESCE(SUM(p.price_monthly), 0)
FROM user_subscriptions s
JOIN subscription_plans p ON s.plan_id = p.id
WHERE s.status = 'active' AND s.current_period_end > NOW()
`).Scan(&stats.TotalRevenue)
if err != nil {
stats.TotalRevenue = 0
}

// MRR – Monthly Recurring Revenue (сумма ежемесячных платежей)
err = database.Pool.QueryRow(context.Background(), `
SELECT COALESCE(SUM(
CASE 
WHEN p.price_monthly IS NOT NULL THEN p.price_monthly
ELSE 0
END
), 0)
FROM user_subscriptions s
JOIN subscription_plans p ON s.plan_id = p.id
WHERE s.status = 'active' AND s.current_period_end > NOW()
`).Scan(&stats.MRR)
if err != nil {
stats.MRR = 0
}

// Самый популярный план
err = database.Pool.QueryRow(context.Background(), `
SELECT p.name, COUNT(s.id) as cnt
FROM subscription_plans p
LEFT JOIN user_subscriptions s ON p.id = s.plan_id AND s.status = 'active'
GROUP BY p.id, p.name
ORDER BY cnt DESC
LIMIT 1
`).Scan(&stats.PopularPlan, &stats.PlanCounts) // PlanCounts тут не используется, но нужно для Scan
if err != nil {
stats.PopularPlan = "Нет данных"
}

// Распределение подписок по планам
rows, err := database.Pool.Query(context.Background(), `
SELECT p.name, COUNT(s.id)
FROM subscription_plans p
LEFT JOIN user_subscriptions s ON p.id = s.plan_id AND s.status = 'active'
GROUP BY p.id, p.name
ORDER BY COUNT(s.id) DESC
`)
if err == nil {
defer rows.Close()
var planCounts []PlanCount
for rows.Next() {
var pc PlanCount
err = rows.Scan(&pc.PlanName, &pc.Count)
if err == nil {
planCounts = append(planCounts, pc)
}
}
stats.PlanCounts = planCounts
}

return stats, nil
}
