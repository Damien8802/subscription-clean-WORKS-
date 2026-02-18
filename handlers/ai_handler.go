package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"

    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/internal/yandex_search"
    "subscription-system/models"
    "github.com/gin-gonic/gin"
)

type AskRequest struct {
    Question string `json:"question" binding:"required"`
}

type YandexGPTRequest struct {
    ModelUri          string `json:"modelUri"`
    CompletionOptions struct {
        Stream      bool    `json:"stream"`
        Temperature float64 `json:"temperature"`
        MaxTokens   int     `json:"maxTokens"`
    } `json:"completionOptions"`
    Messages []struct {
        Role string `json:"role"`
        Text string `json:"text"`
    } `json:"messages"`
}

type YandexGPTResponse struct {
    Result struct {
        Alternatives []struct {
            Message struct {
                Role string `json:"role"`
                Text string `json:"text"`
            } `json:"message"`
        } `json:"alternatives"`
        Usage struct {
            InputTextTokens  string `json:"inputTextTokens"`
            CompletionTokens string `json:"completionTokens"`
            TotalTokens      string `json:"totalTokens"`
        } `json:"usage"`
        ModelVersion string `json:"modelVersion"`
    } `json:"result"`
}

// Поиск по документам пользователя (RAG) – оставляем
func searchUserDocs(ctx context.Context, userID, query string, limit int) ([]string, error) {
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

// Поиск в интернете (Яндекс)
func searchWeb(query string, numResults int) ([]string, error) {
    apiKey := os.Getenv("YANDEX_SEARCH_API_KEY")
    folderID := os.Getenv("YANDEX_CLOUD_FOLDER_ID")
    if apiKey == "" || folderID == "" {
        return nil, fmt.Errorf("YANDEX_SEARCH_API_KEY or YANDEX_CLOUD_FOLDER_ID not set")
    }
    client := yandex_search.NewClient(apiKey, folderID)
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    req := yandex_search.SearchRequest{
        Query:        query,
        GroupsOnPage: numResults,
        DocsInGroup:  1,
        MaxPassages:  3,
    }
    results, err := client.Search(ctx, req)
    if err != nil {
        return nil, err
    }
    var snippets []string
    for _, r := range results {
        snippet := fmt.Sprintf("📌 *%s*\n%s", r.Title, r.Snippet)
        snippets = append(snippets, snippet)
    }
    return snippets, nil
}

// Запрос к OpenWeatherMap (оставляем как опцию)
func getWeather(city string) (string, error) {
    apiKey := os.Getenv("OPENWEATHER_API_KEY")
    if apiKey == "" {
        return "", fmt.Errorf("OPENWEATHER_API_KEY not set")
    }
    url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric&lang=ru", city, apiKey)
    resp, err := http.Get(url)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    if resp.StatusCode != 200 {
        return "", fmt.Errorf("weather API returned status %d", resp.StatusCode)
    }
    var data struct {
        Weather []struct {
            Description string `json:"description"`
        } `json:"weather"`
        Main struct {
            Temp     float64 `json:"temp"`
            Pressure int     `json:"pressure"`
            Humidity int     `json:"humidity"`
        } `json:"main"`
        Wind struct {
            Speed float64 `json:"speed"`
        } `json:"wind"`
        Name string `json:"name"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return "", err
    }
    return fmt.Sprintf("Погода в %s: %s, температура %.1f°C, давление %d гПа, влажность %d%%, ветер %.1f м/с.",
        data.Name, data.Weather[0].Description, data.Main.Temp, data.Main.Pressure, data.Main.Humidity, data.Wind.Speed), nil
}

func AIAskHandler(c *gin.Context) {
    var err error
    userID, exists := c.Get("userID")
    if !exists {
        var id string
        rows, err := database.Pool.Query(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1")
        if err == nil && rows.Next() {
            rows.Scan(&id)
            userID = id
        }
        rows.Close()
        if userID == nil || userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }
    }

    var req AskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    cfg := config.Load()
    var plan *models.Plan
    var isAdmin bool

    if !cfg.SkipAuth {
        role, _ := c.Get("userRole")
        isAdmin = role == "admin"
        if !isAdmin {
            plan, err = models.GetUserActivePlan(userID.(string))
            if err != nil {
                c.JSON(http.StatusForbidden, gin.H{"error": "no active subscription"})
                return
            }
            if plan.AIQuota == 0 {
                c.JSON(http.StatusForbidden, gin.H{"error": "AI assistant not included in your plan"})
                return
            }
        }
    }

    // Определяем, нужен ли веб-поиск или погодный API
    lowerQ := strings.ToLower(req.Question)
    needWeather := strings.Contains(lowerQ, "погода") || strings.Contains(lowerQ, "температура")
    needNews := strings.Contains(lowerQ, "новости") || strings.Contains(lowerQ, "сегодня") || strings.Contains(lowerQ, "завтра") || strings.Contains(lowerQ, "курс")

    var extraInfo []string

    // Если запрос о погоде, пытаемся получить данные
    if needWeather {
        // Простое извлечение города (можно улучшить)
        words := strings.Fields(req.Question)
        var city string
        for i, w := range words {
            if w == "в" || w == "во" || w == "на" {
                if i+1 < len(words) {
                    city = words[i+1]
                    break
                }
            }
        }
        if city == "" && len(words) > 0 {
            city = words[len(words)-1]
        }
        if city != "" {
            weatherStr, err := getWeather(city)
            if err == nil {
                extraInfo = append(extraInfo, "🌦️ "+weatherStr)
            }
        }
    }

    // Если нужны актуальные новости или курс, используем веб-поиск
    if needNews && len(extraInfo) == 0 {
        webResults, err := searchWeb(req.Question, 3)
        if err == nil && len(webResults) > 0 {
            extraInfo = append(extraInfo, "🌐 Актуальная информация из интернета:")
            extraInfo = append(extraInfo, webResults...)
        }
    }

    // Собираем системный промпт
    var sb strings.Builder
    sb.WriteString("Ты — профессиональный AI-ассистент SaaS-платформы. Отвечай на русском языке, вежливо и по делу.\n\n")

    // Добавляем информацию из документов пользователя (RAG)
    docFragments, _ := searchUserDocs(c.Request.Context(), userID.(string), req.Question, 3)
    if len(docFragments) > 0 {
        sb.WriteString("📚 **Информация из ваших документов:**\n")
        for i, frag := range docFragments {
            sb.WriteString(fmt.Sprintf("--- Документ %d ---\n%s\n", i+1, frag))
        }
    }

    // Добавляем дополнительную информацию (погода, новости)
    for _, info := range extraInfo {
        sb.WriteString(info + "\n\n")
    }

    // База знаний платформы
    kbDocs, err := models.SearchSimilar(userID.(string), req.Question, 5)
    if err != nil {
        log.Printf("Ошибка поиска в KB: %v", err)
        kbDocs = []models.KnowledgeBase{}
    }
    if len(kbDocs) > 0 {
        sb.WriteString("\n📋 **Информация из базы знаний платформы:**\n")
        for _, doc := range kbDocs {
            sb.WriteString(fmt.Sprintf("- [%s] %s\n", doc.ContentType, doc.ContentText))
        }
    }

    // Инструкция для модели: теперь она может отвечать из своих знаний
    sb.WriteString("\n\n**ИНСТРУКЦИЯ:**\n")
    sb.WriteString("1. Отвечай на вопрос, используя предоставленную информацию и свои знания.\n")
    sb.WriteString("2. Если в предоставленных данных есть конкретные цифры, обязательно их приведи.\n")
    sb.WriteString("3. Если информации недостаточно, можешь ответить на основе своих знаний.\n")
    sb.WriteString("4. При необходимости можешь дать ссылки на источники (если они есть в результатах поиска), но не перегружай ответ списком ссылок.\n")
    sb.WriteString("5. Будь полезным, точным и дружелюбным.\n")

    contextPrompt := sb.String()

    if cfg.YandexFolderID == "" || cfg.YandexAPIKey == "" {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "YandexGPT API key not configured"})
        return
    }

    yandexReq := YandexGPTRequest{
        ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", cfg.YandexFolderID),
        CompletionOptions: struct {
            Stream      bool    `json:"stream"`
            Temperature float64 `json:"temperature"`
            MaxTokens   int     `json:"maxTokens"`
        }{
            Stream:      false,
            Temperature: 0.7, // немного выше для креативности
            MaxTokens:   2000,
        },
        Messages: []struct {
            Role string `json:"role"`
            Text string `json:"text"`
        }{
            {Role: "system", Text: contextPrompt},
            {Role: "user", Text: req.Question},
        },
    }

    jsonData, _ := json.Marshal(yandexReq)
    log.Println("📤 Отправка запроса в YandexGPT")

    client := &http.Client{Timeout: 30 * time.Second}
    apiReq, err := http.NewRequest("POST", "https://llm.api.cloud.yandex.net/foundationModels/v1/completion", bytes.NewBuffer(jsonData))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
        return
    }
    apiReq.Header.Set("Authorization", "Api-Key "+cfg.YandexAPIKey)
    apiReq.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(apiReq)
    if err != nil {
        log.Printf("❌ Ошибка вызова YandexGPT: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to call YandexGPT"})
        return
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    log.Printf("📥 Код ответа от YandexGPT: %d", resp.StatusCode)

    if resp.StatusCode != http.StatusOK {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":  "YandexGPT returned error",
            "status": resp.StatusCode,
            "body":   string(bodyBytes),
        })
        return
    }

    var yandexResp YandexGPTResponse
    if err := json.Unmarshal(bodyBytes, &yandexResp); err != nil {
        log.Printf("❌ Ошибка парсинга ответа: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid response from YandexGPT"})
        return
    }

    if len(yandexResp.Result.Alternatives) == 0 {
        c.JSON(http.StatusOK, gin.H{
            "answer":  "Не удалось получить ответ от AI.",
            "query":   req.Question,
        })
        return
    }

    answer := yandexResp.Result.Alternatives[0].Message.Text

    // Списываем токены (оставляем как было)
    if !cfg.SkipAuth && !isAdmin && plan != nil {
        totalTokens, err := strconv.Atoi(yandexResp.Result.Usage.TotalTokens)
        if err != nil {
            log.Printf("⚠️ Ошибка преобразования токенов: %v", err)
            totalTokens = 0
        }
        newUsed := plan.AITokensUsed + int64(totalTokens)
        if newUsed > plan.AIQuota {
            c.JSON(http.StatusForbidden, gin.H{"error": "AI quota exceeded"})
            return
        }
        _, err = database.Pool.Exec(c.Request.Context(),
            "UPDATE user_subscriptions SET ai_tokens_used = $1 WHERE user_id = $2 AND status = 'active'",
            newUsed, userID)
        if err != nil {
            log.Printf("❌ Ошибка обновления ai_tokens_used: %v", err)
        } else {
            log.Printf("✅ Списано %d токенов, осталось %d", totalTokens, plan.AIQuota-newUsed)
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "answer":  answer,
        "query":   req.Question,
    })
}
