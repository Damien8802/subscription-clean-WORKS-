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

// LoginHandler обрабатывает вход пользователя
func LoginHandler(c *gin.Context) {
    var req struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
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

    // Генерируем JWT токены
    accessToken, refreshToken, err := utils.GenerateTokens(user.ID, user.Role)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
        return
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
        "user": gin.H{
            "id":    user.ID,
            "email": user.Email,
            "name":  user.Name,
            "role":  user.Role,
        },
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