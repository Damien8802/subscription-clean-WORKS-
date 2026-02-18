package api

import (
	"github.com/gin-gonic/gin"
	"log"
	"logistics/services"
	"net/http"
)

type LogisticsAPI struct {
	service *services.LogisticsService
}

func NewLogisticsAPI(service *services.LogisticsService) *LogisticsAPI {
	return &LogisticsAPI{service: service}
}

// GetLogisticsStats - статистика логистики
func (api *LogisticsAPI) GetLogisticsStats(c *gin.Context) {
	stats := api.service.GetStats()

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"stats":     stats,
		"timestamp": time.Now(),
	})
}

// CreateOrder - создание заказа
func (api *LogisticsAPI) CreateOrder(c *gin.Context) {
	var order models.Order

	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Неверный формат данных",
		})
		return
	}

	orderID, err := api.service.CreateOrder(order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success":  true,
		"message":  "Заказ создан",
		"order_id": orderID,
		"order":    order,
	})
}

// GetOrder - получение заказа
func (api *LogisticsAPI) GetOrder(c *gin.Context) {
	orderID := c.Param("id")

	order, err := api.service.GetOrder(orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"order":   order,
	})
}

// GetOrders - получение всех заказов
func (api *LogisticsAPI) GetOrders(c *gin.Context) {
	status := c.Query("status")

	var orders []models.Order
	if status != "" {
		orders = api.service.GetOrdersByStatus(status)
	} else {
		orders = api.service.GetAllOrders()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   len(orders),
		"orders":  orders,
	})
}

// UpdateOrderStatus - обновление статуса заказа
func (api *LogisticsAPI) UpdateOrderStatus(c *gin.Context) {
	orderID := c.Param("id")

	var request struct {
		Status string `json:"status"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Неверный формат данных",
		})
		return
	}

	err := api.service.UpdateOrderStatus(orderID, request.Status)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Статус обновлен",
		"order_id":   orderID,
		"new_status": request.Status,
	})
}

// GetWarehouses - получение складов
func (api *LogisticsAPI) GetWarehouses(c *gin.Context) {
	// В реальном проекте здесь будет обращение к сервису
	warehouses := []models.Warehouse{
		{
			ID:   "wh1",
			Name: "Основной склад Москва",
			Address: models.Address{
				City:       "Москва",
				Street:     "Ленинский проспект",
				Building:   "32",
				PostalCode: "119049",
				Country:    "Россия",
			},
			IsActive: true,
		},
		{
			ID:   "wh2",
			Name: "Склад Санкт-Петербург",
			Address: models.Address{
				City:       "Санкт-Петербург",
				Street:     "Невский проспект",
				Building:   "100",
				PostalCode: "191025",
				Country:    "Россия",
			},
			IsActive: true,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"count":      len(warehouses),
		"warehouses": warehouses,
	})
}

// GetCouriers - получение курьеров
func (api *LogisticsAPI) GetCouriers(c *gin.Context) {
	// В реальном проекте здесь будет обращение к сервису
	couriers := []models.DeliveryCourier{
		{
			ID:          "c1",
			Name:        "Петров Петр",
			Phone:       "+7 (999) 765-43-21",
			VehicleType: "car",
			Status:      "available",
		},
		{
			ID:          "c2",
			Name:        "Сидоров Алексей",
			Phone:       "+7 (999) 555-12-34",
			VehicleType: "bike",
			Status:      "busy",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"count":    len(couriers),
		"couriers": couriers,
	})
}
