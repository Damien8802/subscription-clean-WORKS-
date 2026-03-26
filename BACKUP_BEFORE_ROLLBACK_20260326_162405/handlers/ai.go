package handlers

import (
    "bytes"
    "crypto/tls"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/smtp"
    "os"
    "regexp"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
)

type AskRequest struct {
    Question    string `json:"question" binding:"required"`
    CRMContext  bool   `json:"crm_context"`
    Recommend   bool   `json:"recommend"`
    RequestType string `json:"request_type"`
    SessionID   string `json:"session_id"`
}

type PriceInfo struct {
    ServiceName  string
    DisplayPrice float64
    SourceCount  int
    LastUpdated  time.Time
    Sources      []string
}

type SearchResult struct {
    Title       string
    Description string
    Price       float64
    Source      string
}

type DialogState struct {
    UserName          string             `json:"user_name"`
    UserService       string             `json:"user_service"`
    UserServiceName   string             `json:"user_service_name"`
    BasePrice         float64            `json:"base_price"`
    CalculatedPrice   float64            `json:"calculated_price"`
    UserPhone         string             `json:"user_phone"`
    UserMessenger     string             `json:"user_messenger"`
    Messages          []string           `json:"messages"`
    LastUpdated       time.Time
    GreetingShown     bool
    AwaitingPhone     bool
    AwaitingMessenger bool
    Completed         bool
    DesignAsked       bool
    DesignAnswer      string
    TechHelpAsked     bool
    TechHelpAdded     bool
    DeadlineAsked     bool
    Deadline          string
    AdditionalService string
    AdditionalPrice   float64
    CurrentStep       int
    LastSearchResults []SearchResult
}

type SearchCache struct {
    Results   []SearchResult
    ExpiresAt time.Time
    Query     string
}

type TelegramNotify struct {
    ChatID    string `json:"chat_id"`
    Text      string `json:"text"`
    ParseMode string `json:"parse_mode"`
}

var (
    dialogStates = make(map[string]*DialogState)
    dialogMutex  = &sync.RWMutex{}
    searchCache  = make(map[string]*SearchCache)
    cacheMutex   = &sync.RWMutex{}
    cacheTTL     = 1 * time.Hour

    telegramBotToken string
    telegramChatID   string
    yandexApiKey     string
    yandexFolderID   string
    
    emailTo       string
    emailFrom     string
    emailPassword string
    smtpHost      string
    smtpPort      string

    ourServices = map[string]string{
        "telegram бот":      "🤖 Разработка Telegram-ботов",
        "телеграм бот":      "🤖 Разработка Telegram-ботов",
        "интернет магазин":  "🛒 Создание интернет-магазинов",
        "интернет-магазин":  "🛒 Создание интернет-магазинов",
        "интеграция":        "🔗 Интеграции",
        "ai ассистент":      "🧠 AI-ассистенты",
        "telegram mini app": "📱 Telegram Mini Apps",
        "crm":               "📊 Настройка CRM-систем",
        "автоматизация":     "⚙️ Автоматизация процессов",
        "партнерская программа": "🎯 Партнёрские программы",
        "дашборд":           "📈 Индивидуальные дашборды",
        "seo":               "📢 SEO и маркетинг",
        "доработка":         "🛠 Доработка",
    }
)

func init() {
    godotenv.Load()
    telegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
    telegramChatID = os.Getenv("ADMIN_CHAT_ID")
    yandexApiKey = os.Getenv("YANDEX_SEARCH_API_KEY")
    yandexFolderID = os.Getenv("YANDEX_FOLDER_ID")
    
    emailTo = os.Getenv("EMAIL_TO")
    if emailTo == "" {
        emailTo = "Skorpion_88-88@mail.ru"
    }
    emailFrom = os.Getenv("EMAIL_FROM")
    emailPassword = os.Getenv("EMAIL_PASSWORD")
    smtpHost = os.Getenv("SMTP_HOST")
    if smtpHost == "" {
        smtpHost = "smtp.mail.ru"
    }
    smtpPort = os.Getenv("SMTP_PORT")
    if smtpPort == "" {
        smtpPort = "587"
    }

    log.Println("==================================================")
    log.Printf("🔍 Яндекс Поиск: %s", maskString(yandexApiKey, 10))
    log.Printf("📧 Email: %s", emailTo)
    log.Println("==================================================")

    os.MkdirAll("orders", 0755)
    go startCleanupRoutine()
    go startCacheCleanupRoutine()
}

