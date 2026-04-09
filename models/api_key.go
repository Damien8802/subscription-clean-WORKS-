package models

import (
    "context"
    "encoding/json"
    "fmt"
    "subscription-system/database"
    "time"

    "golang.org/x/crypto/bcrypt"
)

type APIKey struct {
    ID                  string          `json:"id" db:"id"`
    UserID              string          `json:"user_id" db:"user_id"`
    Name                string          `json:"name" db:"name"`
    KeyHash             string          `json:"-" db:"key_hash"`
    Key                 string          `json:"-" db:"-"` // Только для временного хранения raw key
    ProviderCredentials json.RawMessage `json:"provider_credentials" db:"provider_credentials"`
    QuotaLimit          int64           `json:"quota_limit" db:"quota_limit"`
    QuotaUsed           int64           `json:"quota_used" db:"quota_used"`
    IsActive            bool            `json:"is_active" db:"is_active"`
    CreatedAt           time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`

   PlanID        *string     `json:"plan_id" db:"plan_id"`
    RequestsLimit int         `json:"requests_limit" db:"requests_limit"`
    LastResetAt   *time.Time  `json:"last_reset_at" db:"last_reset_at"`
    LastUsedAt    *time.Time  `json:"last_used_at" db:"last_used_at"`
    TotalRequests int         `json:"total_requests" db:"total_requests"`
// AI Agents поля
AIRequestsUsed  int `json:"ai_requests_used" db:"ai_requests_used"`
AIRequestsLimit int `json:"ai_requests_limit" db:"ai_requests_limit"`
AgentsLimit     int `json:"agents_limit" db:"agents_limit"`
AgentsCreated   int `json:"agents_created" db:"agents_created"`
}

// CreateAPIKey создаёт новый ключ в БД
func CreateAPIKey(key *APIKey) error {
    // Генерируем хеш ключа
    hashBytes, err := bcrypt.GenerateFromPassword([]byte(key.Key), bcrypt.DefaultCost)
    if err != nil {
        return err
    }
    key.KeyHash = string(hashBytes)

    _, err = database.Pool.Exec(context.Background(), `
        INSERT INTO api_keys (
            id, user_id, name, key_hash, provider_credentials, 
            quota_limit, quota_used, is_active, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `,
        key.ID,
        key.UserID,
        key.Name,
        key.KeyHash,
        key.ProviderCredentials,
        key.QuotaLimit,
        0, // quota_used
        key.IsActive,
        key.CreatedAt,
        key.UpdatedAt,
    )
    
    return err
}

// GetAPIKeysByUser возвращает все ключи пользователя
func GetAPIKeysByUser(userID string) ([]APIKey, error) {
    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, user_id, name, key_hash, provider_credentials, 
               quota_limit, quota_used, is_active, created_at, updated_at
        FROM api_keys 
        WHERE user_id = $1 
        ORDER BY created_at DESC
    `, userID)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var keys []APIKey
    for rows.Next() {
        var k APIKey
        err := rows.Scan(
            &k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.ProviderCredentials,
            &k.QuotaLimit, &k.QuotaUsed, &k.IsActive, &k.CreatedAt, &k.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }
        keys = append(keys, k)
    }
    return keys, nil
}

// UpdateAPIKey обновляет лимит, активность, credentials
func UpdateAPIKey(id string, quotaLimit *int64, isActive *bool, providerCredentials map[string]interface{}) error {
    query := `UPDATE api_keys SET updated_at = NOW()`
    args := []interface{}{}
    argPos := 1

    if quotaLimit != nil {
        query += fmt.Sprintf(`, quota_limit = $%d`, argPos)
        args = append(args, *quotaLimit)
        argPos++
    }
    if isActive != nil {
        query += fmt.Sprintf(`, is_active = $%d`, argPos)
        args = append(args, *isActive)
        argPos++
    }
    if providerCredentials != nil {
        credsJSON, _ := json.Marshal(providerCredentials)
        query += fmt.Sprintf(`, provider_credentials = $%d`, argPos)
        args = append(args, credsJSON)
        argPos++
    }
    query += fmt.Sprintf(` WHERE id = $%d`, argPos)
    args = append(args, id)

    _, err := database.Pool.Exec(context.Background(), query, args...)
    return err
}

// DeleteAPIKey удаляет ключ
func DeleteAPIKey(id string) error {
    _, err := database.Pool.Exec(context.Background(), `DELETE FROM api_keys WHERE id = $1`, id)
    return err
}

// IncrementQuotaUsed увеличивает счётчик использованных токенов
func IncrementQuotaUsed(keyID string, tokens int64) error {
    _, err := database.Pool.Exec(context.Background(), `
        UPDATE api_keys 
        SET quota_used = quota_used + $1, updated_at = NOW() 
        WHERE id = $2
    `, tokens, keyID)
    return err
}

// VerifyAPIKey проверяет сырой ключ и возвращает объект APIKey
func VerifyAPIKey(rawKey string) (*APIKey, error) {
    // Получаем все активные ключи
    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, user_id, name, key_hash, provider_credentials, 
               quota_limit, quota_used, is_active, created_at, updated_at
        FROM api_keys 
        WHERE is_active = true
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var key APIKey
        err := rows.Scan(
            &key.ID, &key.UserID, &key.Name, &key.KeyHash, &key.ProviderCredentials,
            &key.QuotaLimit, &key.QuotaUsed, &key.IsActive, &key.CreatedAt, &key.UpdatedAt,
        )
        if err != nil {
            continue
        }
        
        // Сравниваем хеш с сырым ключом
        err = bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawKey))
        if err == nil {
            return &key, nil
        }
    }
    return nil, fmt.Errorf("invalid API key")
}