package handlers

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
)

// VPN Тарифы
type VPNPlan struct {
    ID      int     `json:"id"`
    Name    string  `json:"name"`
    Price   float64 `json:"price"`
    Days    int     `json:"days"`
    Speed   string  `json:"speed"`
    Devices int     `json:"devices"`
}

// Глобальная переменная для БД
var vpnDB *pgxpool.Pool

// Инициализация VPN
func InitVPNWithDB(db *pgxpool.Pool) {
    vpnDB = db
}

// Генерация WireGuard ключей
func generateWireGuardKeys() (privateKey, publicKey string, err error) {
    privateBytes := make([]byte, 32)
    publicBytes := make([]byte, 32)
    rand.Read(privateBytes)
    rand.Read(publicBytes)
    
    privateKey = base64.StdEncoding.EncodeToString(privateBytes)
    publicKey = base64.StdEncoding.EncodeToString(publicBytes)
    return
}

// Страница продажи VPN
func VPNSalesPageHandler(c *gin.Context) {
    // Получаем планы из БД
    rows, err := vpnDB.Query(context.Background(), 
        "SELECT id, name, price, days, speed, devices FROM vpn_plans ORDER BY price")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var plans []VPNPlan
    for rows.Next() {
        var plan VPNPlan
        err := rows.Scan(&plan.ID, &plan.Name, &plan.Price, &plan.Days, &plan.Speed, &plan.Devices)
        if err != nil {
            continue
        }
        plans = append(plans, plan)
    }
    
    c.HTML(http.StatusOK, "vpn-sales.html", gin.H{
        "plans": plans,
        "title": "VPN Сервис - Безопасный и быстрый доступ",
    })
}

// Создать VPN ключ
func CreateVPNKey(c *gin.Context) {
    var req struct {
        PlanID int `json:"plan_id"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    // Получаем план из БД
    var plan VPNPlan
    err := vpnDB.QueryRow(context.Background(),
        "SELECT id, name, price, days, speed, devices FROM vpn_plans WHERE id = $1", req.PlanID).
        Scan(&plan.ID, &plan.Name, &plan.Price, &plan.Days, &plan.Speed, &plan.Devices)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Plan not found"})
        return
    }
    
    // Генерируем ключи
    privateKey, publicKey, err := generateWireGuardKeys()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate keys"})
        return
    }
    
    // Генерируем имя клиента
    clientName := fmt.Sprintf("vpn_%s", uuid.New().String()[:8])
    
    // Находим следующий свободный IP
    var lastIP string
    vpnDB.QueryRow(context.Background(),
        "SELECT client_ip FROM vpn_keys ORDER BY client_ip DESC LIMIT 1").Scan(&lastIP)
    
    var clientIP string
    if lastIP == "" {
        clientIP = "10.0.0.2"
    } else {
        var lastNum int
        fmt.Sscanf(lastIP, "10.0.0.%d", &lastNum)
        clientIP = fmt.Sprintf("10.0.0.%d", lastNum+1)
    }
    
    expiresAt := time.Now().AddDate(0, 0, plan.Days)
    
    // Сохраняем в БД
    _, err = vpnDB.Exec(context.Background(),
        `INSERT INTO vpn_keys (client_name, client_ip, private_key, public_key, plan_id, expires_at, active)
         VALUES ($1, $2, $3, $4, $5, $6, true)`,
        clientName, clientIP, privateKey, publicKey, plan.ID, expiresAt)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save key"})
        return
    }
    
    // Формируем конфиг
    config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = %s
Endpoint = vpn.your-server.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`, privateKey, clientIP, publicKey)
    
    c.JSON(http.StatusOK, gin.H{
        "success":     true,
        "message":     "VPN ключ успешно создан!",
        "client_id":   clientName,
        "client_ip":   clientIP,
        "plan":        plan,
        "config":      config,
        "expires_in":  fmt.Sprintf("%d дней", plan.Days),
        "expires_at":  expiresAt.Format("2006-01-02"),
    })
}

