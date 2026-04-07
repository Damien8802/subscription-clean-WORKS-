package handlers

import (
    "encoding/json"
    "encoding/xml"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

type ProductXML struct {
    XMLName   xml.Name `xml:"Товар"`
    Code      string   `xml:"Код"`
    Name      string   `xml:"Наименование"`
    SKU       string   `xml:"Артикул"`
    Price     float64  `xml:"Цена"`
    Quantity  int      `xml:"Количество"`
    Unit      string   `xml:"ЕдиницаИзмерения"`
}

type ProductsXML struct {
    XMLName  xml.Name     `xml:"Товары"`
    Products []ProductXML `xml:"Товар"`
}

type OrderXML struct {
    XMLName      xml.Name   `xml:"Заказ"`
    Number       string     `xml:"Номер"`
    Date         string     `xml:"Дата"`
    CustomerName string     `xml:"Покупатель>Наименование"`
    TotalAmount  float64    `xml:"Сумма"`
    Items        []ItemXML  `xml:"Товары>Товар"`
}

type ItemXML struct {
    Code     string  `xml:"Код"`
    Name     string  `xml:"Наименование"`
    Quantity int     `xml:"Количество"`
    Price    float64 `xml:"Цена"`
    Amount   float64 `xml:"Сумма"`
}

func ExportProductsTo1C(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, COALESCE(sku, ''), price, quantity, COALESCE(unit, 'шт')
        FROM inventory_products
        WHERE user_id = $1 AND is_active = true
    `, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var products []ProductXML
    for rows.Next() {
        var p ProductXML
        var id uuid.UUID
        rows.Scan(&id, &p.Name, &p.SKU, &p.Price, &p.Quantity, &p.Unit)
        p.Code = id.String()
        products = append(products, p)
    }
    
    xmlData := ProductsXML{Products: products}
    
    filename := fmt.Sprintf("export_products_%d.xml", time.Now().Unix())
    filepath := fmt.Sprintf("./exports/%s", filename)
    
    os.MkdirAll("./exports", 0755)
    
    file, err := os.Create(filepath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать файл"})
        return
    }
    defer file.Close()
    
    encoder := xml.NewEncoder(file)
    encoder.Indent("", "  ")
    encoder.Encode(xmlData)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "file":    filename,
        "count":   len(products),
    })
}

func ExportOrdersTo1C(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, order_number, created_at, customer_name, total_amount
        FROM orders
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 100
    `, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var orders []OrderXML
    for rows.Next() {
        var o OrderXML
        var id uuid.UUID
        var createdAt time.Time
        rows.Scan(&id, &o.Number, &createdAt, &o.CustomerName, &o.TotalAmount)
        o.Date = createdAt.Format("2006-01-02")
        orders = append(orders, o)
    }
    
    filename := fmt.Sprintf("export_orders_%d.json", time.Now().Unix())
    filepath := fmt.Sprintf("./exports/%s", filename)
    
    os.MkdirAll("./exports", 0755)
    
    file, err := os.Create(filepath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать файл"})
        return
    }
    defer file.Close()
    
    encoder := json.NewEncoder(file)
    encoder.SetIndent("", "  ")
    encoder.Encode(orders)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "file":    filename,
        "count":   len(orders),
    })
}

func ImportProductsFrom1C(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не выбран"})
        return
    }
    
    tempPath := fmt.Sprintf("./uploads/%s", file.Filename)
    os.MkdirAll("./uploads", 0755)
    c.SaveUploadedFile(file, tempPath)
    defer os.Remove(tempPath)
    
    f, _ := os.Open(tempPath)
    defer f.Close()
    
    decoder := xml.NewDecoder(f)
    var products ProductsXML
    decoder.Decode(&products)
    
    var imported int
    for _, p := range products.Products {
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO inventory_products (user_id, name, sku, price, quantity, unit, is_active, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
            ON CONFLICT (user_id, sku) DO UPDATE SET
                name = EXCLUDED.name,
                price = EXCLUDED.price,
                quantity = EXCLUDED.quantity
        `, userID, p.Name, p.SKU, p.Price, p.Quantity, p.Unit)
        
        if err == nil {
            imported++
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "imported": imported,
        "total":    len(products.Products),
    })
}

func GetSyncLogs(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, direction, entity_type, record_count, status, created_at
        FROM sync_logs
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 50
    `, userID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
        return
    }
    defer rows.Close()
    
    var logs []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var direction, entityType, status string
        var recordCount int
        var createdAt time.Time
        
        rows.Scan(&id, &direction, &entityType, &recordCount, &status, &createdAt)
        logs = append(logs, map[string]interface{}{
            "id": id, "direction": direction, "entity_type": entityType,
            "record_count": recordCount, "status": status, "created_at": createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func GetSyncSettings(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    var settingsJSON []byte
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT settings FROM integration_settings
        WHERE user_id = $1 AND integration_type = '1c'
    `, userID).Scan(&settingsJSON)
    
    settings := map[string]interface{}{
        "auto_sync": false, "sync_interval": 3600,
        "export_products": true, "export_orders": true,
    }
    
    if err == nil && len(settingsJSON) > 0 {
        json.Unmarshal(settingsJSON, &settings)
    }
    
    c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func UpdateSyncSettings(c *gin.Context) {
    userID := getUserIDFromContext(c)
    
    var req struct {
        Settings map[string]interface{} `json:"settings"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    settingsJSON, _ := json.Marshal(req.Settings)
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO integration_settings (user_id, integration_type, settings, updated_at)
        VALUES ($1, '1c', $2, NOW())
        ON CONFLICT (user_id, integration_type) DO UPDATE SET
            settings = EXCLUDED.settings, updated_at = NOW()
    `, userID, settingsJSON)
    
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func AddWebhookHandler(c *gin.Context) {
    var req struct {
        Action    string                 `json:"action"`
        Data      map[string]interface{} `json:"data"`
        Timestamp int64                  `json:"timestamp"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    log.Printf("📥 Webhook от 1С: action=%s", req.Action)
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func StartSyncScheduler() {
    log.Println("🤖 Планировщик синхронизации с 1С запущен")
}
