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
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/utils"
)
var (
    cfg          = config.Load()
    emailService = utils.NewEmailService(cfg)
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

// ========== НОВАЯ ФУНКЦИЯ ==========
// GetLocationByIP определяет местоположение по IP
func GetLocationByIP(ip string) string {
    // Если IP локальный, не проверяем
    if ip == "::1" || ip == "127.0.0.1" {
        return "Локальный доступ"
    }

    // Используем бесплатный API ip-api.com
    client := &http.Client{Timeout: 3 * time.Second}
    resp, err := client.Get("http://ip-api.com/json/" + ip + "?lang=ru&fields=status,country,city,isp,query")
    if err != nil {
        log.Printf("⚠️ Ошибка определения местоположения для IP %s: %v", ip, err)
        return "Неизвестно"
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    var result struct {
        Status  string `json:"status"`
        Country string `json:"country"`
        City    string `json:"city"`
        ISP     string `json:"isp"`
    }
    
    if err := json.Unmarshal(body, &result); err != nil || result.Status != "success" {
        return "Неизвестно"
    }
    
    if result.City != "" && result.Country != "" {
        return fmt.Sprintf("%s, %s (%s)", result.City, result.Country, result.ISP)
    }
    if result.Country != "" {
        return result.Country
    }
    return "Неизвестно"
}

// SendTelegramNotification отправляет уведомление пользователю в Telegram
func SendTelegramNotification(userID uuid.UUID, message string) error {
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

// SendEmailNotification отправляет уведомление по email
func SendEmailNotification(userID uuid.UUID, notifType string, details map[string]interface{}) error {
    // Получаем email пользователя
    var email, name string
    err := database.Pool.QueryRow(context.Background(),
        "SELECT email, name FROM users WHERE id = $1", userID).Scan(&email, &name)
    if err != nil {
        return err
    }

    switch notifType {
    case NotifLoginNewDevice:
        return emailService.SendLoginNotification(email, name,
            details["ip"].(string),
            details["location"].(string),
            details["device"].(string))
    case Notif2FAEnabled:
        return emailService.Send2FANotification(email, name, "Включена")
    case Notif2FADisabled:
        return emailService.Send2FANotification(email, name, "Отключена")
    default:
        return emailService.SendSecurityAlert(email, name, notifType, convertDetails(details))
    }
}

func convertDetails(details map[string]interface{}) map[string]string {
    result := make(map[string]string)
    for k, v := range details {
        result[k] = fmt.Sprintf("%v", v)
    }
    return result
}

// LogAndNotify логирует событие и отправляет уведомления
func LogAndNotify(c *gin.Context, userID uuid.UUID, notifType string, details map[string]interface{}) {
    // Определяем местоположение для IP (только для логина)
    if notifType == NotifLoginNewDevice || notifType == NotifSuspiciousLogin {
        if ip, ok := details["ip"].(string); ok && ip != "" {
            location := GetLocationByIP(ip)
            details["location"] = location
        }
    }

    // Логируем в БД
    _, err := database.Pool.Exec(context.Background(),
        `INSERT INTO notification_log (user_id, type, details, created_at) 
         VALUES ($1, $2, $3, $4)`,
        userID, notifType, details, time.Now())
    
    if err != nil {
        log.Printf("❌ Ошибка логирования уведомления: %v", err)
    }
    
    // Формируем текст для Telegram
    message := formatNotificationMessage(notifType, details)
    
    // Отправляем в Telegram
    go SendTelegramNotification(userID, message)
    
    // Отправляем на email
    go SendEmailNotification(userID, notifType, details)
}

func formatNotificationMessage(notifType string, details map[string]interface{}) string {
    switch notifType {
    case NotifLoginNewDevice:
        return fmt.Sprintf(`🚨 <b>НОВЫЙ ВХОД В АККАУНТ</b>

📍 <b>IP:</b> <code>%v</code>
🌍 <b>Местоположение:</b> %v
💻 <b>Устройство:</b> %v
⏰ <b>Время:</b> %v

⚠️ <b>Если это были не вы:</b>
1️⃣ Немедленно смените пароль
2️⃣ Проверьте доверенные устройства
3️⃣ Включите 2FA

✅ <b>Если это вы</b> — можете добавить устройство в доверенные в настройках безопасности.`,
            details["ip"], details["location"], details["device"], details["time"])

    case Notif2FAEnabled:
        return "🔒 <b>✅ 2FA ВКЛЮЧЕНА</b>\n\nДвухфакторная аутентификация успешно активирована для вашего аккаунта. Ваш аккаунт теперь под дополнительной защитой!"

    case Notif2FADisabled:
        return "🔓 <b>⚠️ 2FA ОТКЛЮЧЕНА</b>\n\nДвухфакторная аутентификация была отключена. Если это были не вы, срочно примите меры!"

    case NotifPasswordChanged:
        return "🔑 <b>✅ ПАРОЛЬ ИЗМЕНЁН</b>\n\nПароль от вашего аккаунта был успешно изменён."

    case NotifDeviceTrusted:
        return fmt.Sprintf(`📱 <b>✅ НОВОЕ ДОВЕРЕННОЕ УСТРОЙСТВО</b>
        
<b>Устройство:</b> %v
<b>IP:</b> <code>%v</code>
<b>Срок действия:</b> 30 дней

Теперь вход с этого устройства не требует подтверждения.`,
            details["device"], details["ip"])

    case NotifDeviceRevoked:
        return fmt.Sprintf(`🚫 <b>🔐 ДОСТУП УСТРОЙСТВА ОТОЗВАН</b>
        
<b>Устройство:</b> %v больше не имеет доступа к вашему аккаунту.`,
            details["device"])

    case NotifSuspiciousLogin:
        return fmt.Sprintf(`🚨 <b>⚠️ ПОДОЗРИТЕЛЬНАЯ АКТИВНОСТЬ</b>
        
Обнаружена подозрительная попытка входа:
📍 <b>IP:</b> <code>%v</code>
🌍 <b>Местоположение:</b> %v
💻 <b>Устройство:</b> %v

<b>Рекомендуем:</b>
• Немедленно сменить пароль
• Проверить список доверенных устройств
• Включить 2FA, если ещё не сделано`,
            details["ip"], details["location"], details["device"])

    default:
        return "⚠️ Уведомление от системы безопасности"
    }
}

// GetNotifications возвращает список уведомлений пользователя
func GetNotifications(c *gin.Context) {
    // // tenantID := middleware.GetTenantIDFromContext(c)
    userID, exists := c.Get("user_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    
    userUUID, ok := userID.(uuid.UUID)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, type, title, message, link, is_read, created_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 50
    `, userUUID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var notifications []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var notifType, title, message, link string
        var isRead bool
        var createdAt time.Time
        
        rows.Scan(&id, &notifType, &title, &message, &link, &isRead, &createdAt)
        
        notifications = append(notifications, map[string]interface{}{
            "id":         id,
            "type":       notifType,
            "title":      title,
            "message":    message,
            "link":       link,
            "is_read":    isRead,
            "created_at": createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

// MarkNotificationRead отмечает уведомление как прочитанное
func MarkNotificationRead(c *gin.Context) {
    // // tenantID := middleware.GetTenantIDFromContext(c)
    userID, exists := c.Get("user_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    
    userUUID, ok := userID.(uuid.UUID)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }
    
    notificationID := c.Param("id")
    if notificationID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "notification id required"})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE notifications SET is_read = true
        WHERE id = $1 AND user_id = $2
    `, notificationID, userUUID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetUnreadCount возвращает количество непрочитанных уведомлений
func GetUnreadCount(c *gin.Context) {
    // // tenantID := middleware.GetTenantIDFromContext(c)
    userID, exists := c.Get("user_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    
    userUUID, ok := userID.(uuid.UUID)
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
        return
    }
    
    var count int
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM notifications
        WHERE user_id = $1 AND is_read = false
    `, userUUID).Scan(&count)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"unread_count": count})
}