func maskString(s string, visible int) string {
    if len(s) <= visible {
        return s
    }
    return s[:visible] + "..." + s[len(s)-3:]
}

func startCleanupRoutine() {
    for {
        time.Sleep(1 * time.Hour)
        dialogMutex.Lock()
        now := time.Now()
        for id, state := range dialogStates {
            if now.Sub(state.LastUpdated) > 24*time.Hour {
                delete(dialogStates, id)
            }
        }
        dialogMutex.Unlock()
    }
}

func startCacheCleanupRoutine() {
    for {
        time.Sleep(30 * time.Minute)
        cacheMutex.Lock()
        now := time.Now()
        for key, cache := range searchCache {
            if now.After(cache.ExpiresAt) {
                delete(searchCache, key)
            }
        }
        cacheMutex.Unlock()
    }
}

func getCacheKey(query string) string {
    return strings.ToLower(strings.TrimSpace(query))
}

func searchInternet(query string) ([]SearchResult, error) {
    cacheKey := getCacheKey(query)
    cacheMutex.RLock()
    if cached, exists := searchCache[cacheKey]; exists && time.Now().Before(cached.ExpiresAt) {
        cacheMutex.RUnlock()
        return cached.Results, nil
    }
    cacheMutex.RUnlock()

    if yandexApiKey == "" || yandexFolderID == "" {
        return nil, fmt.Errorf("поиск не настроен")
    }

    log.Printf("🔍 Поиск: %s", query)

    searchQueries := []string{
        query,
        fmt.Sprintf("%s цена", query),
        fmt.Sprintf("%s сколько стоит", query),
        fmt.Sprintf("%s дизайн", query),
        fmt.Sprintf("%s советы", query),
        fmt.Sprintf("лучшие практики %s", query),
    }

    var allResults []SearchResult
    
    for _, sq := range searchQueries {
        cleanQuery := strings.ReplaceAll(sq, "–", "-")
        cleanQuery = strings.ReplaceAll(cleanQuery, "—", "-")
        cleanQuery = strings.ReplaceAll(cleanQuery, "\n", " ")
        
        if len(cleanQuery) > 100 {
            cleanQuery = cleanQuery[:100]
        }

        requestBody := map[string]interface{}{
            "query": map[string]interface{}{
                "query_text": cleanQuery,
                "search_type": "SEARCH_TYPE_RU",
            },
            "max_docs": 10,
        }

        jsonBody, _ := json.Marshal(requestBody)
        url := fmt.Sprintf("https://searchapi.api.cloud.yandex.net/v2/web/search?folderId=%s", yandexFolderID)

        req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
        req.Header.Set("Content-Type", "application/json")
        req.Header.Set("Authorization", "Api-Key "+yandexApiKey)

        client := &http.Client{Timeout: 10 * time.Second}
        resp, err := client.Do(req)
        if err != nil {
            continue
        }
        
        body, _ := io.ReadAll(resp.Body)
        resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            continue
        }

        var result struct {
            RawData string `json:"rawData"`
        }
        if err := json.Unmarshal(body, &result); err != nil {
            continue
        }

        xmlData, _ := base64.StdEncoding.DecodeString(result.RawData)
        xmlStr := string(xmlData)
        
        titleRegex := regexp.MustCompile(`<title>(.*?)</title>`)
        passageRegex := regexp.MustCompile(`<passage>(.*?)</passage>`)
        
        titles := titleRegex.FindAllStringSubmatch(xmlStr, -1)
        passages := passageRegex.FindAllStringSubmatch(xmlStr, -1)
        
        priceRegex := regexp.MustCompile(`(\d{1,3}(?:[.\s]?\d{3})*)\s*(?:тыс\.?|тысяч|₽|руб|рублей|р\.)`)
        
        for _, title := range titles {
            if len(title) > 1 {
                result := SearchResult{
                    Title:       title[1],
                    Description: "",
                    Source:      sq,
                }
                
                matches := priceRegex.FindAllStringSubmatch(title[1], -1)
                for _, match := range matches {
                    if len(match) > 1 {
                        priceStr := strings.ReplaceAll(match[1], " ", "")
                        price, _ := strconv.ParseFloat(priceStr, 64)
                        if strings.Contains(match[0], "тыс") {
                            price *= 1000
                        }
                        if price >= 5000 && price <= 5000000 {
                            result.Price = price
                            break
                        }
                    }
                }
                
                allResults = append(allResults, result)
            }
        }
        
        for _, passage := range passages {
            if len(passage) > 1 {
                result := SearchResult{
                    Title:       "",
                    Description: passage[1],
                    Source:      sq,
                }
                
                matches := priceRegex.FindAllStringSubmatch(passage[1], -1)
                for _, match := range matches {
                    if len(match) > 1 {
                        priceStr := strings.ReplaceAll(match[1], " ", "")
                        price, _ := strconv.ParseFloat(priceStr, 64)
                        if strings.Contains(match[0], "тыс") {
                            price *= 1000
                        }
                        if price >= 5000 && price <= 5000000 {
                            result.Price = price
                            break
                        }
                    }
                }
                
                allResults = append(allResults, result)
            }
        }
        
        time.Sleep(100 * time.Millisecond)
    }

    uniqueResults := make(map[string]SearchResult)
    for _, r := range allResults {
        if r.Title != "" || r.Description != "" {
            key := r.Title
            if key == "" {
                key = r.Description[:min(50, len(r.Description))]
            }
            if _, exists := uniqueResults[key]; !exists {
                uniqueResults[key] = r
            }
        }
    }
    
    results := make([]SearchResult, 0, len(uniqueResults))
    for _, r := range uniqueResults {
        results = append(results, r)
    }
    
    cacheMutex.Lock()
    searchCache[cacheKey] = &SearchCache{
        Results:   results,
        ExpiresAt: time.Now().Add(cacheTTL),
        Query:     query,
    }
    cacheMutex.Unlock()
    
    log.Printf("✅ Найдено %d результатов для '%s'", len(results), query)
    return results, nil
}

