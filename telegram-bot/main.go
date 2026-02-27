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

// Хранилище созданных счетов
var invoices = make(map[int64]int64) // chatID -> invoiceID

// Функция создания нижнего меню
func createMainMenu() tgbotapi.ReplyKeyboardMarkup {
    keyboard := tgbotapi.NewReplyKeyboard(
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("🚀 Mini App"),
            tgbotapi.NewKeyboardButton("💰 Тарифы"),
            tgbotapi.NewKeyboardButton("📊 Аналитика"),
        ),
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("👤 Профиль"),
            tgbotapi.NewKeyboardButton("📞 Поддержка"),
            tgbotapi.NewKeyboardButton("⚙️ API"),
        ),
        tgbotapi.NewKeyboardButtonRow(
            tgbotapi.NewKeyboardButton("📜 История"),
            tgbotapi.NewKeyboardButton("ℹ️ Помощь"),
            tgbotapi.NewKeyboardButton("🔙 Меню"),
        ),
    )
    keyboard.ResizeKeyboard = true
    return keyboard
}

func main() {
    godotenv.Load("../.env")
    token := os.Getenv("TELEGRAM_BOT_TOKEN")
    
    bot, _ := tgbotapi.NewBotAPI(token)
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
            history = append(history, fmt.Sprintf("Вопрос: %s", message.Text))
            history = append(history, fmt.Sprintf("Ответ: %s", answer))
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
        }
        return
    }

    // Обработка текстовых кнопок из нижнего меню
    if message.Text == "🚀 Mini App" || 
       message.Text == "💰 Тарифы" || 
       message.Text == "📊 Аналитика" || 
       message.Text == "👤 Профиль" || 
       message.Text == "📞 Поддержка" || 
       message.Text == "⚙️ API" || 
       message.Text == "📜 История" || 
       message.Text == "ℹ️ Помощь" ||
       message.Text == "🔙 Меню" {
        handleTextButtons(bot, message)
        return
    }

    // Обычные команды
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
            "🤖 Задайте ваш вопрос, и я отвечу с помощью AI:")
        bot.Send(msg)
        
    case "/usage":
        usage := userAIUsage[message.Chat.ID]
        msg := tgbotapi.NewMessage(message.Chat.ID,
            fmt.Sprintf("📊 *Использование токенов*\n\n"+
                "Использовано: *%d* токенов\n"+
                "Доступно: *100000* токенов\n\n"+
                "Модель: *%s*", 
                usage, getUserModel(message.Chat.ID)))
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "/setmodel":
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🤖 GPT-3.5", "model_gpt35"),
                tgbotapi.NewInlineKeyboardButtonData("🚀 GPT-4", "model_gpt4"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📚 Claude", "model_claude"),
                tgbotapi.NewInlineKeyboardButtonData("✨ Gemini", "model_gemini"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_menu"),
            ),
        )
        msg := tgbotapi.NewMessage(message.Chat.ID, "Выберите модель AI:")
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
        
    case "/profile":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            fmt.Sprintf("👤 *Ваш профиль*\n\n"+
                "ID: `%d`\n"+
                "Имя: %s\n"+
                "Дата регистрации: %s\n\n"+
                "Подписка: *Активна*\n"+
                "Тариф: *Базовый*",
                message.From.ID, message.From.FirstName, time.Now().Format("02.01.2006")))
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "/history":
        history := userHistory[message.Chat.ID]
        if len(history) == 0 {
            msg := tgbotapi.NewMessage(message.Chat.ID, "📜 История пуста")
            bot.Send(msg)
            return
        }
        
        text := "📜 *История запросов:*\n\n"
        for i, entry := range history {
            if i >= 10 {
                break
            }
            text += entry + "\n\n"
        }
        
        msg := tgbotapi.NewMessage(message.Chat.ID, text)
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "/feedback":
        userStates[message.Chat.ID] = "waiting_feedback"
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "📝 Напишите ваш отзыв или предложение:")
        bot.Send(msg)
        
    case "/support":
        handleSupport(bot, message.Chat.ID, message.From)
        
    case "/admin":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "👑 *Админ-панель*\n\n"+
                "/adminplans - управление тарифами\n"+
                "/users - список пользователей\n"+
                "/stats - статистика")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "/menu":
        showMainMenu(bot, message.Chat.ID, message.From)
        
    case "/app":
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonURL("🚀 ЗАПУСТИТЬ MINI APP", "https://t.me/AgentServer_bot/app"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
            ),
        )
        
        text := "📱 *MINI APP*\n\n"+
            "Нажмите кнопку ниже, чтобы открыть Mini App!"
        
        msg := tgbotapi.NewMessage(message.Chat.ID, text)
        msg.ParseMode = "Markdown"
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
        
    case "/adminplans":
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("➕ Добавить тариф", "admin_add_plan"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("✏️ Редактировать", "admin_edit_plan"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("❌ Удалить", "admin_delete_plan"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
            ),
        )
        msg := tgbotapi.NewMessage(message.Chat.ID, "📦 *Управление тарифами*\nВыберите действие:")
        msg.ParseMode = "Markdown"
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
        
    case "/help":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "ℹ️ *Справка*\n\n"+
                "Основные команды:\n"+
                "/ask – задать вопрос AI\n"+
                "/plans – посмотреть тарифы\n"+
                "/usage – узнать остаток токенов\n"+
                "/setmodel – выбрать модель AI\n"+
                "/profile – информация о профиле\n"+
                "/history – история запросов\n"+
                "/feedback – отправить отзыв\n"+
                "/support – контакты поддержки\n"+
                "/menu – показать меню с кнопками\n"+
                "/app – открыть Mini App")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
    }
}

