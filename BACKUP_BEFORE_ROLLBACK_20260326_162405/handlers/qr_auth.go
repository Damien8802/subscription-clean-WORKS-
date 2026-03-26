package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "github.com/gorilla/websocket"
    
    "subscription-system/database"
)

// QRLoginPageHandler - страница входа по QR коду
func QRLoginPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "qr-login.html", gin.H{
        "title": "Вход по QR коду | SaaSPro",
    })
}

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

// GenerateJWTForUser генерирует JWT токен для пользователя
func GenerateJWTForUser(userID uuid.UUID) (string, error) {
    secret := os.Getenv("JWT_SECRET")
    if secret == "" {
        secret = "your-secret-key-change-in-production"
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "user_id": userID.String(),
        "exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
        "iat":     time.Now().Unix(),
    })
    
    tokenString, err := token.SignedString([]byte(secret))
    if err != nil {
        return "", err
    }
    
    return tokenString, nil
}

// Генерация QR кода для входа/регистрации
func GenerateQRCode(c *gin.Context) {
    // Генерируем уникальный токен сессии
    sessionToken := generateRandomString(64)
    
    // Создаем QR код
    qrData := fmt.Sprintf("saaspro://auth?token=%s&ts=%d", sessionToken, time.Now().Unix())
    
    // Сохраняем сессию
    expiresAt := time.Now().Add(5 * time.Minute)
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO qr_sessions (session_token, qr_code, expires_at)
        VALUES ($1, $2, $3)
    `, sessionToken, qrData, expiresAt)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create QR session"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "session_token": sessionToken,
        "qr_code":       qrData,
        "qr_data_url":   fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=%s", qrData),
        "expires_in":    300,
    })
}

// WebSocket для отслеживания статуса QR кода
func QRStatusWebSocket(c *gin.Context) {
    sessionToken := c.Query("token")
    mode := c.Query("mode") // login или register
    
    if sessionToken == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
        return
    }
    
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()
    
    // Отправляем начальный статус
    var status string
    var userID uuid.UUID
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT status, user_id FROM qr_sessions 
        WHERE session_token = $1 AND expires_at > NOW()
    `, sessionToken).Scan(&status, &userID)
    
    if err != nil {
        conn.WriteJSON(gin.H{"status": "expired", "message": "QR code expired"})
        return
    }
    
    conn.WriteJSON(gin.H{"status": status, "user_id": userID.String()})
    
    // Слушаем изменения статуса
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    lastStatus := status
    
    for {
        select {
        case <-ticker.C:
            var newStatus string
            var newUserID uuid.UUID
            err := database.Pool.QueryRow(c.Request.Context(), `
                SELECT status, user_id FROM qr_sessions 
                WHERE session_token = $1
            `, sessionToken).Scan(&newStatus, &newUserID)
            
            if err != nil {
                conn.WriteJSON(gin.H{"status": "expired"})
                return
            }
            
            if newStatus != lastStatus {
                lastStatus = newStatus
                
                response := gin.H{
                    "status":  newStatus,
                    "user_id": newUserID.String(),
                    "message": getStatusMessage(newStatus),
                }
                
                if newStatus == "approved" {
                    var token string
                    var err error
                    
                    if mode == "register" {
                        // Для регистрации создаем нового пользователя
                        token, err = createUserFromQR(newUserID)
                    } else {
                        token, err = GenerateJWTForUser(newUserID)
                    }
                    
                    if err == nil {
                        response["token"] = token
                    }
                }
                
                conn.WriteJSON(response)
                
                if newStatus == "approved" {
                    return
                }
            }
        }
    }
}

// Создание пользователя из QR регистрации
func createUserFromQR(userID uuid.UUID) (string, error) {
    // Проверяем, существует ли пользователь
    var existingID uuid.UUID
    err := database.Pool.QueryRow(context.Background(), `
        SELECT id FROM users WHERE id = $1
    `, userID).Scan(&existingID)
    
    if err == nil {
        // Пользователь существует, просто выдаем токен
        return GenerateJWTForUser(userID)
    }
    
    // Создаем нового пользователя
    name := fmt.Sprintf("QR_User_%s", userID.String()[:8])
    email := fmt.Sprintf("%s@qr.saaspro.ru", userID.String()[:8])
    
    err = database.Pool.QueryRow(context.Background(), `
        INSERT INTO users (id, name, email, created_at)
        VALUES ($1, $2, $3, NOW())
        RETURNING id
    `, userID, name, email).Scan(&userID)
    
    if err != nil {
        return "", err
    }
    
    return GenerateJWTForUser(userID)
}

