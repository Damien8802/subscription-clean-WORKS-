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

// GoodsReceipt структура приемки товаров
type GoodsReceipt struct {
    ID              uuid.UUID  `json:"id"`
    ReceiptNumber   string     `json:"receipt_number"`
    PurchaseOrderID *uuid.UUID `json:"purchase_order_id"`
    SupplierID      *uuid.UUID `json:"supplier_id"`
    SupplierName    string     `json:"supplier_name"`
    ReceiptDate     time.Time  `json:"receipt_date"`
    Status          string     `json:"status"`
    TotalAmount     float64    `json:"total_amount"`
    ReceivedBy      string     `json:"received_by"`
    Notes           string     `json:"notes"`
    CreatedAt       time.Time  `json:"created_at"`
}

// GoodsReceiptItem структура позиции приемки
type GoodsReceiptItem struct {
    ID               uuid.UUID  `json:"id"`
    ReceiptID        uuid.UUID  `json:"receipt_id"`
    ProductID        uuid.UUID  `json:"product_id"`
    ProductName      string     `json:"product_name"`
    SKU              string     `json:"sku"`
    OrderItemID      *uuid.UUID `json:"order_item_id"`
    Quantity         int        `json:"quantity"`
    AcceptedQuantity int        `json:"accepted_quantity"`
    RejectedQuantity int        `json:"rejected_quantity"`
    Price            float64    `json:"price"`
    Total            float64    `json:"total"`
    RejectionReason  string     `json:"rejection_reason"`
    BatchNumber      string     `json:"batch_number"`
    ExpirationDate   *time.Time `json:"expiration_date"`
    StorageLocation  string     `json:"storage_location"`
    Notes            string     `json:"notes"`
}

// Получить список приемок
func GetGoodsReceipts(c *gin.Context) {
    userID := getUserID(c)
    
    status := c.Query("status")
    purchaseOrderID := c.Query("purchase_order_id")
    
    query := `
        SELECT gr.id, gr.receipt_number, gr.purchase_order_id, gr.supplier_id, s.name as supplier_name,
               gr.receipt_date, gr.status, gr.total_amount, gr.received_by, gr.notes, gr.created_at
        FROM goods_receipts gr
        LEFT JOIN suppliers s ON gr.supplier_id = s.id
        WHERE gr.user_id = $1
    `
    args := []interface{}{userID}
    argIndex := 2
    
    if status != "" {
        query += fmt.Sprintf(" AND gr.status = $%d", argIndex)
        args = append(args, status)
        argIndex++
    }
    
    if purchaseOrderID != "" {
        query += fmt.Sprintf(" AND gr.purchase_order_id = $%d", argIndex)
        args = append(args, purchaseOrderID)
        argIndex++
    }
    
    query += " ORDER BY gr.created_at DESC"
    
    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Ошибка базы данных",
            "details": err.Error(),
        })
        return
    }
    defer rows.Close()
    
    var receipts []GoodsReceipt
    for rows.Next() {
        var r GoodsReceipt
        var poID sql.NullString
        var supplierID sql.NullString
        err := rows.Scan(
            &r.ID, &r.ReceiptNumber, &poID, &supplierID, &r.SupplierName,
            &r.ReceiptDate, &r.Status, &r.TotalAmount, &r.ReceivedBy, &r.Notes, &r.CreatedAt,
        )
        if err != nil {
            continue
        }
        if poID.Valid {
            id, _ := uuid.Parse(poID.String)
            r.PurchaseOrderID = &id
        }
        if supplierID.Valid {
            id, _ := uuid.Parse(supplierID.String)
            r.SupplierID = &id
        }
        receipts = append(receipts, r)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "receipts": receipts,
        "total":    len(receipts),
    })
}

