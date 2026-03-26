package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "strconv"
    "subscription-system/database"
    "subscription-system/models"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type EnsureKeyRequest struct {
    TelegramID   int64  `json:"telegram_id"`
    TelegramName string `json:"telegram_name"`
}

type EnsureKeyResponse struct {
    Token string `json:"token"`
}

func CreateAPIKeyHandler(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Не авторизован"})
        return
    }

    var req struct {
        Name     string                 `json:"name"`
        Creds    map[string]interface{} `json:"credentials"`
        Quota    int64                   `json:"quota_limit"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // ВАЖНО: если credentials не переданы, создаём пустой объект
    if req.Creds == nil {
        req.Creds = make(map[string]interface{})
    }

    rawKey := "sk_live_" + uuid.New().String() + uuid.New().String()
    rawKey = rawKey[:48]

    apiKey := &models.APIKey{
        ID:         uuid.New().String(),
        UserID:     userID.(string),
        Name:       req.Name,
        Key:        rawKey,
        QuotaLimit: req.Quota,
        IsActive:   true,
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    // Теперь req.Creds точно не nil
    credsJSON, _ := json.Marshal(req.Creds)
    apiKey.ProviderCredentials = credsJSON

    if err := models.CreateAPIKey(apiKey); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create API key: " + err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "raw_key": rawKey,
        "api_key": gin.H{
            "id":          apiKey.ID,
            "name":        apiKey.Name,
            "quota_limit": apiKey.QuotaLimit,
            "is_active":   apiKey.IsActive,
            "created_at":  apiKey.CreatedAt,
        },
    })
}
func GetUserAPIKeysHandler(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        if uid, exists := c.Get("userID"); exists {
            userID = uid.(string)
        } else {
            c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
            return
        }
    }

    keys, err := models.GetAPIKeysByUser(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    var safeKeys []gin.H
    for _, key := range keys {
        safeKeys = append(safeKeys, gin.H{
            "id":          key.ID,
            "name":        key.Name,
            "quota_limit": key.QuotaLimit,
            "quota_used":  key.QuotaUsed,
            "is_active":   key.IsActive,
            "created_at":  key.CreatedAt,
            "updated_at":  key.UpdatedAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "keys":    safeKeys,
    })
}

func RevokeAPIKeyHandler(c *gin.Context) {
    var req struct {
        KeyID string `json:"key_id"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    isActive := false
    err := models.UpdateAPIKey(req.KeyID, nil, &isActive, nil)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

func RevokeAPIKeyHandlerWithID(c *gin.Context) {
    keyID := c.Param("id")
    if keyID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "key_id required"})
        return
    }

    isActive := false
    err := models.UpdateAPIKey(keyID, nil, &isActive, nil)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

func ValidateAPIKeyHandler(c *gin.Context) {
    var req struct {
        Key string `json:"key"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    apiKey, err := models.VerifyAPIKey(req.Key)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{
            "valid": false,
            "error": "invalid key",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "valid":   true,
        "user_id": apiKey.UserID,
        "quota":   apiKey.QuotaLimit - apiKey.QuotaUsed,
    })
}

func MyKeysPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "my-keys.html", gin.H{
        "Title": "Мои API ключи",
    })
}

func APIKeysPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "api_keys.html", gin.H{
        "Title": "Мои API ключи",
    })
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

    var userID string
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id FROM users WHERE telegram_id = $1`, req.TelegramID).Scan(&userID)

    if err != nil {
        log.Printf("User with telegram_id=%d not found, creating new user", req.TelegramID)

        email := fmt.Sprintf("tg_%d@placeholder.com", req.TelegramID)
        name := req.TelegramName
        if name == "" {
            name = fmt.Sprintf("user_%d", req.TelegramID)
        }

        randomPass := uuid.New().String()[:12]

        newUser, err := models.CreateUser(email, randomPass, name)
        if err != nil {
            log.Printf("Failed to create user: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
            return
        }
        userID = newUser.ID

        _, err = database.Pool.Exec(c.Request.Context(),
            `UPDATE users SET telegram_id = $1, telegram_username = $2 WHERE id = $3`,
            req.TelegramID, req.TelegramName, userID)
        if err != nil {
            log.Printf("Failed to update telegram_id: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set telegram_id"})
            return
        }
    } else {
        log.Printf("User found with ID: %s", userID)

        adminChatID, _ := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
        if req.TelegramID == adminChatID {
            _, err = database.Pool.Exec(c.Request.Context(),
                `UPDATE users SET role = 'admin' WHERE id = $1 AND role != 'admin'`, userID)
            if err != nil {
                log.Printf("Failed to set admin role: %v", err)
            }
        }

        var keyID string
        err = database.Pool.QueryRow(c.Request.Context(),
            `SELECT id FROM api_keys WHERE user_id = $1 AND is_active = true ORDER BY created_at LIMIT 1`,
            userID).Scan(&keyID)
        if err == nil {
            log.Printf("Existing active key found for user %s", userID)
            c.JSON(http.StatusOK, EnsureKeyResponse{Token: "active_key_exists"})
            return
        }
    }

    rawKey := "sk_live_" + uuid.New().String() + uuid.New().String()
    rawKey = rawKey[:48]

    apiKey := &models.APIKey{
        ID:         uuid.New().String(),
        UserID:     userID,
        Name:       "Telegram Bot Key",
        Key:        rawKey,
        QuotaLimit: 1000000,
        IsActive:   true,
        CreatedAt:  time.Now(),
        UpdatedAt:  time.Now(),
    }

    providerCredentials := make(map[string]interface{})
    credsJSON, _ := json.Marshal(providerCredentials)
    apiKey.ProviderCredentials = credsJSON

    err = models.CreateAPIKey(apiKey)
    if err != nil {
        log.Printf("Failed to create API key: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create API key"})
        return
    }

    log.Printf("API key successfully created for user %s", userID)
    c.JSON(http.StatusOK, EnsureKeyResponse{Token: rawKey})
}