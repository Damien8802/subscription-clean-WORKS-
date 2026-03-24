package handlers

import (
    "encoding/csv"
    "fmt"
    "io"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// ensureUserExists - проверяет существование пользователя и создает при необходимости
func ensureUserExists(c *gin.Context, userID uuid.UUID) (uuid.UUID, error) {
    // Проверяем, что userID не нулевой
    if userID == uuid.Nil {
        // Если userID нулевой, создаем нового пользователя
        email := fmt.Sprintf("user_%d@example.com", time.Now().UnixNano())
        var newUserID uuid.UUID
        err := database.Pool.QueryRow(c.Request.Context(), `
            INSERT INTO users (email, password_hash, role, created_at, updated_at)
            VALUES ($1, $2, 'user', NOW(), NOW())
            RETURNING id
        `, email, "temporary_hash").Scan(&newUserID)
        
        if err != nil {
            return userID, fmt.Errorf("failed to create user: %v", err)
        }
        return newUserID, nil
    }
    
    var exists bool
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)
    `, userID).Scan(&exists)
    
    if err != nil {
        return userID, err
    }
    
    if !exists {
        // Создаем пользователя с указанным ID
        email := fmt.Sprintf("user_%d@example.com", time.Now().UnixNano())
        var newUserID uuid.UUID
        err = database.Pool.QueryRow(c.Request.Context(), `
            INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
            VALUES ($1, $2, $3, 'user', NOW(), NOW())
            ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email
            RETURNING id
        `, userID, email, "temporary_hash").Scan(&newUserID)
        
        if err != nil {
            return userID, fmt.Errorf("failed to create user with ID %s: %v", userID, err)
        }
        return newUserID, nil
    }
    
    return userID, nil
}

// Получить список товаров
func GetProducts(c *gin.Context) {
    userID := getUserID(c)
    
    category := c.Query("category")
    search := c.Query("search")
    
    query := `
        SELECT p.id, p.name, p.sku, p.barcode, p.price, p.cost, p.quantity, p.min_quantity, p.unit, p.category, p.description, p.active, p.created_at
        FROM products p
        WHERE p.user_id = $1 AND p.active = true
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if category != "" {
        query += fmt.Sprintf(" AND p.category = $%d", argIndex)
        args = append(args, category)
        argIndex++
    }
    
    if search != "" {
        query += fmt.Sprintf(" AND (p.name ILIKE $%d OR p.sku ILIKE $%d OR p.barcode ILIKE $%d)", argIndex, argIndex, argIndex)
        args = append(args, "%"+search+"%")
        argIndex++
    }
    
    query += " ORDER BY p.name"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var products []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var name, sku, barcode, unit, category, description string
        var price, cost float64
        var quantity, minQuantity int
        var active bool
        var createdAt time.Time
        
        rows.Scan(&id, &name, &sku, &barcode, &price, &cost, &quantity, &minQuantity, &unit, &category, &description, &active, &createdAt)
        
        products = append(products, map[string]interface{}{
            "id":           id,
            "name":         name,
            "sku":          sku,
            "barcode":      barcode,
            "price":        price,
            "cost":         cost,
            "quantity":     quantity,
            "min_quantity": minQuantity,
            "unit":         unit,
            "category":     category,
            "description":  description,
            "active":       active,
            "created_at":   createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"products": products})
}

