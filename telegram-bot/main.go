package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"

    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
    "github.com/joho/godotenv"
)

type UserData struct {
    Token string
    Model string
}

var (
    userData = make(map[int64]UserData)
    mu       sync.RWMutex
    apiBase  = "http://localhost:8080"

    availableModels = []string{
        "yandex/yandexgpt-lite",
        "deepseek/deepseek-chat",
        "openai/gpt-4.1-mini",
        "gigachat/gigachat-max",
        "ollama/llama3.2",
    }

    newPlanTemp = make(map[int64]*newPlanData)
)

type newPlanData struct {
    Step         int
    Name         string
    Code         string
    Description  string
    PriceMonthly float64
    PriceYearly  float64
    Currency     string
    AIQuota      int64
    AIModels     string
    IsActive     bool
    Features     []string
    MaxUsers     int
    SortOrder    int
}

func main() {
    if err := godotenv.Load(); err != nil {
        log.Println("⚠️ .env file not loaded, using environment variables")
    } else {
        fmt.Println("✅ .env file loaded")
    }

    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    if token == "" {
        log.Fatal("TELEGRAM_BOT_TOKEN not set")
    }

    bot, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        log.Fatal(err)
    }

    bot.Debug = true
    log.Printf("✅ Бот запущен: @%s", bot.Self.UserName)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 30

    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        if update.Message != nil {
            chatID := update.Message.Chat.ID
            text := update.Message.Text

            if strings.HasPrefix(text, "/") {
                handleCommand(bot, chatID, text, update.Message.From)
            } else {
                if data, ok := newPlanTemp[chatID]; ok {
                    handleCreatePlanStep(bot, chatID, text, data)
                    continue
                }
                msg := tgbotapi.NewMessage(chatID, "Используйте команды: /start, /setkey, /ask, /plans, /usage, /setmodel, /profile, /history, /feedback, /support, /admin, /stats, /users, /broadcast, /block, /unblock, /menu, /adminplans, /help")
                bot.Send(msg)
            }
        } else if update.CallbackQuery != nil {
            handleCallback(bot, update.CallbackQuery)
        }
    }
}

func isAdmin(userID int64) bool {
    adminID, err := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
    if err != nil {
        return false
    }
    return userID == adminID
}

func handleCommand(bot *tgbotapi.BotAPI, chatID int64, text string, user *tgbotapi.User) {
    parts := strings.Fields(text)
    cmd := parts[0]

    switch cmd {
    case "/start":
        start(bot, chatID, user)
    case "/setkey":
        if len(parts) < 2 {
            msg := tgbotapi.NewMessage(chatID, "Использование: /setkey ВАШ_API_КЛЮЧ")
            bot.Send(msg)
            return
        }
        setKey(bot, chatID, parts[1])
    case "/ask":
        if len(parts) < 2 {
            msg := tgbotapi.NewMessage(chatID, "Использование: /ask ваш вопрос")
            bot.Send(msg)
            return
        }
        question := strings.Join(parts[1:], " ")
        askAI(bot, chatID, question, user)
    case "/plans":
        showPlans(bot, chatID, user)
    case "/usage":
        showUsage(bot, chatID, user)
    case "/setmodel":
        showModelSelection(bot, chatID, user)
    case "/profile":
        showProfile(bot, chatID, user)
    case "/history":
        showHistory(bot, chatID, user)
    case "/feedback":
        if len(parts) < 2 {
            msg := tgbotapi.NewMessage(chatID, "Использование: /feedback ваш текст")
            bot.Send(msg)
            return
        }
        feedbackText := strings.Join(parts[1:], " ")
        feedback(bot, chatID, feedbackText, user)
    case "/support":
        support(bot, chatID)
    case "/admin":
        if !isAdmin(chatID) {
            msg := tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён")
            bot.Send(msg)
            return
        }
        showAdminHelp(bot, chatID)
    case "/stats":
        if !isAdmin(chatID) {
            msg := tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён")
            bot.Send(msg)
            return
        }
        adminStats(bot, chatID, user)
    case "/users":
        if !isAdmin(chatID) {
            msg := tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён")
            bot.Send(msg)
            return
        }
        adminUsers(bot, chatID, user)
    case "/block", "/unblock":
        if !isAdmin(chatID) {
            msg := tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён")
            bot.Send(msg)
            return
        }
        if len(parts) < 2 {
            msg := tgbotapi.NewMessage(chatID, "Использование: /block <user_id>  или  /unblock <user_id>")
            bot.Send(msg)
            return
        }
        targetUserID := parts[1]
        isActive := cmd == "/unblock"
        adminToggleBlock(bot, chatID, targetUserID, isActive, user)
    case "/broadcast":
        if !isAdmin(chatID) {
            msg := tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён")
            bot.Send(msg)
            return
        }
        if len(parts) < 2 {
            msg := tgbotapi.NewMessage(chatID, "Использование: /broadcast <текст сообщения>")
            bot.Send(msg)
            return
        }
        broadcastText := strings.Join(parts[1:], " ")
        adminBroadcast(bot, chatID, broadcastText, user)
    case "/menu":
        showMainMenu(bot, chatID)
    case "/adminplans":
        if !isAdmin(chatID) {
            msg := tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён")
            bot.Send(msg)
            return
        }
        adminListPlans(bot, chatID, user)
    case "/help":
        showHelp(bot, chatID)
    default:
        msg := tgbotapi.NewMessage(chatID, "Неизвестная команда. Доступно: /start, /setkey, /ask, /plans, /usage, /setmodel, /profile, /history, /feedback, /support, /admin, /stats, /users, /broadcast, /block, /unblock, /menu, /adminplans, /help")
        bot.Send(msg)
    }
}

