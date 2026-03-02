package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "time"

    "github.com/go-telegram-bot-api/telegram-bot-api/v5"
    "github.com/joho/godotenv"
)

// Хранилище состояний пользователей
var userStates = make(map[int64]string)
var userPayments = make(map[int64]PaymentData)

// Хранилище AI истории и токенов
var userAIUsage = make(map[int64]int)      // chatID -> использовано токенов
var userAIModel = make(map[int64]string)   // chatID -> выбранная модель
var userHistory = make(map[int64][]string) // chatID -> история запросов

// Хранилище обращений в поддержку
var supportTickets = make(map[int64]SupportTicket)

// Хранилище созданных счетов
var invoices = make(map[int64]int64) // chatID -> invoiceID

type SupportTicket struct {
    ID        string
    UserID    int64
    UserName  string
    Question  string
    Status    string // "open", "answered", "closed"
    CreatedAt time.Time
}

type PaymentData struct {
    PlanName   string
    Price      string
    Method     string
    CardNumber string
    CardExpiry string
    CardCVC    string
}

// Структура для ответа от Crypto Pay
type CryptoInvoice struct {
    InvoiceID int64  `json:"invoice_id"`
    PayURL    string `json:"pay_url"`
    Status    string `json:"status"`
}

type CryptoResponse struct {
    OK     bool          `json:"ok"`
    Result CryptoInvoice `json:"result"`
}

// ========== НОВЫЕ ФУНКЦИИ ДЛЯ УЛУЧШЕНИЯ БОТА ==========

// Функция создания нижнего меню с улучшенным дизайном
func createMainMenu() tgbotapi.ReplyKeyboardMarkup {
    keyboard := tgbotapi.NewReplyKeyboard(
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("🤖 Задать вопрос"),
            tgbotapi.NewKeyboardButton("💰 Тарифы"),
            tgbotapi.NewKeyboardButton("👤 Профиль"),
        ),
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("📊 Моя статистика"),
            tgbotapi.NewKeyboardButton("📜 История"),
            tgbotapi.NewKeyboardButton("⚙️ Настройки AI"),
        ),
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("📞 Поддержка"),
            tgbotapi.NewKeyboardButton("ℹ️ Помощь"),
            tgbotapi.NewKeyboardButton("🚀 Mini App"),
        ),
    )
    keyboard.ResizeKeyboard = true
    return keyboard
}

// Функция создания inline-клавиатуры для профиля
func createProfileKeyboard() tgbotapi.InlineKeyboardMarkup {
    return tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 История платежей", "profile_payments"),
            tgbotapi.NewInlineKeyboardButtonData("🔑 API ключи", "profile_apikeys"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "profile_stats"),
            tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
        ),
    )
}

// Функция создания клавиатуры для настроек AI
func createAISettingsKeyboard() tgbotapi.InlineKeyboardMarkup {
    return tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🧠 Модель", "ai_model"),
            tgbotapi.NewInlineKeyboardButtonData("🎨 Креативность", "ai_temperature"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📝 Системный промпт", "ai_prompt"),
            tgbotapi.NewInlineKeyboardButtonData("📊 Квоты", "ai_quota"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
        ),
    )
}

// Функция создания клавиатуры для выбора модели
func createModelKeyboard() tgbotapi.InlineKeyboardMarkup {
    return tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📝 Базовая", "model_basic"),
            tgbotapi.NewInlineKeyboardButtonData("🚀 Продвинутая", "model_advanced"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("⭐ Эксперт", "model_expert"),
            tgbotapi.NewInlineKeyboardButtonData("🔙 Настройки", "back_to_ai_settings"),
        ),
    )
}

// Функция создания клавиатуры для креативности
func createTemperatureKeyboard() tgbotapi.InlineKeyboardMarkup {
    return tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🎯 Точный (0.2)", "temp_0.2"),
            tgbotapi.NewInlineKeyboardButtonData("⚖️ Сбаланс. (0.7)", "temp_0.7"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🎨 Креативный (1.0)", "temp_1.0"),
            tgbotapi.NewInlineKeyboardButtonData("🔙 Настройки", "back_to_ai_settings"),
        ),
    )
}

// ========== ОСНОВНАЯ ФУНКЦИЯ ==========

func main() {
    godotenv.Load("../.env")
    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    
    bot, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        log.Panic(err)
    }
    bot.Debug = true
    log.Printf("Бот запущен: @%s", bot.Self.UserName)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        if update.CallbackQuery != nil {
            handleCallback(bot, update.CallbackQuery)
        } else if update.Message != nil {
            handleMessage(bot, update.Message)
        }
    }
}