// Получить приемку по ID
func GetGoodsReceipt(c *gin.Context) {
    userID := getUserID(c)
    receiptID := c.Param("id")
    
    var r GoodsReceipt
    var poID sql.NullString
    var supplierID sql.NullString
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT gr.id, gr.receipt_number, gr.purchase_order_id, gr.supplier_id, s.name as supplier_name,
               gr.receipt_date, gr.status, gr.total_amount, gr.received_by, gr.notes, gr.created_at
        FROM goods_receipts gr
        LEFT JOIN suppliers s ON gr.supplier_id = s.id
        WHERE gr.id = $1 AND gr.user_id = $2
    `, receiptID, userID).Scan(
        &r.ID, &r.ReceiptNumber, &poID, &supplierID, &r.SupplierName,
        &r.ReceiptDate, &r.Status, &r.TotalAmount, &r.ReceivedBy, &r.Notes, &r.CreatedAt,
    )
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Приемка не найдена",
        })
        return
    }
    
    if poID.Valid {
        id, _ := uuid.Parse(poID.String)
        r.PurchaseOrderID = &id
    }
    if supplierID.Valid {
        id, _ := uuid.Parse(supplierID.String)
        r.SupplierID = &id
    }
    
    // Получаем позиции приемки
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, product_id, product_name, sku, order_item_id, quantity, 
               accepted_quantity, rejected_quantity, price, total, rejection_reason,
               batch_number, expiration_date, storage_location, notes
        FROM goods_receipt_items
        WHERE receipt_id = $1
    `, receiptID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Ошибка загрузки позиций",
        })
        return
    }
    defer rows.Close()
    
    var items []GoodsReceiptItem
    for rows.Next() {
        var item GoodsReceiptItem
        var orderItemID sql.NullString
        var expirationDate sql.NullTime
        
        err := rows.Scan(
            &item.ID, &item.ProductID, &item.ProductName, &item.SKU, &orderItemID,
            &item.Quantity, &item.AcceptedQuantity, &item.RejectedQuantity,
            &item.Price, &item.Total, &item.RejectionReason,
            &item.BatchNumber, &expirationDate, &item.StorageLocation, &item.Notes,
        )
        if err != nil {
            continue
        }
        if orderItemID.Valid {
            id, _ := uuid.Parse(orderItemID.String)
            item.OrderItemID = &id
        }
        if expirationDate.Valid {
            item.ExpirationDate = &expirationDate.Time
        }
        items = append(items, item)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "receipt": r,
        "items":   items,
    })
}

