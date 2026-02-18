package models

import (
"context"
"encoding/json"
"fmt"
"subscription-system/database"
"time"
)

// AdminAPIKey – ключ с данными пользователя для админ-панели
type AdminAPIKey struct {
ID                  string          `json:"id"`
UserID              string          `json:"user_id"`
UserEmail           string          `json:"user_email"`
UserName            string          `json:"user_name"`
Name                string          `json:"name"`
ProviderCredentials json.RawMessage `json:"provider_credentials"`
QuotaLimit          int64           `json:"quota_limit"`
QuotaUsed           int64           `json:"quota_used"`
IsActive            bool            `json:"is_active"`
CreatedAt           time.Time       `json:"created_at"`
UpdatedAt           time.Time       `json:"updated_at"`
}

// GetAllAPIKeys возвращает все ключи с пагинацией и поиском
func GetAllAPIKeys(search string, limit, offset int) ([]AdminAPIKey, int64, error) {
var total int64
var keys []AdminAPIKey

// Подсчёт общего количества
countQuery := `SELECT COUNT(*) FROM api_keys`
if search != "" {
countQuery += ` WHERE name ILIKE $1 OR EXISTS (SELECT 1 FROM users WHERE id = api_keys.user_id AND email ILIKE $1)`
}
var countArgs []interface{}
if search != "" {
countArgs = append(countArgs, "%"+search+"%")
}
err := database.Pool.QueryRow(context.Background(), countQuery, countArgs...).Scan(&total)
if err != nil {
return nil, 0, err
}

// Запрос данных
query := `
SELECT 
k.id, k.user_id, u.email, u.name,
k.name, k.provider_credentials, k.quota_limit, k.quota_used,
k.is_active, k.created_at, k.updated_at
FROM api_keys k
JOIN users u ON k.user_id = u.id
WHERE 1=1
`
var args []interface{}
if search != "" {
query += ` AND (k.name ILIKE $1 OR u.email ILIKE $1)`
args = append(args, "%"+search+"%")
}
query += ` ORDER BY k.created_at DESC`
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
var k AdminAPIKey
err := rows.Scan(
&k.ID, &k.UserID, &k.UserEmail, &k.UserName,
&k.Name, &k.ProviderCredentials, &k.QuotaLimit, &k.QuotaUsed,
&k.IsActive, &k.CreatedAt, &k.UpdatedAt,
)
if err != nil {
return nil, 0, err
}
keys = append(keys, k)
}
return keys, total, nil
}
