package models

import (
"context"
"time"
"subscription-system/database"

"github.com/google/uuid"
)

type AIUsageLog struct {
ID               string    `json:"id" db:"id"`
APIKeyID         string    `json:"api_key_id" db:"api_key_id"`
UserID           string    `json:"user_id" db:"user_id"`
Model            string    `json:"model" db:"model"`
PromptTokens     int       `json:"prompt_tokens" db:"prompt_tokens"`
CompletionTokens int       `json:"completion_tokens" db:"completion_tokens"`
TotalTokens      int       `json:"total_tokens" db:"total_tokens"`
DurationMs       int       `json:"duration_ms" db:"duration_ms"`
StatusCode       int       `json:"status_code" db:"status_code"`
Error            *string   `json:"error" db:"error"`
CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

// LogAIRequest сохраняет информацию о вызове AI-шлюза
func LogAIRequest(apiKeyID, userID, model string, promptTokens, completionTokens, totalTokens, durationMs, statusCode int, errorMsg *string) error {
_, err := database.Pool.Exec(context.Background(), `
INSERT INTO ai_usage_logs (id, api_key_id, user_id, model, prompt_tokens, completion_tokens, total_tokens, duration_ms, status_code, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
`, uuid.New().String(), apiKeyID, userID, model, promptTokens, completionTokens, totalTokens, durationMs, statusCode, errorMsg)
return err
}

// GetUserAITotals возвращает суммарную статистику по пользователю
func GetUserAITotals(userID string) (totalTokens int64, totalRequests int64, err error) {
err = database.Pool.QueryRow(context.Background(), `
SELECT COALESCE(SUM(total_tokens), 0), COALESCE(COUNT(*), 0)
FROM ai_usage_logs
WHERE user_id = $1
`, userID).Scan(&totalTokens, &totalRequests)
return
}

// GetUserAIUsageByModel возвращает статистику по моделям для пользователя
func GetUserAIUsageByModel(userID string) ([]struct {
Model    string `json:"model"`
Requests int64  `json:"requests"`
Tokens   int64  `json:"tokens"`
}, error) {
rows, err := database.Pool.Query(context.Background(), `
SELECT model, COUNT(*), COALESCE(SUM(total_tokens), 0)
FROM ai_usage_logs
WHERE user_id = $1
GROUP BY model
ORDER BY SUM(total_tokens) DESC
`, userID)
if err != nil {
return nil, err
}
defer rows.Close()
var result []struct {
Model    string `json:"model"`
Requests int64  `json:"requests"`
Tokens   int64  `json:"tokens"`
}
for rows.Next() {
var r struct {
Model    string `json:"model"`
Requests int64  `json:"requests"`
Tokens   int64  `json:"tokens"`
}
err := rows.Scan(&r.Model, &r.Requests, &r.Tokens)
if err != nil {
return nil, err
}
result = append(result, r)
}
return result, nil
}

// AdminGetAIStats возвращает общую статистику для админки
func AdminGetAIStats(period string) (map[string]interface{}, error) {
// period: day, week, month, all
var interval string
switch period {
case "day":
interval = "NOW() - INTERVAL '1 day'"
case "week":
interval = "NOW() - INTERVAL '7 days'"
case "month":
interval = "NOW() - INTERVAL '30 days'"
default:
interval = "'1970-01-01'"
}

stats := make(map[string]interface{})

// Общее количество запросов и токенов
var totalRequests, totalTokens int64
var avgDuration float64
err := database.Pool.QueryRow(context.Background(), `
SELECT 
COALESCE(COUNT(*), 0),
COALESCE(SUM(total_tokens), 0),
COALESCE(AVG(duration_ms), 0)
FROM ai_usage_logs
WHERE created_at >= `+interval).Scan(&totalRequests, &totalTokens, &avgDuration)
if err != nil {
return nil, err
}
stats["total_requests"] = totalRequests
stats["total_tokens"] = totalTokens
stats["avg_duration_ms"] = avgDuration

// Топ пользователей
rows, err := database.Pool.Query(context.Background(), `
SELECT u.email, u.name, COUNT(*) as req, COALESCE(SUM(l.total_tokens), 0) as tokens
FROM ai_usage_logs l
JOIN users u ON l.user_id = u.id
WHERE l.created_at >= `+interval+`
GROUP BY u.id, u.email, u.name
ORDER BY tokens DESC
LIMIT 10
`)
if err != nil {
return nil, err
}
defer rows.Close()
var topUsers []map[string]interface{}
for rows.Next() {
var email, name string
var req, tokens int64
err := rows.Scan(&email, &name, &req, &tokens)
if err != nil {
continue
}
topUsers = append(topUsers, map[string]interface{}{
"email":    email,
"name":     name,
"requests": req,
"tokens":   tokens,
})
}
stats["top_users"] = topUsers

// Топ моделей
rows, err = database.Pool.Query(context.Background(), `
SELECT model, COUNT(*), COALESCE(SUM(total_tokens), 0)
FROM ai_usage_logs
WHERE created_at >= `+interval+`
GROUP BY model
ORDER BY SUM(total_tokens) DESC
`)
if err != nil {
return nil, err
}
defer rows.Close()
var topModels []map[string]interface{}
for rows.Next() {
var model string
var req, tokens int64
err := rows.Scan(&model, &req, &tokens)
if err != nil {
continue
}
topModels = append(topModels, map[string]interface{}{
"model":    model,
"requests": req,
"tokens":   tokens,
})
}
stats["top_models"] = topModels

// Ошибки
var totalErrors int64
err = database.Pool.QueryRow(context.Background(), `
SELECT COALESCE(COUNT(*), 0)
FROM ai_usage_logs
WHERE status_code >= 400 AND created_at >= `+interval).Scan(&totalErrors)
if err != nil {
return nil, err
}
stats["total_errors"] = totalErrors

return stats, nil
}