func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
    chatID := callback.Message.Chat.ID
    data := callback.Data

    callbackCfg := tgbotapi.NewCallback(callback.ID, "")
    bot.Send(callbackCfg)

    switch {
    case strings.HasPrefix(data, "buy_plan_"):
        planID := strings.TrimPrefix(data, "buy_plan_")
        buyPlan(bot, chatID, planID, callback.Message.From)
    case strings.HasPrefix(data, "setmodel_"):
        model := strings.TrimPrefix(data, "setmodel_")
        setModel(bot, chatID, model)
    case strings.HasPrefix(data, "menu_"):
        action := strings.TrimPrefix(data, "menu_")
        switch action {
        case "ask":
            bot.Send(tgbotapi.NewMessage(chatID, "Отправьте команду /ask <вопрос>"))
        case "plans":
            showPlans(bot, chatID, callback.Message.From)
        case "usage":
            showUsage(bot, chatID, callback.Message.From)
        case "model":
            showModelSelection(bot, chatID, callback.Message.From)
        case "profile":
            showProfile(bot, chatID, callback.Message.From)
        case "history":
            showHistory(bot, chatID, callback.Message.From)
        case "support":
            support(bot, chatID)
        case "help":
            showHelp(bot, chatID)
        case "admin":
            if !isAdmin(chatID) {
                bot.Send(tgbotapi.NewMessage(chatID, "⛔ Доступ запрещён"))
                return
            }
            showAdminHelp(bot, chatID)
        }
    case strings.HasPrefix(data, "edit_plan_"):
        bot.Send(tgbotapi.NewMessage(chatID, "Редактирование плана пока не реализовано. Используйте /adminplans для списка."))
    case strings.HasPrefix(data, "delete_plan_"):
        planID := strings.TrimPrefix(data, "delete_plan_")
        adminDeletePlan(bot, chatID, planID, callback.Message.From)
    case data == "create_plan":
        newPlanTemp[chatID] = &newPlanData{Step: 0}
        msg := tgbotapi.NewMessage(chatID, "Введите название нового плана:")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case data == "plan_active_true":
        d, ok := newPlanTemp[chatID]
        if !ok {
            bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: данные не найдены. Начните заново."))
            return
        }
        d.IsActive = true
        createPlanFinal(bot, chatID, d)
    case data == "plan_active_false":
        d, ok := newPlanTemp[chatID]
        if !ok {
            bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка: данные не найдены. Начните заново."))
            return
        }
        d.IsActive = false
        createPlanFinal(bot, chatID, d)
    }
}

