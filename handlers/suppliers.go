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

// ========== API ПОСТАВЩИКОВ ==========

// GetSuppliers возвращает список поставщиков
func GetSuppliers(c *gin.Context) {
    userID := getUserIDFromContext(c)

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, inn, kpp, ogrn, phone, email, address, contact_person, notes, active, created_at
        FROM suppliers 
        WHERE user_id = $1 AND active = true
        ORDER BY name
    `, userID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"suppliers": []interface{}{}})
        return
    }
    defer rows.Close()

    var suppliers []Supplier
    for rows.Next() {
        var s Supplier
        rows.Scan(&s.ID, &s.Name, &s.Inn, &s.Kpp, &s.Ogrn, &s.Phone, &s.Email,
            &s.Address, &s.ContactPerson, &s.Notes, &s.Active, &s.CreatedAt)
        suppliers = append(suppliers, s)
    }
    c.JSON(http.StatusOK, gin.H{"suppliers": suppliers})
}

// GetSupplier возвращает одного поставщика по ID
func GetSupplier(c *gin.Context) {
    userID := getUserIDFromContext(c)
    id := c.Param("id")

    var supplier Supplier
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, name, inn, kpp, ogrn, phone, email, address, contact_person, notes, active, created_at
        FROM suppliers 
        WHERE id = $1 AND user_id = $2 AND active = true
    `, id, userID).Scan(&supplier.ID, &supplier.Name, &supplier.Inn, &supplier.Kpp,
        &supplier.Ogrn, &supplier.Phone, &supplier.Email, &supplier.Address,
        &supplier.ContactPerson, &supplier.Notes, &supplier.Active, &supplier.CreatedAt)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Supplier not found"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"supplier": supplier})
}

// CreateSupplier создает поставщика
func CreateSupplier(c *gin.Context) {
    userID := getUserIDFromContext(c)

    var req Supplier
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO suppliers (user_id, name, inn, kpp, ogrn, phone, email, address, contact_person, notes, active, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, NOW())
        RETURNING id
    `, userID, req.Name, req.Inn, req.Kpp, req.Ogrn, req.Phone, req.Email,
        req.Address, req.ContactPerson, req.Notes).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "id": id})
}

// UpdateSupplier обновляет поставщика
func UpdateSupplier(c *gin.Context) {
    userID := getUserIDFromContext(c)
    id := c.Param("id")

    var req Supplier
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE suppliers 
        SET name = $1, inn = $2, kpp = $3, ogrn = $4, phone = $5, email = $6, 
            address = $7, contact_person = $8, notes = $9, active = $10
        WHERE id = $11 AND user_id = $12
    `, req.Name, req.Inn, req.Kpp, req.Ogrn, req.Phone, req.Email,
        req.Address, req.ContactPerson, req.Notes, req.Active, id, userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteSupplier удаляет поставщика
func DeleteSupplier(c *gin.Context) {
    userID := getUserIDFromContext(c)
    id := c.Param("id")

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE suppliers SET active = false WHERE id = $1 AND user_id = $2
    `, id, userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== API ЗАКУПОК ==========

// GetPurchaseOrders возвращает список заказов поставщикам
func GetPurchaseOrders(c *gin.Context) {
    userID := getUserIDFromContext(c)

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT po.id, po.order_number, po.supplier_id, s.name as supplier_name, 
               po.status, po.total_amount, po.order_date, po.expected_date, po.notes, po.created_at
        FROM purchase_orders po
        LEFT JOIN suppliers s ON po.supplier_id = s.id
        WHERE po.user_id = $1
        ORDER BY po.created_at DESC
    `, userID)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"purchase_orders": []interface{}{}})
        return
    }
    defer rows.Close()

    var orders []map[string]interface{}
    for rows.Next() {
        var id, supplierID uuid.UUID
        var orderNumber, status, supplierName, notes string
        var totalAmount float64
        var orderDate, createdAt time.Time
        var expectedDate sql.NullTime

        rows.Scan(&id, &orderNumber, &supplierID, &supplierName, &status, &totalAmount,
            &orderDate, &expectedDate, &notes, &createdAt)

        expDate := interface{}(nil)
        if expectedDate.Valid {
            expDate = expectedDate.Time
        }

        orders = append(orders, map[string]interface{}{
            "id":            id,
            "order_number":  orderNumber,
            "supplier_id":   supplierID,
            "supplier_name": supplierName,
            "status":        status,
            "total_amount":  totalAmount,
            "order_date":    orderDate,
            "expected_date": expDate,
            "notes":         notes,
            "created_at":    createdAt,
        })
    }
    c.JSON(http.StatusOK, gin.H{"purchase_orders": orders})
}

