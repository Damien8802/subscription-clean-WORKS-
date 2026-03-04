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

    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5"

    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/internal/yandex_search"
    "subscription-system/models"
)

type AskRequest struct {
    Question   string `json:"question" binding:"required"`
    CRMContext bool   `json:"crm_context"`
    Recommend  bool   `json:"recommend"` // новый флаг для получения рекомендаций
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

// ========== НОВЫЕ ФУНКЦИИ ДЛЯ РЕКОМЕНДАЦИЙ ==========

// getStuckDeals возвращает сделки, которые не двигаются более 7 дней
func getStuckDeals(ctx context.Context, userID string) ([]string, error) {
    rows, err := database.Pool.Query(ctx, `
        SELECT d.title, d.stage, d.updated_at, c.name
        FROM crm_deals d
        JOIN crm_customers c ON c.id = d.customer_id
        WHERE d.user_id = $1::uuid
          AND d.stage NOT IN ('closed_won', 'closed_lost')
          AND d.updated_at < NOW() - INTERVAL '7 days'
        ORDER BY d.updated_at
        LIMIT 10
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var recommendations []string
    for rows.Next() {
        var title, stage, customerName string
        var updatedAt time.Time
        if err := rows.Scan(&title, &stage, &updatedAt, &customerName); err != nil {
            continue
        }
        days := int(time.Since(updatedAt).Hours() / 24)
        line := fmt.Sprintf("📌 Сделка \"%s\" (клиент: %s) на стадии \"%s\" не обновлялась %d дней. Рекомендуется связаться с клиентом.",
            title, customerName, stage, days)
        recommendations = append(recommendations, line)
    }
    return recommendations, nil
}

// getInactiveHighValueClients возвращает клиентов с высоким lead_score, но без активности >14 дней
func getInactiveHighValueClients(ctx context.Context, userID string) ([]string, error) {
    rows, err := database.Pool.Query(ctx, `
        SELECT name, email, lead_score, last_seen
        FROM crm_customers
        WHERE user_id = $1::uuid
          AND lead_score > 0.5
          AND last_seen < NOW() - INTERVAL '14 days'
        ORDER BY lead_score DESC
        LIMIT 5
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var recommendations []string
    for rows.Next() {
        var name, email string
        var leadScore float64
        var lastSeen time.Time
        if err := rows.Scan(&name, &email, &leadScore, &lastSeen); err != nil {
            continue
        }
        days := int(time.Since(lastSeen).Hours() / 24)
        line := fmt.Sprintf("💎 Клиент \"%s\" (email: %s) с высоким lead-скором (%.0f%%) не проявлял активности %d дней. Рекомендуется отправить персональное предложение.",
            name, email, leadScore*100, days)
        recommendations = append(recommendations, line)
    }
    return recommendations, nil
}

// getUpcomingDeals возвращает сделки с ожидаемой датой закрытия в ближайшие 7 дней
func getUpcomingDeals(ctx context.Context, userID string) ([]string, error) {
    rows, err := database.Pool.Query(ctx, `
        SELECT d.title, d.value, d.expected_close, c.name
        FROM crm_deals d
        JOIN crm_customers c ON c.id = d.customer_id
        WHERE d.user_id = $1::uuid
          AND d.expected_close BETWEEN NOW() AND NOW() + INTERVAL '7 days'
          AND d.stage NOT IN ('closed_won', 'closed_lost')
        ORDER BY d.expected_close
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var recommendations []string
    for rows.Next() {
        var title, customerName string
        var value float64
        var expectedClose time.Time
        if err := rows.Scan(&title, &value, &expectedClose, &customerName); err != nil {
            continue
        }
        days := int(time.Until(expectedClose).Hours() / 24)
        line := fmt.Sprintf("⏳ Сделка \"%s\" (клиент: %s) на сумму %.2f должна закрыться через %d дней. Рекомендуется подготовить финальные документы и связаться с клиентом.",
            title, customerName, value, days)
        recommendations = append(recommendations, line)
    }
    return recommendations, nil
}

// getSummaryStats возвращает краткую статистику для рекомендаций
func getSummaryStats(ctx context.Context, userID string) (map[string]interface{}, error) {
    stats := make(map[string]interface{})

    var totalDeals, activeDeals int
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM crm_deals WHERE user_id = $1", userID).Scan(&totalDeals)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM crm_deals WHERE user_id = $1 AND stage NOT IN ('closed_won','closed_lost')", userID).Scan(&activeDeals)
    stats["total_deals"] = totalDeals
    stats["active_deals"] = activeDeals

    var totalCustomers int
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM crm_customers WHERE user_id = $1", userID).Scan(&totalCustomers)
    stats["total_customers"] = totalCustomers

    var totalValue float64
    database.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(value),0) FROM crm_deals WHERE user_id = $1", userID).Scan(&totalValue)
    stats["total_value"] = totalValue

    return stats, nil
}

// --- КОНЕЦ НОВЫХ ФУНКЦИЙ ---

// --- СУЩЕСТВУЮЩИЕ ФУНКЦИИ (CRM-КОНТЕКСТ) ---

// getCRMStats возвращает статистику CRM для пользователя
func getCRMStats(ctx context.Context, userID string) (map[string]interface{}, error) {
    stats := make(map[string]interface{})

    // Общее количество клиентов
    var totalCustomers int
    err := database.Pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM crm_customers WHERE user_id = $1::uuid
    `, userID).Scan(&totalCustomers)
    if err != nil && err != pgx.ErrNoRows {
        return nil, err
    }
    stats["total_customers"] = totalCustomers

    // Общее количество сделок
    var totalDeals int
    err = database.Pool.QueryRow(ctx, `
        SELECT COUNT(*) FROM crm_deals WHERE user_id = $1::uuid
    `, userID).Scan(&totalDeals)
    if err != nil && err != pgx.ErrNoRows {
        return nil, err
    }
    stats["total_deals"] = totalDeals

    // Общая сумма сделок
    var totalValue float64
    err = database.Pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(value), 0) FROM crm_deals WHERE user_id = $1::uuid
    `, userID).Scan(&totalValue)
    if err != nil && err != pgx.ErrNoRows {
        return nil, err
    }
    stats["total_value"] = totalValue

    // Распределение по стадиям
    rows, err := database.Pool.Query(ctx, `
        SELECT stage, COUNT(*) FROM crm_deals 
        WHERE user_id = $1::uuid GROUP BY stage
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    stageStats := make(map[string]int)
    for rows.Next() {
        var stage string
        var count int
        if err := rows.Scan(&stage, &count); err != nil {
            return nil, err
        }
        stageStats[stage] = count
    }
    stats["stage_stats"] = stageStats

    return stats, nil
}

// getRecentCRMRecords возвращает последние 5 клиентов и сделок
func getRecentCRMRecords(ctx context.Context, userID string, limit int) (customers []string, deals []string, err error) {
    // Последние клиенты
    rows, err := database.Pool.Query(ctx, `
        SELECT name, email, company, status
        FROM crm_customers
        WHERE user_id = $1::uuid
        ORDER BY created_at DESC
        LIMIT $2
    `, userID, limit)
    if err != nil {
        return nil, nil, err
    }
    defer rows.Close()
    for rows.Next() {
        var name, email, company, status string
        if err := rows.Scan(&name, &email, &company, &status); err != nil {
            return nil, nil, err
        }
        line := fmt.Sprintf("👤 %s (%s) — %s, статус: %s", name, email, company, status)
        customers = append(customers, line)
    }

    // Последние сделки
    rows, err = database.Pool.Query(ctx, `
        SELECT title, value, stage, expected_close
        FROM crm_deals
        WHERE user_id = $1::uuid
        ORDER BY created_at DESC
        LIMIT $2
    `, userID, limit)
    if err != nil {
        return nil, nil, err
    }
    defer rows.Close()
    for rows.Next() {
        var title, stage string
        var value float64
        var expectedClose *time.Time
        if err := rows.Scan(&title, &value, &stage, &expectedClose); err != nil {
            return nil, nil, err
        }
        closeStr := "не указана"
        if expectedClose != nil {
            closeStr = expectedClose.Format("2006-01-02")
        }
        line := fmt.Sprintf("💰 %s — %.2f руб., стадия: %s, ожидаемая дата: %s", title, value, stage, closeStr)
        deals = append(deals, line)
    }

    return customers, deals, nil
}

// searchCRM выполняет полнотекстовый поиск по клиентам и сделкам
func searchCRM(ctx context.Context, userID, query string) ([]string, error) {
    var results []string

    // Поиск по клиентам (имя, email, компания)
    rows, err := database.Pool.Query(ctx, `
        SELECT name, email, company, status
        FROM crm_customers
        WHERE user_id = $1::uuid
          AND (name ILIKE '%' || $2 || '%' 
               OR email ILIKE '%' || $2 || '%' 
               OR company ILIKE '%' || $2 || '%')
        LIMIT 5
    `, userID, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    for rows.Next() {
        var name, email, company, status string
        if err := rows.Scan(&name, &email, &company, &status); err != nil {
            return nil, err
        }
        results = append(results, fmt.Sprintf("Клиент: %s (%s) — %s, статус: %s", name, email, company, status))
    }

    // Поиск по сделкам (название, комментарий)
    rows, err = database.Pool.Query(ctx, `
        SELECT title, value, stage
        FROM crm_deals
        WHERE user_id = $1::uuid
          AND (title ILIKE '%' || $2 || '%' 
               OR comment ILIKE '%' || $2 || '%')
        LIMIT 5
    `, userID, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    for rows.Next() {
        var title, stage string
        var value float64
        if err := rows.Scan(&title, &value, &stage); err != nil {
            return nil, err
        }
        results = append(results, fmt.Sprintf("Сделка: %s — %.2f руб., стадия: %s", title, value, stage))
    }

    return results, nil
}

// --- КОНЕЦ СУЩЕСТВУЮЩИХ CRM-ФУНКЦИЙ ---

// Поиск по документам пользователя (RAG)
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

// Запрос к OpenWeatherMap
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

// GetUserActivePlan получает активную подписку пользователя
func GetUserActivePlan(userID string) (*models.Plan, *models.UserSubscription, error) {
    var plan models.Plan
    var sub models.UserSubscription

    err := database.Pool.QueryRow(context.Background(), `
        SELECT 
            p.id, p.name, p.code, p.description, 
            p.price_monthly, p.price_yearly, p.currency,
            p.features, p.ai_capabilities, p.max_users,
            p.is_active, p.sort_order,
            us.id, us.user_id, us.plan_id, us.status,
            us.current_period_start, us.current_period_end,
            us.ai_quota_used, us.ai_quota_reset,
            us.created_at, us.updated_at
        FROM subscription_plans p
        JOIN user_subscriptions us ON us.plan_id = p.id
        WHERE us.user_id = $1::uuid AND us.status = 'active'
        AND us.current_period_start <= NOW() 
        AND us.current_period_end >= NOW()
        ORDER BY us.created_at DESC
        LIMIT 1
    `, userID).Scan(
        &plan.ID, &plan.Name, &plan.Code, &plan.Description,
        &plan.PriceMonthly, &plan.PriceYearly, &plan.Currency,
        &plan.Features, &plan.AICapabilities, &plan.MaxUsers,
        &plan.IsActive, &plan.SortOrder,
        &sub.ID, &sub.UserID, &sub.PlanID, &sub.Status,
        &sub.CurrentPeriodStart, &sub.CurrentPeriodEnd,
        &sub.AIQuotaUsed, &sub.AIQuotaReset,
        &sub.CreatedAt, &sub.UpdatedAt,
    )

    if err != nil {
        return nil, nil, err
    }

    return &plan, &sub, nil
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
    var subscription *models.UserSubscription
    var isAdmin bool

    if !cfg.SkipAuth {
        role, _ := c.Get("userRole")
        isAdmin = role == "admin"
        if !isAdmin {
            plan, subscription, err = GetUserActivePlan(userID.(string))
            if err != nil {
                c.JSON(http.StatusForbidden, gin.H{"error": "no active subscription"})
                return
            }
        }
    }

    // ========== НОВЫЙ БЛОК: РЕКОМЕНДАЦИИ ==========
    var recommendations []string
    var isRecommendationMode bool

    if req.Recommend {
        isRecommendationMode = true

        // Получаем данные для рекомендаций
        stuck, _ := getStuckDeals(c.Request.Context(), userID.(string))
        recommendations = append(recommendations, stuck...)

        inactive, _ := getInactiveHighValueClients(c.Request.Context(), userID.(string))
        recommendations = append(recommendations, inactive...)

        upcoming, _ := getUpcomingDeals(c.Request.Context(), userID.(string))
        recommendations = append(recommendations, upcoming...)

        // Статистика для контекста
        stats, _ := getSummaryStats(c.Request.Context(), userID.(string))
        statsLine := fmt.Sprintf("📊 Всего сделок: %v, активных: %v, клиентов: %v, общая сумма: %.2f руб.",
            stats["total_deals"], stats["active_deals"], stats["total_customers"], stats["total_value"])
        // Добавим статистику в начало списка
        recommendations = append([]string{statsLine}, recommendations...)

        // Если нет никаких рекомендаций, добавим сообщение
        if len(recommendations) == 1 { // только статистика
            recommendations = append(recommendations, "✅ На данный момент нет активных рекомендаций. Все сделки в норме.")
        }
    }
    // ========== КОНЕЦ БЛОКА РЕКОМЕНДАЦИЙ ==========

    // Переменные для обычного режима
    var extraInfo []string

    // Если режим рекомендаций – пропускаем обычную обработку
    if !isRecommendationMode {
        // Определяем, нужен ли веб-поиск или погодный API
        lowerQ := strings.ToLower(req.Question)
        needWeather := strings.Contains(lowerQ, "погода") || strings.Contains(lowerQ, "температура")
        needNews := strings.Contains(lowerQ, "новости") || strings.Contains(lowerQ, "сегодня") || strings.Contains(lowerQ, "завтра") || strings.Contains(lowerQ, "курс")

        // Если запрос о погоде, пытаемся получить данные
        if needWeather {
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

        // ========== CRM-КОНТЕКСТ ==========
        if req.CRMContext {
            // Получаем статистику CRM
            stats, err := getCRMStats(c.Request.Context(), userID.(string))
            if err != nil {
                log.Printf("⚠️ Ошибка получения CRM-статистики: %v", err)
            } else {
                extraInfo = append(extraInfo, fmt.Sprintf("📊 Статистика CRM:\n- Клиентов: %v\n- Сделок: %v\n- Общая сумма: %.2f руб.",
                    stats["total_customers"], stats["total_deals"], stats["total_value"]))
                if stageStats, ok := stats["stage_stats"].(map[string]int); ok && len(stageStats) > 0 {
                    var stages []string
                    for stage, count := range stageStats {
                        stages = append(stages, fmt.Sprintf("%s: %d", stage, count))
                    }
                    extraInfo = append(extraInfo, "Распределение по стадиям: "+strings.Join(stages, ", "))
                }
            }

            // Получаем последние записи
            recentCustomers, recentDeals, err := getRecentCRMRecords(c.Request.Context(), userID.(string), 5)
            if err != nil {
                log.Printf("⚠️ Ошибка получения последних записей CRM: %v", err)
            } else {
                if len(recentCustomers) > 0 {
                    extraInfo = append(extraInfo, "🆕 Последние клиенты:\n"+strings.Join(recentCustomers, "\n"))
                }
                if len(recentDeals) > 0 {
                    extraInfo = append(extraInfo, "🆕 Последние сделки:\n"+strings.Join(recentDeals, "\n"))
                }
            }

            // Если в вопросе есть ключевые слова для поиска, выполняем поиск по CRM
            lowerQ := strings.ToLower(req.Question)
            if strings.Contains(lowerQ, "найди") || strings.Contains(lowerQ, "поиск") || strings.Contains(lowerQ, "кто") || strings.Contains(lowerQ, "что") {
                searchResults, err := searchCRM(c.Request.Context(), userID.(string), req.Question)
                if err != nil {
                    log.Printf("⚠️ Ошибка поиска по CRM: %v", err)
                } else if len(searchResults) > 0 {
                    extraInfo = append(extraInfo, "🔍 Результаты поиска по CRM:\n"+strings.Join(searchResults, "\n"))
                }
            }
        }
        // ========== КОНЕЦ CRM-КОНТЕКСТА ==========
    }

    // Собираем системный промпт
    var sb strings.Builder

    if isRecommendationMode {
        sb.WriteString("Ты — AI-ассистент CRM, который даёт практические рекомендации по работе с клиентами и сделками.\n\n")
        sb.WriteString("ИНСТРУКЦИЯ:\n")
        sb.WriteString("1. Проанализируй предоставленные данные о сделках и клиентах.\n")
        sb.WriteString("2. Сформулируй список конкретных рекомендаций (не более 10).\n")
        sb.WriteString("3. Каждая рекомендация должна содержать:\n")
        sb.WriteString("   - Что именно нужно сделать (например, \"связаться с клиентом\", \"подготовить документы\")\n")
        sb.WriteString("   - По какой сделке/клиенту (с названием)\n")
        sb.WriteString("   - Почему это важно (сроки, сумма, потенциал)\n")
        sb.WriteString("4. В конце добавь общий совет по улучшению продаж.\n")
        sb.WriteString("5. Ответ должен быть на русском, чётким и структурированным.\n\n")
        sb.WriteString("Вот данные для анализа:\n")
        for i, rec := range recommendations {
            sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rec))
        }
    } else {
        // Обычный системный промпт
        sb.WriteString("Ты — профессиональный AI-ассистент платформы ServerAgent.\n\n")
        sb.WriteString("🎯 Информация о сервисе:\n")
        sb.WriteString("• ServerAgent — платформа для управления подписками и AI-чатом\n")
        sb.WriteString("• Тарифы: Базовый (2990₽), Профессиональный (29900₽), Семейный (9900₽), Корпоративный (49000₽)\n")
        sb.WriteString("• Способы оплаты: карта, USDT, Bitcoin, СБП, CryptoBot\n")
        sb.WriteString("• Поддержка: @IDamieN66I, support@saaspro.ru\n\n")
        sb.WriteString("📌 Твоя задача:\n")
        sb.WriteString("• Помогать пользователям с выбором тарифа\n")
        sb.WriteString("• Объяснять различия между тарифами\n")
        sb.WriteString("• Отвечать на вопросы об оплате\n")
        sb.WriteString("• Давать ссылки на поддержку\n")
        sb.WriteString("• Консультировать по функционалу платформы\n")
        sb.WriteString("• Если пользователь запросил CRM-контекст, используй предоставленную информацию о его клиентах и сделках для ответов.\n\n")
        sb.WriteString("⚠️ Важно:\n")
        sb.WriteString("• Всегда предлагай лучшее решение под запрос пользователя\n")
        sb.WriteString("• Если вопрос сложный — направляй в поддержку\n")
        sb.WriteString("• Будь вежливым и полезным\n")
        sb.WriteString("• Отвечай на русском языке\n\n")

        // Добавляем информацию из документов пользователя (RAG)
        docFragments, _ := searchUserDocs(c.Request.Context(), userID.(string), req.Question, 3)
        if len(docFragments) > 0 {
            sb.WriteString("📚 **Информация из ваших документов:**\n")
            for i, frag := range docFragments {
                sb.WriteString(fmt.Sprintf("--- Документ %d ---\n%s\n", i+1, frag))
            }
        }

        // Добавляем дополнительную информацию (погода, новости, CRM)
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

        // Инструкция для модели
        sb.WriteString("\n\n**ИНСТРУКЦИЯ:**\n")
        sb.WriteString("1. Отвечай на вопрос, используя предоставленную информацию и свои знания.\n")
        sb.WriteString("2. Если в предоставленных данных есть конкретные цифры, обязательно их приведи.\n")
        sb.WriteString("3. Если информации недостаточно, можешь ответить на основе своих знаний.\n")
        sb.WriteString("4. При необходимости можешь дать ссылки на источники (если они есть в результатах поиска), но не перегружай ответ списком ссылок.\n")
        sb.WriteString("5. Будь полезным, точным и дружелюбным.\n")
    }

    contextPrompt := sb.String()

    // ПРОВЕРКА: выводим ключи для отладки
    log.Printf("YandexFolderID: %s", cfg.YandexFolderID)
    log.Printf("YandexAPIKey: %s", maskString(cfg.YandexAPIKey))

    if cfg.YandexFolderID == "" || cfg.YandexAPIKey == "" {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "YandexGPT API key not configured"})
        return
    }

    // ========== ПРОВЕРКА КВОТЫ ==========
    if plan != nil && !isAdmin && subscription != nil {
        caps := plan.GetAICapabilities()
        maxRequests := int(caps["max_requests"].(float64))

        // Если квота обнулилась (прошёл день)
        if time.Now().After(subscription.AIQuotaReset) {
            _, err = database.Pool.Exec(c.Request.Context(), `
                UPDATE user_subscriptions 
                SET ai_quota_used = 0, ai_quota_reset = NOW() + interval '1 day'
                WHERE user_id = $1::uuid AND status = 'active'
            `, userID)
            if err != nil {
                log.Printf("❌ Ошибка сброса квоты: %v", err)
            }
            subscription.AIQuotaUsed = 0
        }

        if subscription.AIQuotaUsed >= maxRequests {
            c.JSON(http.StatusForbidden, gin.H{
                "error":       "Превышен лимит бесплатных запросов. Купите подписку для продолжения.",
                "upgrade_url": "/pricing",
            })
            return
        }
    }
    // ========== КОНЕЦ ПРОВЕРКИ КВОТЫ ==========

    yandexReq := YandexGPTRequest{
        ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", cfg.YandexFolderID),
        CompletionOptions: struct {
            Stream      bool    `json:"stream"`
            Temperature float64 `json:"temperature"`
            MaxTokens   int     `json:"maxTokens"`
        }{
            Stream:      false,
            Temperature: 0.7,
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
    apiReq.Header.Set("x-folder-id", cfg.YandexFolderID)

    resp, err := client.Do(apiReq)
    if err != nil {
        log.Printf("❌ Ошибка вызова YandexGPT: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to call YandexGPT"})
        return
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    log.Printf("📥 Код ответа от YandexGPT: %d", resp.StatusCode)
    log.Printf("📥 Тело ответа: %s", string(bodyBytes))

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
            "answer": "Не удалось получить ответ от AI.",
            "query":  req.Question,
        })
        return
    }

    answer := yandexResp.Result.Alternatives[0].Message.Text

    // ========== СПИСЫВАЕМ ТОКЕНЫ ==========
    if plan != nil && !isAdmin && subscription != nil {
        totalTokens, _ := strconv.Atoi(yandexResp.Result.Usage.TotalTokens)

        _, err = database.Pool.Exec(c.Request.Context(), `
            UPDATE user_subscriptions 
            SET ai_quota_used = ai_quota_used + $1 
            WHERE user_id = $2::uuid AND status = 'active'
        `, totalTokens, userID)
        if err != nil {
            log.Printf("❌ Ошибка обновления ai_quota_used: %v", err)
        } else {
            caps := plan.GetAICapabilities()
            maxRequests := int(caps["max_requests"].(float64))

            var newUsed int
            database.Pool.QueryRow(c.Request.Context(),
                "SELECT ai_quota_used FROM user_subscriptions WHERE user_id = $1::uuid AND status = 'active'",
                userID).Scan(&newUsed)

            log.Printf("✅ Списано %d токенов, осталось %d", totalTokens, maxRequests-newUsed)
        }
    }
    // ========== КОНЕЦ СПИСЫВАНИЯ ТОКЕНОВ ==========

    c.JSON(http.StatusOK, gin.H{
        "answer": answer,
        "query":  req.Question,
    })
}

// Вспомогательная функция для маскировки ключа в логах
func maskString(s string) string {
    if len(s) <= 4 {
        return "****"
    }
    return s[:4] + "..." + s[len(s)-4:]
}