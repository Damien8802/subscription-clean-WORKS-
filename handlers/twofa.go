package handlers

import (
    "encoding/base64"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/pquerna/otp/totp"
    "github.com/skip2/go-qrcode"

    "subscription-system/database"
)

// GenerateTwoFASecret создаёт новый секрет для 2FA
func GenerateTwoFASecret(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        userID = c.Query("user_id")
        if userID == "" {
            userID = "test-user-123"
        }
    }

    // Получаем email пользователя
    var email string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT email FROM users WHERE id = $1", userID).Scan(&email)
    if err != nil {
        email = "user@example.com"
    }

    // Генерируем секрет
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "SaaSPro",
        AccountName: email,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate secret"})
        return
    }

    // Сохраняем секрет в БД
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO twofa (user_id, secret, enabled) 
        VALUES ($1, $2, false)
        ON CONFLICT (user_id) 
        DO UPDATE SET secret = $2, enabled = false, updated_at = NOW()
    `, userID, key.Secret())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save secret"})
        return
    }

    // Генерируем QR-код
    var png []byte
    png, err = qrcode.Encode(key.URL(), qrcode.Medium, 256)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "secret":  key.Secret(),
        "qr":      base64.StdEncoding.EncodeToString(png),
        "url":     key.URL(),
    })
}

// VerifyTwoFACode проверяет код из Google Authenticator
func VerifyTwoFACode(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
        Code   string `json:"code" binding:"required,len=6"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем секрет из БД
    var secret string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT secret FROM twofa WHERE user_id = $1", req.UserID).Scan(&secret)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "2FA not set up"})
        return
    }

    // Проверяем код
    valid := totp.Validate(req.Code, secret)
    if !valid {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid code"})
        return
    }

    // Активируем 2FA
    _, err = database.Pool.Exec(c.Request.Context(),
        "UPDATE twofa SET enabled = true, updated_at = NOW() WHERE user_id = $1", req.UserID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2FA"})
        return
    }

    // ОТПРАВЛЯЕМ УВЕДОМЛЕНИЕ
    go LogAndNotify(c, req.UserID, Notif2FAEnabled, map[string]interface{}{
        "time": time.Now().Format("02.01.2006 15:04"),
    })

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "2FA enabled successfully",
    })
}

// DisableTwoFA отключает 2FA
func DisableTwoFA(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
        Code   string `json:"code" binding:"required,len=6"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем секрет из БД
    var secret string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT secret FROM twofa WHERE user_id = $1", req.UserID).Scan(&secret)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "2FA not set up"})
        return
    }

    // Проверяем код перед отключением
    valid := totp.Validate(req.Code, secret)
    if !valid {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid code"})
        return
    }

    // Отключаем 2FA
    _, err = database.Pool.Exec(c.Request.Context(),
        "UPDATE twofa SET enabled = false, updated_at = NOW() WHERE user_id = $1", req.UserID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable 2FA"})
        return
    }

    // ОТПРАВЛЯЕМ УВЕДОМЛЕНИЕ
    go LogAndNotify(c, req.UserID, Notif2FADisabled, map[string]interface{}{
        "time": time.Now().Format("02.01.2006 15:04"),
    })

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "2FA disabled successfully",
    })
}

// GetTwoFAStatus возвращает статус 2FA
func GetTwoFAStatus(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    var enabled bool
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT enabled FROM twofa WHERE user_id = $1", userID).Scan(&enabled)

    if err != nil {
        // Нет записи — 2FA не настроена
        c.JSON(http.StatusOK, gin.H{
            "enabled": false,
            "exists":  false,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "enabled": enabled,
        "exists":  true,
    })
}