package services

import (
    "context"
    "time"
    "github.com/google/uuid"
    "subscription-system/database"
    "subscription-system/models"
)

type LogisticsService struct{}

func NewLogisticsService() *LogisticsService {
    return &LogisticsService{}
}

func (s *LogisticsService) CreateOrder(ctx context.Context, order *models.LogisticsOrder) (*models.LogisticsOrder, error) {
    order.ID = uuid.New()
    order.CreatedAt = time.Now()
    order.UpdatedAt = time.Now()
    order.Status = "pending"
    
    err := database.Pool.QueryRow(ctx, `
        INSERT INTO logistics_orders (
            id, order_number, customer_name, customer_phone, customer_email,
            delivery_address, city, weight, price, status, priority, 
            tracking_number, created_at, updated_at, tenant_id
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
        RETURNING id, order_number, customer_name, customer_phone, customer_email,
                  delivery_address, city, weight, price, status, priority,
                  tracking_number, created_at, updated_at
    `, order.ID, order.OrderNumber, order.CustomerName, order.CustomerPhone,
        order.CustomerEmail, order.DeliveryAddress, order.City, order.Weight,
        order.Price, order.Status, order.Priority, order.TrackingNumber,
        order.CreatedAt, order.UpdatedAt, order.TenantID).Scan(
        &order.ID, &order.OrderNumber, &order.CustomerName, &order.CustomerPhone,
        &order.CustomerEmail, &order.DeliveryAddress, &order.City, &order.Weight,
        &order.Price, &order.Status, &order.Priority, &order.TrackingNumber,
        &order.CreatedAt, &order.UpdatedAt,
    )
    return order, err
}

func (s *LogisticsService) GetOrders(ctx context.Context, limit, offset int) ([]models.LogisticsOrder, int, error) {
    var total int
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders").Scan(&total)
    
    rows, err := database.Pool.Query(ctx, `
        SELECT id, order_number, customer_name, customer_phone, customer_email,
               delivery_address, city, weight, price, status, priority,
               tracking_number, created_at, updated_at
        FROM logistics_orders ORDER BY created_at DESC LIMIT $1 OFFSET $2
    `, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    
    var orders []models.LogisticsOrder
    for rows.Next() {
        var o models.LogisticsOrder
        err := rows.Scan(&o.ID, &o.OrderNumber, &o.CustomerName, &o.CustomerPhone,
            &o.CustomerEmail, &o.DeliveryAddress, &o.City, &o.Weight, &o.Price,
            &o.Status, &o.Priority, &o.TrackingNumber, &o.CreatedAt, &o.UpdatedAt)
        if err != nil {
            continue
        }
        orders = append(orders, o)
    }
    return orders, total, nil
}

func (s *LogisticsService) UpdateStatus(ctx context.Context, id string, status string) error {
    _, err := database.Pool.Exec(ctx, `
        UPDATE logistics_orders SET status = $1, updated_at = NOW() WHERE id = $2
    `, status, id)
    return err
}

func (s *LogisticsService) GetStats(ctx context.Context) (map[string]interface{}, error) {
    stats := make(map[string]interface{})
    var total, inTransit, delivered, pending int
    
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders").Scan(&total)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'in_transit'").Scan(&inTransit)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'delivered'").Scan(&delivered)
    database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM logistics_orders WHERE status = 'pending'").Scan(&pending)
    
    stats["total"] = total
    stats["in_transit"] = inTransit
    stats["delivered"] = delivered
    stats["pending"] = pending
    
    return stats, nil
}