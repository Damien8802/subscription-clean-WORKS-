package handlers

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "golang.org/x/crypto/bcrypt"
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/models"
    "subscription-system/utils"
)

// InitAuthHandler инициализирует обработчики авторизации
func InitAuthHandler(cfg *config.Config) {
    log.Println("✅ Auth handler initialized")
}

// generateRandomStringAuth генерирует случайную строку
func generateRandomStringAuth(length int) string {
    bytes := make([]byte, length)
    rand.Read(bytes)
    return hex.EncodeToString(bytes)[:length]
}

// SendPhoneCode отправляет код на телефон
func SendPhoneCode(c *gin.Context) {
    var req struct {
        Phone string `json:"phone" binding:"required"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
    expiresAt := time.Now().Add(5 * time.Minute)

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO phone_auth_codes (phone, code, expires_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (phone) DO UPDATE SET code = $2, expires_at = $3
    `, req.Phone, code, expiresAt)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save code"})
        return
    }

    log.Printf("📱 Код для %s: %s", req.Phone, code)

    c.JSON(http.StatusOK, gin.H{
        "message":    "Код отправлен",
        "expires_in": 300,
    })
}

// VerifyPhoneCode проверяет код с телефона
func VerifyPhoneCode(c *gin.Context) {
    var req struct {
        Phone string `json:"phone" binding:"required"`
        Code  string `json:"code" binding:"required"`
        Name  string `json:"name"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var storedCode string
    var expiresAt time.Time
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT code, expires_at FROM phone_auth_codes
        WHERE phone = $1 AND expires_at > NOW()
    `, req.Phone).Scan(&storedCode, &expiresAt)

    if err != nil || storedCode != req.Code {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired code"})
        return
    }

    var userID uuid.UUID
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT id FROM users WHERE phone = $1
    `, req.Phone).Scan(&userID)

    userName := req.Name
    if userName == "" {
        userName = "User_" + req.Phone[len(req.Phone)-4:]
    }

    if err != nil {
        email := fmt.Sprintf("%s@phone.saaspro.ru", generateRandomStringAuth(8))
        err = database.Pool.QueryRow(c.Request.Context(), `
            INSERT INTO users (phone, name, email, role, tenant_id, password_changed_at, email_verified) 
            VALUES ($1, $2, $3, 'user', '11111111-1111-1111-1111-111111111111', NOW(), true) 
            RETURNING id
        `, req.Phone, userName, email).Scan(&userID)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
            return
        }
    }

    token, err := GenerateJWTForUser(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }

    database.Pool.Exec(c.Request.Context(), "DELETE FROM phone_auth_codes WHERE phone = $1", req.Phone)

    c.JSON(http.StatusOK, gin.H{
        "token": token,
        "user": gin.H{
            "id":   userID,
            "name": userName,
        },
    })
}

// LoginHandler обрабатывает вход пользователя
func LoginHandler(c *gin.Context) {
    var req struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
        Remember bool   `json:"remember"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var user models.User
    var tenantID string
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id, email, password_hash, name, role, COALESCE(tenant_id, '11111111-1111-1111-1111-111111111111') as tenant_id 
         FROM users WHERE email = $1`,
        req.Email).Scan(
        &user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &tenantID)

    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
        return
    }
    user.TenantID = tenantID

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
        return
    }

    var accessExpiry, refreshExpiry time.Duration
    if req.Remember {
        accessExpiry = 30 * 24 * time.Hour
        refreshExpiry = 90 * 24 * time.Hour
    } else {
        accessExpiry = 15 * time.Minute
        refreshExpiry = 24 * time.Hour
    }

    accessToken, refreshToken, err := utils.GenerateTokensWithExpiry(user.ID.String(), user.Email, user.Role, accessExpiry, refreshExpiry)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    // Сохраняем refresh token
    _, err = database.Pool.Exec(c.Request.Context(),
        `INSERT INTO user_tokens (user_id, token, expires_at, created_at, tenant_id) 
         VALUES ($1, $2, NOW() + $3 * interval '1 second', NOW(), $4)`,
        user.ID.String(), refreshToken, int(refreshExpiry.Seconds()), user.TenantID)
    if err != nil {
        log.Printf("⚠️ Failed to save refresh token: %v", err)
    }

    // Записываем историю входа
    database.Pool.Exec(context.Background(),
        `INSERT INTO login_history (user_id, ip_address, user_agent, login_time, tenant_id) 
         VALUES ($1, $2, $3, NOW(), $4)`,
        user.ID.String(), c.ClientIP(), c.GetHeader("User-Agent"), user.TenantID)

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "remember":      req.Remember,
        "expires_in":    accessExpiry.Seconds(),
        "user": gin.H{
            "id":    user.ID.String(),
            "email": user.Email,
            "name":  user.Name,
            "role":  user.Role,
        },
    })
}

// LogoutHandler обрабатывает выход пользователя
func LogoutHandler(c *gin.Context) {
    var req struct {
        RefreshToken string `json:"refresh_token" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(),
        "DELETE FROM user_tokens WHERE token = $1", req.RefreshToken)
    if err != nil {
        log.Printf("⚠️ Failed to delete refresh token: %v", err)
    }

    c.SetCookie("access_token", "", -1, "/", "", false, true)
    c.SetCookie("refresh_token", "", -1, "/", "", false, true)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Successfully logged out",
    })
}