func getUserName(user *tgbotapi.User) string {
    if user.UserName != "" {
        return "@" + user.UserName
    }
    return user.FirstName
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
    // Проверяем состояние пользователя
    if state, exists := userStates[message.Chat.ID]; exists {
        handleUserState(bot, message, state)
        return
    }

    // Обработка текстовых кнопок из нижнего меню
    switch message.Text {
    case "🤖 Задать вопрос":
        userStates[message.Chat.ID] = "waiting_question"
        msg := tgbotapi.NewMessage(message.Chat.ID, "🤖 Задайте ваш вопрос:")
        bot.Send(msg)
        
    case "💰 Тарифы":
        showPlans(bot, message.Chat.ID)
        
    case "👤 Профиль":
        showProfile(bot, message.Chat.ID, message.From)
        
    case "📊 Моя статистика":
        showStats(bot, message.Chat.ID)
        
    case "📜 История":
        showHistory(bot, message.Chat.ID)
        
    case "⚙️ Настройки AI":
        showAISettings(bot, message.Chat.ID)
        
    case "📞 Поддержка":
        handleSupport(bot, message.Chat.ID, message.From)
        
    case "ℹ️ Помощь":
        showHelp(bot, message.Chat.ID)
        
    case "🚀 Mini App":
        showMiniApp(bot, message.Chat.ID)
        
    default:
        // Обычные команды
        handleCommand(bot, message)
    }
}

func handleUserState(bot *tgbotapi.BotAPI, message *tgbotapi.Message, state string) {
    switch state {
    case "waiting_card_number":
        data := userPayments[message.Chat.ID]
        data.CardNumber = message.Text
        userPayments[message.Chat.ID] = data
        
        msg := tgbotapi.NewMessage(message.Chat.ID, "📅 Введите срок действия (ММ/ГГ):")
        bot.Send(msg)
        userStates[message.Chat.ID] = "waiting_card_expiry"
        
    case "waiting_card_expiry":
        data := userPayments[message.Chat.ID]
        data.CardExpiry = message.Text
        userPayments[message.Chat.ID] = data
        
        msg := tgbotapi.NewMessage(message.Chat.ID, "🔐 Введите CVC код (3 цифры):")
        bot.Send(msg)
        userStates[message.Chat.ID] = "waiting_card_cvc"
        
    case "waiting_card_cvc":
        data := userPayments[message.Chat.ID]
        data.CardCVC = message.Text
        userPayments[message.Chat.ID] = data
        
        msg := tgbotapi.NewMessage(message.Chat.ID, "🔄 Обработка платежа...")
        bot.Send(msg)
        
        result := fmt.Sprintf("✅ Оплата успешно выполнена!\n\n"+
            "Тариф: *%s*\n"+
            "Сумма: *%s ₽*\n"+
            "Карта: *%s*\n\n"+
            "Подписка активирована!",
            data.PlanName, data.Price, maskCardNumber(data.CardNumber))
        
        msg2 := tgbotapi.NewMessage(message.Chat.ID, result)
        msg2.ParseMode = "Markdown"
        bot.Send(msg2)
        
        delete(userStates, message.Chat.ID)
        delete(userPayments, message.Chat.ID)
        
    case "waiting_question":
        answer := askAI(message.Text)
        userAIUsage[message.Chat.ID] += len(message.Text) / 2
        
        history := userHistory[message.Chat.ID]
        history = append(history, fmt.Sprintf("❓ Вопрос: %s", message.Text))
        history = append(history, fmt.Sprintf("🤖 Ответ: %s", answer))
        if len(history) > 20 {
            history = history[len(history)-20:]
        }
        userHistory[message.Chat.ID] = history
        
        msg := tgbotapi.NewMessage(message.Chat.ID, answer)
        bot.Send(msg)
        delete(userStates, message.Chat.ID)
        
    case "waiting_feedback":
        msg := tgbotapi.NewMessage(message.Chat.ID, 
            "✅ Спасибо за отзыв! Мы обязательно его учтем.")
        bot.Send(msg)
        delete(userStates, message.Chat.ID)
        
    case "waiting_ticket_description":
        ticket := supportTickets[message.Chat.ID]
        ticket.Question = message.Text
        supportTickets[message.Chat.ID] = ticket
        
        confirmText := fmt.Sprintf("✅ Обращение принято!\n\n"+
            "Номер: %s\n"+
            "Ваш вопрос: %s\n\n"+
            "Мы ответим вам в ближайшее время.",
            ticket.ID, message.Text)
        
        msg := tgbotapi.NewMessage(message.Chat.ID, confirmText)
        bot.Send(msg)
        
        log.Printf("Новое обращение %s от %d: %s", ticket.ID, message.Chat.ID, message.Text)
        delete(userStates, message.Chat.ID)
        
    case "waiting_system_prompt":
        userAIPrompt[message.Chat.ID] = message.Text
        msg := tgbotapi.NewMessage(message.Chat.ID, "✅ Системный промпт сохранён!")
        bot.Send(msg)
        delete(userStates, message.Chat.ID)
        showAISettings(bot, message.Chat.ID)
    }
}

func handleCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
    switch message.Text {
    case "/start":
        userName := getUserName(message.From)
        text := fmt.Sprintf(
            "✨ *Добро пожаловать, %s!* ✨\n\n"+
            "┌────────────────────────────────────┐\n"+
            "│  🤖 *SaaS Platform*                │\n"+
            "│  💻 *Сервер: saaspro.ru*           │\n"+
            "│  📊 *Статус: ONLINE*               │\n"+
            "│  ⚡ *Uptime: 99.9%%*                 │\n"+
            "└────────────────────────────────────┘\n\n"+
            "📋 *Наши возможности:*\n"+
            "• 🤖 AI обработка данных\n"+
            "• 🔄 Интеграция с Битрикс24\n"+
            "• 📦 Синхронизация с 1С\n"+
            "• 📈 CRM аналитика\n"+
            "• 🔑 Генерация API ключей\n"+
            "• 🌐 REST API интеграции\n"+
            "• 📊 Дашборды и отчеты\n"+
            "• 🔒 Безопасное хранение данных\n\n"+
            "👤 *Пользователь:* %s\n\n"+
            "👇 *Используйте кнопки внизу для навигации*",
            userName, userName)
        
        msg := tgbotapi.NewMessage(message.Chat.ID, text)
        msg.ParseMode = "Markdown"
        msg.ReplyMarkup = createMainMenu()
        bot.Send(msg)

    case "/plans":
        showPlans(bot, message.Chat.ID)
        
    case "/ask":
        userStates[message.Chat.ID] = "waiting_question"
        msg := tgbotapi.NewMessage(message.Chat.ID, 
            "🤖 Задайте ваш вопрос:")
        bot.Send(msg)
        
    case "/usage":
        showStats(bot, message.Chat.ID)
        
    case "/profile":
        showProfile(bot, message.Chat.ID, message.From)
        
    case "/history":
        showHistory(bot, message.Chat.ID)
        
    case "/feedback":
        userStates[message.Chat.ID] = "waiting_feedback"
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "📝 Напишите ваш отзыв или предложение:")
        bot.Send(msg)
        
    case "/support":
        handleSupport(bot, message.Chat.ID, message.From)
        
    case "/help":
        showHelp(bot, message.Chat.ID)
        
    case "/menu":
        showMainMenu(bot, message.Chat.ID, message.From)
        
    case "/app":
        showMiniApp(bot, message.Chat.ID)
        
    case "/ai-settings":
        showAISettings(bot, message.Chat.ID)
        
    default:
        msg := tgbotapi.NewMessage(message.Chat.ID, 
            "❓ Неизвестная команда. Нажмите /help для списка команд.")
        bot.Send(msg)
    }
}

// ========== НОВЫЕ ФУНКЦИИ ДЛЯ ОТОБРАЖЕНИЯ ==========

func showProfile(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    text := fmt.Sprintf("👤 *Ваш профиль*\n\n"+
        "ID: `%d`\n"+
        "Имя: %s\n"+
        "Telegram: %s\n"+
        "Дата регистрации: %s\n\n"+
        "📊 *Статистика*\n"+
        "• Запросов AI: %d\n"+
        "• Модель: %s\n"+
        "• Токенов: %d/100000\n\n"+
        "💳 *Подписка*\n"+
        "• Статус: Активна\n"+
        "• Тариф: Базовый\n"+
        "• Действует до: %s",
        user.ID, user.FirstName, getUserName(user),
        time.Now().Format("02.01.2006"),
        userAIUsage[chatID],
        getUserModel(chatID),
        userAIUsage[chatID],
        time.Now().AddDate(0, 1, 0).Format("02.01.2006"))
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = createProfileKeyboard()
    bot.Send(msg)
}

