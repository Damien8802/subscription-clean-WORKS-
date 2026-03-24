package handlers

import (
    "encoding/json"
    "encoding/xml"
    "fmt"
    "net/http"
    "os"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// ProductXML структура для экспорта товаров в XML (формат 1С)
type ProductXML struct {
    XMLName   xml.Name `xml:"Товар"`
    Code      string   `xml:"Код"`
    Name      string   `xml:"Наименование"`
    SKU       string   `xml:"Артикул"`
    Price     float64  `xml:"Цена"`
    Quantity  int      `xml:"Количество"`
    Unit      string   `xml:"ЕдиницаИзмерения"`
}

// ProductsXML обертка для списка товаров
type ProductsXML struct {
    XMLName  xml.Name     `xml:"Товары"`
    Products []ProductXML `xml:"Товар"`
}

// OrderXML структура для экспорта заказов
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

// ==================== ЭКСПОРТ В 1С ====================

// ExportProductsTo1C - экспорт товаров в XML формате 1С
func ExportProductsTo1C(c *gin.Context) {
    userID := getUserID(c)
    
    // Получаем товары
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, COALESCE(sku, ''), price, quantity, COALESCE(unit, 'шт')
        FROM products
        WHERE user_id = $1 AND active = true
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
    
    // Формируем XML
    xmlData := ProductsXML{Products: products}
    
    // Сохраняем лог
    var logID uuid.UUID
    database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO sync_logs (user_id, direction, entity_type, record_count, status, started_at)
        VALUES ($1, 'export', 'products', $2, 'processing', NOW())
        RETURNING id
    `, userID, len(products)).Scan(&logID)
    
    // Генерируем файл
    filename := fmt.Sprintf("export_products_%d.xml", time.Now().Unix())
    filepath := fmt.Sprintf("./exports/%s", filename)
    
    // Создаем папку если нет
    os.MkdirAll("./exports", 0755)
    
    // Записываем XML в файл
    file, err := os.Create(filepath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать файл"})
        return
    }
    defer file.Close()
    
    encoder := xml.NewEncoder(file)
    encoder.Indent("", "  ")
    if err := encoder.Encode(xmlData); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации XML"})
        return
    }
    
    // Обновляем лог
    database.Pool.Exec(c.Request.Context(), `
        UPDATE sync_logs SET status = 'completed', completed_at = NOW(), file_path = $1
        WHERE id = $2
    `, filepath, logID)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "message":   "Экспорт выполнен",
        "file":      filename,
        "count":     len(products),
        "log_id":    logID,
    })
}

// ExportOrdersTo1C - экспорт заказов в XML
func ExportOrdersTo1C(c *gin.Context) {
    userID := getUserID(c)
    
    // Получаем заказы с товарами
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT o.id, o.order_number, o.created_at, o.customer_name, o.total_amount
        FROM orders o
        WHERE o.user_id = $1
        ORDER BY o.created_at DESC
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
        
        // Получаем позиции заказа
        itemsRows, err := database.Pool.Query(c.Request.Context(), `
            SELECT product_name, sku, quantity, price, total
            FROM order_items
            WHERE order_id = $1
        `, id)
        if err == nil {
            for itemsRows.Next() {
                var item ItemXML
                itemsRows.Scan(&item.Name, &item.Code, &item.Quantity, &item.Price, &item.Amount)
                o.Items = append(o.Items, item)
            }
            itemsRows.Close()
        }
        
        orders = append(orders, o)
    }
    
    // Сохраняем лог
    var logID uuid.UUID
    database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO sync_logs (user_id, direction, entity_type, record_count, status, started_at)
        VALUES ($1, 'export', 'orders', $2, 'processing', NOW())
        RETURNING id
    `, userID, len(orders)).Scan(&logID)
    
    // Генерируем JSON (для разнообразия)
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
    if err := encoder.Encode(orders); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации JSON"})
        return
    }
    
    database.Pool.Exec(c.Request.Context(), `
        UPDATE sync_logs SET status = 'completed', completed_at = NOW(), file_path = $1
        WHERE id = $2
    `, filepath, logID)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Экспорт заказов выполнен",
        "file":    filename,
        "count":   len(orders),
    })
}

// ==================== ИМПОРТ ИЗ 1С ====================

