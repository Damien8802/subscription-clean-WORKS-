package models

import (
"context"
"encoding/json"
"subscription-system/database"
"time"

"github.com/google/uuid"
)

type KnowledgeBase struct {
ID          string          `json:"id"`
UserID      string          `json:"user_id"`
ContentType string          `json:"content_type"`
ContentText string          `json:"content_text"`
Embedding   json.RawMessage `json:"-"`
Metadata    json.RawMessage `json:"metadata"`
CreatedAt   time.Time       `json:"created_at"`
UpdatedAt   time.Time       `json:"updated_at"`
}

// AddDocument сохраняет документ в базу знаний пользователя
func AddDocument(userID, contentType, contentText string, metadata map[string]interface{}) error {
metadataJSON, _ := json.Marshal(metadata)
_, err := database.Pool.Exec(context.Background(), `
INSERT INTO ai_knowledge_base (id, user_id, content_type, content_text, metadata)
VALUES ($1, $2, $3, $4, $5)
`, uuid.New().String(), userID, contentType, contentText, metadataJSON)
return err
}

// SearchSimilar ищет документы по тексту (полнотекстовый поиск, пока без векторов)
func SearchSimilar(userID, query string, limit int) ([]KnowledgeBase, error) {
rows, err := database.Pool.Query(context.Background(), `
SELECT id, user_id, content_type, content_text, metadata, created_at, updated_at
FROM ai_knowledge_base
WHERE user_id = $1 
  AND to_tsvector('russian', content_text) @@ plainto_tsquery('russian', $2)
ORDER BY created_at DESC
LIMIT $3
`, userID, query, limit)
if err != nil {
return nil, err
}
defer rows.Close()

var docs []KnowledgeBase
for rows.Next() {
var d KnowledgeBase
err := rows.Scan(&d.ID, &d.UserID, &d.ContentType, &d.ContentText, &d.Metadata, &d.CreatedAt, &d.UpdatedAt)
if err != nil {
return nil, err
}
docs = append(docs, d)
}
return docs, nil
}