func showStats(bot *tgbotapi.BotAPI, chatID int64) {
    usage := userAIUsage[chatID]
    model := getUserModel(chatID)
    
    text := fmt.Sprintf("📊 *Ваша статистика*\n\n"+
        "🤖 *AI использование*\n"+
        "• Запросов: %d\n"+
        "• Модель: %s\n"+
        "• Токенов: %d/100000\n"+
        "• Осталось: %d\n\n"+
        "📈 *Активность*\n"+
        "• Всего диалогов: %d\n"+
        "• Последний: %s",
        usage, model, usage, 100000-usage,
        len(userHistory[chatID])/2,
        getLastActivity(chatID))
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func showHistory(bot *tgbotapi.BotAPI, chatID int64) {
    history := userHistory[chatID]
    if len(history) == 0 {
        msg := tgbotapi.NewMessage(chatID, "📜 История пуста. Задайте свой первый вопрос!")
        bot.Send(msg)
        return
    }
    
    text := "📜 *Последние диалоги:*\n\n"
    for i, entry := range history {
        if i >= 6 { // Показываем последние 3 диалога (вопрос+ответ = 2 строки)
            break
        }
        text += entry + "\n\n"
    }
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func showAISettings(bot *tgbotapi.BotAPI, chatID int64) {
    text := fmt.Sprintf("⚙️ *Настройки AI*\n\n"+
        "🧠 Модель: %s\n"+
        "🎨 Креативность: %s\n"+
        "📊 Квота: %d/100000\n\n"+
        "Выберите параметр для настройки:",
        getUserModel(chatID),
        getUserTemperature(chatID),
        userAIUsage[chatID])
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = createAISettingsKeyboard()
    bot.Send(msg)
}

func showHelp(bot *tgbotapi.BotAPI, chatID int64) {
    text := "ℹ️ *Справка*\n\n"+
        "📱 *Основные команды:*\n"+
        "/start – перезапустить бота\n"+
        "/menu – главное меню\n"+
        "/ask – задать вопрос AI\n"+
        "/plans – посмотреть тарифы\n"+
        "/profile – информация о профиле\n"+
        "/usage – статистика использования\n"+
        "/history – история запросов\n"+
        "/ai-settings – настройки AI\n"+
        "/support – контакты поддержки\n"+
        "/help – эта справка"
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func showMiniApp(bot *tgbotapi.BotAPI, chatID int64) {
    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("🚀 ЗАПУСТИТЬ MINI APP", "https://t.me/AgentServer_bot/app"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
        ),
    )
    
    text := "📱 *MINI APP*\n\n"+
        "Нажмите кнопку ниже, чтобы открыть Mini App!\n\n"+
        "Там вы можете:\n"+
        "• Управлять подписками\n"+
        "• Смотреть аналитику\n"+
        "• Настраивать AI\n"+
        "• Управлять API ключами"
    
    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func showMainMenu(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📱 *Главное меню*\n\nПривет, %s!", getUserName(user)))
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = createMainMenu()
    bot.Send(msg)
}

// ========== ФУНКЦИИ ОБРАБОТКИ CALLBACK ==========

func handleCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    callback := tgbotapi.NewCallback(query.ID, "")
    bot.Request(callback)
    
    log.Printf("Нажата кнопка: %s", query.Data)

    // Обработка навигации
    if query.Data == "back_to_menu" {
        showMainMenu(bot, query.Message.Chat.ID, query.From)
        return
    }
    
    if query.Data == "back_to_plans" {
        showPlans(bot, query.Message.Chat.ID)
        return
    }
    
    if query.Data == "back_to_ai_settings" {
        showAISettings(bot, query.Message.Chat.ID)
        return
    }

    // Обработка профиля
    if strings.HasPrefix(query.Data, "profile_") {
        handleProfileCallback(bot, query)
        return
    }

    // Обработка AI настроек
    if strings.HasPrefix(query.Data, "ai_") {
        handleAICallback(bot, query)
        return
    }

    // Обработка моделей
    if strings.HasPrefix(query.Data, "model_") {
        handleModelCallback(bot, query)
        return
    }

    // Обработка температуры
    if strings.HasPrefix(query.Data, "temp_") {
        handleTemperatureCallback(bot, query)
        return
    }

    // Обработка поддержки
    if strings.HasPrefix(query.Data, "support_") {
        handleSupportCallback(bot, query)
        return
    }

    // Обработка платежей
    if strings.HasPrefix(query.Data, "pay_") || strings.HasPrefix(query.Data, "copy_") || 
       strings.HasPrefix(query.Data, "confirm_") || strings.HasPrefix(query.Data, "check_") {
        handlePaymentCallback(bot, query)
        return
    }

    // Обработка тарифов
    if len(query.Data) > 5 && query.Data[:5] == "plan_" {
        showPaymentMethods(bot, query.Message.Chat.ID, query.Data)
        return
    }

    log.Printf("⚠️ Неизвестная кнопка: %s", query.Data)
}

func handleProfileCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    switch query.Data {
    case "profile_payments":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
            "💰 *История платежей*\n\n"+
            "• 01.03.2026 - 2990₽ (Базовый)\n"+
            "• 01.02.2026 - 2990₽ (Базовый)\n"+
            "• 01.01.2026 - 2990₽ (Базовый)")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "profile_apikeys":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID,
            "🔑 *Ваши API ключи*\n\n"+
            "• PROD-xxxxxxxx\n"+
            "• TEST-xxxxxxxx\n\n"+
            "Создать новый: /generate_key")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "profile_stats":
        showStats(bot, query.Message.Chat.ID)
    }
}

func handleAICallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    switch query.Data {
    case "ai_model":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, "🧠 Выберите модель AI:")
        msg.ReplyMarkup = createModelKeyboard()
        bot.Send(msg)
        
    case "ai_temperature":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, "🎨 Выберите уровень креативности:")
        msg.ReplyMarkup = createTemperatureKeyboard()
        bot.Send(msg)
        
    case "ai_prompt":
        userStates[query.Message.Chat.ID] = "waiting_system_prompt"
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
            "📝 Введите системный промпт (инструкцию для AI):")
        bot.Send(msg)
        
    case "ai_quota":
        usage := userAIUsage[query.Message.Chat.ID]
        msg := tgbotapi.NewMessage(query.Message.Chat.ID,
            fmt.Sprintf("📊 *Использование квот*\n\n"+
                "Использовано: %d токенов\n"+
                "Доступно: 100000 токенов\n"+
                "Осталось: %d токенов\n\n"+
                "Сброс квоты: через %d дней",
                usage, 100000-usage, 30-time.Now().Day()))
        msg.ParseMode = "Markdown"
        bot.Send(msg)
    }
}

func handleModelCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    var modelName string
    
    switch query.Data {
    case "model_basic":
        modelName = "Базовая"
        userAIModel[query.Message.Chat.ID] = "basic"
    case "model_advanced":
        modelName = "Продвинутая"
        userAIModel[query.Message.Chat.ID] = "advanced"
    case "model_expert":
        modelName = "Эксперт"
        userAIModel[query.Message.Chat.ID] = "expert"
    }
    
    msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
        fmt.Sprintf("✅ Модель изменена на *%s*", modelName))
    msg.ParseMode = "Markdown"
    bot.Send(msg)
    
    // Возвращаемся к настройкам AI
    showAISettings(bot, query.Message.Chat.ID)
}

func handleTemperatureCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    var tempName string
    temp := 0.7
    
    switch query.Data {
    case "temp_0.2":
        tempName = "Точный"
        temp = 0.2
    case "temp_0.7":
        tempName = "Сбалансированный"
        temp = 0.7
    case "temp_1.0":
        tempName = "Креативный"
        temp = 1.0
    }
    
    userAITemp[query.Message.Chat.ID] = temp
    
    msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
        fmt.Sprintf("✅ Креативность изменена на *%s* (%.1f)", tempName, temp))
    msg.ParseMode = "Markdown"
    bot.Send(msg)
    
    // Возвращаемся к настройкам AI
    showAISettings(bot, query.Message.Chat.ID)
}

func handlePaymentCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    data := query.Data
    
    switch {
    case strings.HasPrefix(data, "pay_card_"):
        planClean := strings.TrimPrefix(data, "pay_card_")
        startCardPayment(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "pay_usdt_"):
        planClean := strings.TrimPrefix(data, "pay_usdt_")
        startUSDTPayment(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "pay_btc_"):
        planClean := strings.TrimPrefix(data, "pay_btc_")
        startBTCPayment(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "pay_sbp_"):
        planClean := strings.TrimPrefix(data, "pay_sbp_")
        startSBPPayment(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "pay_crypto_"):
        planClean := strings.TrimPrefix(data, "pay_crypto_")
        startCryptoPayment(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "copy_usdt_"):
        planClean := strings.TrimPrefix(data, "copy_usdt_")
        copyUSDTAddress(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "copy_btc_"):
        planClean := strings.TrimPrefix(data, "copy_btc_")
        copyBTCAddress(bot, query.Message.Chat.ID, planClean)
        
    case strings.HasPrefix(data, "confirm_usdt_"):
        planClean := strings.TrimPrefix(data, "confirm_usdt_")
        confirmPayment(bot, query.Message.Chat.ID, "USDT", planClean)
        
    case strings.HasPrefix(data, "confirm_btc_"):
        planClean := strings.TrimPrefix(data, "confirm_btc_")
        confirmPayment(bot, query.Message.Chat.ID, "Bitcoin", planClean)
        
    case strings.HasPrefix(data, "confirm_sbp_"):
        planClean := strings.TrimPrefix(data, "confirm_sbp_")
        confirmPayment(bot, query.Message.Chat.ID, "СБП", planClean)
        
    case strings.HasPrefix(data, "confirm_crypto_"):
        planClean := strings.TrimPrefix(data, "confirm_crypto_")
        confirmPayment(bot, query.Message.Chat.ID, "Crypto Bot", planClean)
        
    case data == "check_crypto_status":
        checkCryptoPayment(bot, query.Message.Chat.ID)
    }
}

func handleSupportCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    switch query.Data {
    case "support_chat":
        text := "💬 Чат с поддержкой\n\n" +
            "Нажмите кнопку ниже, чтобы написать @IDamieN66I\n\n" +
            "Мы онлайн 24/7 и ответим в течение нескольких минут!"

        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonURL("💬 Написать", "https://t.me/IDamieN66I"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_support"),
            ),
        )

        msg := tgbotapi.NewMessage(query.Message.Chat.ID, text)
        msg.ReplyMarkup = keyboard
        bot.Send(msg)

    case "support_faq":
        text := "❓ Часто задаваемые вопросы\n\n" +
            "1️⃣ Как оформить подписку?\n" +
            "   Нажмите /plans, выберите тариф и следуйте инструкциям.\n\n" +
            "2️⃣ Какие способы оплаты?\n" +
            "   Карта, USDT, Bitcoin, СБП, Crypto Bot.\n\n" +
            "3️⃣ Как сменить тариф?\n" +
            "   В разделе /profile есть кнопка 'Сменить тариф'.\n\n" +
            "4️⃣ Как отменить подписку?\n" +
            "   Напишите в поддержку, мы поможем.\n\n" +
            "5️⃣ Сколько токенов в день?\n" +
            "   100 000 токенов в месяц на всех тарифах."

        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_support"),
            ),
        )

        msg := tgbotapi.NewMessage(query.Message.Chat.ID, text)
        msg.ReplyMarkup = keyboard
        bot.Send(msg)

    case "support_ticket":
        ticketID := fmt.Sprintf("TICKET-%d", time.Now().UnixNano()%10000)
        supportTickets[query.Message.Chat.ID] = SupportTicket{
            ID:        ticketID,
            UserID:    query.From.ID,
            UserName:  query.From.FirstName,
            Status:    "open",
            CreatedAt: time.Now(),
        }

        text := fmt.Sprintf("📝 Создание обращения\n\n"+
            "Ваш номер обращения: %s\n\n"+
            "Опишите вашу проблему одним сообщением.\n"+
            "Мы ответим в ближайшее время.",
            ticketID)

        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "back_to_support"),
            ),
        )

        msg := tgbotapi.NewMessage(query.Message.Chat.ID, text)
        msg.ReplyMarkup = keyboard
        bot.Send(msg)

        userStates[query.Message.Chat.ID] = "waiting_ticket_description"
    }
}