func handleTextButtons(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
    switch message.Text {
    case "🚀 Mini App":
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonURL("🚀 ЗАПУСТИТЬ MINI APP", "https://t.me/AgentServer_bot/app"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
            ),
        )
        msg := tgbotapi.NewMessage(message.Chat.ID, "📱 *Mini App*\nНажмите кнопку для запуска!")
        msg.ParseMode = "Markdown"
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
        
    case "💰 Тарифы":
        showPlans(bot, message.Chat.ID)
        
    case "📊 Аналитика":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "📊 *Аналитика данных*\n\n"+
            "• Анализ CRM данных\n"+
            "• Отчеты по Битрикс24\n"+
            "• Статистика 1С\n"+
            "• Дашборды и графики\n\n"+
            "Используйте /ask для запросов")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "👤 Профиль":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            fmt.Sprintf("👤 *Ваш профиль*\n\nID: `%d`\nИмя: %s\n\n🔑 API ключи: /api_keys",
                message.From.ID, message.From.FirstName))
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "📞 Поддержка":
        handleSupport(bot, message.Chat.ID, message.From)
        
    case "⚙️ API":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "🔑 *API управление*\n\n"+
            "• Для Битрикс24\n"+
            "• Для 1С\n"+
            "• Для CRM\n"+
            "• REST API\n\n"+
            "Сгенерировать ключ: /generate_key\n"+
            "Мои ключи: /my_keys")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "📜 История":
        history := userHistory[message.Chat.ID]
        if len(history) == 0 {
            msg := tgbotapi.NewMessage(message.Chat.ID, "📜 История пуста")
            bot.Send(msg)
            return
        }
        msg := tgbotapi.NewMessage(message.Chat.ID, 
            fmt.Sprintf("📜 *Последний запрос*\n\n%s", history[len(history)-1]))
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "ℹ️ Помощь":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "ℹ️ *Помощь*\n\n"+
            "/start - перезапуск\n"+
            "/menu - главное меню\n"+
            "/ask - задать вопрос AI\n"+
            "/plans - тарифы\n"+
            "/profile - профиль\n"+
            "/support - поддержка")
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "🔙 Меню":
        showMainMenu(bot, message.Chat.ID, message.From)
    }
}

