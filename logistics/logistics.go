package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// DeliveryService - служба доставки
type DeliveryService struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Price  float64 `json:"price"`
	Days   int     `json:"days"`
	Active bool    `json:"active"`
}

// DeliveryRequest - запрос на расчет доставки
type DeliveryRequest struct {
	FromCity string  `json:"from_city"`
	ToCity   string  `json:"to_city"`
	Weight   float64 `json:"weight"`
	Volume   float64 `json:"volume,omitempty"`
}

// DeliveryResponse - ответ с расчетом
type DeliveryResponse struct {
	ServiceID string  `json:"service_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Days      int     `json:"days"`
	Total     float64 `json:"total"`
	Date      string  `json:"delivery_date"`
}

// GetAllServices - возвращает все службы доставки
func GetAllServices() []DeliveryService {
	return []DeliveryService{
		{ID: "cdek", Name: "СДЭК", Price: 350, Days: 2, Active: true},
		{ID: "russian_post", Name: "Почта России", Price: 200, Days: 7, Active: true},
		{ID: "boxberry", Name: "Boxberry", Price: 300, Days: 3, Active: true},
		{ID: "dhl", Name: "DHL International", Price: 1500, Days: 5, Active: true},
		{ID: "yandex", Name: "Яндекс Доставка", Price: 400, Days: 1, Active: true},
	}
}

// CalculateDelivery - рассчитывает стоимость доставки
func CalculateDelivery(req DeliveryRequest, serviceID string) (DeliveryResponse, error) {
	services := GetAllServices()
	var service DeliveryService
	found := false

	for _, s := range services {
		if s.ID == serviceID && s.Active {
			service = s
			found = true
			break
		}
	}

	if !found {
		return DeliveryResponse{}, nil
	}

	// Расчет
	total := service.Price

	// Надбавка за вес
	if req.Weight > 5 {
		total += (req.Weight - 5) * 15
	}

	// Надбавка за расстояние (условно)
	if req.FromCity != req.ToCity {
		total *= 1.3
	}

	// Округление
	total = float64(int(total*100)) / 100

	// Дата доставки
	deliveryDate := time.Now().AddDate(0, 0, service.Days).Format("2006-01-02")

	return DeliveryResponse{
		ServiceID: service.ID,
		Name:      service.Name,
		Price:     service.Price,
		Days:      service.Days,
		Total:     total,
		Date:      deliveryDate,
	}, nil
}

// SetupLogisticsAPI - настраивает API маршруты
func SetupLogisticsAPI(router *gin.Engine) {
	// Получить все службы
	router.GET("/api/v1/logistics/services", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    GetAllServices(),
			"count":   len(GetAllServices()),
		})
	})

	// Рассчитать доставку
	router.POST("/api/v1/logistics/calculate", func(c *gin.Context) {
		var req DeliveryRequest
		serviceID := c.Query("service")

		if serviceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите службу доставки"})
			return
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := CalculateDelivery(req, serviceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    result,
		})
	})

	// Быстрый расчет для всех служб
	router.POST("/api/v1/logistics/quick-calc", func(c *gin.Context) {
		var req DeliveryRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		services := GetAllServices()
		var results []DeliveryResponse

		for _, service := range services {
			if service.Active {
				result, _ := CalculateDelivery(req, service.ID)
				results = append(results, result)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    results,
			"count":   len(results),
		})
	})
}