func getAveragePrice(query string) float64 {
    results, err := searchInternet(query + " цена")
    if err != nil {
        return 50000
    }
    
    var prices []float64
    for _, r := range results {
        if r.Price > 0 {
            prices = append(prices, r.Price)
        }
    }
    
    if len(prices) > 0 {
        sort.Float64s(prices)
        medianIndex := len(prices) / 2
        medianPrice := prices[medianIndex]
        if len(prices)%2 == 0 && len(prices) > 1 {
            medianPrice = (prices[medianIndex-1] + prices[medianIndex]) / 2
        }
        return medianPrice
    }
    
    switch {
    case strings.Contains(strings.ToLower(query), "telegram") || strings.Contains(strings.ToLower(query), "телеграм"):
        return 50000
    case strings.Contains(strings.ToLower(query), "интернет магазин"):
        return 150000
    case strings.Contains(strings.ToLower(query), "crm"):
        return 60000
    case strings.Contains(strings.ToLower(query), "ai") || strings.Contains(strings.ToLower(query), "ассистент"):
        return 80000
    default:
        return 50000
    }
}

func getAdvice(query string) string {
    results, err := searchInternet(query + " советы рекомендации")
    if err != nil {
        return getDefaultAdvice(query)
    }
    
    var advice strings.Builder
    advice.WriteString("💡 **Рекомендации:**\n\n")
    
    count := 0
    for _, r := range results {
        if count >= 3 {
            break
        }
        if r.Description != "" && len(r.Description) > 50 {
            advice.WriteString(fmt.Sprintf("• %s\n\n", truncateText(r.Description, 200)))
            count++
        } else if r.Title != "" {
            advice.WriteString(fmt.Sprintf("• %s\n\n", r.Title))
            count++
        }
    }
    
    if count == 0 {
        return getDefaultAdvice(query)
    }
    
    return advice.String()
}

