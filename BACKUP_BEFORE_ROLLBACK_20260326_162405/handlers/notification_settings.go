package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/jackc/pgx/v5"
    "subscription-system/database"
    "subscription-system/models"
)

// GetNotificationSettings возвращает настройки уведомлений текущего пользователя
func GetNotificationSettings(c *gin.Context) {
    userID := getUserIDFromContext(c)
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    var settings models.NotificationSettings
    var events []string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT user_id, telegram_enabled, email_enabled, events, created_at, updated_at
        FROM user_notification_settings
        WHERE user_id = $1
    `, userID).Scan(
        &settings.UserID, &settings.TelegramEnabled, &settings.EmailEnabled,
        &events, &settings.CreatedAt, &settings.UpdatedAt,
    )
    if err != nil {
        if err == pgx.ErrNoRows {
            // Настроек нет – возвращаем значения по умолчанию
            settings = models.NotificationSettings{
                UserID:          userID,
                TelegramEnabled: false,
                EmailEnabled:    true,
                Events:          []string{},
            }
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
            return
        }
    } else {
        settings.Events = events
    }

    c.JSON(http.StatusOK, settings)
}

// UpdateNotificationSettings обновляет настройки уведомлений
func UpdateNotificationSettings(c *gin.Context) {
    userID := getUserIDFromContext(c)
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    var req models.NotificationSettings
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Вставляем или обновляем запись
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO user_notification_settings (user_id, telegram_enabled, email_enabled, events, updated_at)
        VALUES ($1, $2, $3, $4, NOW())
        ON CONFLICT (user_id) DO UPDATE
        SET telegram_enabled = EXCLUDED.telegram_enabled,
            email_enabled = EXCLUDED.email_enabled,
            events = EXCLUDED.events,
            updated_at = NOW()
    `, userID, req.TelegramEnabled, req.EmailEnabled, req.Events)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}