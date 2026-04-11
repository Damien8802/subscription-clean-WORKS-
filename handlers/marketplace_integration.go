package handlers

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// ConnectMarketplace - подключение маркетплейса
func ConnectMarketplace(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Marketplace   string `json:"marketplace" binding:"required"`
        APIKey        string `json:"api_key"`
        ClientID      string `json:"client_id"`
        ClientSecret  string `json:"client_secret"`
        SellerID      string `json:"seller_id"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    integrationID := uuid.New()
    settings := map[string]string{
        "api_key":       req.APIKey,
        "client_id":     req.ClientID,
        "client_secret": req.ClientSecret,
        "seller_id":     req.SellerID,
    }
    settingsJSON, _ := json.Marshal(settings)

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO marketplace_integrations (id, company_id, marketplace, api_key, client_id, client_secret, seller_id, settings, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
    `, integrationID, companyID, req.Marketplace, req.APIKey, req.ClientID, req.ClientSecret, req.SellerID, settingsJSON)

    if err != nil {
        log.Printf("❌ Ошибка подключения маркетплейса: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect marketplace"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message":        "Маркетплейс подключён",
        "integration_id": integrationID,
    })
}

// SyncMarketplaceOrders - синхронизация заказов
func SyncMarketplaceOrders(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    integrationID := c.Param("id")

    var marketplace, apiKey, clientID, clientSecret, sellerID string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT marketplace, api_key, client_id, client_secret, seller_id
        FROM marketplace_integrations
        WHERE id = $1 AND company_id = $2 AND is_active = true
    `, integrationID, companyID).Scan(&marketplace, &apiKey, &clientID, &clientSecret, &sellerID)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Integration not found"})
        return
    }

    orders := simulateMarketplaceOrders(marketplace)

    var importedCount int
    for _, order := range orders {
        itemsJSON, _ := json.Marshal(order.Items)
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO marketplace_orders (id, company_id, marketplace, order_id, order_date, 
                customer_name, customer_phone, customer_email, total_amount, status, items, delivery_address, imported_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
            ON CONFLICT (order_id) DO UPDATE SET status = $10
        `, uuid.New(), companyID, marketplace, order.OrderID, order.OrderDate,
            order.CustomerName, order.CustomerPhone, order.CustomerEmail,
            order.TotalAmount, order.Status, itemsJSON, order.DeliveryAddress)

        if err == nil {
            importedCount++
        }
    }

    database.Pool.Exec(c.Request.Context(), `
        UPDATE marketplace_integrations SET last_sync = NOW(), updated_at = NOW()
        WHERE id = $1
    `, integrationID)

    c.JSON(http.StatusOK, gin.H{
        "message":        fmt.Sprintf("Синхронизировано %d заказов", importedCount),
        "imported_count": importedCount,
    })
}

// GetMarketplaceOrders - список заказов
func GetMarketplaceOrders(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, marketplace, order_id, order_date, customer_name, customer_phone, 
               customer_email, total_amount, status, items, imported_at
        FROM marketplace_orders
        WHERE company_id = $1
        ORDER BY order_date DESC
        LIMIT 100
    `, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load orders"})
        return
    }
    defer rows.Close()

    var orders []gin.H
    for rows.Next() {
        var id uuid.UUID
        var marketplace, orderID, customerName, customerPhone, customerEmail, status string
        var orderDate, importedAt time.Time
        var totalAmount float64
        var itemsJSON []byte

        rows.Scan(&id, &marketplace, &orderID, &orderDate, &customerName, &customerPhone,
            &customerEmail, &totalAmount, &status, &itemsJSON, &importedAt)

        var items interface{}
        json.Unmarshal(itemsJSON, &items)

        orders = append(orders, gin.H{
            "id":             id,
            "marketplace":    marketplace,
            "order_id":       orderID,
            "order_date":     orderDate.Format("2006-01-02 15:04"),
            "customer_name":  customerName,
            "customer_phone": customerPhone,
            "customer_email": customerEmail,
            "total_amount":   totalAmount,
            "status":         status,
            "items":          items,
        })
    }

    c.JSON(http.StatusOK, gin.H{"orders": orders, "total": len(orders)})
}

// GetMarketplaceIntegrationsList - список интеграций
func GetMarketplaceIntegrationsList(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, marketplace, is_active, last_sync, created_at
        FROM marketplace_integrations
        WHERE company_id = $1
        ORDER BY created_at DESC
    `, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load integrations"})
        return
    }
    defer rows.Close()

    var integrations []gin.H
    for rows.Next() {
        var id uuid.UUID
        var marketplace string
        var isActive bool
        var lastSync, createdAt *time.Time

        rows.Scan(&id, &marketplace, &isActive, &lastSync, &createdAt)

        integrations = append(integrations, gin.H{
            "id":          id,
            "marketplace": marketplace,
            "is_active":   isActive,
            "last_sync":   lastSync,
            "created_at":  createdAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{"integrations": integrations})
}

// UpdateMarketplaceStock - обновление остатков
func UpdateMarketplaceStock(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Marketplace string `json:"marketplace" binding:"required"`
        Products    []struct {
            ProductID string `json:"product_id"`
            SKU       string `json:"sku"`
            Quantity  int    `json:"quantity"`
            Price     float64 `json:"price"`
        } `json:"products" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var updatedCount int
    for _, p := range req.Products {
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO marketplace_stocks (id, company_id, marketplace, product_id, sku, quantity, price, last_sync)
            VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
            ON CONFLICT (company_id, marketplace, product_id) DO UPDATE
            SET quantity = $6, price = $7, last_sync = NOW()
        `, uuid.New(), companyID, req.Marketplace, p.ProductID, p.SKU, p.Quantity, p.Price)

        if err == nil {
            updatedCount++
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "message":       fmt.Sprintf("Обновлено %d позиций", updatedCount),
        "updated_count": updatedCount,
    })
}