func getDefaultAdvice(query string) string {
    advice := "💡 **Рекомендации:**\n\n"
    
    if strings.Contains(strings.ToLower(query), "дизайн") {
        advice += "• Используйте современный минимализм для лучшего UX\n"
        advice += "• Адаптивный дизайн обязателен для мобильных устройств\n"
        advice += "• Используйте контрастные цвета для важных элементов\n"
        advice += "• Добавьте анимации для улучшения восприятия\n"
    } else if strings.Contains(strings.ToLower(query), "telegram") || strings.Contains(strings.ToLower(query), "бот") {
        advice += "• Добавьте inline-кнопки для удобной навигации\n"
        advice += "• Используйте WebApp для сложного функционала\n"
        advice += "• Настройте систему платежей через Telegram Stars\n"
        advice += "• Добавьте аналитику для отслеживания действий\n"
    } else if strings.Contains(strings.ToLower(query), "интернет магазин") {
        advice += "• Упростите процесс оформления заказа\n"
        advice += "• Добавьте фильтры и поиск для удобства\n"
        advice += "• Показывайте отзывы и рейтинги товаров\n"
        advice += "• Интегрируйте с популярными платежными системами\n"
    } else {
        advice += "• Изучите конкурентов для лучших решений\n"
        advice += "• Сделайте акцент на пользовательском опыте\n"
        advice += "• Добавьте аналитику для сбора данных\n"
        advice += "• Обеспечьте быструю загрузку и производительность\n"
    }
    
    return advice
}

func truncateText(text string, maxLen int) string {
    if len(text) <= maxLen {
        return text
    }
    return text[:maxLen] + "..."
}

func formatPrice(price float64) string {
    if price >= 1000000 {
        return fmt.Sprintf("%.1f млн", price/1000000)
    }
    if price >= 1000 {
        return fmt.Sprintf("%.0f тыс", price/1000)
    }
    return fmt.Sprintf("%.0f", price)
}

func getGreeting() string {
    hour := time.Now().Hour()
    switch {
    case hour >= 5 && hour < 12:
        return "Доброе утро"
    case hour >= 12 && hour < 17:
        return "Добрый день"
    case hour >= 17 && hour < 24:
        return "Добрый вечер"
    default:
        return "Доброй ночи"
    }
}

func isNameResponse(query string) bool {
    q := strings.ToLower(query)
    if len(q) >= 2 && len(q) <= 15 {
        match, _ := regexp.MatchString(`^[а-яa-z]+$`, q)
        if match {
            notName := []string{"бот", "телеграм", "интернет", "магазин", "crm", "доработка", 
                "сколько", "цена", "стоимость", "привет", "платежи", "база", "админ", 
                "рассылки", "помощь", "разработка", "да", "нет", "хочу", "нужно", "разработчик",
                "что", "посоветуешь", "дизайн", "креативный", "стиль", "минимализм", "дизайна"}
            for _, w := range notName {
                if strings.Contains(q, w) {
                    return false
                }
            }
            return true
        }
    }
    return false
}

func detectMessenger(text string) string {
    lower := strings.ToLower(text)
    if strings.Contains(lower, "telegram") || strings.Contains(lower, "тг") || 
       strings.Contains(lower, "телеграм") || strings.Contains(lower, "телеграмм") {
        return "Telegram"
    }
    if strings.Contains(lower, "whatsapp") || strings.Contains(lower, "ватсап") {
        return "WhatsApp"
    }
    if strings.Contains(lower, "viber") || strings.Contains(lower, "вайбер") {
        return "Viber"
    }
    return ""
}

func saveToFile(userName, service, phone, messenger, price, deadline, design, techHelp, additionalInfo string) {
    os.MkdirAll("orders", 0755)
    filename := fmt.Sprintf("orders/order_%s.txt", time.Now().Format("2006-01-02_15-04-05"))
    
    content := fmt.Sprintf("╔════════════════════════════════════════╗\n")
    content += fmt.Sprintf("║         🔥 НОВАЯ ЗАЯВКА 🔥            ║\n")
    content += fmt.Sprintf("╚════════════════════════════════════════╝\n\n")
    content += fmt.Sprintf("📅 Дата: %s\n", time.Now().Format("2006-01-02 15:04:05"))
    content += fmt.Sprintf("👤 Клиент: %s\n", userName)
    content += fmt.Sprintf("📋 Услуга: %s\n", service)
    content += fmt.Sprintf("💰 Стоимость: %s ₽\n", price)
    if design != "" && design != "нет" {
        content += fmt.Sprintf("🎨 Дизайн: %s\n", design)
    }
    if techHelp == "да" {
        content += fmt.Sprintf("🛠 Техподдержка: 15 000 ₽/мес\n")
    }
    if additionalInfo != "" {
        content += fmt.Sprintf("📝 Дополнительно: %s\n", additionalInfo)
    }
    if deadline != "" {
        content += fmt.Sprintf("⏰ Срок: %s\n", deadline)
    }
    if phone != "" {
        content += fmt.Sprintf("📱 Телефон: %s\n", phone)
    }
    if messenger != "" {
        content += fmt.Sprintf("💬 Мессенджер: %s\n", messenger)
    }
    content += fmt.Sprintf("\n📌 Статус: В обработке\n")
    content += fmt.Sprintf("⏱ Время обработки: 15 минут\n")
    content += fmt.Sprintf("\n════════════════════════════════════════\n")
    
    os.WriteFile(filename, []byte(content), 0644)
    log.Printf("✅ Заявка сохранена: %s", filename)
}

