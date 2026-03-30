package models

import (
    "time"
    "github.com/google/uuid"
)

type LogisticsOrder struct {
    ID              uuid.UUID  `json:"id" db:"id"`
    OrderNumber     string     `json:"order_number" db:"order_number"`
    CustomerName    string     `json:"customer_name" db:"customer_name"`
    CustomerPhone   *string    `json:"customer_phone" db:"customer_phone"`
    CustomerEmail   *string    `json:"customer_email" db:"customer_email"`
    DeliveryAddress string     `json:"delivery_address" db:"delivery_address"`
    City            *string    `json:"city" db:"city"`
    Weight          *float64   `json:"weight" db:"weight"`
    Price           *float64   `json:"price" db:"price"`
    Status          string     `json:"status" db:"status"`
    Priority        string     `json:"priority" db:"priority"`
    TrackingNumber  *string    `json:"tracking_number" db:"tracking_number"`
    CreatedAt       time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
    TenantID        *uuid.UUID `json:"tenant_id" db:"tenant_id"`
}

type LogisticsShipment struct {
    ID                uuid.UUID  `json:"id" db:"id"`
    OrderID           uuid.UUID  `json:"order_id" db:"order_id"`
    Carrier           *string    `json:"carrier" db:"carrier"`
    TrackingNumber    string     `json:"tracking_number" db:"tracking_number"`
    Status            string     `json:"status" db:"status"`
    CurrentLocation   *string    `json:"current_location" db:"current_location"`
    EstimatedDelivery *time.Time `json:"estimated_delivery" db:"estimated_delivery"`
    CreatedAt         time.Time  `json:"created_at" db:"created_at"`
}