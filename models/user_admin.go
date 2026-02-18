package models

import (
"context"
"fmt"
"subscription-system/database"
)

// UserWithStats – пользователь с дополнительной информацией для админки
type UserWithStats struct {
ID                 string  `json:"id"`
Email              string  `json:"email"`
Name               string  `json:"name"`
Role               string  `json:"role"`
EmailVerified      bool    `json:"email_verified"`
CreatedAt          string  `json:"created_at"`
ActiveSubscriptions int64  `json:"active_subscriptions"`
TotalSpent         float64 `json:"total_spent"`
}

// GetUsersPaginated возвращает список пользователей с пагинацией и поиском
func GetUsersPaginated(search string, limit, offset int) ([]UserWithStats, int64, error) {
var total int64
var users []UserWithStats

// Подсчёт общего количества
countQuery := `SELECT COUNT(*) FROM users`
var countArgs []interface{}
if search != "" {
countQuery += ` WHERE email ILIKE $1 OR name ILIKE $1`
countArgs = append(countArgs, "%"+search+"%")
}
err := database.Pool.QueryRow(context.Background(), countQuery, countArgs...).Scan(&total)
if err != nil {
return nil, 0, err
}

// Запрос данных с пагинацией и присоединением статистики
query := `
SELECT 
u.id, u.email, u.name, u.role, u.email_verified, 
TO_CHAR(u.created_at, 'DD.MM.YYYY HH24:MI') as created_at,
COUNT(s.id) FILTER (WHERE s.status = 'active' AND s.current_period_end > NOW()) as active_subscriptions,
COALESCE(SUM(p.price_monthly) FILTER (WHERE s.status = 'active'), 0) as total_spent
FROM users u
LEFT JOIN user_subscriptions s ON u.id = s.user_id
LEFT JOIN subscription_plans p ON s.plan_id = p.id
`
var args []interface{}
if search != "" {
query += ` WHERE u.email ILIKE $1 OR u.name ILIKE $1`
args = append(args, "%"+search+"%")
}

query += ` GROUP BY u.id, u.email, u.name, u.role, u.email_verified, u.created_at`
query += ` ORDER BY u.created_at DESC`

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
var u UserWithStats
err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.EmailVerified, &u.CreatedAt, &u.ActiveSubscriptions, &u.TotalSpent)
if err != nil {
return nil, 0, err
}
users = append(users, u)
}
return users, total, nil
}

// UpdateUserRole обновляет роль пользователя
func UpdateUserRole(userID, newRole string) error {
_, err := database.Pool.Exec(context.Background(), `
UPDATE users SET role = $1, updated_at = NOW() WHERE id = $2
`, newRole, userID)
return err
}

// DeleteUser удаляет пользователя (каскадно удалятся и подписки благодаря ON DELETE CASCADE)
func DeleteUser(userID string) error {
_, err := database.Pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
return err
}