func sendEmailNotification(userName, service, phone, messenger, price, deadline, design, techHelp, additionalInfo string) {
    if emailFrom == "" || emailPassword == "" {
        log.Println("⚠️ Email не настроен")
        return
    }
    
    subject := fmt.Sprintf("🔥 Новая заявка от %s", userName)
    
    body := fmt.Sprintf(`<h2>🔥 НОВАЯ ЗАЯВКА</h2>
    <p><strong>👤 Клиент:</strong> %s</p>
    <p><strong>📋 Услуга:</strong> %s</p>
    <p><strong>💰 Стоимость:</strong> %s ₽</p>`, userName, service, price)
    
    if design != "" && design != "нет" {
        body += fmt.Sprintf("<p><strong>🎨 Дизайн:</strong> %s</p>", design)
    }
    if techHelp == "да" {
        body += "<p><strong>🛠 Техподдержка:</strong> 15 000 ₽/мес</p>"
    }
    if additionalInfo != "" {
        body += fmt.Sprintf("<p><strong>📝 Дополнительно:</strong> %s</p>", additionalInfo)
    }
    if deadline != "" {
        body += fmt.Sprintf("<p><strong>⏰ Срок:</strong> %s</p>", deadline)
    }
    if phone != "" {
        body += fmt.Sprintf("<p><strong>📱 Телефон:</strong> %s</p>", phone)
    }
    if messenger != "" {
        body += fmt.Sprintf("<p><strong>💬 Мессенджер:</strong> %s</p>", messenger)
    }
    
    msg := []byte(fmt.Sprintf("To: %s\r\n", emailTo) +
        fmt.Sprintf("From: %s\r\n", emailFrom) +
        fmt.Sprintf("Subject: %s\r\n", subject) +
        "MIME-Version: 1.0\r\n" +
        "Content-Type: text/html; charset=UTF-8\r\n" +
        "\r\n" + body)
    
    addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
    auth := smtp.PlainAuth("", emailFrom, emailPassword, smtpHost)
    
    conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: smtpHost})
    if err != nil {
        log.Printf("❌ Ошибка подключения к SMTP: %v", err)
        return
    }
    defer conn.Close()
    
    client, err := smtp.NewClient(conn, smtpHost)
    if err != nil {
        log.Printf("❌ Ошибка создания SMTP клиента: %v", err)
        return
    }
    defer client.Quit()
    
    if err = client.Auth(auth); err != nil {
        log.Printf("❌ Ошибка аутентификации: %v", err)
        return
    }
    
    if err = client.Mail(emailFrom); err != nil {
        log.Printf("❌ Ошибка отправителя: %v", err)
        return
    }
    
    if err = client.Rcpt(emailTo); err != nil {
        log.Printf("❌ Ошибка получателя: %v", err)
        return
    }
    
    w, err := client.Data()
    if err != nil {
        log.Printf("❌ Ошибка данных: %v", err)
        return
    }
    
    _, err = w.Write(msg)
    if err != nil {
        log.Printf("❌ Ошибка записи: %v", err)
        return
    }
    
    err = w.Close()
    if err != nil {
        log.Printf("❌ Ошибка закрытия: %v", err)
        return
    }
    
    log.Printf("✅ Email отправлен на %s", emailTo)
}

