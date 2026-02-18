package handlers

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "subscription-system/database"
    "subscription-system/models"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type EnsureKeyRequest struct {
    TelegramID   int64  `json:"telegram_id" binding:"required"`
    TelegramName string `json:"telegram_name"`
}

type EnsureKeyResponse struct {
    Token string `json:"token"`
}

func EnsureAPIKeyForTelegram(c *gin.Context) {
    log.Println("=== EnsureAPIKeyForTelegram started ===")

    var req EnsureKeyRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        log.Printf("Invalid request: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    log.Printf("Processing ensure-key for telegram_id=%d, name=%s", req.TelegramID, req.TelegramName)

    // Ищем пользователя по telegram_id
    var userID string
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id FROM users WHERE telegram_id = $1`, req.TelegramID).Scan(&userID)

    if err != nil { // пользователь не найден – создаём нового
        log.Printf("User with telegram_id=%d not found, creating new user", req.TelegramID)

        // Генерируем email вида tg_123456@placeholder.com
        email := fmt.Sprintf("tg_%d@placeholder.com", req.TelegramID)
        name := req.TelegramName
        if name == "" {
            name = fmt.Sprintf("user_%d", req.TelegramID)
        }

        // Генерируем случайный пароль
        randomPass := uuid.New().String()[:12]

        // Создаём пользователя
        newUser, err := models.CreateUser(email, randomPass, name)
        if err != nil {
            log.Printf("Failed to create user: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user", "details": err.Error()})
            return
        }
        userID = newUser.ID
        log.Printf("User created with ID: %s", userID)

        // Обновляем telegram_id у созданного пользователя
        _, err = database.Pool.Exec(c.Request.Context(),
            `UPDATE users SET telegram_id = $1, telegram_username = $2 WHERE id = $3`,
            req.TelegramID, req.TelegramName, userID)
        if err != nil {
            log.Printf("Failed to update telegram_id: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set telegram_id", "details": err.Error()})
            return
        }
        log.Printf("Telegram ID %d assigned to user %s", req.TelegramID, userID)
    } else {
        // Пользователь найден, проверяем, есть ли у него уже активный ключ
        log.Printf("User found with ID: %s", userID)

        // Проверяем, является ли пользователь администратором
        adminChatID, _ := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
        if req.TelegramID == adminChatID {
            // Обновляем роль пользователя на admin, если ещё не admin
            _, err = database.Pool.Exec(c.Request.Context(),
                `UPDATE users SET role = 'admin' WHERE id = $1 AND role != 'admin'`, userID)
            if err != nil {
                log.Printf("Failed to set admin role: %v", err)
            } else {
                log.Printf("User %s promoted to admin", userID)
            }
        }

        var existingToken string
        err = database.Pool.QueryRow(c.Request.Context(),
            `SELECT token FROM api_keys WHERE user_id = $1 AND is_active = true ORDER BY created_at LIMIT 1`,
            userID).Scan(&existingToken)
        if err == nil {
            // Уже есть активный ключ – возвращаем его
            log.Printf("Existing key found for user %s", userID)
            c.JSON(http.StatusOK, EnsureKeyResponse{Token: existingToken})
            return
        }
        log.Printf("No active key found for user %s, will generate new one", userID)
    }

    // Создаём новый API-ключ для пользователя
    providerCredentials := make(map[string]interface{}) // пустой объект
    quotaLimit := int64(1000000) // лимит по умолчанию
    rawKey, _, err := models.GenerateAPIKey(userID, "Telegram Bot Key", providerCredentials, quotaLimit)
    if err != nil {
        log.Printf("Failed to generate API key: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate API key", "details": err.Error()})
        return
    }

    // Принудительно активируем ключ (на случай, если вставка не установила is_active = true)
    _, err = database.Pool.Exec(c.Request.Context(),
        `UPDATE api_keys SET is_active = true WHERE user_id = $1 AND token = $2`,
        userID, rawKey)
    if err != nil {
        log.Printf("Warning: failed to force activate key: %v", err)
    } else {
        log.Printf("Key forcefully activated for user %s", userID)
    }

    log.Printf("API key successfully created for user %s", userID)
    c.JSON(http.StatusOK, EnsureKeyResponse{Token: rawKey})
}