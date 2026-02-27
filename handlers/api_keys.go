package handlers

import (
    "net/http"
    "subscription-system/models"

    "github.com/gin-gonic/gin"
)

// CreateAPIKeyHandler - создание ключа
func CreateAPIKeyHandler(c *gin.Context) {
    var req struct {
        UserID   string                 `json:"user_id" binding:"required"`
        Name     string                 `json:"name" binding:"required"`
        Creds    map[string]interface{} `json:"credentials"`
        Quota    int64                  `json:"quota_limit"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    rawKey, apiKey, err := models.GenerateAPIKey(req.UserID, req.Name, req.Creds, req.Quota)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "raw_key":   rawKey,  // показываем только один раз!
        "api_key":   apiKey,
    })
}

// GetUserAPIKeysHandler - список ключей пользователя
func GetUserAPIKeysHandler(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
        return
    }

    keys, err := models.GetAPIKeysByUser(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "keys":    keys,
    })
}

// RevokeAPIKeyHandler - отзыв ключа
func RevokeAPIKeyHandler(c *gin.Context) {
    var req struct {
        KeyID string `json:"key_id" binding:"required"`
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

// ValidateAPIKeyHandler - проверка ключа для внешних сервисов
func ValidateAPIKeyHandler(c *gin.Context) {
    var req struct {
        Key string `json:"key" binding:"required"`
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

// MyKeysPageHandler - страница с ключами пользователя
func MyKeysPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "my-keys.html", gin.H{
        "Title": "Мои API ключи",
    })
}

// APIKeysPageHandler - страница управления ключами
func APIKeysPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "api_keys.html", gin.H{
        "Title": "Мои API ключи",
    })
}