// Типы для имитации
type simMarketplaceOrder struct {
    OrderID         string
    OrderDate       time.Time
    CustomerName    string
    CustomerPhone   string
    CustomerEmail   string
    TotalAmount     float64
    Status          string
    DeliveryAddress string
    Items           []simOrderItem
}

type simOrderItem struct {
    Name     string
    Quantity int
    Price    float64
}

func simulateMarketplaceOrders(marketplace string) []simMarketplaceOrder {
    return []simMarketplaceOrder{
        {
            OrderID:         fmt.Sprintf("%s-001", marketplace[:3]),
            OrderDate:       time.Now().AddDate(0, 0, -1),
            CustomerName:    "Иван Петров",
            CustomerPhone:   "+7 (999) 123-45-67",
            CustomerEmail:   "ivan@example.com",
            TotalAmount:     4990.00,
            Status:          "new",
            DeliveryAddress: "г. Москва, ул. Тверская, д. 1",
            Items:           []simOrderItem{{Name: "Товар 1", Quantity: 2, Price: 1990.00}, {Name: "Товар 2", Quantity: 1, Price: 1010.00}},
        },
        {
            OrderID:         fmt.Sprintf("%s-002", marketplace[:3]),
            OrderDate:       time.Now().AddDate(0, 0, -2),
            CustomerName:    "Мария Сидорова",
            CustomerPhone:   "+7 (999) 987-65-43",
            CustomerEmail:   "maria@example.com",
            TotalAmount:     2990.00,
            Status:          "processing",
            DeliveryAddress: "г. Санкт-Петербург, Невский пр., д. 10",
            Items:           []simOrderItem{{Name: "Товар 3", Quantity: 1, Price: 2990.00}},
        },
    }
}

// GetMarketplaceProducts - получение товаров из маркетплейса
func GetMarketplaceProducts(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    integrationID := c.Param("id")
    
    // Получаем настройки интеграции
    var marketplace, apiKey, clientID, clientSecret, sellerID string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT marketplace, api_key, client_id, client_secret, seller_id
        FROM marketplace_integrations
        WHERE id = $1 AND company_id = $2 AND is_active = true
    `, integrationID, companyID).Scan(&marketplace, &apiKey, &clientID, &clientSecret, &sellerID)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Integration not found"})
        return
    }

    // Имитация получения товаров из API маркетплейса
    products := simulateMarketplaceProducts(marketplace)

    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "products": products,
        "count":    len(products),
    })
}

// UpdateMarketplacePrices - обновление цен в маркетплейсе
func UpdateMarketplacePrices(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        IntegrationID string `json:"integration_id" binding:"required"`
        Products      []struct {
            ProductID string  `json:"product_id"`
            Price     float64 `json:"price"`
            OldPrice  float64 `json:"old_price"`
        } `json:"products" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем настройки интеграции
    var marketplace, apiKey string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT marketplace, api_key FROM marketplace_integrations
        WHERE id = $1 AND company_id = $2 AND is_active = true
    `, req.IntegrationID, companyID).Scan(&marketplace, &apiKey)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Integration not found"})
        return
    }

    // Обновляем цены (имитация)
    updatedCount := len(req.Products)

    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "message":       fmt.Sprintf("Обновлено %d цен в %s", updatedCount, marketplace),
        "updated_count": updatedCount,
    })
}

// GetMarketplaceAnalytics - аналитика продаж из маркетплейса
func GetMarketplaceAnalytics(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    integrationID := c.Param("id")
    period := c.DefaultQuery("period", "month") // week, month, year

    // Получаем настройки
    var marketplace string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT marketplace FROM marketplace_integrations
        WHERE id = $1 AND company_id = $2 AND is_active = true
    `, integrationID, companyID).Scan(&marketplace)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Integration not found"})
        return
    }

    // Имитация аналитики
    analytics := gin.H{
        "marketplace": marketplace,
        "period":      period,
        "orders_count": 156,
        "revenue":      1250000.00,
        "avg_order":    8012.82,
        "top_products": []gin.H{
            {"name": "Товар 1", "sales": 45, "revenue": 225000},
            {"name": "Товар 2", "sales": 32, "revenue": 160000},
            {"name": "Товар 3", "sales": 28, "revenue": 140000},
        },
        "daily_stats": []gin.H{
            {"date": "2026-04-01", "orders": 5, "revenue": 45000},
            {"date": "2026-04-02", "orders": 7, "revenue": 63000},
            {"date": "2026-04-03", "orders": 4, "revenue": 36000},
        },
    }

    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "analytics":  analytics,
    })
}

// DisconnectMarketplace - отключение маркетплейса
func DisconnectMarketplace(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    integrationID := c.Param("id")

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE marketplace_integrations 
        SET is_active = false, updated_at = NOW()
        WHERE id = $1 AND company_id = $2
    `, integrationID, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Маркетплейс отключён",
    })
}

// simulateMarketplaceProducts - имитация товаров из маркетплейса
func simulateMarketplaceProducts(marketplace string) []gin.H {
    return []gin.H{
        {"id": "1001", "name": "Смартфон X100", "sku": "PHONE-001", "price": 29990, "stock": 45, "category": "Электроника"},
        {"id": "1002", "name": "Наушники Pro", "sku": "AUDIO-002", "price": 5990, "stock": 120, "category": "Аудио"},
        {"id": "1003", "name": "Чехол для смартфона", "sku": "CASE-003", "price": 990, "stock": 200, "category": "Аксессуары"},
        {"id": "1004", "name": "Зарядное устройство", "sku": "POWER-004", "price": 1990, "stock": 85, "category": "Электроника"},
    }
}
