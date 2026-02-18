package logistics

import "time"

// Order - заказ клиента
type Order struct {
	ID            string `json:"id"`
	CustomerID    string `json:"customer_id"`
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
	CustomerEmail string `json:"customer_email"`

	Products    []OrderProduct `json:"products"`
	TotalAmount float64        `json:"total_amount"`
	Currency    string         `json:"currency"`

	DeliveryType    string  `json:"delivery_type"` // pickup, courier, post
	DeliveryAddress Address `json:"delivery_address"`

	Status         string `json:"status"` // new, processing, shipped, delivered, canceled
	TrackingNumber string `json:"tracking_number"`

	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	EstimatedDelivery time.Time `json:"estimated_delivery"`
}

// OrderProduct - товар в заказе
type OrderProduct struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	SKU       string  `json:"sku"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Total     float64 `json:"total"`
}

// Address - адрес доставки
type Address struct {
	City       string `json:"city"`
	Street     string `json:"street"`
	Building   string `json:"building"`
	Apartment  string `json:"apartment"`
	PostalCode string `json:"postal_code"`
	Region     string `json:"region"`
	Country    string `json:"country"`
	Notes      string `json:"notes"`
}

// Warehouse - склад
type Warehouse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Address  Address `json:"address"`
	Contact  string  `json:"contact"`
	Phone    string  `json:"phone"`
	Email    string  `json:"email"`
	IsActive bool    `json:"is_active"`
}

// DeliveryCourier - курьер
type DeliveryCourier struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Phone           string      `json:"phone"`
	VehicleType     string      `json:"vehicle_type"` // car, bike, walk
	Status          string      `json:"status"`       // available, busy, offline
	CurrentLocation GeoLocation `json:"current_location"`
}

// GeoLocation - географические координаты
type GeoLocation struct {
	Lat  float64 `json:"lat"`
	Long float64 `json:"long"`
}

// LogisticsStats - статистика логистики
type LogisticsStats struct {
	TotalOrders     int `json:"total_orders"`
	OrdersToday     int `json:"orders_today"`
	OrdersThisMonth int `json:"orders_this_month"`

	AvgDeliveryTime     float64 `json:"avg_delivery_time"`
	DeliverySuccessRate float64 `json:"delivery_success_rate"`

	ActiveCouriers      int `json:"active_couriers"`
	AvailableWarehouses int `json:"available_warehouses"`

	RevenueToday     float64 `json:"revenue_today"`
	RevenueThisMonth float64 `json:"revenue_this_month"`
}
