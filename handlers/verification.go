package handlers

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/utils"
)

// VerificationCode представляет код подтверждения
type VerificationCode struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Code      string    `json:"code"`
    Type      string    `json:"type"` // email, telegram
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
    UsedAt    *time.Time `json:"used_at,omitempty"`
}

// GenerateVerificationCode создаёт код подтверждения
func GenerateVerificationCode(userID, codeType string) (string, error) {
    // Генерируем 6-значный код
    bytes := make([]byte, 3)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    code := hex.EncodeToString(bytes)[:6]
    
    // Сохраняем в БД
    _, err := database.Pool.Exec(context.Background(),
        `INSERT INTO verification_codes (user_id, code, type, expires_at, created_at)
         VALUES ($1, $2, $3, NOW() + interval '15 minutes', NOW())`,
        userID, code, codeType)
    
    if err != nil {
        return "", err
    }
    
    return code, nil
}

// VerifyCode проверяет код подтверждения
func VerifyCode(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
        Code   string `json:"code" binding:"required"`
        Type   string `json:"type" binding:"required,oneof=email telegram"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Проверяем код в БД
    var id string
    var expiresAt time.Time
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id, expires_at FROM verification_codes 
         WHERE user_id = $1 AND code = $2 AND type = $3 AND used_at IS NULL
         ORDER BY created_at DESC LIMIT 1`,
        req.UserID, req.Code, req.Type).Scan(&id, &expiresAt)

    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
        return
    }

    if time.Now().After(expiresAt) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Verification code expired"})
        return
    }

    // Активируем пользователя
    _, err = database.Pool.Exec(c.Request.Context(),
        `UPDATE users SET email_verified = true WHERE id = $1`,
        req.UserID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user"})
        return
    }

    // Помечаем код как использованный
    _, _ = database.Pool.Exec(c.Request.Context(),
        `UPDATE verification_codes SET used_at = NOW() WHERE id = $1`,
        id)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Email successfully verified",
    })
}

// SendVerificationEmail отправляет код подтверждения на email
func SendVerificationEmail(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем email пользователя
    var email, name string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT email, name FROM users WHERE id = $1",
        req.UserID).Scan(&email, &name)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    // Генерируем код
    code, err := GenerateVerificationCode(req.UserID, "email")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate code"})
        return
    }

    // Отправляем email
    cfg := config.Load()
    emailService := utils.NewEmailService(cfg)
    err = emailService.SendVerificationEmail(email, name, code)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send email"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Verification code sent to email",
    })
}

// SendVerificationTelegram отправляет код подтверждения в Telegram
func SendVerificationTelegram(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем Telegram ID пользователя
    var telegramID int64
    var name string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT telegram_id, name FROM users WHERE id = $1",
        req.UserID).Scan(&telegramID, &name)

    if err != nil || telegramID == 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram not connected"})
        return
    }

    // Генерируем код
    code, err := GenerateVerificationCode(req.UserID, "telegram")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate code"})
        return
    }

    // Отправляем в Telegram
    message := fmt.Sprintf("🔐 Ваш код подтверждения: <b>%s</b>\n\nКод действителен 15 минут.", code)
    err = SendTelegramNotification(req.UserID, message)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send telegram"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Verification code sent to Telegram",
    })
}

// CheckVerificationStatus проверяет статус верификации пользователя
func CheckVerificationStatus(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
        return
    }

    var emailVerified bool
    var telegramID *int64
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT email_verified, telegram_id FROM users WHERE id = $1",
        userID).Scan(&emailVerified, &telegramID)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":          true,
        "email_verified":   emailVerified,
        "telegram_connected": telegramID != nil,
    })
}