// ImportProductsFrom1C - импорт товаров из 1С XML
func ImportProductsFrom1C(c *gin.Context) {
    userID := getUserID(c)
    
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Файл не выбран"})
        return
    }
    
    // Сохраняем временный файл
    tempPath := fmt.Sprintf("./uploads/%s", file.Filename)
    os.MkdirAll("./uploads", 0755)
    
    if err := c.SaveUploadedFile(file, tempPath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить файл"})
        return
    }
    defer os.Remove(tempPath)
    
    // Открываем файл
    f, err := os.Open(tempPath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось открыть файл"})
        return
    }
    defer f.Close()
    
    // Парсим XML
    decoder := xml.NewDecoder(f)
    var products ProductsXML
    
    if err := decoder.Decode(&products); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка парсинга XML: " + err.Error()})
        return
    }
    
    // Сохраняем лог
    var logID uuid.UUID
    database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO sync_logs (user_id, direction, entity_type, record_count, status, started_at)
        VALUES ($1, 'import', 'products', $2, 'processing', NOW())
        RETURNING id
    `, userID, len(products.Products)).Scan(&logID)
    
    var imported int
    var errors []string
    
    // Импортируем товары
    for _, p := range products.Products {
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO products (user_id, name, sku, price, quantity, unit, active, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, true, NOW(), NOW())
            ON CONFLICT (user_id, sku) DO UPDATE SET
                name = EXCLUDED.name,
                price = EXCLUDED.price,
                quantity = EXCLUDED.quantity,
                updated_at = NOW()
        `, userID, p.Name, p.SKU, p.Price, p.Quantity, p.Unit)
        
        if err != nil {
            errors = append(errors, fmt.Sprintf("%s: %v", p.Name, err))
        } else {
            imported++
        }
    }
    
    // Обновляем лог
    status := "completed"
    errorMsg := ""
    if len(errors) > 0 {
        status = "partial"
        errorMsg = fmt.Sprintf("Импортировано %d из %d, ошибок: %d", imported, len(products.Products), len(errors))
    }
    
    database.Pool.Exec(c.Request.Context(), `
        UPDATE sync_logs SET status = $1, completed_at = NOW(), error_message = $2
        WHERE id = $3
    `, status, errorMsg, logID)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "imported":  imported,
        "total":     len(products.Products),
        "errors":    errors,
        "message":   fmt.Sprintf("Импортировано %d товаров", imported),
        "log_id":    logID,
    })
}

// ==================== ЖУРНАЛЫ СИНХРОНИЗАЦИИ ====================

// GetSyncLogs - получить логи синхронизации
func GetSyncLogs(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, direction, entity_type, record_count, status, error_message, file_path, started_at, completed_at
        FROM sync_logs
        WHERE user_id = $1
        ORDER BY started_at DESC
        LIMIT 50
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var logs []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var direction, entityType, status, errorMsg, filePath string
        var recordCount int
        var startedAt, completedAt time.Time
        var completedAtPtr *time.Time
        
        rows.Scan(&id, &direction, &entityType, &recordCount, &status, &errorMsg, &filePath, &startedAt, &completedAt)
        
        if !completedAt.IsZero() {
            completedAtPtr = &completedAt
        }
        
        logs = append(logs, map[string]interface{}{
            "id":           id,
            "direction":    direction,
            "entity_type":  entityType,
            "record_count": recordCount,
            "status":       status,
            "error_message": errorMsg,
            "file_path":    filePath,
            "started_at":   startedAt,
            "completed_at": completedAtPtr,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "logs":    logs,
    })
}

// GetSyncSettings - получить настройки интеграции
func GetSyncSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var settingsJSON []byte
    var lastSync time.Time
    var syncStatus string
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT settings, last_sync, sync_status
        FROM integration_settings
        WHERE user_id = $1 AND integration_type = '1c'
    `, userID).Scan(&settingsJSON, &lastSync, &syncStatus)
    
    if err != nil {
        // Настройки по умолчанию
        defaultSettings := map[string]interface{}{
            "auto_sync":          false,
            "sync_interval":      3600,
            "export_products":    true,
            "export_orders":      true,
            "import_products":    false,
            "last_sync_status":   "never",
        }
        c.JSON(http.StatusOK, gin.H{
            "success":   true,
            "settings":  defaultSettings,
            "last_sync": nil,
            "status":    "idle",
        })
        return
    }
    
    var settings map[string]interface{}
    json.Unmarshal(settingsJSON, &settings)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "settings":  settings,
        "last_sync": lastSync,
        "status":    syncStatus,
    })
}

// UpdateSyncSettings - обновить настройки интеграции
func UpdateSyncSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Settings map[string]interface{} `json:"settings" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    settingsJSON, _ := json.Marshal(req.Settings)
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO integration_settings (user_id, integration_type, settings, updated_at)
        VALUES ($1, '1c', $2, NOW())
        ON CONFLICT (user_id, integration_type) DO UPDATE SET
            settings = EXCLUDED.settings,
            updated_at = NOW()
    `, userID, settingsJSON)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить настройки"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Настройки сохранены",
    })
}