// ИСПРАВЛЕННАЯ ФУНКЦИЯ START – использует прямой HTTP-запрос к Telegram API
func start(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    miniAppURL := os.Getenv("MINI_APP_URL")
    if miniAppURL == "" {
        miniAppURL = "https://default-url.com"
    }

    // 1. Формируем приветственный текст
    welcome := fmt.Sprintf(
        "👋 Привет, %s!\n\n"+
            "Я бот SaaS-платформы. Я автоматически создам для вас аккаунт и API-ключ при первом запросе.\n\n"+
            "После этого вы сможете:\n"+
            "/ask – задать вопрос AI\n"+
            "/plans – посмотреть тарифы\n"+
            "/usage – узнать остаток токенов\n"+
            "/setmodel – выбрать модель AI\n"+
            "/profile – информация о вашем профиле\n"+
            "/history – история AI-запросов\n"+
            "/feedback – отправить отзыв\n"+
            "/support – контакты поддержки\n"+
            "/admin – админ-панель (для администратора)\n"+
            "/menu – главное меню\n"+
            "/adminplans – управление тарифами (админ)\n"+
            "/help – справка",
        user.FirstName)

    // 2. Создаём структуру для inline-клавиатуры с WebApp-кнопкой
    keyboard := map[string]interface{}{
        "inline_keyboard": [][]map[string]interface{}{
            {
                {
                    "text": "🚀 Открыть мини-приложение",
                    "web_app": map[string]string{
                        "url": miniAppURL,
                    },
                },
            },
        },
    }

    // 3. Преобразуем в JSON
    payload := map[string]interface{}{
        "chat_id":      chatID,
        "text":         welcome,
        "reply_markup": keyboard,
    }
    jsonPayload, _ := json.Marshal(payload)

    // 4. Отправляем POST-запрос к Telegram API
    apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", bot.Token)
    resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(jsonPayload))
    if err != nil {
        log.Printf("Ошибка отправки сообщения: %v", err)
        return
    }
    defer resp.Body.Close()

    // Если хотите проверить ответ, можно прочитать:
    // body, _ := io.ReadAll(resp.Body)
    // log.Printf("Ответ Telegram: %s", body)
}

func setKey(bot *tgbotapi.BotAPI, chatID int64, key string) {
    mu.Lock()
    userData[chatID] = UserData{Token: key, Model: "yandex/yandexgpt-lite"}
    mu.Unlock()
    msg := tgbotapi.NewMessage(chatID, "✅ Ключ сохранён! Модель по умолчанию: YandexGPT. Используйте /setmodel для смены.")
    bot.Send(msg)
}

func ensureUserKey(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) (string, error) {
    mu.RLock()
    data, ok := userData[chatID]
    mu.RUnlock()
    if ok && data.Token != "" {
        return data.Token, nil
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    reqBody := map[string]interface{}{
        "telegram_id":   chatID,
        "telegram_name": user.UserName,
    }
    jsonBody, _ := json.Marshal(reqBody)

    resp, err := http.Post(apiBase+"/api/telegram/ensure-key", "application/json", bytes.NewBuffer(jsonBody))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("server returned %d", resp.StatusCode)
    }

    var keyResp struct {
        Token string `json:"token"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&keyResp); err != nil {
        return "", err
    }

    mu.Lock()
    userData[chatID] = UserData{Token: keyResp.Token, Model: "yandex/yandexgpt-lite"}
    mu.Unlock()

    return keyResp.Token, nil
}

func askAI(bot *tgbotapi.BotAPI, chatID int64, question string, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        log.Printf("Ошибка получения ключа: %v", err)
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    mu.RLock()
    model := userData[chatID].Model
    mu.RUnlock()

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    body := map[string]interface{}{
        "model": model,
        "messages": []map[string]string{
            {"role": "user", "content": question},
        },
        "stream": false,
    }
    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", apiBase+"/api/v1/chat/completions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("❌ Ошибка вызова AI Gateway: %v", err)
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка связи с сервером.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    log.Printf("📥 Полный ответ от провайдера: %s", string(bodyBytes))

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка: сервер вернул код %d", resp.StatusCode))
        bot.Send(msg)
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
        log.Printf("❌ Ошибка парсинга ответа: %v", err)
        msg := tgbotapi.NewMessage(chatID, "❌ Неверный ответ от сервера.")
        bot.Send(msg)
        return
    }

    if len(result.Choices) == 0 {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить ответ от AI.")
        bot.Send(msg)
        return
    }

    answer := result.Choices[0].Message.Content
    for _, chunk := range splitString(answer, 4000) {
        bot.Send(tgbotapi.NewMessage(chatID, chunk))
    }
}

func showPlans(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    req, _ := http.NewRequest("GET", apiBase+"/api/plans", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при загрузке тарифов.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var plansResp struct {
        Plans []struct {
            ID           int     `json:"id"`
            Name         string  `json:"name"`
            Description  string  `json:"description"`
            PriceMonthly float64 `json:"price_monthly"`
        } `json:"plans"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&plansResp); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    if len(plansResp.Plans) == 0 {
        msg := tgbotapi.NewMessage(chatID, "Нет доступных тарифов.")
        bot.Send(msg)
        return
    }

    var text string
    var keyboardRows [][]tgbotapi.InlineKeyboardButton

    for _, p := range plansResp.Plans {
        text += fmt.Sprintf("*%s*\n%s\n💰 %.2f ₽/мес\n\n", p.Name, p.Description, p.PriceMonthly)
        btn := tgbotapi.NewInlineKeyboardButtonData(
            fmt.Sprintf("💰 Купить %s", p.Name),
            fmt.Sprintf("buy_plan_%d", p.ID),
        )
        keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(btn))
    }

    msg := tgbotapi.NewMessage(chatID, text)
    if len(keyboardRows) > 0 {
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
    }
    bot.Send(msg)
}