// ========== ФУНКЦИИ ДЛЯ РАБОТЫ С AI ==========

var userAITemp = make(map[int64]float64)   // chatID -> температура
var userAIPrompt = make(map[int64]string)   // chatID -> системный промпт

func getUserModel(chatID int64) string {
    if model, ok := userAIModel[chatID]; ok {
        return model
    }
    return "Базовая"
}

func getUserTemperature(chatID int64) string {
    if temp, ok := userAITemp[chatID]; ok {
        switch temp {
        case 0.2:
            return "Точный"
        case 0.7:
            return "Сбалансированный"
        case 1.0:
            return "Креативный"
        }
    }
    return "Сбалансированный"
}

func getLastActivity(chatID int64) string {
    history := userHistory[chatID]
    if len(history) == 0 {
        return "нет активности"
    }
    return "сегодня"
}

func askAI(question string) string {
    // Отправляем запрос к бэкенду
    resp, err := http.Post("http://localhost:8080/api/ai/ask", 
        "application/json", 
        strings.NewReader(fmt.Sprintf(`{"question":"%s"}`, question)))
    
    if err != nil {
        return "❌ Ошибка вызова AI. Бэкенд недоступен."
    }
    defer resp.Body.Close()

    var result struct {
        Answer string `json:"answer"`
    }
    
    body, _ := io.ReadAll(resp.Body)
    json.Unmarshal(body, &result)

    if result.Answer == "" {
        return "❌ Не удалось получить ответ от AI"
    }

    return "🤖 " + result.Answer
}

// ========== ФУНКЦИИ ДЛЯ ПЛАТЕЖЕЙ ==========

func showPlans(bot *tgbotapi.BotAPI, chatID int64) {
    plansText := "*💰 Доступные тарифы*\n\n" +
        "┌─────────────────────┐\n" +
        "│ *Базовый*           │\n" +
        "│ Для небольших команд │\n" +
        "│ 💰 2 990 ₽/мес      │\n" +
        "├─────────────────────┤\n" +
        "│ *Профессиональный*  │\n" +
        "│ Для растущего бизнеса│\n" +
        "│ 💰 29 900 ₽/мес     │\n" +
        "├─────────────────────┤\n" +
        "│ *Корпоративный*     │\n" +
        "│ Для крупных компаний │\n" +
        "│ 💰 49 000 ₽/мес     │\n" +
        "├─────────────────────┤\n" +
        "│ *Семейный*          │\n" +
        "│ Для всей семьи       │\n" +
        "│ 💰 9 900 ₽/мес      │\n" +
        "└─────────────────────┘"

    msg := tgbotapi.NewMessage(chatID, plansText)
    msg.ParseMode = "Markdown"
    bot.Send(msg)

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Базовый", "plan_basic"),
            tgbotapi.NewInlineKeyboardButtonData("💰 Семейный", "plan_family"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Профессиональный", "plan_pro"),
            tgbotapi.NewInlineKeyboardButtonData("💰 Корпоративный", "plan_enterprise"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
        ),
    )

    msg2 := tgbotapi.NewMessage(chatID, "👇 Нажмите для оплаты:")
    msg2.ReplyMarkup = keyboard
    bot.Send(msg2)
}

func showPaymentMethods(bot *tgbotapi.BotAPI, chatID int64, planType string) {
    var planName, price string

    switch planType {
    case "plan_basic":
        planName = "Базовый"
        price = "2990"
    case "plan_family":
        planName = "Семейный"
        price = "9900"
    case "plan_pro":
        planName = "Профессиональный"
        price = "29900"
    case "plan_enterprise":
        planName = "Корпоративный"
        price = "49000"
    }

    planClean := strings.TrimPrefix(planType, "plan_")

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💳 Карта", "pay_card_"+planClean),
            tgbotapi.NewInlineKeyboardButtonData("₮ USDT", "pay_usdt_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("₿ Bitcoin", "pay_btc_"+planClean),
            tgbotapi.NewInlineKeyboardButtonData("📱 СБП", "pay_sbp_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🪙 Crypto Bot", "pay_crypto_"+planClean),
            tgbotapi.NewInlineKeyboardButtonData("❓ FAQ", "support_faq"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 К тарифам", "back_to_plans"),
            tgbotapi.NewInlineKeyboardButtonData("🔝 В меню", "back_to_menu"),
        ),
    )

    text := fmt.Sprintf("✅ *%s*\n", planName) +
        fmt.Sprintf("💰 Сумма: *%s ₽*\n\n", price) +
        "Выберите способ оплаты:"

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func startCardPayment(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    var planName, price string

    switch planClean {
    case "basic":
        planName = "Базовый"
        price = "2990"
    case "family":
        planName = "Семейный"
        price = "9900"
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    }

    userPayments[chatID] = PaymentData{
        PlanName: planName,
        Price:    price,
        Method:   "card",
    }

    text := "💳 *Оплата картой*\n\n" +
        "Введите номер карты (16 цифр):"

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "back_to_plans"),
        ),
    )

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)

    userStates[chatID] = "waiting_card_number"
}

