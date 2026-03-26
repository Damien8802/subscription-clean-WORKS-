package handlers

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/config"
    "subscription-system/database"
)

type VisionRequest struct {
    Model    string `json:"model"`
    Messages []struct {
        Role    string `json:"role"`
        Content []struct {
            Type     string `json:"type"`
            Text     string `json:"text,omitempty"`
            ImageURL *struct {
                URL string `json:"url"`
            } `json:"image_url,omitempty"`
        } `json:"content"`
    } `json:"messages"`
    MaxTokens int `json:"max_tokens"`
}

func AskWithFileHandler(c *gin.Context) {
    cfg := config.Load()
    userID, exists := c.Get("userID")
    if !exists {
        if cfg.SkipAuth {
            var id string
            err := database.Pool.QueryRow(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1").Scan(&id)
            if err != nil {
                log.Printf("AskWithFileHandler: no users found: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "no users found"})
                return
            }
            userID = id
        } else {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }
    }

    question := c.PostForm("question")
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        log.Printf("AskWithFileHandler: no file uploaded: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
        return
    }
    defer file.Close()

    fileBytes, err := io.ReadAll(file)
    if err != nil {
        log.Printf("AskWithFileHandler: failed to read file: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
        return
    }

    mimeType := header.Header.Get("Content-Type")
    if mimeType == "" {
        mimeType = http.DetectContentType(fileBytes)
    }

    base64Data := base64.StdEncoding.EncodeToString(fileBytes)
    dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

    openRouterKey := os.Getenv("OPENROUTER_API_KEY")
    if openRouterKey == "" {
        log.Printf("AskWithFileHandler: OPENROUTER_API_KEY not set")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenRouter key not configured"})
        return
    }

    visionReq := VisionRequest{
        Model: "openai/gpt-4-vision-preview", // можно заменить на другую модель
        Messages: []struct {
            Role    string `json:"role"`
            Content []struct {
                Type     string `json:"type"`
                Text     string `json:"text,omitempty"`
                ImageURL *struct {
                    URL string `json:"url"`
                } `json:"image_url,omitempty"`
            } `json:"content"`
        }{
            {
                Role: "user",
                Content: []struct {
                    Type     string `json:"type"`
                    Text     string `json:"text,omitempty"`
                    ImageURL *struct {
                        URL string `json:"url"`
                    } `json:"image_url,omitempty"`
                }{
                    {
                        Type: "text",
                        Text: question,
                    },
                    {
                        Type: "image_url",
                        ImageURL: &struct {
                            URL string `json:"url"`
                        }{
                            URL: dataURL,
                        },
                    },
                },
            },
        },
        MaxTokens: 1000,
    }

    jsonBody, _ := json.Marshal(visionReq)
    log.Printf("AskWithFileHandler: sending request to OpenRouter, model=%s, question=%s", visionReq.Model, question)

    req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    if err != nil {
        log.Printf("AskWithFileHandler: error creating request: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
        return
    }
    req.Header.Set("Authorization", "Bearer "+openRouterKey)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("AskWithFileHandler: OpenRouter request failed: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to call AI: " + err.Error()})
        return
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    log.Printf("AskWithFileHandler: OpenRouter response status=%d, body=%s", resp.StatusCode, string(bodyBytes))

    if resp.StatusCode != http.StatusOK {
        // Пытаемся распарсить ошибку OpenRouter
        var errResp struct {
            Error struct {
                Message string `json:"message"`
            } `json:"error"`
        }
        if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error.Message != "" {
            c.JSON(resp.StatusCode, gin.H{"error": "OpenRouter: " + errResp.Error.Message})
        } else {
            c.JSON(resp.StatusCode, gin.H{"error": "AI service error"})
        }
        return
    }

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
    }
    if err := json.Unmarshal(bodyBytes, &result); err != nil {
        log.Printf("AskWithFileHandler: JSON parse error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid response"})
        return
    }

    if len(result.Choices) == 0 {
        c.JSON(http.StatusOK, gin.H{"answer": "No response from AI"})
        return
    }

    answer := result.Choices[0].Message.Content

    // Сохраняем в историю асинхронно
    go func() {
        ctx := context.Background()
        _, _ = database.Pool.Exec(ctx,
            `INSERT INTO chat_history (user_id, role, content) VALUES ($1, $2, $3)`,
            userID, "user", question)
        _, _ = database.Pool.Exec(ctx,
            `INSERT INTO chat_history (user_id, role, content) VALUES ($1, $2, $3)`,
            userID, "assistant", answer)
    }()

    c.JSON(http.StatusOK, gin.H{"answer": answer})
}
