package handlers

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"

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
    // Получаем userID из контекста (устанавливается middleware)
    userID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    // Получаем text из формы
    question := c.PostForm("question")
    // Получаем файл
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
        return
    }
    defer file.Close()

    // Читаем файл в память
    fileBytes, err := io.ReadAll(file)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
        return
    }

    // Определяем MIME-тип
    mimeType := header.Header.Get("Content-Type")
    if mimeType == "" {
        mimeType = http.DetectContentType(fileBytes)
    }

    // Кодируем файл в base64
    base64Data := base64.StdEncoding.EncodeToString(fileBytes)
    dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

    // Получаем OpenRouter ключ напрямую из окружения
    openRouterKey := os.Getenv("OPENROUTER_API_KEY")
    if openRouterKey == "" {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "OpenRouter key not configured"})
        return
    }

    visionReq := VisionRequest{
        Model: "openai/gpt-4-vision-preview", // или другая модель, поддерживающая vision
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

    // Отправляем запрос в OpenRouter
    req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    if err != nil {
        log.Printf("Error creating request: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
        return
    }
    req.Header.Set("Authorization", "Bearer "+openRouterKey)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("OpenRouter error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to call AI"})
        return
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    log.Printf("OpenRouter response: %s", string(bodyBytes))

    if resp.StatusCode != http.StatusOK {
        c.JSON(resp.StatusCode, gin.H{"error": "AI service error"})
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
        log.Printf("JSON parse error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid response"})
        return
    }

    if len(result.Choices) == 0 {
        c.JSON(http.StatusOK, gin.H{"answer": "No response from AI"})
        return
    }

    answer := result.Choices[0].Message.Content

    // Сохраняем сообщения в историю (в фоне, чтобы не задерживать ответ)
    go func() {
        // Используем новый фоновый контекст, так как c.Request.Context() может быть отменён
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