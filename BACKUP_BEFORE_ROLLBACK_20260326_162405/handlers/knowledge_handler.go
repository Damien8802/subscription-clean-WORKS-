package handlers

import (
    "context"
    "io"
    "log"
    "net/http"
    "strings"
    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// UploadKnowledgeHandler загружает документ
func UploadKnowledgeHandler(c *gin.Context) {
    userID := c.PostForm("user_id")
    if userID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
        return
    }

    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
        return
    }
    defer file.Close()

    filename := header.Filename
    if !strings.HasSuffix(strings.ToLower(filename), ".txt") && !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
        c.JSON(http.StatusBadRequest, gin.H{"error": "only .txt and .pdf files are allowed"})
        return
    }

    contentBytes, err := io.ReadAll(file)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
        return
    }
    content := string(contentBytes)

    _, err = database.Pool.Exec(c.Request.Context(),
        `INSERT INTO knowledge_docs (user_id, filename, content) VALUES ($1, $2, $3)`,
        userID, filename, content)
    if err != nil {
        log.Printf("Ошибка вставки документа: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": "uploaded"})
}

// ListKnowledgeHandler возвращает список документов пользователя
func ListKnowledgeHandler(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(),
        `SELECT id, filename FROM knowledge_docs WHERE user_id = $1 ORDER BY created_at DESC`,
        userID)
    if err != nil {
        log.Printf("Ошибка запроса документов: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    defer rows.Close()

    type Doc struct {
        ID       int    `json:"id"`
        Filename string `json:"filename"`
    }
    var docs []Doc
    for rows.Next() {
        var d Doc
        if err := rows.Scan(&d.ID, &d.Filename); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "scan error"})
            return
        }
        docs = append(docs, d)
    }
    c.JSON(http.StatusOK, gin.H{"docs": docs})
}

// DeleteKnowledgeHandler удаляет документ
func DeleteKnowledgeHandler(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(),
        `DELETE FROM knowledge_docs WHERE id = $1`, id)
    if err != nil {
        log.Printf("Ошибка удаления документа: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// SearchKnowledge ищет релевантные документы (используется из AIAskHandler)
func SearchKnowledge(ctx context.Context, userID, query string, limit int) ([]string, error) {
    rows, err := database.Pool.Query(ctx,
        `SELECT content FROM knowledge_docs 
         WHERE user_id = $1 
           AND to_tsvector('russian', content) @@ plainto_tsquery('russian', $2)
         ORDER BY ts_rank(to_tsvector('russian', content), plainto_tsquery('russian', $2)) DESC
         LIMIT $3`,
        userID, query, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var fragments []string
    for rows.Next() {
        var content string
        if err := rows.Scan(&content); err != nil {
            return nil, err
        }
        if len(content) > 1000 {
            content = content[:1000] + "..."
        }
        fragments = append(fragments, content)
    }
    return fragments, nil
}