// RegisterHandler обрабатывает регистрацию пользователя
func RegisterHandler(c *gin.Context) {
    var req struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required,min=6"`
        Name     string `json:"name" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var exists bool
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    if exists {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Email already registered"})
        return
    }

    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }

    var user models.User
    err = database.Pool.QueryRow(c.Request.Context(),
        `INSERT INTO users (email, password_hash, name, role, email_verified, tenant_id, password_changed_at)
         VALUES ($1, $2, $3, 'user', false, '11111111-1111-1111-1111-111111111111', NOW())
         RETURNING id, email, name, role`,
        req.Email, string(hashedPassword), req.Name).Scan(
        &user.ID, &user.Email, &user.Name, &user.Role)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }
    user.TenantID = "11111111-1111-1111-1111-111111111111"

    // Генерируем и отправляем код подтверждения email
    verificationCode, err := GenerateVerificationCode(user.ID.String(), "email")
    if err != nil {
        log.Printf("❌ Failed to generate verification code: %v", err)
    } else {
        go func() {
            emailService := utils.NewEmailService(config.Load())
            err := emailService.SendVerificationEmail(user.Email, user.Name, verificationCode)
            if err != nil {
                log.Printf("❌ Failed to send verification email: %v", err)
            } else {
                log.Printf("✅ Verification email sent to %s", user.Email)
            }
        }()
    }

    accessToken, refreshToken, err := utils.GenerateTokens(user.ID.String(), user.Role)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    // Сохраняем refresh token
    _, err = database.Pool.Exec(c.Request.Context(),
        `INSERT INTO user_tokens (user_id, token, expires_at, created_at, tenant_id) 
         VALUES ($1, $2, NOW() + INTERVAL '24 hours', NOW(), $3)`,
        user.ID.String(), refreshToken, user.TenantID)
    if err != nil {
        log.Printf("⚠️ Failed to save refresh token: %v", err)
    }

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "user": gin.H{
            "id":             user.ID.String(),
            "email":          user.Email,
            "name":           user.Name,
            "role":           user.Role,
            "email_verified": false,
        },
        "message": "Registration successful! Please check your email for verification code.",
    })
}

// RefreshHandler обновляет access token
func RefreshHandler(c *gin.Context) {
    var req struct {
        RefreshToken string `json:"refresh_token" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var exists bool
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT EXISTS(SELECT 1 FROM user_tokens WHERE token = $1 AND expires_at > NOW())",
        req.RefreshToken).Scan(&exists)
    if err != nil || !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
        return
    }

    newAccessToken, err := utils.RefreshToken(req.RefreshToken)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "access_token": newAccessToken,
    })
}

// VerifyEmailHandler подтверждает email пользователя
func VerifyEmailHandler(c *gin.Context) {
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

// ResendVerificationHandler отправляет код подтверждения повторно
func ResendVerificationHandler(c *gin.Context) {
    var req struct {
        Email string `json:"email" binding:"required,email"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var user models.User
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id, name, email_verified FROM users WHERE email = $1`,
        req.Email).Scan(&user.ID, &user.Name, &user.EmailVerified)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    if user.EmailVerified {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
        return
    }

    verificationCode, err := GenerateVerificationCode(user.ID.String(), "email")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate code"})
        return
    }

    go func() {
        emailService := utils.NewEmailService(config.Load())
        err := emailService.SendVerificationEmail(req.Email, user.Name, verificationCode)
        if err != nil {
            log.Printf("❌ Failed to send verification email: %v", err)
        } else {
            log.Printf("✅ Verification email resent to %s", req.Email)
        }
    }()

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Verification code sent",
    })
}