// Создать товар - ИСПРАВЛЕНА (с проверкой user_id)
func CreateProduct(c *gin.Context) {
    userID := getUserID(c)
    
    // Проверяем и создаем пользователя при необходимости
    validUserID, err := ensureUserExists(c, userID)
    if err != nil {
        log.Printf("Error ensuring user exists: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Системная ошибка: пользователь не найден",
        })
        return
    }
    userID = validUserID
    
    var req struct {
        Name        string  `json:"name" binding:"required"`
        Sku         string  `json:"sku"`
        Barcode     string  `json:"barcode"`
        Price       float64 `json:"price" binding:"required"`
        Cost        float64 `json:"cost"`
        Quantity    int     `json:"quantity"`
        MinQuantity int     `json:"min_quantity"`
        Unit        string  `json:"unit"`
        Category    string  `json:"category"`
        Description string  `json:"description"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "Неверный формат данных",
            "details": err.Error(),
        })
        return
    }
    
    // Проверка обязательных полей
    if req.Name == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Название товара обязательно",
        })
        return
    }
    
    if req.Price <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Цена должна быть больше 0",
        })
        return
    }
    
    if req.Unit == "" {
        req.Unit = "шт"
    }
    
    // Проверяем, существует ли товар с таким SKU
    if req.Sku != "" {
        var exists bool
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT EXISTS(SELECT 1 FROM products WHERE user_id = $1 AND sku = $2 AND active = true)
        `, userID, req.Sku).Scan(&exists)
        
        if err == nil && exists {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Товар с таким артикулом уже существует",
            })
            return
        }
    }
    
    var productID uuid.UUID
    err = database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO products (user_id, name, sku, barcode, price, cost, quantity, min_quantity, unit, category, description, active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true, NOW(), NOW())
        RETURNING id
    `, userID, req.Name, req.Sku, req.Barcode, req.Price, req.Cost, req.Quantity, req.MinQuantity, req.Unit, req.Category, req.Description).Scan(&productID)
    
    if err != nil {
        log.Printf("Error creating product: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось создать товар",
            "details": err.Error(),
        })
        return
    }
    
    // Возвращаем полную информацию о созданном товаре
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "message":    "Товар успешно создан",
        "product_id": productID,
        "product": gin.H{
            "id":           productID,
            "name":         req.Name,
            "sku":          req.Sku,
            "barcode":      req.Barcode,
            "price":        req.Price,
            "cost":         req.Cost,
            "quantity":     req.Quantity,
            "min_quantity": req.MinQuantity,
            "unit":         req.Unit,
            "category":     req.Category,
            "description":  req.Description,
            "active":       true,
        },
    })
}

// Обновить товар
func UpdateProduct(c *gin.Context) {
    userID := getUserID(c)
    productID := c.Param("id")
    
    var req struct {
        Name        string  `json:"name"`
        Sku         string  `json:"sku"`
        Barcode     string  `json:"barcode"`
        Price       float64 `json:"price"`
        Cost        float64 `json:"cost"`
        Quantity    int     `json:"quantity"`
        MinQuantity int     `json:"min_quantity"`
        Unit        string  `json:"unit"`
        Category    string  `json:"category"`
        Description string  `json:"description"`
        Active      bool    `json:"active"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE products 
        SET name = $1, sku = $2, barcode = $3, price = $4, cost = $5, quantity = $6, min_quantity = $7, unit = $8, category = $9, description = $10, active = $11, updated_at = NOW()
        WHERE id = $12 AND user_id = $13
    `, req.Name, req.Sku, req.Barcode, req.Price, req.Cost, req.Quantity, req.MinQuantity, req.Unit, req.Category, req.Description, req.Active, productID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Товар обновлен"})
}

// Удалить товар
func DeleteProduct(c *gin.Context) {
    userID := getUserID(c)
    productID := c.Param("id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE products SET active = false, updated_at = NOW()
        WHERE id = $1 AND user_id = $2
    `, productID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Товар удален"})
}

