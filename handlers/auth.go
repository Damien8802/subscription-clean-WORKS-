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
    
    // Генерируем 6-значный код
    code := fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
    
    // Сохраняем код в БД
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
    
    // Проверяем код
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
    
    // Находим или создаем пользователя
    var userID uuid.UUID
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT id FROM users WHERE phone = $1
    `, req.Phone).Scan(&userID)
    
    userName := req.Name
    if userName == "" {
        userName = "User_" + req.Phone[len(req.Phone)-4:]
    }
    
    if err != nil {
        // Создаем нового пользователя
        email := fmt.Sprintf("%s@phone.saaspro.ru", generateRandomStringAuth(8))
        err = database.Pool.QueryRow(c.Request.Context(), `
            INSERT INTO users (phone, name, email, role) VALUES ($1, $2, $3, 'user') RETURNING id
        `, req.Phone, userName, email).Scan(&userID)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
            return
        }
    }
    
    // Генерируем JWT токен
    token, err := GenerateJWTForUser(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
        return
    }
    
    // Удаляем использованный код
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
    var passwordHash string
    var emailVerified bool
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT id, email, password_hash, name, role, email_verified FROM users WHERE email = $1",
        req.Email).Scan(&user.ID, &user.Email, &passwordHash, &user.Name, &user.Role, &emailVerified)
    
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
        return
    }

    if !emailVerified {
        c.JSON(http.StatusUnauthorized, gin.H{
            "error":                  "Email not verified. Please check your email for verification code.",
            "requires_verification": true,
            "user_id":               user.ID,
        })
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
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

    accessToken, refreshToken, err := utils.GenerateTokensWithExpiry(user.ID, user.Role, accessExpiry, refreshExpiry)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    _, err = database.Pool.Exec(c.Request.Context(),
        `INSERT INTO user_tokens (user_id, token, expires_at, created_at) 
         VALUES ($1, $2, NOW() + $3 * interval '1 second', NOW())`,
        user.ID, refreshToken, int(refreshExpiry.Seconds()))
    if err != nil {
        log.Printf("⚠️ Failed to save refresh token: %v", err)
    }

    userID := user.ID
    
    var lastLoginIP string
    database.Pool.QueryRow(context.Background(),
        "SELECT ip_address FROM login_history WHERE user_id = $1 ORDER BY login_time DESC LIMIT 1",
        userID).Scan(&lastLoginIP)

    if lastLoginIP != "" && lastLoginIP != c.ClientIP() {
        details := map[string]interface{}{
            "ip":       c.ClientIP(),
            "location": "Неизвестно",
            "device":   c.GetHeader("User-Agent"),
            "time":     time.Now().Format("02.01.2006 15:04"),
        }
       userUUID, _ := uuid.Parse(user.ID)
      LogAndNotify(c, userUUID, NotifLoginNewDevice, details)
    }

    database.Pool.Exec(context.Background(),
        "INSERT INTO login_history (user_id, ip_address, user_agent, login_time) VALUES ($1, $2, $3, $4)",
        userID, c.ClientIP(), c.GetHeader("User-Agent"), time.Now())

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "remember":      req.Remember,
        "expires_in":    accessExpiry.Seconds(),
        "user": gin.H{
            "id":    user.ID,
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
        `INSERT INTO users (email, password_hash, name, role, email_verified) 
         VALUES ($1, $2, $3, 'user', false) 
         RETURNING id, email, name, role`,
        req.Email, string(hashedPassword), req.Name).Scan(
        &user.ID, &user.Email, &user.Name, &user.Role)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
        return
    }

    verificationCode, err := GenerateVerificationCode(user.ID, "email")
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

    accessToken, refreshToken, err := utils.GenerateTokens(user.ID, user.Role)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "access_token":  accessToken,
        "refresh_token": refreshToken,
        "user": gin.H{
            "id":             user.ID,
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