func startUSDTPayment(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    var planName, price string

    switch planClean {
    case "basic":
        planName = "Базовый"
        price = "2990"
    case "family":
        planName = "Семейный"
        price = "9900"
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    }

    address := "TXmRt1UqWqfJ1XxqZQk3yL7vFhKpDnA2jB"
    usdtAmount := fmt.Sprintf("%.2f", float64(atoi(price))/90)

    text := fmt.Sprintf("💰 *Оплата USDT (TRC-20)*\n\n") +
        fmt.Sprintf("Тариф: *%s*\n", planName) +
        fmt.Sprintf("Сумма: *%s ₽* = *%s USDT*\n\n", price, usdtAmount) +
        "📤 *Адрес для перевода:*\n" +
        fmt.Sprintf("`%s`\n\n", address) +
        "1️⃣ Нажмите 'Копировать адрес'\n" +
        "2️⃣ Отправьте USDT\n" +
        "3️⃣ После отправки нажмите '✅ Я оплатил'"

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📋 Копировать адрес", "copy_usdt_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("✅ Я оплатил", "confirm_usdt_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_plans"),
        ),
    )

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func startBTCPayment(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    var planName, price string

    switch planClean {
    case "basic":
        planName = "Базовый"
        price = "2990"
    case "family":
        planName = "Семейный"
        price = "9900"
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    }

    address := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
    btcAmount := fmt.Sprintf("%.6f", float64(atoi(price))/4500000)

    text := fmt.Sprintf("₿ *Оплата Bitcoin*\n\n") +
        fmt.Sprintf("Тариф: *%s*\n", planName) +
        fmt.Sprintf("Сумма: *%s ₽* = *%s BTC*\n\n", price, btcAmount) +
        "📤 *Адрес для перевода:*\n" +
        fmt.Sprintf("`%s`\n\n", address) +
        "1️⃣ Нажмите 'Копировать адрес'\n" +
        "2️⃣ Отправьте Bitcoin\n" +
        "3️⃣ После отправки нажмите '✅ Я оплатил'"

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📋 Копировать адрес", "copy_btc_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("✅ Я оплатил", "confirm_btc_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_plans"),
        ),
    )

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func startSBPPayment(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    var planName, price string

    switch planClean {
    case "basic":
        planName = "Базовый"
        price = "2990"
    case "family":
        planName = "Семейный"
        price = "9900"
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    }

    qrData := fmt.Sprintf("СБП оплата %s %s руб", planName, price)
    qrURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=%s", qrData)

    text := fmt.Sprintf("📱 *Оплата по СБП*\n\n") +
        fmt.Sprintf("Тариф: *%s*\n", planName) +
        fmt.Sprintf("Сумма: *%s ₽*\n\n", price) +
        "1️⃣ Нажмите кнопку 'Показать QR-код'\n" +
        "2️⃣ Отсканируйте код в приложении банка\n" +
        "3️⃣ После оплаты нажмите '✅ Я оплатил'"

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("📱 Показать QR-код", qrURL),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("✅ Я оплатил", "confirm_sbp_"+planClean),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_plans"),
        ),
    )

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func startCryptoPayment(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    var planName, price string
    var usdtAmount float64

    switch planClean {
    case "basic":
        planName = "Базовый"
        price = "2990"
        usdtAmount = 33.22
    case "family":
        planName = "Семейный"
        price = "9900"
        usdtAmount = 110.00
    case "pro":
        planName = "Профессиональный"
        price = "29900"
        usdtAmount = 332.22
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
        usdtAmount = 544.44
    }

    log.Printf("🪙 CRYPTO PAY: создание счета для %s на %s RUB (%.2f USDT)", planName, price, usdtAmount)

    cryptoToken := os.Getenv("CRYPTO_PAY_TOKEN")
    if cryptoToken == "" {
        cryptoToken = "539564:AA31bHY40rT3NI0Fhw6no5BHCwWmftxquGM"
    }

    invoice, err := createCryptoInvoice(cryptoToken, usdtAmount, planName)
    if err != nil {
        log.Printf("Ошибка создания счета: %v", err)
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка создания счета. Попробуйте позже.")
        bot.Send(msg)
        return
    }

    invoices[chatID] = invoice.InvoiceID

    text := fmt.Sprintf("🪙 *Оплата через Crypto Bot*\n\n") +
        fmt.Sprintf("Тариф: *%s*\n", planName) +
        fmt.Sprintf("Сумма: *%s ₽* = *%.2f USDT*\n", price, usdtAmount) +
        fmt.Sprintf("ID счета: `%d`\n\n", invoice.InvoiceID) +
        "🔗 *Ссылка для оплаты:*\n" +
        fmt.Sprintf("[Перейти к оплате](%s)\n\n", invoice.PayURL) +
        "1️⃣ Нажмите на ссылку выше\n" +
        "2️⃣ Оплатите в @CryptoBot\n" +
        "3️⃣ Нажмите 'Проверить оплату'"

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("🪙 Перейти к оплате", invoice.PayURL),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔄 Проверить оплату", "check_crypto_status"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_plans"),
        ),
    )

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