func buyPlan(bot *tgbotapi.BotAPI, chatID int64, planID string, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    id, err := strconv.Atoi(planID)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Неверный идентификатор тарифа.")
        bot.Send(msg)
        return
    }

    body := map[string]interface{}{
        "plan_id":      id,
        "period_month": 1,
    }
    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", apiBase+"/api/subscriptions", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("❌ Ошибка вызова API подписок: %v", err)
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка связи с сервером при оформлении подписки.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        bodyBytes, _ := io.ReadAll(resp.Body)
        log.Printf("❌ Ошибка создания подписки: %d %s", resp.StatusCode, string(bodyBytes))
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Не удалось оформить подписку (код %d). Возможно, у вас уже есть активная подписка.", resp.StatusCode))
        bot.Send(msg)
        return
    }

    msg := tgbotapi.NewMessage(chatID, "✅ Подписка успешно оформлена! Спасибо за покупку.")
    bot.Send(msg)
}

func showUsage(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    req, _ := http.NewRequest("GET", apiBase+"/api/user/keys", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе данных.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var keysResp struct {
        Keys []struct {
            QuotaLimit int64 `json:"quota_limit"`
            QuotaUsed  int64 `json:"quota_used"`
        } `json:"keys"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&keysResp); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    if len(keysResp.Keys) == 0 {
        msg := tgbotapi.NewMessage(chatID, "У вас нет активных ключей.")
        bot.Send(msg)
        return
    }

    keyInfo := keysResp.Keys[0]
    quotaText := fmt.Sprintf("📊 *Использование токенов*\n\nИспользовано: %d", keyInfo.QuotaUsed)
    if keyInfo.QuotaLimit == -1 {
        quotaText += "\nЛимит: безлимит"
    } else {
        quotaText += fmt.Sprintf(" из %d", keyInfo.QuotaLimit)
        percent := float64(keyInfo.QuotaUsed) / float64(keyInfo.QuotaLimit) * 100
        quotaText += fmt.Sprintf("\nИзрасходовано: %.1f%%", percent)
    }
    msg := tgbotapi.NewMessage(chatID, quotaText)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func showModelSelection(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    _ = token

    var keyboardRows [][]tgbotapi.InlineKeyboardButton
    for _, model := range availableModels {
        btn := tgbotapi.NewInlineKeyboardButtonData(model, "setmodel_"+model)
        keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(btn))
    }

    msg := tgbotapi.NewMessage(chatID, "Выберите модель AI:")
    msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
    bot.Send(msg)
}

func setModel(bot *tgbotapi.BotAPI, chatID int64, model string) {
    mu.Lock()
    data, ok := userData[chatID]
    if ok {
        data.Model = model
        userData[chatID] = data
    }
    mu.Unlock()

    if ok {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Модель изменена на: %s", model))
        bot.Send(msg)
    } else {
        msg := tgbotapi.NewMessage(chatID, "❌ Сначала добавьте API-ключ командой /setkey")
        bot.Send(msg)
    }
}

func showProfile(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    req, _ := http.NewRequest("GET", apiBase+"/api/user/profile", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе профиля.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var profileResp struct {
        User struct {
            ID        string  `json:"id"`
            Email     string  `json:"email"`
            Name      *string `json:"name"`
            Role      string  `json:"role"`
            CreatedAt string  `json:"created_at"`
            UpdatedAt string  `json:"updated_at"`
        } `json:"user"`
        AIRequestsCount int `json:"ai_requests_count"`
        Subscription    *struct {
            ID                int     `json:"id"`
            PlanID            int     `json:"plan_id"`
            PlanName          *string `json:"plan_name"`
            Status            string  `json:"status"`
            CurrentPeriodStart *string `json:"current_period_start"`
            CurrentPeriodEnd   *string `json:"current_period_end"`
            CancelAtPeriodEnd  bool    `json:"cancel_at_period_end"`
            TrialEnd           *string `json:"trial_end"`
            PaymentMethod      *string `json:"payment_method"`
            AITokensUsed       *int64  `json:"ai_tokens_used"`
            CreatedAt          string  `json:"created_at"`
            UpdatedAt          string  `json:"updated_at"`
        } `json:"subscription"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&profileResp); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    text := fmt.Sprintf("👤 *Ваш профиль*\n\n")
    if profileResp.User.Name != nil {
        text += fmt.Sprintf("Имя: %s\n", *profileResp.User.Name)
    }
    text += fmt.Sprintf("Email: %s\n", profileResp.User.Email)
    text += fmt.Sprintf("Роль: %s\n", profileResp.User.Role)
    text += fmt.Sprintf("ID: %s\n", profileResp.User.ID)
    text += fmt.Sprintf("Дата регистрации: %s\n", profileResp.User.CreatedAt[:10])
    text += fmt.Sprintf("AI-запросов: %d\n", profileResp.AIRequestsCount)

    if profileResp.Subscription != nil {
        sub := profileResp.Subscription
        text += "\n📋 *Активная подписка*\n"
        if sub.PlanName != nil {
            text += fmt.Sprintf("Тариф: %s\n", *sub.PlanName)
        } else {
            text += fmt.Sprintf("ID тарифа: %d\n", sub.PlanID)
        }
        text += fmt.Sprintf("Статус: %s\n", sub.Status)
        if sub.CurrentPeriodStart != nil && sub.CurrentPeriodEnd != nil {
            text += fmt.Sprintf("Период: %s – %s\n", (*sub.CurrentPeriodStart)[:10], (*sub.CurrentPeriodEnd)[:10])
        }
        if sub.CancelAtPeriodEnd {
            text += "⚠️ Подписка будет отменена в конце периода\n"
        }
        if sub.AITokensUsed != nil {
            text += fmt.Sprintf("Использовано токенов AI: %d\n", *sub.AITokensUsed)
        }
        if sub.PaymentMethod != nil {
            text += fmt.Sprintf("Метод оплаты: %s\n", *sub.PaymentMethod)
        }
    } else {
        text += "\n*Подписка*: нет активной подписки"
    }

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func showHistory(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ. Попробуйте позже или введите вручную командой /setkey")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    req, _ := http.NewRequest("GET", apiBase+"/api/user/ai-history", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе истории.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var historyResp struct {
        History []struct {
            ID        int    `json:"id"`
            Question  string `json:"question"`
            Answer    string `json:"answer"`
            CreatedAt string `json:"created_at"`
        } `json:"history"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    if len(historyResp.History) == 0 {
        msg := tgbotapi.NewMessage(chatID, "📭 У вас пока нет AI-запросов.")
        bot.Send(msg)
        return
    }

    text := "📜 *Последние AI-запросы:*\n\n"
    for i, req := range historyResp.History {
        if i >= 10 {
            break
        }
        date := req.CreatedAt[:10] // YYYY-MM-DD
        question := req.Question
        if len(question) > 50 {
            question = question[:50] + "..."
        }
        text += fmt.Sprintf("%d. *%s*\n   Вопрос: %s\n   Ответ: %s\n\n",
            i+1, date, question, req.Answer)
    }

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func feedback(bot *tgbotapi.BotAPI, chatID int64, text string, user *tgbotapi.User) {
    adminID, err := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
    if err != nil {
        log.Printf("ADMIN_CHAT_ID not set or invalid")
        msg := tgbotapi.NewMessage(chatID, "❌ Функция обратной связи временно недоступна.")
        bot.Send(msg)
        return
    }

    feedbackText := fmt.Sprintf("📬 *Новый отзыв от %s (@%s):*\n\n%s",
        user.FirstName, user.UserName, text)
    msg := tgbotapi.NewMessage(adminID, feedbackText)
    msg.ParseMode = "Markdown"
    if _, err := bot.Send(msg); err != nil {
        log.Printf("Ошибка отправки отзыва админу: %v", err)
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось отправить отзыв. Попробуйте позже.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewMessage(chatID, "✅ Спасибо за ваш отзыв!"))
}

func support(bot *tgbotapi.BotAPI, chatID int64) {
    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("💬 Чат поддержки", "https://t.me/your_support_chat"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("🌐 Сайт", "https://example.com/support"),
        ),
    )
    msg := tgbotapi.NewMessage(chatID, "📞 *Поддержка*\n\nВыберите способ связи:")
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func showMainMenu(bot *tgbotapi.BotAPI, chatID int64) {
    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🤖 Задать вопрос", "menu_ask"),
            tgbotapi.NewInlineKeyboardButtonData("📋 Тарифы", "menu_plans"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📊 Использование", "menu_usage"),
            tgbotapi.NewInlineKeyboardButtonData("⚙️ Модель", "menu_model"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("👤 Профиль", "menu_profile"),
            tgbotapi.NewInlineKeyboardButtonData("📜 История", "menu_history"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📞 Поддержка", "menu_support"),
            tgbotapi.NewInlineKeyboardButtonData("ℹ️ Помощь", "menu_help"),
        ),
    )
    if isAdmin(chatID) {
        adminRow := tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("👑 Админка", "menu_admin"),
            tgbotapi.NewInlineKeyboardButtonData("📦 Управление тарифами", "adminplans"),
        )
        keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, adminRow)
    }

    msg := tgbotapi.NewMessage(chatID, "📱 *Главное меню*\nВыберите действие:")
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func showAdminHelp(bot *tgbotapi.BotAPI, chatID int64) {
    text := `👑 *Админ-панель*

/stats – статистика
/users – список пользователей
/broadcast <текст> – рассылка
/block <user_id> – заблокировать пользователя
/unblock <user_id> – разблокировать
/adminplans – управление тарифами

*Внимание:* команды доступны только администратору.`
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func adminStats(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    req, _ := http.NewRequest("GET", apiBase+"/api/admin/stats", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе статистики.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var stats struct {
        TotalUsers         int `json:"total_users"`
        ActiveSubscriptions int `json:"active_subscriptions"`
        TotalAIRequests    int `json:"total_ai_requests"`
        TotalAPIKeys       int `json:"total_api_keys"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    text := fmt.Sprintf(`📊 *Статистика системы*

👥 Всего пользователей: %d
✅ Активных подписок: %d
🤖 AI-запросов: %d
🔑 API-ключей: %d`,
        stats.TotalUsers, stats.ActiveSubscriptions, stats.TotalAIRequests, stats.TotalAPIKeys)

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func adminUsers(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    req, _ := http.NewRequest("GET", apiBase+"/api/admin/users", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе списка пользователей.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var data struct {
        Users []struct {
            ID               string  `json:"id"`
            Email            string  `json:"email"`
            Name             *string `json:"name"`
            Role             string  `json:"role"`
            TelegramID       *int64  `json:"telegram_id"`
            TelegramUsername *string `json:"telegram_username"`
            IsActive         bool    `json:"is_active"`
            CreatedAt        string  `json:"created_at"`
        } `json:"users"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    if len(data.Users) == 0 {
        bot.Send(tgbotapi.NewMessage(chatID, "Нет пользователей."))
        return
    }

    escapeHTML := func(s string) string {
        s = strings.ReplaceAll(s, "&", "&amp;")
        s = strings.ReplaceAll(s, "<", "&lt;")
        s = strings.ReplaceAll(s, ">", "&gt;")
        return s
    }

    text := "<b>👥 Последние пользователи</b>\n\n"
    for i, u := range data.Users {
        if i >= 10 {
            break
        }
        status := "✅"
        if !u.IsActive {
            status = "❌"
        }
        name := ""
        if u.Name != nil {
            name = escapeHTML(*u.Name)
        }
        email := escapeHTML(u.Email)
        role := escapeHTML(u.Role)
        tg := ""
        if u.TelegramUsername != nil {
            tg = "@" + escapeHTML(*u.TelegramUsername)
        } else if u.TelegramID != nil {
            tg = fmt.Sprintf("id%d", *u.TelegramID)
        }
        text += fmt.Sprintf("%s <b>%s</b> (%s) %s\n   Роль: %s, создан: %s\n\n",
            status, name, email, tg, role, u.CreatedAt[:10])
    }

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "HTML"
    bot.Send(msg)
}

func adminToggleBlock(bot *tgbotapi.BotAPI, chatID int64, targetUserID string, isActive bool, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    body := map[string]bool{"is_active": isActive}
    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequest("PUT", apiBase+"/api/admin/users/"+targetUserID+"/block", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при выполнении операции.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    action := "разблокирован"
    if !isActive {
        action = "заблокирован"
    }
    bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Пользователь %s %s.", targetUserID, action)))
}

func adminBroadcast(bot *tgbotapi.BotAPI, chatID int64, message string, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    body := map[string]string{"message": message}
    jsonBody, _ := json.Marshal(body)

    req, _ := http.NewRequest("POST", apiBase+"/api/admin/broadcast", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе рассылки.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var broadcastResp struct {
        Recipients []int64 `json:"recipients"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&broadcastResp); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    if len(broadcastResp.Recipients) == 0 {
        bot.Send(tgbotapi.NewMessage(chatID, "Нет получателей для рассылки."))
        return
    }

    bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("⏳ Начинаю рассылку %d пользователям...", len(broadcastResp.Recipients))))

    go func() {
        sent := 0
        failed := 0
        for _, tid := range broadcastResp.Recipients {
            msg := tgbotapi.NewMessage(tid, "📢 *Рассылка от администратора*\n\n"+message)
            msg.ParseMode = "Markdown"
            _, err := bot.Send(msg)
            if err != nil {
                failed++
                log.Printf("Ошибка отправки пользователю %d: %v", tid, err)
            } else {
                sent++
            }
            time.Sleep(50 * time.Millisecond)
        }
        bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Рассылка завершена. Отправлено: %d, ошибок: %d", sent, failed)))
    }()
}

func adminListPlans(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    req, _ := http.NewRequest("GET", apiBase+"/api/admin/plans", nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при запросе планов.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d", resp.StatusCode))
        bot.Send(msg)
        return
    }

    var plansResp struct {
        Plans []struct {
            ID           int      `json:"id"`
            Name         string   `json:"name"`
            Code         string   `json:"code"`
            Description  string   `json:"description"`
            PriceMonthly float64  `json:"price_monthly"`
            PriceYearly  float64  `json:"price_yearly"`
            Currency     string   `json:"currency"`
            AIQuota      int64    `json:"ai_quota"`
            AIModels     []string `json:"ai_models"`
            IsActive     bool     `json:"is_active"`
        } `json:"plans"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&plansResp); err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось обработать ответ.")
        bot.Send(msg)
        return
    }

    if len(plansResp.Plans) == 0 {
        bot.Send(tgbotapi.NewMessage(chatID, "Нет доступных тарифов."))
        return
    }

    var text string
    var keyboardRows [][]tgbotapi.InlineKeyboardButton

    for _, p := range plansResp.Plans {
        status := "✅"
        if !p.IsActive {
            status = "❌"
        }
        text += fmt.Sprintf("*%s* %s (ID: %d)\n", status, p.Name, p.ID)
        text += fmt.Sprintf("Код: `%s`\n", p.Code)
        text += fmt.Sprintf("Описание: %s\n", p.Description)
        text += fmt.Sprintf("💰 Месяц: %.2f %s\n", p.PriceMonthly, p.Currency)
        text += fmt.Sprintf("💰 Год: %.2f %s\n", p.PriceYearly, p.Currency)
        text += fmt.Sprintf("🤖 Квота AI: %d\n", p.AIQuota)
        text += fmt.Sprintf("📋 Модели: %v\n\n", p.AIModels)

        row := tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("✏️ Ред. %d", p.ID), fmt.Sprintf("edit_plan_%d", p.ID)),
            tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🗑️ Удалить %d", p.ID), fmt.Sprintf("delete_plan_%d", p.ID)),
        )
        keyboardRows = append(keyboardRows, row)
    }

    keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(
        tgbotapi.NewInlineKeyboardButtonData("➕ Создать план", "create_plan"),
    ))

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    if len(keyboardRows) > 0 {
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
    }
    bot.Send(msg)
}

func adminDeletePlan(bot *tgbotapi.BotAPI, chatID int64, planID string, user *tgbotapi.User) {
    token, err := ensureUserKey(bot, chatID, user)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ.")
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))

    req, _ := http.NewRequest("DELETE", apiBase+"/api/admin/plans/"+planID, nil)
    req.Header.Set("Authorization", "Bearer "+token)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка при удалении плана.")
        bot.Send(msg)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d\n%s", resp.StatusCode, string(bodyBytes)))
        bot.Send(msg)
        return
    }

    bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ План %s успешно удалён.", planID)))
    adminListPlans(bot, chatID, user)
}

func handleCreatePlanStep(bot *tgbotapi.BotAPI, chatID int64, text string, data *newPlanData) {
    switch data.Step {
    case 0:
        data.Name = text
        data.Step = 1
        msg := tgbotapi.NewMessage(chatID, "Введите код плана (уникальный идентификатор, например 'basic'):")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 1:
        data.Code = text
        data.Step = 2
        msg := tgbotapi.NewMessage(chatID, "Введите описание плана:")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 2:
        data.Description = text
        data.Step = 3
        msg := tgbotapi.NewMessage(chatID, "Введите цену за месяц (число, например 990):")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 3:
        price, err := strconv.ParseFloat(text, 64)
        if err != nil {
            bot.Send(tgbotapi.NewMessage(chatID, "❌ Неверное число. Попробуйте ещё раз:"))
            return
        }
        data.PriceMonthly = price
        data.Step = 4
        msg := tgbotapi.NewMessage(chatID, "Введите цену за год (число, например 9900):")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 4:
        price, err := strconv.ParseFloat(text, 64)
        if err != nil {
            bot.Send(tgbotapi.NewMessage(chatID, "❌ Неверное число. Попробуйте ещё раз:"))
            return
        }
        data.PriceYearly = price
        data.Step = 5
        msg := tgbotapi.NewMessage(chatID, "Введите валюту (например, RUB):")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 5:
        data.Currency = text
        data.Step = 6
        msg := tgbotapi.NewMessage(chatID, "Введите AI-квоту (количество токенов, например 1000000):")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 6:
        quota, err := strconv.ParseInt(text, 10, 64)
        if err != nil {
            bot.Send(tgbotapi.NewMessage(chatID, "❌ Неверное число. Попробуйте ещё раз:"))
            return
        }
        data.AIQuota = quota
        data.Step = 7
        msg := tgbotapi.NewMessage(chatID, "Введите разрешённые модели через запятую (или * для всех):")
        msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
        bot.Send(msg)
    case 7:
        data.AIModels = text
        data.Step = 8
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("✅ Да", "plan_active_true"),
                tgbotapi.NewInlineKeyboardButtonData("❌ Нет", "plan_active_false"),
            ),
        )
        msg := tgbotapi.NewMessage(chatID, "Активировать план сейчас?")
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
    }
}

func createPlanFinal(bot *tgbotapi.BotAPI, chatID int64, data *newPlanData) {
    defer delete(newPlanTemp, chatID)

    token, err := ensureUserKey(bot, chatID, &tgbotapi.User{ID: chatID})
    if err != nil {
        bot.Send(tgbotapi.NewMessage(chatID, "❌ Не удалось получить API-ключ."))
        return
    }

    var models []string
    if data.AIModels == "*" {
        models = []string{"*"}
    } else {
        for _, m := range strings.Split(data.AIModels, ",") {
            models = append(models, strings.TrimSpace(m))
        }
    }

    reqBody := map[string]interface{}{
        "name":          data.Name,
        "code":          data.Code,
        "description":   data.Description,
        "price_monthly": data.PriceMonthly,
        "price_yearly":  data.PriceYearly,
        "currency":      data.Currency,
        "ai_quota":      data.AIQuota,
        "ai_models":     models,
        "is_active":     data.IsActive,
        "max_users":     1,
        "features":      []string{},
        "sort_order":    0,
    }
    jsonBody, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST", apiBase+"/api/admin/plans", bytes.NewBuffer(jsonBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка при создании плана."))
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        bodyBytes, _ := io.ReadAll(resp.Body)
        bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ Ошибка сервера: %d\n%s", resp.StatusCode, string(bodyBytes))))
        return
    }

    bot.Send(tgbotapi.NewMessage(chatID, "✅ План успешно создан!"))
    adminListPlans(bot, chatID, &tgbotapi.User{ID: chatID})
}

func showHelp(bot *tgbotapi.BotAPI, chatID int64) {
    helpText := `🤖 *Доступные команды:*

/start – приветствие
/setkey <ключ> – установить API-ключ (если хотите свой)
/ask <вопрос> – задать вопрос AI
/plans – показать тарифы
/usage – узнать остаток токенов
/setmodel – выбрать модель AI
/profile – информация о вашем профиле
/history – история AI-запросов
/feedback <текст> – отправить отзыв
/support – контакты поддержки
/admin – админ-панель (доступно администратору)
/menu – главное меню
/adminplans – управление тарифами (админ)
/help – эта справка

*Доступные модели:*
• yandex/yandexgpt-lite
• deepseek/deepseek-chat
• openai/gpt-4.1-mini
• gigachat/gigachat-max
• ollama/llama3.2
`
    msg := tgbotapi.NewMessage(chatID, helpText)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func splitString(s string, maxLen int) []string {
    var chunks []string
    for len(s) > maxLen {
        idx := strings.LastIndex(s[:maxLen], "\n")
        if idx == -1 {
            idx = maxLen
        }
        chunks = append(chunks, s[:idx])
        s = s[idx:]
    }
    if len(s) > 0 {
        chunks = append(chunks, s)
    }
    return chunks
}