// Сканирование QR кода (мобильное приложение)
func ScanQRCode(c *gin.Context) {
    var req struct {
        SessionToken string `json:"session_token" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Проверяем сессию
    var sessionID uuid.UUID
    var expiresAt time.Time
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, expires_at FROM qr_sessions 
        WHERE session_token = $1 AND status = 'pending' AND expires_at > NOW()
    `, req.SessionToken).Scan(&sessionID, &expiresAt)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or expired QR code"})
        return
    }
    
    // Обновляем статус на scanned
    _, err = database.Pool.Exec(c.Request.Context(), `
        UPDATE qr_sessions SET status = 'scanned', scanned_at = NOW()
        WHERE session_token = $1
    `, req.SessionToken)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "scanned",
        "message": "QR code scanned, waiting for approval",
    })
}

// Подтверждение входа (после сканирования)
func ApproveQRLogin(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        SessionToken string `json:"session_token" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Проверяем сессию
    var sessionID uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id FROM qr_sessions 
        WHERE session_token = $1 AND status = 'scanned' AND expires_at > NOW()
    `, req.SessionToken).Scan(&sessionID)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Session not found or already approved"})
        return
    }
    
    // Подтверждаем вход
    _, err = database.Pool.Exec(c.Request.Context(), `
        UPDATE qr_sessions SET status = 'approved', user_id = $1, approved_at = NOW()
        WHERE session_token = $2
    `, userID, req.SessionToken)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "approved",
        "message": "Login approved",
    })
}

// Регистрация устройства для пуш-уведомлений
func RegisterPushDevice(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        DeviceToken string `json:"device_token" binding:"required"`
        DeviceType  string `json:"device_type" binding:"required"`
        DeviceName  string `json:"device_name"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Удаляем старые устройства
    database.Pool.Exec(c.Request.Context(), `
        DELETE FROM push_devices WHERE device_token = $1
    `, req.DeviceToken)
    
    // Регистрируем новое устройство
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO push_devices (user_id, device_token, device_type, device_name)
        VALUES ($1, $2, $3, $4)
    `, userID, req.DeviceToken, req.DeviceType, req.DeviceName)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register device"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "status":  "registered",
        "message": "Device registered for push notifications",
    })
}

// Получить устройства пользователя
func GetUserDevices(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, device_token, device_type, device_name, active, created_at, last_used_at
        FROM push_devices
        WHERE user_id = $1 AND active = true
        ORDER BY created_at DESC
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get devices"})
        return
    }
    defer rows.Close()
    
    var devices []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var deviceToken, deviceType, deviceName string
        var active bool
        var createdAt, lastUsedAt time.Time
        
        rows.Scan(&id, &deviceToken, &deviceType, &deviceName, &active, &createdAt, &lastUsedAt)
        
        devices = append(devices, map[string]interface{}{
            "id":           id,
            "device_token": deviceToken,
            "device_type":  deviceType,
            "device_name":  deviceName,
            "active":       active,
            "created_at":   createdAt,
            "last_used_at": lastUsedAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"devices": devices})
}

// Удалить устройство
func RemovePushDevice(c *gin.Context) {
    userID := getUserID(c)
    deviceID := c.Param("id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM push_devices WHERE id = $1 AND user_id = $2
    `, deviceID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove device"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

// Отправить пуш-уведомление пользователю
func SendPushNotification(userID uuid.UUID, title, body string, data map[string]interface{}) error {
    // Получаем устройства пользователя
    rows, err := database.Pool.Query(context.Background(), `
        SELECT device_token, device_type FROM push_devices
        WHERE user_id = $1 AND active = true
    `, userID)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    var devices []struct {
        Token string
        Type  string
    }
    
    for rows.Next() {
        var d struct {
            Token string
            Type  string
        }
        rows.Scan(&d.Token, &d.Type)
        devices = append(devices, d)
    }
    
    // Сохраняем уведомление в БД
    dataJSON, _ := json.Marshal(data)
    _, err = database.Pool.Exec(context.Background(), `
        INSERT INTO push_notifications (user_id, title, body, data)
        VALUES ($1, $2, $3, $4)
    `, userID, title, body, dataJSON)
    if err != nil {
        return err
    }
    
    // Отправляем push через Web Push (для браузеров)
    for _, device := range devices {
        go sendWebPush(device.Token, title, body, data)
    }
    
    return nil
}

// Вспомогательные функции
func getStatusMessage(status string) string {
    switch status {
    case "pending":
        return "Ожидание сканирования"
    case "scanned":
        return "Отсканировано! Подтвердите вход в приложении"
    case "approved":
        return "Вход подтвержден! Перенаправление..."
    default:
        return "Неизвестный статус"
    }
}

func getUserID(c *gin.Context) uuid.UUID {
    userID, _ := c.Get("user_id")
    if uid, ok := userID.(uuid.UUID); ok {
        return uid
    }
    if uidStr, ok := userID.(string); ok {
        if parsed, err := uuid.Parse(uidStr); err == nil {
            return parsed
        }
    }
    return uuid.Nil
}

func sendWebPush(endpoint, title, body string, data map[string]interface{}) {
    log.Printf("📱 Отправка push уведомления на %s: %s - %s", endpoint, title, body)
}