package utils

import (
    "fmt"
    "net/smtp"
    "time"
    "subscription-system/config"
)

type EmailService struct {
    config *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
    return &EmailService{config: cfg}
}

// SendEmail отправляет email через SMTP
func (s *EmailService) SendEmail(to, subject, body string) error {
    if s.config.SMTPHost == "" || s.config.SMTPUser == "" {
        return fmt.Errorf("SMTP not configured")
    }

    auth := smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPassword, s.config.SMTPHost)
    
    msg := []byte(fmt.Sprintf("To: %s\r\n"+
        "Subject: %s\r\n"+
        "Content-Type: text/html; charset=utf-8\r\n"+
        "\r\n"+
        "%s\r\n", to, subject, body))

    addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
    return smtp.SendMail(addr, auth, s.config.EmailFrom, []string{to}, msg)
}

// SendSecurityAlert отправляет уведомление о безопасности
func (s *EmailService) SendSecurityAlert(to, username, alertType string, details map[string]string) error {
    subject := fmt.Sprintf("🔐 Уведомление безопасности - SaaSPro")
    
    body := fmt.Sprintf(`
        <h2>Уведомление безопасности</h2>
        <p>Здравствуйте, <strong>%s</strong>!</p>
        <p>Тип события: <strong>%s</strong></p>
        <table border="1" cellpadding="5" style="border-collapse: collapse;">
    `, username, alertType)
    
    for key, value := range details {
        body += fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>", key, value)
    }
    
    body += `
        </table>
        <p>Если это были не вы, немедленно смените пароль.</p>
        <p>С уважением,<br>Команда SaaSPro</p>
    `
    
    return s.SendEmail(to, subject, body)
}

// SendLoginNotification уведомление о входе
func (s *EmailService) SendLoginNotification(to, username, ip, location, device string) error {
    details := map[string]string{
        "IP адрес":        ip,
        "Местоположение": location,
        "Устройство":     device,
        "Время":          time.Now().Format("02.01.2006 15:04:05"),
    }
    return s.SendSecurityAlert(to, username, "Новый вход в аккаунт", details)
}

// Send2FANotification уведомление о 2FA
func (s *EmailService) Send2FANotification(to, username, action string) error {
    details := map[string]string{
        "Действие": action,
        "Время":    time.Now().Format("02.01.2006 15:04:05"),
    }
    return s.SendSecurityAlert(to, username, "Изменение 2FA", details)
}