// Получить список заказов
func GetOrders(c *gin.Context) {
    userID := getUserID(c)
    
    status := c.Query("status")
    
    query := `
        SELECT id, order_number, customer_name, customer_phone, customer_email, total_amount, status, payment_status, created_at
        FROM orders
        WHERE user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if status != "" {
        query += fmt.Sprintf(" AND status = $%d", argIndex)
        args = append(args, status)
        argIndex++
    }
    
    query += " ORDER BY created_at DESC"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var orders []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var orderNumber, customerName, customerPhone, customerEmail, status, paymentStatus string
        var totalAmount float64
        var createdAt time.Time
        
        rows.Scan(&id, &orderNumber, &customerName, &customerPhone, &customerEmail, &totalAmount, &status, &paymentStatus, &createdAt)
        
        orders = append(orders, map[string]interface{}{
            "id":              id,
            "order_number":    orderNumber,
            "customer_name":   customerName,
            "customer_phone":  customerPhone,
            "customer_email":  customerEmail,
            "total_amount":    totalAmount,
            "status":          status,
            "payment_status":  paymentStatus,
            "created_at":      createdAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"orders": orders})
}

// Создать заказ - ИСПРАВЛЕНА (с проверкой user_id и товаров)
func CreateOrder(c *gin.Context) {
    userID := getUserID(c)
    
    // Проверяем и создаем пользователя при необходимости
    validUserID, err := ensureUserExists(c, userID)
    if err != nil {
        log.Printf("Error ensuring user exists: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Системная ошибка: пользователь не найден",
        })
        return
    }
    userID = validUserID
    
    var req struct {
        CustomerName    string `json:"customer_name" binding:"required"`
        CustomerPhone   string `json:"customer_phone"`
        CustomerEmail   string `json:"customer_email"`
        DeliveryAddress string `json:"delivery_address"`
        Notes           string `json:"notes"`
        Items           []struct {
            ProductID string  `json:"product_id" binding:"required"`
            Quantity  int     `json:"quantity" binding:"required"`
            Price     float64 `json:"price"`
        } `json:"items" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "Неверный формат данных",
            "details": err.Error(),
        })
        return
    }
    
    // Проверка обязательных полей
    if req.CustomerName == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Имя клиента обязательно",
        })
        return
    }
    
    if len(req.Items) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Заказ должен содержать хотя бы один товар",
        })
        return
    }
    
    // Генерируем номер заказа
    orderNumber := fmt.Sprintf("ORD-%d", time.Now().UnixNano()%1000000)
    
    var totalAmount float64
    var items []struct {
        ProductID uuid.UUID
        Name      string
        Sku       string
        Quantity  int
        Price     float64
        Total     float64
    }
    
    // Проверяем все товары
    for _, item := range req.Items {
        // Проверяем, что quantity положительный
        if item.Quantity <= 0 {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Количество товара должно быть больше 0",
            })
            return
        }
        
        // Получаем информацию о товаре
        var productID uuid.UUID
        var name, sku string
        var currentPrice float64
        var currentQuantity int
        
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT id, name, sku, price, quantity FROM products 
            WHERE id = $1 AND user_id = $2 AND active = true
        `, item.ProductID, userID).Scan(&productID, &name, &sku, &currentPrice, &currentQuantity)
        
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Товар не найден: " + item.ProductID,
            })
            return
        }
        
        // Проверяем наличие на складе
        if currentQuantity < item.Quantity {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": fmt.Sprintf("Недостаточно товара на складе: %s (доступно: %d, заказано: %d)", name, currentQuantity, item.Quantity),
            })
            return
        }
        
        price := item.Price
        if price == 0 {
            price = currentPrice
        }
        
        total := price * float64(item.Quantity)
        totalAmount += total
        
        items = append(items, struct {
            ProductID uuid.UUID
            Name      string
            Sku       string
            Quantity  int
            Price     float64
            Total     float64
        }{productID, name, sku, item.Quantity, price, total})
    }
    
    // Создаем заказ в транзакции
    tx, err := database.Pool.Begin(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось начать транзакцию",
        })
        return
    }
    defer tx.Rollback(c.Request.Context())
    
    var orderID uuid.UUID
    err = tx.QueryRow(c.Request.Context(), `
        INSERT INTO orders (user_id, order_number, customer_name, customer_phone, customer_email, total_amount, delivery_address, notes, status, payment_status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', 'pending', NOW())
        RETURNING id
    `, userID, orderNumber, req.CustomerName, req.CustomerPhone, req.CustomerEmail, totalAmount, req.DeliveryAddress, req.Notes).Scan(&orderID)
    
    if err != nil {
        log.Printf("Error creating order: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось создать заказ",
            "details": err.Error(),
        })
        return
    }
    
    // Добавляем позиции и обновляем остатки
    for _, item := range items {
        _, err = tx.Exec(c.Request.Context(), `
            INSERT INTO order_items (order_id, product_id, product_name, sku, quantity, price, total)
            VALUES ($1, $2, $3, $4, $5, $6, $7)
        `, orderID, item.ProductID, item.Name, item.Sku, item.Quantity, item.Price, item.Total)
        
        if err != nil {
            log.Printf("Failed to add order item: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Не удалось добавить позиции заказа",
            })
            return
        }
        
        // Уменьшаем остаток
        _, err = tx.Exec(c.Request.Context(), `
            UPDATE products SET quantity = quantity - $1, updated_at = NOW()
            WHERE id = $2 AND user_id = $3
        `, item.Quantity, item.ProductID, userID)
        
        if err != nil {
            log.Printf("Failed to update product stock: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Не удалось обновить остатки",
            })
            return
        }
    }
    
    // Фиксируем транзакцию
    if err := tx.Commit(c.Request.Context()); err != nil {
        log.Printf("Failed to commit order transaction: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось завершить создание заказа",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "order_id":     orderID,
        "order_number": orderNumber,
        "total":        totalAmount,
        "message":      "Заказ успешно создан",
    })
}

// Получить детали заказа
func GetOrderDetails(c *gin.Context) {
    userID := getUserID(c)
    orderID := c.Param("id")
    
    var order struct {
        ID              uuid.UUID
        OrderNumber     string
        CustomerName    string
        CustomerPhone   string
        CustomerEmail   string
        TotalAmount     float64
        Status          string
        PaymentStatus   string
        DeliveryAddress string
        Notes           string
        CreatedAt       time.Time
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, order_number, customer_name, customer_phone, customer_email, total_amount, status, payment_status, delivery_address, notes, created_at
        FROM orders
        WHERE id = $1 AND user_id = $2
    `, orderID, userID).Scan(&order.ID, &order.OrderNumber, &order.CustomerName, &order.CustomerPhone, &order.CustomerEmail, &order.TotalAmount, &order.Status, &order.PaymentStatus, &order.DeliveryAddress, &order.Notes, &order.CreatedAt)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
        return
    }
    
    // Получаем позиции
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT product_name, sku, quantity, price, total
        FROM order_items
        WHERE order_id = $1
    `, orderID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get items"})
        return
    }
    defer rows.Close()
    
    var items []map[string]interface{}
    for rows.Next() {
        var name, sku string
        var quantity int
        var price, total float64
        
        rows.Scan(&name, &sku, &quantity, &price, &total)
        
        items = append(items, map[string]interface{}{
            "name":     name,
            "sku":      sku,
            "quantity": quantity,
            "price":    price,
            "total":    total,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "order": order,
        "items": items,
    })
}

// Получить статистику склада
func GetInventoryStats(c *gin.Context) {
    userID := getUserID(c)
    
    var stats struct {
        TotalProducts   int     `json:"total_products"`
        TotalValue      float64 `json:"total_value"`
        LowStockCount   int     `json:"low_stock_count"`
        OutOfStockCount int     `json:"out_of_stock_count"`
    }
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT 
            COUNT(*) as total_products,
            COALESCE(SUM(price * quantity), 0) as total_value,
            COUNT(CASE WHEN quantity <= min_quantity AND quantity > 0 THEN 1 END) as low_stock,
            COUNT(CASE WHEN quantity = 0 THEN 1 END) as out_of_stock
        FROM products
        WHERE user_id = $1 AND active = true
    `, userID).Scan(&stats.TotalProducts, &stats.TotalValue, &stats.LowStockCount, &stats.OutOfStockCount)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// Страница инвентаризации
func InventoryPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "inventory.html", gin.H{
        "title": "Складской учет | SaaSPro",
    })
}

// Обновить статус заказа
func UpdateOrderStatus(c *gin.Context) {
    userID := getUserID(c)
    orderID := c.Param("id")
    
    var req struct {
        Status string `json:"status" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    validStatuses := map[string]bool{
        "pending": true, "processing": true, "shipped": true, 
        "delivered": true, "cancelled": true,
    }
    
    if !validStatuses[req.Status] {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE orders 
        SET status = $1, updated_at = NOW()
        WHERE id = $2 AND user_id = $3
    `, req.Status, orderID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Статус заказа обновлен"})
}

// Получить отчет по продажам
func GetSalesReport(c *gin.Context) {
    userID := getUserID(c)
    
    period := c.Query("period")
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    var query string
    var args []interface{}
    
    if startDate != "" && endDate != "" {
        query = `
            SELECT 
                DATE(created_at) as date,
                COUNT(*) as orders_count,
                SUM(total_amount) as total_sales,
                AVG(total_amount) as avg_order
            FROM orders
            WHERE user_id = $1 AND created_at BETWEEN $2 AND $3 AND status != 'cancelled'
            GROUP BY DATE(created_at)
            ORDER BY date DESC
        `
        args = []interface{}{userID, startDate, endDate}
    } else {
        interval := "30 days"
        switch period {
        case "day":
            interval = "1 day"
        case "week":
            interval = "7 days"
        case "month":
            interval = "30 days"
        case "year":
            interval = "365 days"
        }
        
        query = `
            SELECT 
                DATE(created_at) as date,
                COUNT(*) as orders_count,
                SUM(total_amount) as total_sales,
                AVG(total_amount) as avg_order
            FROM orders
            WHERE user_id = $1 AND created_at > NOW() - $2::INTERVAL AND status != 'cancelled'
            GROUP BY DATE(created_at)
            ORDER BY date DESC
        `
        args = []interface{}{userID, interval}
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var report []map[string]interface{}
    for rows.Next() {
        var date time.Time
        var ordersCount int
        var totalSales, avgOrder float64
        
        rows.Scan(&date, &ordersCount, &totalSales, &avgOrder)
        
        report = append(report, map[string]interface{}{
            "date":         date.Format("2006-01-02"),
            "orders_count": ordersCount,
            "total_sales":  totalSales,
            "avg_order":    avgOrder,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"report": report})
}

// Получить топ товаров
func GetTopProducts(c *gin.Context) {
    userID := getUserID(c)
    
    limit := c.DefaultQuery("limit", "10")
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT 
            p.name,
            p.sku,
            SUM(oi.quantity) as total_sold,
            SUM(oi.total) as total_revenue
        FROM order_items oi
        JOIN orders o ON oi.order_id = o.id
        JOIN products p ON oi.product_id = p.id
        WHERE o.user_id = $1 AND o.status != 'cancelled'
        GROUP BY p.id, p.name, p.sku
        ORDER BY total_sold DESC
        LIMIT $2
    `, userID, limit)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    var products []map[string]interface{}
    for rows.Next() {
        var name, sku string
        var totalSold int
        var totalRevenue float64
        
        rows.Scan(&name, &sku, &totalSold, &totalRevenue)
        
        products = append(products, map[string]interface{}{
            "name":          name,
            "sku":           sku,
            "total_sold":    totalSold,
            "total_revenue": totalRevenue,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"top_products": products})
}

// Экспорт товаров в CSV
func ExportProductsCSV(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT name, sku, price, quantity, unit, category
        FROM products
        WHERE user_id = $1 AND active = true
        ORDER BY name
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()
    
    c.Header("Content-Type", "text/csv")
    c.Header("Content-Disposition", "attachment;filename=products.csv")
    
    writer := csv.NewWriter(c.Writer)
    writer.Write([]string{"Название", "Артикул", "Цена", "Количество", "Ед.изм", "Категория"})
    
    for rows.Next() {
        var name, sku, unit, category string
        var price float64
        var quantity int
        
        rows.Scan(&name, &sku, &price, &quantity, &unit, &category)
        
        writer.Write([]string{
            name, sku,
            fmt.Sprintf("%.2f", price),
            fmt.Sprintf("%d", quantity),
            unit, category,
        })
    }
    
    writer.Flush()
}

// Импорт товаров из CSV
func ImportProductsCSV(c *gin.Context) {
    userID := getUserID(c)
    
    // Проверяем и создаем пользователя при необходимости
    validUserID, err := ensureUserExists(c, userID)
    if err != nil {
        log.Printf("Error ensuring user exists: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Системная ошибка: пользователь не найден",
        })
        return
    }
    userID = validUserID
    
    file, _, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
        return
    }
    defer file.Close()
    
    reader := csv.NewReader(file)
    reader.FieldsPerRecord = -1
    
    // Пропускаем заголовок
    _, err = reader.Read()
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CSV format"})
        return
    }
    
    var imported int
    var errors []string
    
    for {
        record, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            errors = append(errors, err.Error())
            continue
        }
        
        if len(record) < 5 {
            errors = append(errors, "Invalid row: "+strings.Join(record, ","))
            continue
        }
        
        name := record[0]
        sku := record[1]
        price, _ := strconv.ParseFloat(record[2], 64)
        quantity, _ := strconv.Atoi(record[3])
        unit := record[4]
        category := ""
        if len(record) > 5 {
            category = record[5]
        }
        
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO products (user_id, name, sku, price, quantity, unit, category, active, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, true, NOW(), NOW())
            ON CONFLICT (user_id, sku) DO UPDATE SET
                name = EXCLUDED.name,
                price = EXCLUDED.price,
                quantity = EXCLUDED.quantity,
                unit = EXCLUDED.unit,
                category = EXCLUDED.category,
                updated_at = NOW()
        `, userID, name, sku, price, quantity, unit, category)
        
        if err != nil {
            errors = append(errors, fmt.Sprintf("Failed to import %s: %v", name, err))
        } else {
            imported++
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "imported": imported,
        "errors":   errors,
        "message":  fmt.Sprintf("Импортировано %d товаров", imported),
    })
}