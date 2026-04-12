package models

import (
    "time"
    "github.com/google/uuid"
)

type LogisticsOrder struct {
    ID              uuid.UUID  `json:"id"`
    OrderNumber     string     `json:"order_number"`
    TrackingNumber  *string    `json:"tracking_number"`
    Status          string     `json:"status"`
    CustomerName    string     `json:"customer_name"`
    CustomerPhone   *string    `json:"customer_phone"`
    CustomerAddress string     `json:"customer_address"`
    Weight          *float64   `json:"weight"`
    Price           *float64   `json:"price"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
    TenantID        *uuid.UUID `json:"tenant_id"`
    UserID          *uuid.UUID `json:"user_id"`
    Processing      bool       `json:"processing"`
    ShippedAt       *time.Time `json:"shipped_at"`
    DeliveredAt     *time.Time `json:"delivered_at"`
    CourierName     *string    `json:"courier_name"`
    Notes           *string    `json:"notes"`
}