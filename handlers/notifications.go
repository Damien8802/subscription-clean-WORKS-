package handlers

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "subscription-system/database"
    "github.com/gin-gonic/gin"
)

// Notification types
const (
    NotifLoginNewDevice   = "new_device_login"
    Notif2FAEnabled       = "2fa_enabled"
    Notif2FADisabled      = "2fa_disabled"
    NotifPasswordChanged  = "password_changed"
    NotifDeviceTrusted    = "device_trusted"
    NotifDeviceRevoked    = "device_revoked"
    NotifSuspiciousLogin  = "suspicious_login"
)

// SendTelegramNotification отправляет уведомление пользователю в Telegram
func SendTelegramNotification(userID string, message string) error {
    // Получаем Telegram ID пользователя из БД
    var telegramID int64
    err := database.Pool.QueryRow(context.Background(),
        "SELECT telegram_id FROM users WHERE id = $1", userID).Scan(&telegramID)
    if err != nil || telegramID == 0 {
        return fmt.Errorf("telegram ID not found for user %s", userID)
    }

    botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    if botToken == "" {
        return fmt.Errorf("TELEGRAM_BOT_TOKEN not set")
    }

    url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
    
    payload := map[string]interface{}{
        "chat_id":    telegramID,
        "text":       message,
        "parse_mode": "HTML",
    }
    
    jsonData, _ := json.Marshal(payload)
    
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}

// LogAndNotify логирует событие и отправляет уведомление
func LogAndNotify(c *gin.Context, userID string, notifType string, details map[string]interface{}) {
    // Логируем в БД
    _, err := database.Pool.Exec(context.Background(),
        `INSERT INTO notification_log (user_id, type, details, created_at) 
         VALUES ($1, $2, $3, $4)`,
        userID, notifType, details, time.Now())
    
    if err != nil {
        log.Printf("❌ Ошибка логирования уведомления: %v", err)
    }
    
    // Формируем текст уведомления
    message := formatNotificationMessage(notifType, details)
    
    // Отправляем в Telegram
    go SendTelegramNotification(userID, message)
}

func formatNotificationMessage(notifType string, details map[string]interface{}) string {
    switch notifType {
    case NotifLoginNewDevice:
        return fmt.Sprintf(`🔐 <b>Новый вход в аккаунт</b>
        
📍 IP: %v
🌍 Локация: %v
🖥️ Устройство: %v
⏰ Время: %v

Если это были не вы, немедленно смените пароль!`,
            details["ip"], details["location"], details["device"], details["time"])

    case Notif2FAEnabled:
        return "🔒 <b>2FA включена</b>\n\nДвухфакторная аутентификация успешно активирована для вашего аккаунта."

    case Notif2FADisabled:
        return "🔓 <b>2FA отключена</b>\n\nДвухфакторная аутентификация была отключена. Если это были не вы, срочно примите меры!"

    case NotifPasswordChanged:
        return "🔑 <b>Пароль изменён</b>\n\nПароль от вашего аккаунта был успешно изменён."

    case NotifDeviceTrusted:
        return fmt.Sprintf(`📱 <b>Новое доверенное устройство</b>
        
Устройство: %v
IP: %v
Срок действия: 30 дней`,
            details["device"], details["ip"])

    case NotifDeviceRevoked:
        return fmt.Sprintf(`🚫 <b>Доступ устройства отозван</b>
        
Устройство: %v больше не имеет доступа к аккаунту.`,
            details["device"])

    case NotifSuspiciousLogin:
        return fmt.Sprintf(`🚨 <b>Подозрительная активность</b>
        
Обнаружена подозрительная попытка входа:
📍 IP: %v
🌍 Локация: %v
🖥️ Устройство: %v

Рекомендуем сменить пароль.`,
            details["ip"], details["location"], details["device"])

    default:
        return "⚠️ Уведомление от системы безопасности"
    }
}