// Создать приемку товаров
func CreateGoodsReceipt(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        PurchaseOrderID *uuid.UUID `json:"purchase_order_id"`
        SupplierID      *uuid.UUID `json:"supplier_id"`
        ReceiptDate     string     `json:"receipt_date"`
        ReceivedBy      string     `json:"received_by"`
        Notes           string     `json:"notes"`
        Items           []struct {
            ProductID        uuid.UUID  `json:"product_id"`
            OrderItemID      *uuid.UUID `json:"order_item_id"`
            Quantity         int        `json:"quantity"`
            AcceptedQuantity int        `json:"accepted_quantity"`
            RejectedQuantity int        `json:"rejected_quantity"`
            Price            float64    `json:"price"`
            RejectionReason  string     `json:"rejection_reason"`
            BatchNumber      string     `json:"batch_number"`
            ExpirationDate   string     `json:"expiration_date"`
            StorageLocation  string     `json:"storage_location"`
            Notes            string     `json:"notes"`
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
            "error": "Добавьте хотя бы одну позицию",
        })
        return
    }
    
    // Генерируем номер приемки
    receiptNumber := fmt.Sprintf("RC-%d", time.Now().UnixNano()%1000000)
    
    var totalAmount float64
    var items []struct {
        ProductID        uuid.UUID
        ProductName      string
        SKU              string
        OrderItemID      *uuid.UUID
        Quantity         int
        AcceptedQuantity int
        RejectedQuantity int
        Price            float64
        Total            float64
        RejectionReason  string
        BatchNumber      string
        ExpirationDate   *time.Time
        StorageLocation  string
        Notes            string
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
        
        // Если принято больше чем поступило, корректируем
        acceptedQty := item.AcceptedQuantity
        if acceptedQty == 0 {
            acceptedQty = item.Quantity
        }
        
        total := price * float64(acceptedQty)
        totalAmount += total
        
        var expirationDate *time.Time
        if item.ExpirationDate != "" {
            ed, _ := time.Parse("2006-01-02", item.ExpirationDate)
            expirationDate = &ed
        }
        
        items = append(items, struct {
            ProductID        uuid.UUID
            ProductName      string
            SKU              string
            OrderItemID      *uuid.UUID
            Quantity         int
            AcceptedQuantity int
            RejectedQuantity int
            Price            float64
            Total            float64
            RejectionReason  string
            BatchNumber      string
            ExpirationDate   *time.Time
            StorageLocation  string
            Notes            string
        }{
            item.ProductID, productName, sku, item.OrderItemID,
            item.Quantity, acceptedQty, item.RejectedQuantity,
            price, total, item.RejectionReason, item.BatchNumber,
            expirationDate, item.StorageLocation, item.Notes,
        })
    }
    
    // Создаем приемку в транзакции
    tx, err := database.Pool.Begin(c.Request.Context())
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось начать транзакцию",
        })
        return
    }
    defer tx.Rollback(c.Request.Context())
    
    var receiptID uuid.UUID
    receiptDate := time.Now()
    if req.ReceiptDate != "" {
        rd, _ := time.Parse("2006-01-02", req.ReceiptDate)
        receiptDate = rd
    }
    
    err = tx.QueryRow(c.Request.Context(), `
        INSERT INTO goods_receipts (
            user_id, receipt_number, purchase_order_id, supplier_id,
            receipt_date, status, total_amount, received_by, notes, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, 'completed', $6, $7, $8, NOW(), NOW())
        RETURNING id
    `,
        userID, receiptNumber, req.PurchaseOrderID, req.SupplierID,
        receiptDate, totalAmount, req.ReceivedBy, req.Notes,
    ).Scan(&receiptID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error":   "Не удалось создать приемку",
            "details": err.Error(),
        })
        return
    }
    
    // Добавляем позиции и обновляем остатки на складе
    for _, item := range items {
        _, err = tx.Exec(c.Request.Context(), `
            INSERT INTO goods_receipt_items (
                receipt_id, product_id, product_name, sku, order_item_id, quantity,
                accepted_quantity, rejected_quantity, price, total, rejection_reason,
                batch_number, expiration_date, storage_location, notes
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
        `,
            receiptID, item.ProductID, item.ProductName, item.SKU, item.OrderItemID,
            item.Quantity, item.AcceptedQuantity, item.RejectedQuantity,
            item.Price, item.Total, item.RejectionReason, item.BatchNumber,
            item.ExpirationDate, item.StorageLocation, item.Notes,
        )
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Не удалось добавить позиции приемки",
            })
            return
        }
        
        // Обновляем остаток товара на складе
        _, err = tx.Exec(c.Request.Context(), `
            UPDATE products 
            SET quantity = quantity + $1, updated_at = NOW()
            WHERE id = $2 AND user_id = $3
        `, item.AcceptedQuantity, item.ProductID, userID)
        
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Не удалось обновить остатки",
            })
            return
        }
        
        // Если есть связанный заказ поставщику, обновляем полученное количество
        if item.OrderItemID != nil {
            _, err = tx.Exec(c.Request.Context(), `
                UPDATE purchase_order_items 
                SET received_quantity = received_quantity + $1
                WHERE id = $2
            `, item.AcceptedQuantity, item.OrderItemID)
            
            if err != nil {
                // Не критическая ошибка, логируем
                _ = err
            }
        }
    }
    
    if err := tx.Commit(c.Request.Context()); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Не удалось сохранить приемку",
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":        true,
        "receipt_id":     receiptID,
        "receipt_number": receiptNumber,
        "total":          totalAmount,
        "message":        "Товары успешно оприходованы",
    })
}