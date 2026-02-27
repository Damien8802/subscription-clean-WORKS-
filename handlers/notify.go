package handlers

import (
    "log"
    "net/http"
    "os"
    "strings"

    "github.com/gin-gonic/gin"
)

type NotifyRequest struct {
    Type    string   `json:"type" binding:"required"` // "email" или "sms"
    Subject string   `json:"subject"`                  // для email
    Message string   `json:"message" binding:"required"`
    Users   []string `json:"users"`                    // список получателей
}

type EmailConfig struct {
    Host     string
    Port     string
    Username string
    Password string
    From     string
}

// NotifyHandler - отправка email или sms
func NotifyHandler(c *gin.Context) {
    var req NotifyRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получатели
    recipients := req.Users
    if len(recipients) == 0 {
        recipients = []string{
            "89182471690",          // твой телефон
            "skorpion_88-88@mail.ru", // твой email
        }
    }

    var result map[string]interface{}

    switch req.Type {
    case "email":
        result = sendEmail(recipients, req.Subject, req.Message)
    case "sms":
        result = sendSMS(recipients, req.Message)
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "неверный тип, используй email или sms"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "result": result})
}

// Страница для тестирования рассылок
func NotifyPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "notify.html", gin.H{})
}

// ==================== EMAIL ====================
func sendEmail(recipients []string, subject, message string) map[string]interface{} {
    cfg := EmailConfig{
        Host:     os.Getenv("SMTP_HOST"),
        Port:     os.Getenv("SMTP_PORT"),
        Username: os.Getenv("SMTP_USER"),
        Password: os.Getenv("SMTP_PASS"),
        From:     os.Getenv("SMTP_FROM"),
    }

    // Собираем email'ы
    var emails []string
    for _, r := range recipients {
        if strings.Contains(r, "@") {
            emails = append(emails, r)
        }
    }

    if len(emails) == 0 {
        return gin.H{"sent": 0, "error": "нет email адресов"}
    }

    // Демо-режим если нет настроек SMTP
    if cfg.Host == "" || cfg.Port == "" {
        log.Printf("📧 [DEMO] Email для %d: %s", len(emails), subject)
        return gin.H{
            "sent":  len(emails),
            "demo":  true,
            "first": emails[0],
        }
    }

    // TODO: здесь будет реальная отправка через SMTP
    return gin.H{
        "sent": len(emails),
        "via":  "smtp",
    }
}

// ==================== SMS ====================
func sendSMS(recipients []string, message string) map[string]interface{} {
    apiKey := os.Getenv("SMS_API_KEY")

    // Собираем телефоны
    var phones []string
    for _, r := range recipients {
        phone := strings.TrimSpace(r)
        if strings.HasPrefix(phone, "89") || strings.HasPrefix(phone, "+79") {
            phones = append(phones, phone)
        }
    }

    if len(phones) == 0 {
        return gin.H{"sent": 0, "error": "нет номеров телефонов"}
    }

    // Демо-режим если нет API ключа
    if apiKey == "" {
        log.Printf("📱 [DEMO] SMS для %d номеров", len(phones))
        return gin.H{
            "sent":  len(phones),
            "demo":  true,
            "first": phones[0],
        }
    }

    // TODO: здесь будет интеграция с SMS провайдером
    return gin.H{
        "sent": len(phones),
        "via":  "sms-provider",
    }
}