package models

import (
"context"
"encoding/json"
"fmt"
"subscription-system/database"
"time"

"github.com/google/uuid"
"golang.org/x/crypto/bcrypt"
)

type APIKey struct {
ID                  string          `json:"id" db:"id"`
UserID              string          `json:"user_id" db:"user_id"`
Name                string          `json:"name" db:"name"`
KeyHash             string          `json:"-" db:"key_hash"`
ProviderCredentials json.RawMessage `json:"provider_credentials" db:"provider_credentials"`
QuotaLimit          int64           `json:"quota_limit" db:"quota_limit"`
QuotaUsed           int64           `json:"quota_used" db:"quota_used"`
IsActive            bool            `json:"is_active" db:"is_active"`
CreatedAt           time.Time       `json:"created_at" db:"created_at"`
UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`
}

// GenerateAPIKey создаёт новый ключ и возвращает его в открытом виде
func GenerateAPIKey(userID, name string, providerCredentials map[string]interface{}, quotaLimit int64) (string, *APIKey, error) {
// Генерируем случайный ключ формата sk-saaspro-xxxxxxxx
rawKey := "sk-saaspro-" + uuid.New().String()
hashBytes, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
if err != nil {
return "", nil, err
}
keyHash := string(hashBytes)

credsJSON, _ := json.Marshal(providerCredentials)

var key APIKey
query := `
INSERT INTO api_keys (id, user_id, name, key_hash, provider_credentials, quota_limit, quota_used, is_active)
VALUES ($1, $2, $3, $4, $5, $6, 0, true)
RETURNING id, user_id, name, key_hash, provider_credentials, quota_limit, quota_used, is_active, created_at, updated_at
`
id := uuid.New().String()
err = database.Pool.QueryRow(context.Background(), query,
id, userID, name, keyHash, credsJSON, quotaLimit,
).Scan(&key.ID, &key.UserID, &key.Name, &key.KeyHash, &key.ProviderCredentials, &key.QuotaLimit, &key.QuotaUsed, &key.IsActive, &key.CreatedAt, &key.UpdatedAt)
if err != nil {
return "", nil, err
}
return rawKey, &key, nil
}

// FindAPIKeyByHash ищет ключ по хешу (используется в middleware)
func FindAPIKeyByHash(keyHash string) (*APIKey, error) {
var key APIKey
query := `SELECT id, user_id, name, key_hash, provider_credentials, quota_limit, quota_used, is_active, created_at, updated_at
          FROM api_keys WHERE key_hash = $1`
err := database.Pool.QueryRow(context.Background(), query, keyHash).Scan(
&key.ID, &key.UserID, &key.Name, &key.KeyHash, &key.ProviderCredentials,
&key.QuotaLimit, &key.QuotaUsed, &key.IsActive, &key.CreatedAt, &key.UpdatedAt,
)
if err != nil {
return nil, err
}
return &key, nil
}

// GetAPIKeysByUser возвращает все ключи пользователя (для админки)
func GetAPIKeysByUser(userID string) ([]APIKey, error) {
rows, err := database.Pool.Query(context.Background(), `
SELECT id, user_id, name, key_hash, provider_credentials, quota_limit, quota_used, is_active, created_at, updated_at
FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC
`, userID)
if err != nil {
return nil, err
}
defer rows.Close()
var keys []APIKey
for rows.Next() {
var k APIKey
err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyHash, &k.ProviderCredentials, &k.QuotaLimit, &k.QuotaUsed, &k.IsActive, &k.CreatedAt, &k.UpdatedAt)
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
query += `, quota_limit = $` + fmt.Sprint(argPos)
args = append(args, *quotaLimit)
argPos++
}
if isActive != nil {
query += `, is_active = $` + fmt.Sprint(argPos)
args = append(args, *isActive)
argPos++
}
if providerCredentials != nil {
credsJSON, _ := json.Marshal(providerCredentials)
query += `, provider_credentials = $` + fmt.Sprint(argPos)
args = append(args, credsJSON)
argPos++
}
query += ` WHERE id = $` + fmt.Sprint(argPos)
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
UPDATE api_keys SET quota_used = quota_used + $1, updated_at = NOW() WHERE id = $2
`, tokens, keyID)
return err
}

// VerifyAPIKey проверяет сырой ключ и возвращает объект APIKey
func VerifyAPIKey(rawKey string) (*APIKey, error) {
// Из-за особенностей bcrypt мы не можем просто захешировать и сравнить,
// потому что соль случайная. В реальности мы бы хранили префикс и искали кандидатов.
// Для MVP мы будем использовать упрощение: ключ имеет формат sk-saaspro-{uuid},
// мы можем вычислить хеш для полученного ключа и искать по нему.
// Это будет работать, потому что bcrypt детерминирован при одинаковой соли,
// но при генерации соль всегда разная, поэтому хеш будет разным.
// Поэтому такой подход не сработает. Нужно перебирать все хеши и проверять bcrypt.CompareHashAndPassword.

// Получаем все активные ключи и проверяем bcrypt.CompareHashAndPassword
rows, err := database.Pool.Query(context.Background(), `
SELECT id, user_id, name, key_hash, provider_credentials, quota_limit, quota_used, is_active, created_at, updated_at
FROM api_keys WHERE is_active = true
`)
if err != nil {
return nil, err
}
defer rows.Close()

for rows.Next() {
var key APIKey
err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.KeyHash, &key.ProviderCredentials, &key.QuotaLimit, &key.QuotaUsed, &key.IsActive, &key.CreatedAt, &key.UpdatedAt)
if err != nil {
continue
}
err = bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawKey))
if err == nil {
return &key, nil
}
}
return nil, fmt.Errorf("invalid API key")
}