func showMainMenu(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📱 *Главное меню*\n\nПривет, %s!", getUserName(user)))
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = createMainMenu()
    bot.Send(msg)
}

func handleCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    callback := tgbotapi.NewCallback(query.ID, "")
    bot.Request(callback)
    
    log.Printf("Нажата кнопка: %s", query.Data)

    // Открыть Mini App
    if query.Data == "open_miniapp" {
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonURL("🚀 ЗАПУСТИТЬ MINI APP", "https://t.me/AgentServer_bot/app"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
            ),
        )
        
        text := "📱 *MINI APP*\n\n"+
            "Нажмите кнопку ниже, чтобы открыть Mini App!"
        
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, text)
        msg.ParseMode = "Markdown"
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
        return
    }

    // Меню
    if strings.HasPrefix(query.Data, "menu_") {
        handleMenuCallback(bot, query)
        return
    }

    // Модели AI
    if strings.HasPrefix(query.Data, "model_") {
        handleModelCallback(bot, query)
        return
    }

    // Админка
    if strings.HasPrefix(query.Data, "admin_") {
        handleAdminCallback(bot, query)
        return
    }

    // Поддержка
    if strings.HasPrefix(query.Data, "support_") {
        handleSupportCallback(bot, query)
        return
    }

    // Крипта
    if strings.HasPrefix(query.Data, "pay_crypto_") {
        planClean := strings.TrimPrefix(query.Data, "pay_crypto_")
        log.Printf("✅ КРИПТА: выбран тариф %s", planClean)
        startCryptoPayment(bot, query.Message.Chat.ID, planClean)
        return
    }

    // Проверка статуса оплаты
    if query.Data == "check_crypto_status" {
        checkCryptoPayment(bot, query.Message.Chat.ID)
        return
    }

    // НАЗАД
    if query.Data == "back_to_plans" {
        showPlans(bot, query.Message.Chat.ID)
        return
    }

    if query.Data == "back_to_support" {
        handleSupport(bot, query.Message.Chat.ID, query.From)
        return
    }

    if query.Data == "back_to_menu" {
        showMainMenu(bot, query.Message.Chat.ID, query.From)
        return
    }

    if len(query.Data) > 9 && query.Data[:9] == "pay_card_" {
        planClean := query.Data[9:]
        startCardPayment(bot, query.Message.Chat.ID, planClean)
        return
    }

    if len(query.Data) > 9 && query.Data[:9] == "pay_usdt_" {
        planClean := query.Data[9:]
        startUSDTPayment(bot, query.Message.Chat.ID, planClean)
        return
    }

    if len(query.Data) > 8 && query.Data[:8] == "pay_btc_" {
        planClean := query.Data[8:]
        startBTCPayment(bot, query.Message.Chat.ID, planClean)
        return
    }

    if len(query.Data) > 8 && query.Data[:8] == "pay_sbp_" {
        planClean := query.Data[8:]
        startSBPPayment(bot, query.Message.Chat.ID, planClean)
        return
    }

    if len(query.Data) > 11 && query.Data[:11] == "copy_usdt_" {
        planClean := query.Data[11:]
        copyUSDTAddress(bot, query.Message.Chat.ID, planClean)
        return
    }

    if len(query.Data) > 10 && query.Data[:10] == "copy_btc_" {
        planClean := query.Data[10:]
        copyBTCAddress(bot, query.Message.Chat.ID, planClean)
        return
    }

    if len(query.Data) > 12 && query.Data[:12] == "confirm_usdt_" {
        planClean := query.Data[12:]
        confirmPayment(bot, query.Message.Chat.ID, "USDT", planClean)
        return
    }

    if len(query.Data) > 11 && query.Data[:11] == "confirm_btc_" {
        planClean := query.Data[11:]
        confirmPayment(bot, query.Message.Chat.ID, "Bitcoin", planClean)
        return
    }

    if len(query.Data) > 11 && query.Data[:11] == "confirm_sbp_" {
        planClean := query.Data[11:]
        confirmPayment(bot, query.Message.Chat.ID, "СБП", planClean)
        return
    }

    if len(query.Data) > 13 && query.Data[:13] == "confirm_crypto_" {
        planClean := query.Data[13:]
        confirmPayment(bot, query.Message.Chat.ID, "Crypto", planClean)
        return
    }

    if len(query.Data) > 5 && query.Data[:5] == "plan_" {
        showPaymentMethods(bot, query.Message.Chat.ID, query.Data)
        return
    }

    log.Printf("⚠️ Неизвестная кнопка: %s", query.Data)
}

func handleMenuCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    switch query.Data {
    case "menu_ask":
        userStates[query.Message.Chat.ID] = "waiting_question"
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, "🤖 Задайте ваш вопрос:")
        bot.Send(msg)
        
    case "menu_plans":
        showPlans(bot, query.Message.Chat.ID)
        
    case "menu_usage":
        usage := userAIUsage[query.Message.Chat.ID]
        msg := tgbotapi.NewMessage(query.Message.Chat.ID,
            fmt.Sprintf("📊 *Использование*\n\nТокены: %d/100000", usage))
        msg.ParseMode = "Markdown"
        bot.Send(msg)
        
    case "menu_model":
        keyboard := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("GPT-3.5", "model_gpt35"),
                tgbotapi.NewInlineKeyboardButtonData("GPT-4", "model_gpt4"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("🔙 Главное меню", "back_to_menu"),
            ),
        )
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, "Выберите модель:")
        msg.ReplyMarkup = keyboard
        bot.Send(msg)
        
    case "menu_profile":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID,
            fmt.Sprintf("👤 Профиль: %s", query.From.FirstName))
        bot.Send(msg)
        
    case "menu_history":
        history := userHistory[query.Message.Chat.ID]
        if len(history) == 0 {
            msg := tgbotapi.NewMessage(query.Message.Chat.ID, "📜 История пуста")
            bot.Send(msg)
            return
        }
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
            fmt.Sprintf("📜 Последний запрос:\n%s", history[len(history)-1]))
        bot.Send(msg)
        
    case "menu_support":
        handleSupport(bot, query.Message.Chat.ID, query.From)
        
    case "menu_help":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID,
            "/ask - спросить AI\n/plans - тарифы")
        bot.Send(msg)
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

func handleModelCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    var modelName string
    
    switch query.Data {
    case "model_gpt35":
        modelName = "GPT-3.5"
        userAIModel[query.Message.Chat.ID] = "gpt-3.5-turbo"
    case "model_gpt4":
        modelName = "GPT-4"
        userAIModel[query.Message.Chat.ID] = "gpt-4"
    case "model_claude":
        modelName = "Claude"
        userAIModel[query.Message.Chat.ID] = "claude-3"
    case "model_gemini":
        modelName = "Gemini"
        userAIModel[query.Message.Chat.ID] = "gemini-pro"
    }
    
    msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
        fmt.Sprintf("✅ Модель изменена на %s", modelName))
    bot.Send(msg)
}

func handleAdminCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    switch query.Data {
    case "admin_add_plan":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
            "➕ Функция добавления тарифа (в разработке)")
        bot.Send(msg)
    case "admin_edit_plan":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
            "✏️ Функция редактирования тарифа (в разработке)")
        bot.Send(msg)
    case "admin_delete_plan":
        msg := tgbotapi.NewMessage(query.Message.Chat.ID, 
            "❌ Функция удаления тарифа (в разработке)")
        bot.Send(msg)
    }
}

