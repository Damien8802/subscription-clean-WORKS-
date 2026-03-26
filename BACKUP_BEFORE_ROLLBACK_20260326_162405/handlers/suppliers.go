package handlers

import (
    "database/sql"
    "fmt"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// Supplier структура поставщика
type Supplier struct {
    ID            uuid.UUID `json:"id"`
    Name          string    `json:"name"`
    Inn           string    `json:"inn"`
    Kpp           string    `json:"kpp"`
    Ogrn          string    `json:"ogrn"`
    Phone         string    `json:"phone"`
    Email         string    `json:"email"`
    Address       string    `json:"address"`
    ContactPerson string    `json:"contact_person"`
    Notes         string    `json:"notes"`
    Active        bool      `json:"active"`
    CreatedAt     time.Time `json:"created_at"`
}

// PurchaseOrder структура заказа поставщику
type PurchaseOrder struct {
    ID           uuid.UUID  `json:"id"`
    OrderNumber  string     `json:"order_number"`
    SupplierID   uuid.UUID  `json:"supplier_id"`
    SupplierName string     `json:"supplier_name"`
    Status       string     `json:"status"`
    TotalAmount  float64    `json:"total_amount"`
    OrderDate    time.Time  `json:"order_date"`
    ExpectedDate *time.Time `json:"expected_date"`
    Notes        string     `json:"notes"`
    CreatedAt    time.Time  `json:"created_at"`
}

// PurchaseOrderItem структура позиции заказа
type PurchaseOrderItem struct {
    ID               uuid.UUID `json:"id"`
    OrderID          uuid.UUID `json:"order_id"`
    ProductID        uuid.UUID `json:"product_id"`
    ProductName      string    `json:"product_name"`
    SKU              string    `json:"sku"`
    Quantity         int       `json:"quantity"`
    Price            float64   `json:"price"`
    Total            float64   `json:"total"`
    ReceivedQuantity int       `json:"received_quantity"`
}

// ==================== ПОСТАВЩИКИ ====================

// GetSuppliers - получить список поставщиков
func GetSuppliers(c *gin.Context) {
    userID := getUserID(c)
    
    search := c.Query("search")
    active := c.DefaultQuery("active", "true")
    
    query := `
        SELECT id, name, inn, kpp, ogrn, phone, email, address, 
               contact_person, notes, active, created_at
        FROM suppliers
        WHERE user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if active == "true" {
        query += " AND active = true"
    } else if active == "false" {
        query += " AND active = false"
    }
    
    if search != "" {
        query += fmt.Sprintf(" AND (name ILIKE $%d OR inn ILIKE $%d OR phone ILIKE $%d)", argIndex, argIndex, argIndex)
        args = append(args, "%"+search+"%", "%"+search+"%", "%"+search+"%")
        argIndex += 3
    }
    
    query += " ORDER BY name"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Ошибка базы данных",
            "details": err.Error(),
        })
        return
    }
    defer rows.Close()
    
    var suppliers []Supplier
    for rows.Next() {
        var s Supplier
        err := rows.Scan(
            &s.ID, &s.Name, &s.Inn, &s.Kpp, &s.Ogrn,
            &s.Phone, &s.Email, &s.Address, &s.ContactPerson,
            &s.Notes, &s.Active, &s.CreatedAt,
        )
        if err != nil {
            continue
        }
        suppliers = append(suppliers, s)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "suppliers": suppliers,
        "total":     len(suppliers),
    })
}

// GetSupplier - получить поставщика по ID
func GetSupplier(c *gin.Context) {
    userID := getUserID(c)
    supplierID := c.Param("id")
    
    var s Supplier
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, name, inn, kpp, ogrn, phone, email, address, 
               contact_person, notes, active, created_at
        FROM suppliers
        WHERE id = $1 AND user_id = $2
    `, supplierID, userID).Scan(
        &s.ID, &s.Name, &s.Inn, &s.Kpp, &s.Ogrn,
        &s.Phone, &s.Email, &s.Address, &s.ContactPerson,
        &s.Notes, &s.Active, &s.CreatedAt,
    )
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Поставщик не найден",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "supplier": s,
    })
}

// CreateSupplier - создать поставщика
func CreateSupplier(c *gin.Context) {
    userID := getUserID(c)
    
    var req Supplier
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "Неверный формат данных",
            "details": err.Error(),
        })
        return
    }
    
    if req.Name == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Название поставщика обязательно",
        })
        return
    }
    
    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO suppliers (
            user_id, name, inn, kpp, ogrn, phone, email, address,
            contact_person, notes, active, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, NOW(), NOW())
        RETURNING id
    `,
        userID, req.Name, req.Inn, req.Kpp, req.Ogrn,
        req.Phone, req.Email, req.Address, req.ContactPerson, req.Notes,
    ).Scan(&id)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось создать поставщика",
            "details": err.Error(),
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "id":      id,
        "message": "Поставщик успешно создан",
    })
}

// UpdateSupplier - обновить поставщика
func UpdateSupplier(c *gin.Context) {
    userID := getUserID(c)
    supplierID := c.Param("id")
    
    var req Supplier
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "Неверный формат данных",
            "details": err.Error(),
        })
        return
    }
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE suppliers SET
            name = $1, inn = $2, kpp = $3, ogrn = $4,
            phone = $5, email = $6, address = $7,
            contact_person = $8, notes = $9, active = $10,
            updated_at = NOW()
        WHERE id = $11 AND user_id = $12
    `,
        req.Name, req.Inn, req.Kpp, req.Ogrn,
        req.Phone, req.Email, req.Address,
        req.ContactPerson, req.Notes, req.Active,
        supplierID, userID,
    )
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось обновить поставщика",
            "details": err.Error(),
        })
        return
    }
    
    rowsAffected := result.RowsAffected()
    if rowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Поставщик не найден",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Поставщик обновлен",
    })
}

