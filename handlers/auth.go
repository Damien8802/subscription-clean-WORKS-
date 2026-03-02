package handlers

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "golang.org/x/crypto/bcrypt"
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/models"
    "subscription-system/utils"
)

// InitAuthHandler инициализирует обработчики авторизации
func InitAuthHandler(cfg *config.Config) {
    // Здесь можно добавить инициализацию, например:
    // - Проверку подключения к БД
    // - Загрузку ключей
    // - Настройку параметров
    log.Println("✅ Auth handler initialized")
}

// LoginHandler обрабатывает вход пользователя с поддержкой "Запомнить меня"
func LoginHandler(c *gin.Context) {
    var req struct {
        Email     string `json:"email" binding:"required,email"`
        Password  string `json:"password" binding:"required"`
        Remember  bool   `json:"remember"` // флаг "Запомнить меня"
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем пользователя из БД
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

    // Проверяем, подтверждён ли email
    if !emailVerified {
        c.JSON(http.StatusUnauthorized, gin.H{
            "error": "Email not verified. Please check your email for verification code.",
            "requires_verification": true,
            "user_id": user.ID,
        })
        return
    }

    // Проверяем пароль
    if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
        return
    }

    // Определяем срок действия токена в зависимости от флага Remember
    var accessExpiry, refreshExpiry time.Duration
    if req.Remember {
        // Если "Запомнить меня" - токен на 30 дней
        accessExpiry = 30 * 24 * time.Hour
        refreshExpiry = 90 * 24 * time.Hour
    } else {
        // Обычный вход - токен на 15 минут
        accessExpiry = 15 * time.Minute
        refreshExpiry = 24 * time.Hour
    }

    // Генерируем JWT токены с учётом срока
    accessToken, refreshToken, err := utils.GenerateTokensWithExpiry(user.ID, user.Role, accessExpiry, refreshExpiry)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
    }

    // Сохраняем refresh token в БД для возможности инвалидации
    _, err = database.Pool.Exec(c.Request.Context(),
        `INSERT INTO user_tokens (user_id, token, expires_at, created_at) 
         VALUES ($1, $2, NOW() + $3 * interval '1 second', NOW())`,
        user.ID, refreshToken, int(refreshExpiry.Seconds()))
    if err != nil {
        log.Printf("⚠️ Failed to save refresh token: %v", err)
    }

    // Проверяем устройство
    userID := user.ID
    
    // Получаем последний вход
    var lastLoginIP string
    database.Pool.QueryRow(context.Background(),
        "SELECT ip_address FROM login_history WHERE user_id = $1 ORDER BY login_time DESC LIMIT 1",
        userID).Scan(&lastLoginIP)

    // Если устройство новое - отправляем уведомление
    if lastLoginIP != "" && lastLoginIP != c.ClientIP() {
        details := map[string]interface{}{
            "ip":       c.ClientIP(),
            "location": "Неизвестно",
            "device":   c.GetHeader("User-Agent"),
            "time":     time.Now().Format("02.01.2006 15:04"),
        }
        LogAndNotify(c, userID, NotifLoginNewDevice, details)
    }

    // Сохраняем вход в историю
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

    // Удаляем refresh token из БД (инвалидация)
    _, err := database.Pool.Exec(c.Request.Context(),
        "DELETE FROM user_tokens WHERE token = $1", req.RefreshToken)
    if err != nil {
        log.Printf("⚠️ Failed to delete refresh token: %v", err)
    }

    // Очищаем куки, если они используются
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

    // Проверяем, не занят ли email
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

    // Хешируем пароль
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
        return
    }

    // Создаём пользователя в БД (email_verified = false по умолчанию)
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

    // Генерируем код подтверждения
    verificationCode, err := GenerateVerificationCode(user.ID, "email")
    if err != nil {
        log.Printf("❌ Failed to generate verification code: %v", err)
    } else {
        // Отправляем код на email (в фоне)
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

    // Генерируем токены (хотя пользователь ещё не верифицирован)
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
            "id":    user.ID,
            "email": user.Email,
            "name":  user.Name,
            "role":  user.Role,
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

    // Проверяем, существует ли refresh token в БД (не был ли отозван)
    var exists bool
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT EXISTS(SELECT 1 FROM user_tokens WHERE token = $1 AND expires_at > NOW())",
        req.RefreshToken).Scan(&exists)
    if err != nil || !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired refresh token"})
        return
    }

    // Валидируем refresh token и получаем новый access token
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