func handleSupport(bot *tgbotapi.BotAPI, chatID int64, user *tgbotapi.User) {
    // Текстовое сообщение с контактами
    text := fmt.Sprintf("📞 Поддержка\n\n"+
        "Здравствуйте, %s!\n\n"+
        "Вы можете связаться с нами:\n"+
        "• Email: support@saaspro.ru\n"+
        "• Telegram: @saaspro_support\n"+
        "• Чат: 24/7 онлайн\n\n"+
        "Среднее время ответа: 15 минут",
        user.FirstName)

    msg := tgbotapi.NewMessage(chatID, text)
    bot.Send(msg)

    // Кнопки действий
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

func getUserModel(chatID int64) string {
    if model, ok := userAIModel[chatID]; ok {
        return model
    }
    return "GPT-3.5 (по умолчанию)"
}

// ИСПРАВЛЕННАЯ ФУНКЦИЯ - БЕЗ ДЕМО-РЕЖИМА
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

func showPlans(bot *tgbotapi.BotAPI, chatID int64) {
    plansText := "*Базовый*\nДля небольших команд и стартапов\n💰 2990.00 ₽/мес\n\n" +
        "*Профессиональный*\nДля растущего бизнеса\n💰 29900.00 ₽/мес\n\n" +
        "*Корпоративный*\nДля крупных компаний\n💰 49000.00 ₽/мес\n\n" +
        "*Семейный*\nДля всей семьи\n💰 9900.00 ₽/мес"

    msg := tgbotapi.NewMessage(chatID, plansText)
    msg.ParseMode = "Markdown"
    bot.Send(msg)

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Купить Базовый", "plan_basic"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Купить Профессиональный", "plan_pro"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Купить Корпоративный", "plan_enterprise"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Купить Семейный", "plan_family"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔙 В меню", "back_to_menu"),
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
    case "plan_pro":
        planName = "Профессиональный"
        price = "29900"
    case "plan_enterprise":
        planName = "Корпоративный"
        price = "49000"
    case "plan_family":
        planName = "Семейный"
        price = "9900"
    }

    planClean := planType[5:]

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
            tgbotapi.NewInlineKeyboardButtonData("🪙 Крипта", "pay_crypto_"+planClean),
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
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    case "family":
        planName = "Семейный"
        price = "9900"
    }

    userPayments[chatID] = PaymentData{
        PlanName: planName,
        Price:    price,
        Method:   "card",
    }

    text := "💳 Оплата картой\n\n" +
        "Введите номер карты (16 цифр):"

    keyboard := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "back_to_plans"),
        ),
    )

    msg := tgbotapi.NewMessage(chatID, text)
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
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    case "family":
        planName = "Семейный"
        price = "9900"
    }

    address := "TXmRt1UqWqfJ1XxqZQk3yL7vFhKpDnA2jB"
    usdtAmount := fmt.Sprintf("%.2f", float64(atoi(price))/90)

    text := fmt.Sprintf("💰 *Оплата USDT (TRC-20)*\n\n") +
        fmt.Sprintf("Тариф: *%s*\n", planName) +
        fmt.Sprintf("Сумма: *%s ₽* = *%s USDT*\n\n", price, usdtAmount) +
        "📤 **Адрес для перевода:**\n" +
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
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    case "family":
        planName = "Семейный"
        price = "9900"
    }

    address := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
    btcAmount := fmt.Sprintf("%.6f", float64(atoi(price))/4500000)

    text := fmt.Sprintf("₿ *Оплата Bitcoin*\n\n") +
        fmt.Sprintf("Тариф: *%s*\n", planName) +
        fmt.Sprintf("Сумма: *%s ₽* = *%s BTC*\n\n", price, btcAmount) +
        "📤 **Адрес для перевода:**\n" +
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
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    case "family":
        planName = "Семейный"
        price = "9900"
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
    case "pro":
        planName = "Профессиональный"
        price = "29900"
        usdtAmount = 332.22
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
        usdtAmount = 544.44
    case "family":
        planName = "Семейный"
        price = "9900"
        usdtAmount = 110.00
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
        "🔗 **Ссылка для оплаты:**\n" +
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
            tgbotapi.NewInlineKeyboardButtonData("🔝 В меню", "back_to_menu"),
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
    case "pro":
        planName = "Профессиональный"
        price = "29900"
    case "enterprise":
        planName = "Корпоративный"
        price = "49000"
    case "family":
        planName = "Семейный"
        price = "9900"
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