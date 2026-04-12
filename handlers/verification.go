package handlers

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/utils"
)

// GenerateVerificationCode создаёт код подтверждения для пользователя
func GenerateVerificationCode(userID, codeType string) (string, error) {
    // Генерируем случайный 6-значный код
    code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
    
    expiresAt := time.Now().Add(15 * time.Minute)
    
    _, err := database.Pool.Exec(context.Background(),
        `INSERT INTO verification_codes (user_id, code, type, expires_at, created_at)
         VALUES ($1, $2, $3, $4, NOW())`,
        userID, code, codeType, expiresAt)
    
    if err != nil {
        return "", err
    }
    
    return code, nil
}

// SendVerificationEmail отправляет код подтверждения на email
func SendVerificationEmail(c *gin.Context) {
    var req struct {
        Email string `json:"email" binding:"required,email"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var userID, userName string
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id, name FROM users WHERE email = $1`, req.Email).Scan(&userID, &userName)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    verificationCode, err := GenerateVerificationCode(userID, "email")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate code"})
        return
    }

    go func() {
        emailService := utils.NewEmailService(config.Load())
        err := emailService.SendVerificationEmail(req.Email, userName, verificationCode)
        if err != nil {
            log.Printf("❌ Failed to send verification email: %v", err)
        }
    }()

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Verification code sent",
    })
}

// SendVerificationTelegram отправляет код подтверждения в Telegram
func SendVerificationTelegram(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Telegram verification - coming soon",
    })
}

// VerifyCode проверяет код подтверждения
func VerifyCode(c *gin.Context) {
    var req struct {
        Email string `json:"email" binding:"required,email"`
        Code  string `json:"code" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var userID string
    var expiresAt time.Time
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT user_id, expires_at FROM verification_codes 
         WHERE code = $1 AND type = 'email' AND used_at IS NULL`,
        req.Code).Scan(&userID, &expiresAt)

    if err != nil || time.Now().After(expiresAt) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification code"})
        return
    }

    // Обновляем статус верификации email
    _, err = database.Pool.Exec(c.Request.Context(),
        `UPDATE users SET email_verified = true, updated_at = NOW() WHERE id = $1 AND email = $2`,
        userID, req.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
        return
    }

    // Отмечаем код как использованный
    _, err = database.Pool.Exec(c.Request.Context(),
        `UPDATE verification_codes SET used_at = NOW() WHERE code = $1`,
        req.Code)
    if err != nil {
        log.Printf("⚠️ Failed to mark code as used: %v", err)
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Email verified successfully",
    })
}

// CheckVerificationStatus проверяет статус верификации
func CheckVerificationStatus(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
        return
    }

    var emailVerified bool
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT email_verified FROM users WHERE id = $1`, userID).Scan(&emailVerified)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":        true,
        "email_verified": emailVerified,
    })
}