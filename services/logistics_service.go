package services

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    "github.com/google/uuid"
    "subscription-system/database"
    "subscription-system/models"
)

type LogisticsService struct{}

func NewLogisticsService() *LogisticsService {
    return &LogisticsService{}
}

// CreateOrder - создание заказа
func (s *LogisticsService) CreateOrder(ctx context.Context, order *models.LogisticsOrder) (*models.LogisticsOrder, error) {
    if order.ID == uuid.Nil {
        order.ID = uuid.New()
    }
    if order.OrderNumber == "" {
        order.OrderNumber = fmt.Sprintf("ORD-%d", time.Now().UnixNano())
    }
    if order.Status == "" {
        order.Status = "pending"
    }
    order.CreatedAt = time.Now()
    order.UpdatedAt = time.Now()

    query := `
        INSERT INTO logistics_orders (
            id, order_number, tracking_number, status, customer_name, 
            customer_phone, customer_address, weight, price, 
            created_at, updated_at, tenant_id, user_id,
            processing, shipped_at, delivered_at, courier_name, notes
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
        RETURNING id, order_number, tracking_number, status, customer_name,
                  customer_phone, customer_address, weight, price,
                  created_at, updated_at, tenant_id, user_id,
                  processing, shipped_at, delivered_at, courier_name, notes
    `

    err := database.Pool.QueryRow(ctx, query,
        order.ID, order.OrderNumber, order.TrackingNumber, order.Status, order.CustomerName,
        order.CustomerPhone, order.CustomerAddress, order.Weight, order.Price,
        order.CreatedAt, order.UpdatedAt, order.TenantID, order.UserID,
        order.Processing, order.ShippedAt, order.DeliveredAt, order.CourierName, order.Notes,
    ).Scan(
        &order.ID, &order.OrderNumber, &order.TrackingNumber, &order.Status, &order.CustomerName,
        &order.CustomerPhone, &order.CustomerAddress, &order.Weight, &order.Price,
        &order.CreatedAt, &order.UpdatedAt, &order.TenantID, &order.UserID,
        &order.Processing, &order.ShippedAt, &order.DeliveredAt, &order.CourierName, &order.Notes,
    )

    return order, err
}

// GetOrders - получение списка заказов
func (s *LogisticsService) GetOrders(ctx context.Context, limit, offset int) ([]models.LogisticsOrder, int, error) {
    var total int
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders").Scan(&total)

    rows, err := database.Pool.Query(ctx, `
        SELECT id, order_number, tracking_number, status, customer_name,
               customer_phone, customer_address, weight, price,
               created_at, updated_at, tenant_id, user_id,
               processing, shipped_at, delivered_at, courier_name, notes
        FROM logistics_orders
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var orders []models.LogisticsOrder
    for rows.Next() {
        var o models.LogisticsOrder
        var trackingNumber, customerPhone, courierName, notes sql.NullString
        var weight, price sql.NullFloat64
        var shippedAt, deliveredAt sql.NullTime
        var tenantID, userID sql.NullString
        var processing bool

        err := rows.Scan(
            &o.ID, &o.OrderNumber, &trackingNumber, &o.Status, &o.CustomerName,
            &customerPhone, &o.CustomerAddress, &weight, &price,
            &o.CreatedAt, &o.UpdatedAt, &tenantID, &userID,
            &processing, &shippedAt, &deliveredAt, &courierName, &notes,
        )
        if err != nil {
            continue
        }

        if trackingNumber.Valid {
            o.TrackingNumber = &trackingNumber.String
        }
        if customerPhone.Valid {
            o.CustomerPhone = &customerPhone.String
        }
        if weight.Valid {
            o.Weight = &weight.Float64
        }
        if price.Valid {
            o.Price = &price.Float64
        }
        if courierName.Valid {
            o.CourierName = &courierName.String
        }
        if notes.Valid {
            o.Notes = &notes.String
        }
        if shippedAt.Valid {
            o.ShippedAt = &shippedAt.Time
        }
        if deliveredAt.Valid {
            o.DeliveredAt = &deliveredAt.Time
        }
        o.Processing = processing

        orders = append(orders, o)
    }

    return orders, total, nil
}

// UpdateStatus - обновление статуса заказа
func (s *LogisticsService) UpdateStatus(ctx context.Context, id string, status string) error {
    orderID, err := uuid.Parse(id)
    if err != nil {
        return err
    }

    shippedAt := sql.NullTime{}
    deliveredAt := sql.NullTime{}
    
    if status == "shipped" {
        shippedAt = sql.NullTime{Time: time.Now(), Valid: true}
    }
    if status == "delivered" {
        deliveredAt = sql.NullTime{Time: time.Now(), Valid: true}
    }

    _, err = database.Pool.Exec(ctx, `
        UPDATE logistics_orders 
        SET status = $1, updated_at = NOW(), 
            shipped_at = COALESCE($2, shipped_at),
            delivered_at = COALESCE($3, delivered_at)
        WHERE id = $4
    `, status, shippedAt, deliveredAt, orderID)
    return err
}

// GetStats - получение статистики
func (s *LogisticsService) GetStats(ctx context.Context) (map[string]interface{}, error) {
    var total, pending, shipped, delivered, cancelled, processing int

    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders").Scan(&total)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'pending'").Scan(&pending)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'shipped'").Scan(&shipped)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'delivered'").Scan(&delivered)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'cancelled'").Scan(&cancelled)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE processing = true").Scan(&processing)

    return map[string]interface{}{
        "total":       total,
        "pending":     pending,
        "shipped":     shipped,
        "delivered":   delivered,
        "cancelled":   cancelled,
        "processing":  processing,
        "in_progress": pending + shipped + processing,
    }, nil
}

// GetOrderByTracking - поиск по трек-номеру
func (s *LogisticsService) GetOrderByTracking(ctx context.Context, trackingNumber string) (*models.LogisticsOrder, error) {
    var o models.LogisticsOrder
    var trackNum, customerPhone, courierName, notes sql.NullString
    var weight, price sql.NullFloat64
    var shippedAt, deliveredAt sql.NullTime
    var processing bool

    query := `
        SELECT id, order_number, tracking_number, status, customer_name,
               customer_phone, customer_address, weight, price,
               created_at, updated_at, processing, shipped_at, delivered_at, courier_name, notes
        FROM logistics_orders
        WHERE tracking_number = $1 OR order_number = $1
        LIMIT 1
    `

    err := database.Pool.QueryRow(ctx, query, trackingNumber).Scan(
        &o.ID, &o.OrderNumber, &trackNum, &o.Status, &o.CustomerName,
        &customerPhone, &o.CustomerAddress, &weight, &price,
        &o.CreatedAt, &o.UpdatedAt, &processing, &shippedAt, &deliveredAt, &courierName, &notes,
    )
    if err != nil {
        return nil, err
    }

    if trackNum.Valid {
        o.TrackingNumber = &trackNum.String
    }
    if customerPhone.Valid {
        o.CustomerPhone = &customerPhone.String
    }
    if weight.Valid {
        o.Weight = &weight.Float64
    }
    if price.Valid {
        o.Price = &price.Float64
    }
    if courierName.Valid {
        o.CourierName = &courierName.String
    }
    if notes.Valid {
        o.Notes = &notes.String
    }
    if shippedAt.Valid {
        o.ShippedAt = &shippedAt.Time
    }
    if deliveredAt.Valid {
        o.DeliveredAt = &deliveredAt.Time
    }
    o.Processing = processing

    return &o, nil
}