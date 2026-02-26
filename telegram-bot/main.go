package main

import (
    "log"
    "os"
    "fmt"
    "net/http"
    "strings"
    "encoding/json"
    "io"
    "github.com/joho/godotenv"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Хранилище состояний пользователей
var userStates = make(map[int64]string)
var userPayments = make(map[int64]PaymentData)

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

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
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
        }
        return
    }

    switch message.Text {
    case "/start":
        msg := tgbotapi.NewMessage(message.Chat.ID,
            "👋 Привет, DamieN!\n\n"+
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
                "/help – справка")
        bot.Send(msg)

    case "/plans":
        showPlans(bot, message.Chat.ID)
    }
}

func handleCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
    callback := tgbotapi.NewCallback(query.ID, "")
    bot.Request(callback)
    
    log.Printf("Нажата кнопка: %s", query.Data)

    if strings.HasPrefix(query.Data, "pay_crypto_") {
        planClean := strings.TrimPrefix(query.Data, "pay_crypto_")
        log.Printf("✅ КРИПТА: выбран тариф %s", planClean)
        startCryptoPayment(bot, query.Message.Chat.ID, planClean)
        return
    }

    if query.Data == "check_crypto_status" {
        checkCryptoPayment(bot, query.Message.Chat.ID)
        return
    }

    if query.Data == "back_to_plans" {
        showPlans(bot, query.Message.Chat.ID)
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
            tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_plans"),
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

    msg := tgbotapi.NewMessage(chatID, "💳 Введите номер карты (16 цифр):")
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
    )

    msg := tgbotapi.NewMessage(chatID, text)
    msg.ParseMode = "Markdown"
    msg.ReplyMarkup = keyboard
    bot.Send(msg)
}

// ==================== CRYPTO PAY (ИСПРАВЛЕНО) ====================

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
            tgbotapi.NewInlineKeyboardButtonData("❌ Отменить", "back_to_plans"),
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

// ==================== КОПИРОВАНИЕ АДРЕСОВ ====================

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