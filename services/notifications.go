package services

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "net/smtp"
    "subscription-system/config"
)

type NotificationService struct {
    cfg *config.Config
}

func NewNotificationService(cfg *config.Config) *NotificationService {
    return &NotificationService{cfg: cfg}
}

// SendTelegram отправляет сообщение в Telegram через бота
func (ns *NotificationService) SendTelegram(message string) error {
    if ns.cfg.TelegramBotToken == "" || ns.cfg.TelegramChatID == "" {
        log.Println("Telegram не настроен, пропускаем уведомление")
        return nil
    }

    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ns.cfg.TelegramBotToken)
    payload := map[string]interface{}{
        "chat_id":    ns.cfg.TelegramChatID,
        "text":       message,
        "parse_mode": "HTML",
    }
    jsonData, _ := json.Marshal(payload)

    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("ошибка отправки в Telegram: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("Telegram вернул статус %s", resp.Status)
    }
    return nil
}

// SendEmail отправляет email через SMTP
func (ns *NotificationService) SendEmail(to, subject, body string) error {
    if ns.cfg.SMTPHost == "" || ns.cfg.SMTPUser == "" || ns.cfg.EmailFrom == "" {
        log.Println("SMTP не настроен, пропускаем email")
        return nil
    }

    auth := smtp.PlainAuth("", ns.cfg.SMTPUser, ns.cfg.SMTPPassword, ns.cfg.SMTPHost)
    addr := fmt.Sprintf("%s:%d", ns.cfg.SMTPHost, ns.cfg.SMTPPort)

    msg := []byte("To: " + to + "\r\n" +
        "From: " + ns.cfg.EmailFrom + "\r\n" +
        "Subject: " + subject + "\r\n" +
        "Content-Type: text/html; charset=UTF-8\r\n" +
        "\r\n" +
        body + "\r\n")

    err := smtp.SendMail(addr, auth, ns.cfg.EmailFrom, []string{to}, msg)
    if err != nil {
        return fmt.Errorf("ошибка отправки email: %w", err)
    }
    return nil
}

// NotifyCustomerCreated уведомление о создании клиента
func (ns *NotificationService) NotifyCustomerCreated(name, email, phone, company, responsible string) {
    msg := fmt.Sprintf("🆕 Новый клиент создан:\n<b>Имя:</b> %s\n<b>Email:</b> %s\n<b>Телефон:</b> %s\n<b>Компания:</b> %s\n<b>Ответственный:</b> %s",
        name, email, phone, company, responsible)
    ns.SendTelegram(msg)

    // Можно также отправить email, если нужен
    // ns.SendEmail("manager@example.com", "Новый клиент в CRM", msg)
}

// NotifyCustomerUpdated уведомление об изменении клиента
func (ns *NotificationService) NotifyCustomerUpdated(id, name, email, phone string) {
    msg := fmt.Sprintf("✏️ Клиент обновлён:\n<b>ID:</b> %s\n<b>Имя:</b> %s\n<b>Email:</b> %s\n<b>Телефон:</b> %s",
        id, name, email, phone)
    ns.SendTelegram(msg)
}

// NotifyDealCreated уведомление о создании сделки
func (ns *NotificationService) NotifyDealCreated(title string, value float64, stage, responsible, customerID string) {
    msg := fmt.Sprintf("💰 Новая сделка:\n<b>Название:</b> %s\n<b>Сумма:</b> %.2f\n<b>Стадия:</b> %s\n<b>Ответственный:</b> %s\n<b>Клиент ID:</b> %s",
        title, value, stage, responsible, customerID)
    ns.SendTelegram(msg)
}

// NotifyDealUpdated уведомление об изменении сделки
func (ns *NotificationService) NotifyDealUpdated(id, title string, value float64, stage string) {
    msg := fmt.Sprintf("🔄 Сделка обновлена:\n<b>ID:</b> %s\n<b>Название:</b> %s\n<b>Сумма:</b> %.2f\n<b>Стадия:</b> %s",
        id, title, value, stage)
    ns.SendTelegram(msg)
}