// GetPurchaseOrder возвращает детали заказа поставщику
func GetPurchaseOrder(c *gin.Context) {
    userID := getUserIDFromContext(c)
    id := c.Param("id")

    var order PurchaseOrder
    var expectedDate sql.NullTime

    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT po.id, po.order_number, po.supplier_id, s.name as supplier_name,
               po.status, po.total_amount, po.order_date, po.expected_date, po.notes, po.created_at
        FROM purchase_orders po
        LEFT JOIN suppliers s ON po.supplier_id = s.id
        WHERE po.id = $1 AND po.user_id = $2
    `, id, userID).Scan(&order.ID, &order.OrderNumber, &order.SupplierID, &order.SupplierName,
        &order.Status, &order.TotalAmount, &order.OrderDate, &expectedDate, &order.Notes, &order.CreatedAt)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
        return
    }

    if expectedDate.Valid {
        order.ExpectedDate = &expectedDate.Time
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT product_id, product_name, quantity, price, total
        FROM purchase_order_items
        WHERE purchase_order_id = $1
    `, id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var items []map[string]interface{}
    for rows.Next() {
        var productID uuid.UUID
        var productName string
        var quantity int
        var price, total float64
        rows.Scan(&productID, &productName, &quantity, &price, &total)
        items = append(items, map[string]interface{}{
            "product_id":   productID,
            "product_name": productName,
            "quantity":     quantity,
            "price":        price,
            "total":        total,
        })
    }

    c.JSON(http.StatusOK, gin.H{"order": order, "items": items})
}

// CreatePurchaseOrder создает заказ поставщику
func CreatePurchaseOrder(c *gin.Context) {
    userID := getUserIDFromContext(c)

    var req struct {
        SupplierID   uuid.UUID   `json:"supplier_id" binding:"required"`
        ExpectedDate *time.Time  `json:"expected_date"`
        Notes        string      `json:"notes"`
        Items        []struct {
            ProductID uuid.UUID `json:"product_id" binding:"required"`
            Quantity  int       `json:"quantity" binding:"required"`
            Price     float64   `json:"price" binding:"required"`
        } `json:"items" binding:"required"`
    }

    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    orderNumber := fmt.Sprintf("PO-%d", time.Now().UnixNano()%1000000)

    var totalAmount float64
    for _, item := range req.Items {
        totalAmount += item.Price * float64(item.Quantity)
    }

    tx, err := database.Pool.Begin(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer tx.Rollback(c.Request.Context())

    var orderID uuid.UUID
    err = tx.QueryRow(c.Request.Context(), `
        INSERT INTO purchase_orders (user_id, supplier_id, order_number, total_amount, 
                                     expected_date, notes, status, order_date, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, 'draft', NOW(), NOW())
        RETURNING id
    `, userID, req.SupplierID, orderNumber, totalAmount, req.ExpectedDate, req.Notes).Scan(&orderID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    for _, item := range req.Items {
        var productName string
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT name FROM inventory_products WHERE id = $1
        `, item.ProductID).Scan(&productName)

        _, err = tx.Exec(c.Request.Context(), `
            INSERT INTO purchase_order_items (purchase_order_id, product_id, product_name, 
                                              quantity, price, total)
            VALUES ($1, $2, $3, $4, $5, $6)
        `, orderID, item.ProductID, productName, item.Quantity, item.Price,
            item.Price*float64(item.Quantity))

        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
    }

    if err := tx.Commit(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "order_id": orderID, "order_number": orderNumber})
}

// UpdatePurchaseOrderStatus обновляет статус заказа
func UpdatePurchaseOrderStatus(c *gin.Context) {
    userID := getUserIDFromContext(c)
    id := c.Param("id")

    var req struct {
        Status string `json:"status" binding:"required"`
    }

    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE purchase_orders SET status = $1 WHERE id = $2 AND user_id = $3
    `, req.Status, id, userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeletePurchaseOrder удаляет заказ поставщику
func DeletePurchaseOrder(c *gin.Context) {
    userID := getUserIDFromContext(c)
    id := c.Param("id")

    tx, err := database.Pool.Begin(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer tx.Rollback(c.Request.Context())

    _, err = tx.Exec(c.Request.Context(), `DELETE FROM purchase_order_items WHERE purchase_order_id = $1`, id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    _, err = tx.Exec(c.Request.Context(), `DELETE FROM purchase_orders WHERE id = $1 AND user_id = $2`, id, userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    if err := tx.Commit(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}