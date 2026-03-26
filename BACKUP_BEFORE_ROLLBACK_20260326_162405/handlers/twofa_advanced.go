package handlers

import (
    "crypto/rand"
    "encoding/base64"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/pquerna/otp/totp"

    "subscription-system/database"
)

// Get2FASettings возвращает расширенные настройки 2FA
func Get2FASettings(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    // Для тестового пользователя
    if userID == "test-user-123" {
        c.JSON(http.StatusOK, gin.H{
            "enabled":            false,
            "has_backup_codes":   false,
            "backup_codes_count": 0,
            "trusted_devices":    []interface{}{},
        })
        return
    }

    // Получаем информацию о 2FA
    var enabled bool
    var backupCodes []string
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT enabled, COALESCE(backup_codes, '{}') FROM twofa WHERE user_id = $1::uuid`,
        userID).Scan(&enabled, &backupCodes)
    if err != nil {
        // Если нет записи, значит 2FA не настроена
        c.JSON(http.StatusOK, gin.H{
            "enabled":            false,
            "has_backup_codes":   false,
            "backup_codes_count": 0,
            "trusted_devices":    []interface{}{},
        })
        return
    }

    // Получаем доверенные устройства
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT device_name, ip_address, expires_at, last_used_at 
        FROM trusted_devices 
        WHERE user_id = $1::uuid AND expires_at > NOW()
        ORDER BY last_used_at DESC`,
        userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    var devices []gin.H
    for rows.Next() {
        var deviceName, ipAddress string
        var expiresAt, lastUsedAt time.Time
        err := rows.Scan(&deviceName, &ipAddress, &expiresAt, &lastUsedAt)
        if err != nil {
            continue
        }
        devices = append(devices, gin.H{
            "name":       deviceName,
            "ip":         ipAddress,
            "expires_at": expiresAt,
            "last_used":  lastUsedAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "enabled":            enabled,
        "has_backup_codes":   len(backupCodes) > 0,
        "backup_codes_count": len(backupCodes),
        "trusted_devices":    devices,
    })
}

// GenerateBackupCodes создает резервные коды
func GenerateBackupCodes(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Для тестового пользователя
    if req.UserID == "test-user-123" {
        testCodes := []string{"11111-aaaaa", "22222-bbbbb", "33333-ccccc", "44444-ddddd"}
        c.JSON(http.StatusOK, gin.H{
            "success":      true,
            "backup_codes": testCodes,
            "message":      "Тестовые коды (не сохраняются в БД)",
        })
        return
    }

    // Генерируем 8 резервных кодов
    codes := make([]string, 8)
    for i := 0; i < 8; i++ {
        codes[i] = generateRandomCode(12)
    }

    // Сохраняем в БД
    _, err := database.Pool.Exec(c.Request.Context(),
        `UPDATE twofa SET backup_codes = $2 WHERE user_id = $1::uuid`,
        req.UserID, codes)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save backup codes"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "backup_codes": codes,
        "message":      "Сохраните эти коды в надёжном месте! Каждый код можно использовать только один раз.",
    })
}

// VerifyWithBackupCode проверяет резервный код
func VerifyWithBackupCode(c *gin.Context) {
    var req struct {
        UserID string `json:"user_id" binding:"required"`
        Code   string `json:"code" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Для тестового пользователя
    if req.UserID == "test-user-123" {
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "message": "Test mode: backup code accepted",
        })
        return
    }

    var backupCodes []string
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT backup_codes FROM twofa WHERE user_id = $1::uuid`,
        req.UserID).Scan(&backupCodes)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "2FA not set up"})
        return
    }

    // Ищем код
    found := -1
    for i, code := range backupCodes {
        if code == req.Code {
            found = i
            break
        }
    }

    if found == -1 {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid backup code"})
        return
    }

    // Удаляем использованный код
    backupCodes = append(backupCodes[:found], backupCodes[found+1:]...)
    
    _, err = database.Pool.Exec(c.Request.Context(),
        `UPDATE twofa SET backup_codes = $2 WHERE user_id = $1::uuid`,
        req.UserID, backupCodes)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update backup codes"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Login successful with backup code",
    })
}

// TrustDevice доверять устройству на 30 дней
func TrustDevice(c *gin.Context) {
    var req struct {
        UserID   string `json:"user_id" binding:"required"`
        DeviceID string `json:"device_id" binding:"required"`
        Code     string `json:"code" binding:"required,len=6"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Для тестового пользователя
    if req.UserID == "test-user-123" {
        c.JSON(http.StatusOK, gin.H{
            "success":    true,
            "message":    "Test mode: device trusted",
            "expires_at": time.Now().AddDate(0, 0, 30),
        })
        return
    }

    // Проверяем код 2FA
    var secret string
    err := database.Pool.QueryRow(c.Request.Context(),
        "SELECT secret FROM twofa WHERE user_id = $1::uuid AND enabled = true",
        req.UserID).Scan(&secret)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "2FA not enabled"})
        return
    }

    if !totp.Validate(req.Code, secret) {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid code"})
        return
    }

    // Информация об устройстве
    userAgent := c.GetHeader("User-Agent")
    ipAddress := c.ClientIP()
    deviceName := parseDeviceName(userAgent)
    expiresAt := time.Now().AddDate(0, 0, 30)

    // Сохраняем
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO trusted_devices (user_id, device_id, device_name, ip_address, user_agent, expires_at)
        VALUES ($1::uuid, $2, $3, $4, $5, $6)
        ON CONFLICT (user_id, device_id) 
        DO UPDATE SET expires_at = $6, last_used_at = NOW()`,
        req.UserID, req.DeviceID, deviceName, ipAddress, userAgent, expiresAt)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trust device"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "message":    "Device trusted for 30 days",
        "expires_at": expiresAt,
    })
}

// CheckTrustedDevice проверяет доверенное устройство
func CheckTrustedDevice(c *gin.Context) {
    userID := c.Query("user_id")
    deviceID := c.Query("device_id")

    if userID == "" || deviceID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and device_id required"})
        return
    }

    // Для тестового пользователя
    if userID == "test-user-123" {
        c.JSON(http.StatusOK, gin.H{
            "trusted":    true,
            "expires_at": time.Now().AddDate(0, 0, 30),
        })
        return
    }

    var expiresAt time.Time
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT expires_at FROM trusted_devices 
        WHERE user_id = $1::uuid AND device_id = $2 AND expires_at > NOW()`,
        userID, deviceID).Scan(&expiresAt)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"trusted": false})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "trusted":    true,
        "expires_at": expiresAt,
    })
}

// Вспомогательная функция для генерации случайных кодов
func generateRandomCode(length int) string {
    bytes := make([]byte, length)
    rand.Read(bytes)
    return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// Вспомогательная функция для определения имени устройства
func parseDeviceName(userAgent string) string {
    switch {
    case strings.Contains(userAgent, "Windows"):
        return "Windows"
    case strings.Contains(userAgent, "Mac"):
        return "macOS"
    case strings.Contains(userAgent, "Linux"):
        return "Linux"
    case strings.Contains(userAgent, "Android"):
        return "Android"
    case strings.Contains(userAgent, "iPhone"):
        return "iPhone"
    case strings.Contains(userAgent, "iPad"):
        return "iPad"
    default:
        return "Unknown device"
    }
}