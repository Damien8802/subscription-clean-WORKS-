package services

import (
	"fmt"
	"logistics/models"
	"sync"
	"time"
)

type LogisticsService struct {
	mu         sync.RWMutex
	orders     map[string]models.Order
	warehouses map[string]models.Warehouse
	couriers   map[string]models.DeliveryCourier

	// Статистика
	stats models.LogisticsStats
}

func NewLogisticsService() *LogisticsService {
	service := &LogisticsService{
		orders:     make(map[string]models.Order),
		warehouses: make(map[string]models.Warehouse),
		couriers:   make(map[string]models.DeliveryCourier),
	}

	// Инициализируем тестовые данные
	service.initTestData()

	return service
}

func (s *LogisticsService) initTestData() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Тестовые склады
	s.warehouses["wh1"] = models.Warehouse{
		ID:   "wh1",
		Name: "Основной склад Москва",
		Address: models.Address{
			City:       "Москва",
			Street:     "Ленинский проспект",
			Building:   "32",
			PostalCode: "119049",
			Country:    "Россия",
		},
		Contact:  "Иванов Иван",
		Phone:    "+7 (999) 123-45-67",
		Email:    "warehouse@company.com",
		IsActive: true,
	}

	// Тестовые курьеры
	s.couriers["c1"] = models.DeliveryCourier{
		ID:          "c1",
		Name:        "Петров Петр",
		Phone:       "+7 (999) 765-43-21",
		VehicleType: "car",
		Status:      "available",
		CurrentLocation: models.GeoLocation{
			Lat:  55.7558,
			Long: 37.6173,
		},
	}

	log.Println("✅ Логистика: тестовые данные инициализированы")
}

// CreateOrder - создание нового заказа
func (s *LogisticsService) CreateOrder(order models.Order) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Генерируем ID
	order.ID = fmt.Sprintf("order_%d", time.Now().UnixNano())
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	order.Status = "new"

	// Рассчитываем доставку (примерно через 2 дня)
	order.EstimatedDelivery = time.Now().Add(48 * time.Hour)

	s.orders[order.ID] = order

	// Обновляем статистику
	s.updateStats()

	log.Printf("✅ Логистика: создан заказ %s на сумму %.2f", order.ID, order.TotalAmount)

	return order.ID, nil
}

// GetOrder - получение заказа по ID
func (s *LogisticsService) GetOrder(orderID string) (*models.Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	order, exists := s.orders[orderID]
	if !exists {
		return nil, fmt.Errorf("заказ не найден: %s", orderID)
	}

	return &order, nil
}

// UpdateOrderStatus - обновление статуса заказа
func (s *LogisticsService) UpdateOrderStatus(orderID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	order, exists := s.orders[orderID]
	if !exists {
		return fmt.Errorf("заказ не найден: %s", orderID)
	}

	order.Status = status
	order.UpdatedAt = time.Now()
	s.orders[orderID] = order

	log.Printf("📦 Логистика: заказ %s обновлен статус: %s", orderID, status)

	return nil
}

// GetOrdersByStatus - получение заказов по статусу
func (s *LogisticsService) GetOrdersByStatus(status string) []models.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.Order
	for _, order := range s.orders {
		if order.Status == status {
			result = append(result, order)
		}
	}

	return result
}

// GetAllOrders - получение всех заказов
func (s *LogisticsService) GetAllOrders() []models.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []models.Order
	for _, order := range s.orders {
		result = append(result, order)
	}

	return result
}

// GetStats - получение статистики
func (s *LogisticsService) GetStats() models.LogisticsStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.stats
}

// updateStats - обновление статистики
func (s *LogisticsService) updateStats() {
	now := time.Now()
	var todayOrders, monthOrders int
	var todayRevenue, monthRevenue float64

	for _, order := range s.orders {
		// Заказы за сегодня
		if order.CreatedAt.Year() == now.Year() &&
			order.CreatedAt.Month() == now.Month() &&
			order.CreatedAt.Day() == now.Day() {
			todayOrders++
			todayRevenue += order.TotalAmount
		}

		// Заказы за этот месяц
		if order.CreatedAt.Year() == now.Year() &&
			order.CreatedAt.Month() == now.Month() {
			monthOrders++
			monthRevenue += order.TotalAmount
		}
	}

	s.stats = models.LogisticsStats{
		TotalOrders:         len(s.orders),
		OrdersToday:         todayOrders,
		OrdersThisMonth:     monthOrders,
		RevenueToday:        todayRevenue,
		RevenueThisMonth:    monthRevenue,
		ActiveCouriers:      len(s.couriers),
		AvailableWarehouses: len(s.warehouses),
		AvgDeliveryTime:     48, // 48 часов в среднем
		DeliverySuccessRate: 97.5,
	}
}

// AddWarehouse - добавление склада
func (s *LogisticsService) AddWarehouse(warehouse models.Warehouse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.warehouses[warehouse.ID] = warehouse
	s.updateStats()
}

// AddCourier - добавление курьера
func (s *LogisticsService) AddCourier(courier models.DeliveryCourier) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.couriers[courier.ID] = courier
	s.updateStats()
}