func createCryptoInvoice(token string, amount float64, description string) (*CryptoInvoice, error) {
    url := "https://pay.crypt.bot/api/createInvoice"
    
    amountStr := fmt.Sprintf("%.2f", amount)
    
    client := &http.Client{}
    reqBody := fmt.Sprintf("asset=USDT&amount=%s&description=%s", amountStr, description)
    
    req, err := http.NewRequest("POST", url, strings.NewReader(reqBody))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Crypto-Pay-API-Token", token)
    
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    log.Printf("Crypto Pay response: %s", string(body))
    
    var result CryptoResponse
    if err := json.Unmarshal(body, &result); err != nil {
        return nil, err
    }
    
    if !result.OK {
        return nil, fmt.Errorf("API error: %s", string(body))
    }
    
    return &result.Result, nil
}

func checkCryptoPayment(bot *tgbotapi.BotAPI, chatID int64) {
    invoiceID, exists := invoices[chatID]
    if !exists {
        msg := tgbotapi.NewMessage(chatID, "❌ Счет не найден. Создайте новый платеж.")
        bot.Send(msg)
        return
    }

    cryptoToken := os.Getenv("CRYPTO_PAY_TOKEN")
    if cryptoToken == "" {
        cryptoToken = "539564:AA31bHY40rT3NI0Fhw6no5BHCwWmftxquGM"
    }

    status, err := getInvoiceStatus(cryptoToken, invoiceID)
    if err != nil {
        msg := tgbotapi.NewMessage(chatID, "❌ Ошибка проверки статуса. Попробуйте позже.")
        bot.Send(msg)
        return
    }

    if status == "paid" {
        msg := tgbotapi.NewMessage(chatID,
            "✅ *Платеж подтвержден!*\n\n"+
                "Подписка активирована!")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
        delete(invoices, chatID)
    } else {
        msg := tgbotapi.NewMessage(chatID, "⏳ Платеж еще не получен. Ожидайте подтверждения сети.")
        bot.Send(msg)
    }
}

func getInvoiceStatus(token string, invoiceID int64) (string, error) {
    url := fmt.Sprintf("https://pay.crypt.bot/api/getInvoice?invoice_id=%d", invoiceID)
    
    client := &http.Client{}
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return "", err
    }
    
    req.Header.Set("Crypto-Pay-API-Token", token)
    
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var result struct {
        OK     bool `json:"ok"`
        Result struct {
            Status string `json:"status"`
        } `json:"result"`
    }
    
    body, _ := io.ReadAll(resp.Body)
    json.Unmarshal(body, &result)
    
    if !result.OK {
        return "unknown", nil
    }
    
    return result.Result.Status, nil
}

func copyUSDTAddress(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    address := "TXmRt1UqWqfJ1XxqZQk3yL7vFhKpDnA2jB"
    msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Адрес скопирован:\n`%s`", address))
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func copyBTCAddress(bot *tgbotapi.BotAPI, chatID int64, planClean string) {
    address := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
    msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Адрес скопирован:\n`%s`", address))
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func confirmPayment(bot *tgbotapi.BotAPI, chatID int64, method, planClean string) {
    var planName, price string

    switch planClean {
    case "basic":
        planName = "Базовый"
        price = "2990"
    case "family":
        planName = "Семейный"
        price = "9900"
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    }

    msg := tgbotapi.NewMessage(chatID,
        fmt.Sprintf("✅ *Платеж подтвержден!*\n\n")+
            fmt.Sprintf("Способ: %s\n", method)+
            fmt.Sprintf("Тариф: %s\n", planName)+
            fmt.Sprintf("Сумма: %s ₽\n\n", price)+
            "Подписка активирована!")
    msg.ParseMode = "Markdown"
    bot.Send(msg)
}

func maskCardNumber(card string) string {
    if len(card) >= 16 {
        return card[:4] + " **** **** " + card[12:]
    }
    return "****"
}

func atoi(s string) int {
    var result int
    fmt.Sscanf(s, "%d", &result)
    return result
}

func handleSupport(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    text := fmt.Sprintf("📞 *Поддержка*\n\n"+
        "Здравствуйте, %s!\n\n"+
        "Вы можете связаться с нами:\n"+
        "• Email: support@saaspro.ru\n"+
        "• Telegram: @saaspro_support\n"+
        "• Чат: 24/7 онлайн\n\n"+
        "Среднее время ответа: 15 минут",
        user.FirstName)

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    bot.Send(msg)

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("📱 Написать в Telegram", "https://t.me/saaspro_support"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💬 Чат", "support_chat"),
            tgbotapi.NewInlineKeyboardButtonData("❓ FAQ", "support_faq"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📝 Обращение", "support_ticket"),
            tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
        ),
    )

    keyboardMsg := tgbotapi.NewMessage(chatID, "Выберите действие:")
    keyboardMsg.ReplyMarkup = keyboard
    bot.Send(keyboardMsg)
}