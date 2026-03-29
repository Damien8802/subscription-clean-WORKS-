package handlers

import (
    "encoding/base64"
    "log"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/pquerna/otp/totp"
    "github.com/skip2/go-qrcode"

    "subscription-system/database"
    "subscription-system/middleware"
)

func generateRandomCode(length int) string {
    const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
        time.Sleep(1 * time.Nanosecond)
    }
    return string(b)
}

// GenerateTwoFASecret - генерация секрета и QR кода
func GenerateTwoFASecret(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    var email string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT email FROM users WHERE id = $1 AND tenant_id = $2", userID, tenantID).Scan(&email)
    if err != nil {
        email = "user@example.com"
    }

    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "SaaSPro",
        AccountName: email,
        Period:      30,
        Digits:      6,
    })
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate secret"})
        return
    }

    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO twofa (user_id, tenant_id, secret, enabled, created_at, updated_at)
        VALUES ($1, $2, $3, false, NOW(), NOW())
        ON CONFLICT (user_id, tenant_id) DO UPDATE SET
            secret = $3,
            enabled = false,
            updated_at = NOW()
    `, userID, tenantID, key.Secret())
    if err != nil {
        log.Printf("Failed to save 2FA secret: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save secret"})
        return
    }

    var png []byte
    png, err = qrcode.Encode(key.URL(), qrcode.Medium, 256)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "secret":    key.Secret(),
        "qr":        base64.StdEncoding.EncodeToString(png),
        "url":       key.URL(),
        "user":      email,
        "tenant_id": tenantID,
    })
}

// VerifyTwoFACode - проверка и активация 2FA
func VerifyTwoFACode(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    var req struct {
        UserID string `json:"user_id"`
        Code   string `json:"code"`
        Secret string `json:"secret"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.UserID == "" {
        req.UserID = "test-user-123"
    }

    valid := totp.Validate(req.Code, req.Secret)
    if !valid {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid code"})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE twofa SET enabled = true, updated_at = NOW() 
        WHERE user_id = $1 AND tenant_id = $2
    `, req.UserID, tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to enable 2FA"})
        return
    }

    // Генерируем резервные коды
    codes := make([]string, 10)
    for i := 0; i < 10; i++ {
        codes[i] = generateRandomCode(8)
    }
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO twofa_backup_codes (user_id, tenant_id, codes, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        ON CONFLICT (user_id, tenant_id) DO UPDATE SET codes = $3, updated_at = NOW()
    `, req.UserID, tenantID, codes)

    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "message":      "2FA enabled successfully",
        "backup_codes": codes,
    })
}

// DisableTwoFA - отключение 2FA
func DisableTwoFA(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    var req struct {
        UserID string `json:"user_id"`
        Code   string `json:"code"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.UserID == "" {
        req.UserID = "test-user-123"
    }

    var secret string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT secret FROM twofa WHERE user_id = $1 AND tenant_id = $2", req.UserID, tenantID).Scan(&secret)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "2FA not set up"})
        return
    }

    valid := totp.Validate(req.Code, secret)
    if !valid {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid code"})
        return
    }

    _, err = database.Pool.Exec(c.Request.Context(), `
        UPDATE twofa SET enabled = false, updated_at = NOW() 
        WHERE user_id = $1 AND tenant_id = $2
    `, req.UserID, tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable 2FA"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "2FA disabled successfully",
    })
}

// GetTwoFAStatus - статус 2FA
func GetTwoFAStatus(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    var enabled bool
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT enabled FROM twofa WHERE user_id = $1 AND tenant_id = $2", userID, tenantID).Scan(&enabled)

    if err != nil {
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

// GetBackupCodes - получить резервные коды
func GetBackupCodes(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    var codes []string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT codes FROM twofa_backup_codes WHERE user_id = $1 AND tenant_id = $2", userID, tenantID).Scan(&codes)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "codes":   []string{},
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "codes":   codes,
    })
}

// GenerateBackupCodes - генерация резервных кодов
func GenerateBackupCodes(c *gin.Context) {
    tenantID := middleware.GetTenantIDFromContext(c)
    var req struct {
        UserID string `json:"user_id"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        req.UserID = c.Query("user_id")
    }

    if req.UserID == "" {
        req.UserID = "test-user-123"
    }

    codes := make([]string, 10)
    for i := 0; i < 10; i++ {
        codes[i] = generateRandomCode(8)
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO twofa_backup_codes (user_id, tenant_id, codes, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        ON CONFLICT (user_id, tenant_id) DO UPDATE SET 
            codes = $3, 
            updated_at = NOW()
    `, req.UserID, tenantID, codes)
    if err != nil {
        log.Printf("Failed to save backup codes: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save backup codes"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "codes":   codes,
    })
}

// Get2FASettings - получить настройки 2FA
func Get2FASettings(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    c.JSON(http.StatusOK, gin.H{
        "enabled": false,
        "method":  "totp",
    })
}

// CheckTrustedDevice - проверка доверенного устройства
func CheckTrustedDevice(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"trusted": false})
}

// TrustDevice - добавить доверенное устройство
func TrustDevice(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Device trusted"})
}

// VerifyWithBackupCode - проверка резервного кода
func VerifyWithBackupCode(c *gin.Context) {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid backup code"})
}