// DeleteSupplier - удалить поставщика
func DeleteSupplier(c *gin.Context) {
    userID := getUserID(c)
    supplierID := c.Param("id")
    
    // Проверяем, есть ли у поставщика заказы
    var orderCount int
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM purchase_orders 
        WHERE supplier_id = $1 AND user_id = $2
    `, supplierID, userID).Scan(&orderCount)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Ошибка проверки заказов",
        })
        return
    }
    
    if orderCount > 0 {
        // Если есть заказы, просто деактивируем
        _, err = database.Pool.Exec(c.Request.Context(), `
            UPDATE suppliers SET active = false, updated_at = NOW()
            WHERE id = $1 AND user_id = $2
        `, supplierID, userID)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Не удалось деактивировать поставщика",
            })
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "message": "Поставщик деактивирован (есть связанные заказы)",
        })
        return
    }
    
    // Если заказов нет, удаляем
    result, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM suppliers WHERE id = $1 AND user_id = $2
    `, supplierID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось удалить поставщика",
        })
        return
    }
    
    rowsAffected := result.RowsAffected()
    if rowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Поставщик не найден",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Поставщик удален",
    })
}

// ==================== ЗАКАЗЫ ПОСТАВЩИКАМ ====================

// GetPurchaseOrders - получить список заказов поставщикам
func GetPurchaseOrders(c *gin.Context) {
    userID := getUserID(c)
    
    status := c.Query("status")
    supplierID := c.Query("supplier_id")
    
    query := `
        SELECT po.id, po.order_number, po.supplier_id, s.name as supplier_name,
               po.status, po.total_amount, po.order_date, po.expected_date,
               po.notes, po.created_at
        FROM purchase_orders po
        LEFT JOIN suppliers s ON po.supplier_id = s.id
        WHERE po.user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if status != "" {
        query += fmt.Sprintf(" AND po.status = $%d", argIndex)
        args = append(args, status)
        argIndex++
    }
    
    if supplierID != "" {
        query += fmt.Sprintf(" AND po.supplier_id = $%d", argIndex)
        args = append(args, supplierID)
        argIndex++
    }
    
    query += " ORDER BY po.created_at DESC"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Ошибка базы данных",
            "details": err.Error(),
        })
        return
    }
    defer rows.Close()
    
    var orders []PurchaseOrder
    for rows.Next() {
        var o PurchaseOrder
        var expectedDate sql.NullTime
        err := rows.Scan(
            &o.ID, &o.OrderNumber, &o.SupplierID, &o.SupplierName,
            &o.Status, &o.TotalAmount, &o.OrderDate, &expectedDate,
            &o.Notes, &o.CreatedAt,
        )
        if err != nil {
            continue
        }
        if expectedDate.Valid {
            o.ExpectedDate = &expectedDate.Time
        }
        orders = append(orders, o)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "orders":  orders,
        "total":   len(orders),
    })
}

// GetPurchaseOrder - получить заказ по ID
func GetPurchaseOrder(c *gin.Context) {
    userID := getUserID(c)
    orderID := c.Param("id")
    
    var o PurchaseOrder
    var expectedDate sql.NullTime
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT po.id, po.order_number, po.supplier_id, s.name as supplier_name,
               po.status, po.total_amount, po.order_date, po.expected_date,
               po.notes, po.created_at
        FROM purchase_orders po
        LEFT JOIN suppliers s ON po.supplier_id = s.id
        WHERE po.id = $1 AND po.user_id = $2
    `, orderID, userID).Scan(
        &o.ID, &o.OrderNumber, &o.SupplierID, &o.SupplierName,
        &o.Status, &o.TotalAmount, &o.OrderDate, &expectedDate,
        &o.Notes, &o.CreatedAt,
    )
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Заказ не найден",
        })
        return
    }
    
    if expectedDate.Valid {
        o.ExpectedDate = &expectedDate.Time
    }
    
    // Получаем позиции заказа
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, product_id, product_name, sku, quantity, price, total, received_quantity
        FROM purchase_order_items
        WHERE order_id = $1
    `, orderID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Ошибка загрузки позиций",
        })
        return
    }
    defer rows.Close()
    
    var items []PurchaseOrderItem
    for rows.Next() {
        var item PurchaseOrderItem
        err := rows.Scan(
            &item.ID, &item.ProductID, &item.ProductName, &item.SKU,
            &item.Quantity, &item.Price, &item.Total, &item.ReceivedQuantity,
        )
        if err != nil {
            continue
        }
        items = append(items, item)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "order":   o,
        "items":   items,
    })
}

// CreatePurchaseOrder - создать заказ поставщику
func CreatePurchaseOrder(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        SupplierID   uuid.UUID `json:"supplier_id" binding:"required"`
        ExpectedDate string    `json:"expected_date"`
        Notes        string    `json:"notes"`
        Items        []struct {
            ProductID uuid.UUID `json:"product_id" binding:"required"`
            Quantity  int       `json:"quantity" binding:"required"`
            Price     float64   `json:"price"`
        } `json:"items" binding:"required"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "Неверный формат данных",
            "details": err.Error(),
        })
        return
    }
    
    if len(req.Items) == 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Заказ должен содержать хотя бы одну позицию",
        })
        return
    }
    
    // Проверяем существование поставщика
    var supplierExists bool
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT EXISTS(SELECT 1 FROM suppliers WHERE id = $1 AND user_id = $2 AND active = true)
    `, req.SupplierID, userID).Scan(&supplierExists)
    
    if err != nil || !supplierExists {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Поставщик не найден или неактивен",
        })
        return
    }
    
    // Генерируем номер заказа
    orderNumber := fmt.Sprintf("PO-%d", time.Now().UnixNano()%1000000)
    
    var totalAmount float64
    var items []struct {
        ProductID   uuid.UUID
        ProductName string
        SKU         string
        Quantity    int
        Price       float64
        Total       float64
    }
    
    // Получаем информацию о товарах
    for _, item := range req.Items {
        var productName string
        var sku string
        var currentPrice float64
        
        err := database.Pool.QueryRow(c.Request.Context(), `
            SELECT name, COALESCE(sku, ''), price FROM products 
            WHERE id = $1 AND user_id = $2
        `, item.ProductID, userID).Scan(&productName, &sku, &currentPrice)
        
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{
                "error": "Товар не найден: " + item.ProductID.String(),
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
            ProductID   uuid.UUID
            ProductName string
            SKU         string
            Quantity    int
            Price       float64
            Total       float64
        }{item.ProductID, productName, sku, item.Quantity, price, total})
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
    var expectedDate *time.Time
    if req.ExpectedDate != "" {
        ed, _ := time.Parse("2006-01-02", req.ExpectedDate)
        expectedDate = &ed
    }
    
    err = tx.QueryRow(c.Request.Context(), `
        INSERT INTO purchase_orders (
            user_id, order_number, supplier_id, total_amount, 
            expected_date, notes, status, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, 'draft', NOW(), NOW())
        RETURNING id
    `, userID, orderNumber, req.SupplierID, totalAmount, expectedDate, req.Notes).Scan(&orderID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось создать заказ",
            "details": err.Error(),
        })
        return
    }
    
    // Добавляем позиции
    for _, item := range items {
        _, err = tx.Exec(c.Request.Context(), `
            INSERT INTO purchase_order_items (
                order_id, product_id, product_name, sku, quantity, price, total
            ) VALUES ($1, $2, $3, $4, $5, $6, $7)
        `, orderID, item.ProductID, item.ProductName, item.SKU, item.Quantity, item.Price, item.Total)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Не удалось добавить позиции заказа",
            })
            return
        }
    }
    
    if err := tx.Commit(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось сохранить заказ",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":      true,
        "order_id":     orderID,
        "order_number": orderNumber,
        "total":        totalAmount,
        "message":      "Заказ поставщику успешно создан",
    })
}

// UpdatePurchaseOrderStatus - обновить статус заказа
func UpdatePurchaseOrderStatus(c *gin.Context) {
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
        "draft": true, "confirmed": true, "shipped": true, 
        "received": true, "cancelled": true,
    }
    
    if !validStatuses[req.Status] {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Недопустимый статус",
        })
        return
    }
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE purchase_orders 
        SET status = $1, updated_at = NOW()
        WHERE id = $2 AND user_id = $3
    `, req.Status, orderID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось обновить статус",
        })
        return
    }
    
    rowsAffected := result.RowsAffected()
    if rowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Заказ не найден",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Статус заказа обновлен",
    })
}

// DeletePurchaseOrder - удалить заказ
func DeletePurchaseOrder(c *gin.Context) {
    userID := getUserID(c)
    orderID := c.Param("id")
    
    // Проверяем статус заказа
    var status string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT status FROM purchase_orders 
        WHERE id = $1 AND user_id = $2
    `, orderID, userID).Scan(&status)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Заказ не найден",
        })
        return
    }
    
    if status != "draft" && status != "cancelled" {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Можно удалить только черновики или отмененные заказы",
        })
        return
    }
    
    result, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM purchase_orders WHERE id = $1 AND user_id = $2
    `, orderID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось удалить заказ",
        })
        return
    }
    
    rowsAffected := result.RowsAffected()
    if rowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Заказ не найден",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Заказ удален",
    })
}