// Получить конфиг клиента
func GetVPNConfig(c *gin.Context) {
    clientName := c.Param("client")
    
    var privateKey, clientIP, publicKey string
    err := vpnDB.QueryRow(context.Background(),
        `SELECT private_key, client_ip, public_key FROM vpn_keys 
         WHERE client_name = $1 AND active = true AND expires_at > NOW()`,
        clientName).Scan(&privateKey, &clientIP, &publicKey)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Key not found or expired"})
        return
    }
    
    config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/24
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = %s
Endpoint = vpn.your-server.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`, privateKey, clientIP, publicKey)
    
    c.String(http.StatusOK, config)
}

// Проверить статус VPN ключа
func CheckVPNKey(c *gin.Context) {
    clientName := c.Param("client")
    
    var expiresAt time.Time
    var planID int
    err := vpnDB.QueryRow(context.Background(),
        `SELECT expires_at, plan_id FROM vpn_keys 
         WHERE client_name = $1 AND active = true`,
        clientName).Scan(&expiresAt, &planID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
        return
    }
    
    now := time.Now()
    if now.After(expiresAt) {
        // Деактивируем истекший ключ
        vpnDB.Exec(context.Background(),
            "UPDATE vpn_keys SET active = false WHERE client_name = $1", clientName)
        c.JSON(http.StatusOK, gin.H{
            "active":  false,
            "expired": true,
            "message": "VPN ключ истек. Требуется продление",
        })
    } else {
        daysLeft := int(expiresAt.Sub(now).Hours() / 24)
        hoursLeft := int(expiresAt.Sub(now).Hours()) % 24
        
        c.JSON(http.StatusOK, gin.H{
            "active":     true,
            "expired":    false,
            "days_left":  daysLeft,
            "hours_left": hoursLeft,
            "expires_at": expiresAt.Format("2006-01-02 15:04:05"),
        })
    }
}

// Получить статистику VPN
func GetVPNStats(c *gin.Context) {
    var totalClients int
    var activeClients int
    
    vpnDB.QueryRow(context.Background(),
        "SELECT COUNT(*) FROM vpn_keys WHERE active = true").Scan(&totalClients)
    vpnDB.QueryRow(context.Background(),
        "SELECT COUNT(*) FROM vpn_keys WHERE active = true AND expires_at > NOW()").Scan(&activeClients)
    
    c.JSON(http.StatusOK, gin.H{
        "status":         "active",
        "total_clients":  totalClients,
        "active_clients": activeClients,
        "servers": []string{
            "🇷🇺 Россия (Москва) - 5 мс",
            "🇺🇸 США (Нью-Йорк) - 120 мс",
            "🇩🇪 Германия (Франкфурт) - 45 мс",
        },
    })
}

// Продлить VPN ключ
func RenewVPNKey(c *gin.Context) {
    clientName := c.Param("client")
    
    var req struct {
        PlanID int `json:"plan_id"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }
    
    // Получаем план
    var plan VPNPlan
    err := vpnDB.QueryRow(context.Background(),
        "SELECT id, days FROM vpn_plans WHERE id = $1", req.PlanID).
        Scan(&plan.ID, &plan.Days)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Plan not found"})
        return
    }
    
    // Обновляем дату истечения
    _, err = vpnDB.Exec(context.Background(),
        `UPDATE vpn_keys 
         SET expires_at = expires_at + ($1 || ' days')::INTERVAL, plan_id = $2, active = true
         WHERE client_name = $3`,
        plan.Days, plan.ID, clientName)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to renew"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "message":    "VPN ключ продлен!",
        "plan_id":    plan.ID,
        "days_added": plan.Days,
    })
}

// Админ: получить все ключи
func GetAllVPNKeys(c *gin.Context) {
    rows, err := vpnDB.Query(context.Background(),
        `SELECT client_name, client_ip, plan_id, expires_at, active 
         FROM vpn_keys ORDER BY created_at DESC`)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var keys []map[string]interface{}
    for rows.Next() {
        var clientName, clientIP string
        var planID int
        var expiresAt time.Time
        var active bool
        rows.Scan(&clientName, &clientIP, &planID, &expiresAt, &active)
        
        keys = append(keys, map[string]interface{}{
            "client_name": clientName,
            "client_ip":   clientIP,
            "plan_id":     planID,
            "expires_at":  expiresAt,
            "active":      active,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// Админ: статистика
func AdminVPNHandler(c *gin.Context) {
    GetAllVPNKeys(c)
}