func AIAskHandler(c *gin.Context) {
    var req AskRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.SessionID == "" {
        req.SessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
    }

    dialogMutex.Lock()
    state, exists := dialogStates[req.SessionID]
    if !exists {
        state = &DialogState{
            Messages:    []string{},
            LastUpdated: time.Now(),
            CurrentStep: 0,
        }
        dialogStates[req.SessionID] = state
    }
    state.LastUpdated = time.Now()
    dialogMutex.Unlock()

    question := strings.TrimSpace(req.Question)
    lowerQ := strings.ToLower(question)

    answer := ""
    phoneRegex := regexp.MustCompile(`^(\+7|8|7)?[\s-]?\(?\d{3}\)?[\s-]?\d{3}[\s-]?\d{2}[\s-]?\d{2}$`)
    
    if state.AwaitingPhone {
        if phoneRegex.MatchString(question) {
            state.UserPhone = question
            state.AwaitingPhone = false
            state.AwaitingMessenger = true
            answer = "📱 Отлично! На этом номере есть мессенджер? (Telegram/WhatsApp/Viber)"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        } else {
            answer = "📱 Пожалуйста, введите номер телефона в формате: +7 XXX XXX-XX-XX"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
    }

    if state.AwaitingMessenger {
        messenger := detectMessenger(question)
        if messenger != "" {
            state.UserMessenger = messenger
            state.AwaitingMessenger = false
            state.Completed = true
            
            techHelpStatus := "нет"
            if state.TechHelpAdded {
                techHelpStatus = "да"
            }
            
            go saveToFile(state.UserName, state.UserService, state.UserPhone, state.UserMessenger,
                formatPrice(state.CalculatedPrice), state.Deadline, state.DesignAnswer, techHelpStatus, state.AdditionalService)
            go sendEmailNotification(state.UserName, state.UserService, state.UserPhone, state.UserMessenger,
                formatPrice(state.CalculatedPrice), state.Deadline, state.DesignAnswer, techHelpStatus, state.AdditionalService)
            
            answer = "✅ **Ваша заявка принята!** ✅\n\n" +
                "👨‍💻 Специалист свяжется с вами через 15 минут.\n\n" +
                "🌟 Всего наилучшего!"
            
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        } else {
            answer = "Пожалуйста, укажите мессенджер: Telegram, WhatsApp или Viber"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
    }

    if !state.GreetingShown {
        greeting := getGreeting()
        answer = fmt.Sprintf("%s! 👋 Я AI-помощник студии разработки.\n\n", greeting) +
            "🎯 **Как к вам можно обращаться?**"
        state.GreetingShown = true
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    if state.UserName == "" {
        if isNameResponse(question) {
            state.UserName = question
            answer = fmt.Sprintf("Приятно познакомиться, %s! 🌟\n\n", state.UserName) +
                "📝 **Напишите, что хотите разработать:**\n" +
                "• Telegram бот\n" +
                "• Интернет-магазин\n" +
                "• CRM система\n" +
                "• AI ассистент\n\n" +
                "Или просто опишите свою задачу"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        } else {
            answer = "Пожалуйста, напишите ваше имя:"
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
    }

    if state.UserService == "" {
        price := getAveragePrice(question)
        state.UserService = question
        state.BasePrice = price
        state.CalculatedPrice = price
        
        advice := getAdvice(question)
        
        serviceName := question
        for key, name := range ourServices {
            if strings.Contains(strings.ToLower(question), key) {
                serviceName = name
                break
            }
        }
        
        answer = fmt.Sprintf("🎯 **%s**\n\n", serviceName)
        answer += fmt.Sprintf("💰 **Стоимость разработки:** %s ₽\n\n", formatPrice(price))
        answer += advice + "\n"
        answer += "🎨 **Расскажите о пожеланиях по дизайну:**\n" +
            "• Есть готовый дизайн?\n" +
            "• Нужна разработка с нуля?\n" +
            "• Или могу предложить варианты"
        
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }
    
    if !state.DesignAsked && state.UserService != "" {
        state.DesignAnswer = question
        state.DesignAsked = true
        
        if strings.Contains(lowerQ, "дизайн") || strings.Contains(lowerQ, "вариант") || strings.Contains(lowerQ, "посоветуй") {
            designAdvice := getAdvice(state.UserService + " дизайн примеры")
            answer = designAdvice + "\n\n" +
                "💡 **Хотите добавить техническую поддержку?**\n\n" +
                "🛠️ Техподдержка: 15 000 ₽/мес\n\n" +
                "Добавляем? (Да/Нет)"
        } else {
            answer = "💡 **Хотите добавить техническую поддержку?**\n\n" +
                "🛠️ Техподдержка: 15 000 ₽/мес\n\n" +
                "Добавляем? (Да/Нет)"
        }
        
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }
    
    if !state.TechHelpAsked && state.DesignAsked {
        state.TechHelpAsked = true
        
        if strings.Contains(lowerQ, "да") {
            state.CalculatedPrice += 15000
            state.TechHelpAdded = true
            answer = fmt.Sprintf("✅ Техподдержка добавлена!\n\n💰 **Итоговая стоимость:** %s ₽\n\n", formatPrice(state.CalculatedPrice)) +
                "⏰ **В какие сроки нужен проект?**\n\n" +
                "• Чем быстрее, тем лучше\n" +
                "• 2 недели\n" +
                "• 1 месяц"
            state.DeadlineAsked = true
        } else if strings.Contains(lowerQ, "нет") {
            answer = fmt.Sprintf("✅ Хорошо!\n\n💰 **Итоговая стоимость:** %s ₽\n\n", formatPrice(state.CalculatedPrice)) +
                "⏰ **В какие сроки нужен проект?**\n\n" +
                "• Чем быстрее, тем лучше\n" +
                "• 2 недели\n" +
                "• 1 месяц"
            state.DeadlineAsked = true
        } else {
            answer = "Пожалуйста, ответьте **Да** или **Нет**: добавить техподдержку?"
            state.TechHelpAsked = false
            c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
            return
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }
    
    if state.DeadlineAsked && !state.AwaitingPhone {
        if strings.Contains(lowerQ, "быстре") || strings.Contains(lowerQ, "сроч") {
            originalPrice := state.CalculatedPrice
            state.CalculatedPrice = state.CalculatedPrice * 1.3
            state.Deadline = "Срочно (5-7 дней)"
            
            answer = fmt.Sprintf("🚀 **Срочная разработка!**\n\n") +
                fmt.Sprintf("💰 Стоимость: %s ₽\n", formatPrice(state.CalculatedPrice)) +
                fmt.Sprintf("(было %s ₽, +30%% за срочность)\n\n", formatPrice(originalPrice)) +
                "❓ Согласны? (Да/Нет)"
        } else if strings.Contains(lowerQ, "2 недел") || strings.Contains(lowerQ, "две") {
            state.Deadline = "2 недели"
            answer = fmt.Sprintf("✅ Срок: 2 недели\n\n💰 **Итоговая стоимость:** %s ₽\n\n", formatPrice(state.CalculatedPrice)) +
                "📝 **Для оформления оставьте номер телефона:**"
            state.AwaitingPhone = true
        } else if strings.Contains(lowerQ, "месяц") {
            state.Deadline = "1 месяц"
            answer = fmt.Sprintf("✅ Срок: 1 месяц\n\n💰 **Итоговая стоимость:** %s ₽\n\n", formatPrice(state.CalculatedPrice)) +
                "📝 **Для оформления оставьте номер телефона:**"
            state.AwaitingPhone = true
        } else {
            answer = "Пожалуйста, укажите срок:\n" +
                "• Чем быстрее, тем лучше\n" +
                "• 2 недели\n" +
                "• 1 месяц"
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }
    
    if state.Deadline == "Срочно (5-7 дней)" && !state.AwaitingPhone {
        if strings.Contains(lowerQ, "да") {
            answer = "📝 **Для оформления оставьте номер телефона:**"
            state.AwaitingPhone = true
        } else if strings.Contains(lowerQ, "нет") {
            state.DeadlineAsked = false
            state.Deadline = ""
            answer = "Хорошо, выберите другой срок:\n" +
                "• 2 недели\n" +
                "• 1 месяц"
        } else {
            answer = "Пожалуйста, ответьте **Да** или **Нет**"
        }
        c.JSON(http.StatusOK, gin.H{"answer": answer, "session_id": req.SessionID})
        return
    }

    if answer == "" {
        answer = "Пожалуйста, уточните ваш вопрос, и я помогу вам!"
    }

    state.Messages = append(state.Messages, "assistant: "+answer)
    c.JSON(http.StatusOK, gin.H{
        "answer":     answer,
        "session_id": req.SessionID,
    })
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}