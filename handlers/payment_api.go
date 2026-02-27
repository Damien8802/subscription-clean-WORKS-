package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"subscription-system/database"
	"subscription-system/models"
	"time"

	"github.com/gin-gonic/gin"
)

// Структуры запросов/ответов
type CreatePaymentRequest struct {
	Plan     string  `json:"plan" binding:"required"`
	Method   string  `json:"method" binding:"required"`
	Amount   float64 `json:"amount"`
	UserID   string  `json:"userId"`
	CardData struct {
		Number string `json:"cardNumber"`
		Expiry string `json:"cardExpiry"`
		Cvc    string `json:"cardCvc"`
		Name   string `json:"cardName"`
	} `json:"cardData"`
}

type PaymentResponse struct {
	Success    bool        `json:"success"`
	PaymentID  uint        `json:"paymentId,omitempty"`
	RedirectUrl string     `json:"redirectUrl,omitempty"`
	Invoice    interface{} `json:"invoice,omitempty"`
	Message    string      `json:"message,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Константы для платежей
const (
	USDTAddress = "TXmRt1UqWqfJ1XxqZQk3yL7vFhKpDnA2jB"
	BTCAddress  = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
)

// Цены тарифов
var planPrices = map[string]float64{
	"basic":      2990,
	"pro":        29900,
	"family":     9900,
	"enterprise": 49000,
}

// Создание платежа
func CreatePaymentHandler(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PaymentResponse{
			Success: false,
			Error:   "Неверный формат запроса: " + err.Error(),
		})
		return
	}

	// Получаем сумму
	amount := req.Amount
	if amount == 0 {
		if val, ok := planPrices[req.Plan]; ok {
			amount = val
		} else {
			amount = 1000
		}
	}

	// Создаём запись в БД
	payment := models.Payment{
		UserID:    req.UserID,
		Plan:      req.Plan,
		Amount:    amount,
		Method:    req.Method,
		Status:    "pending",
		OrderID:   generateOrderID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := database.DB.Create(&payment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, PaymentResponse{
			Success: false,
			Error:   "Ошибка сохранения платежа: " + err.Error(),
		})
		return
	}

	// Обработка в зависимости от метода
	switch req.Method {
	case "card":
		// Здесь будет интеграция с ЮKassa
		c.JSON(http.StatusOK, PaymentResponse{
			Success:     true,
			PaymentID:   payment.ID,
			RedirectUrl: "https://yoomoney.ru/payments/order",
			Message:     "Перенаправление на страницу оплаты картой",
		})

	case "usdt":
		c.JSON(http.StatusOK, PaymentResponse{
			Success:   true,
			PaymentID: payment.ID,
			Message:   fmt.Sprintf("Адрес USDT (TRC-20): %s", USDTAddress),
		})

	case "btc":
		c.JSON(http.StatusOK, PaymentResponse{
			Success:   true,
			PaymentID: payment.ID,
			Message:   fmt.Sprintf("Bitcoin адрес: %s", BTCAddress),
		})

	case "sbp":
		// Генерация QR-кода через API
		qrURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=СБП оплата %d руб", int(amount))
		c.JSON(http.StatusOK, PaymentResponse{
			Success:   true,
			PaymentID: payment.ID,
			Message:   qrURL,
		})

	case "crypto":
		// Интеграция с CryptoBot
		invoice := map[string]interface{}{
			"pay_url":   fmt.Sprintf("https://t.me/CryptoBot?start=%s", payment.OrderID),
			"invoice_id": payment.ID,
			"amount":    amount,
		}
		c.JSON(http.StatusOK, PaymentResponse{
			Success:   true,
			PaymentID: payment.ID,
			Invoice:   invoice,
			Message:   "Счет создан в CryptoBot",
		})

	default:
		c.JSON(http.StatusOK, PaymentResponse{
			Success:   true,
			PaymentID: payment.ID,
			Message:   "Платёж создан",
		})
	}
}

// Подтверждение платежа (вебхук)
func ConfirmPaymentHandler(c *gin.Context) {
	var req struct {
		PaymentID uint `json:"paymentId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var payment models.Payment
	if err := database.DB.First(&payment, req.PaymentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Платёж не найден"})
		return
	}

	payment.Status = "completed"
	payment.UpdatedAt = time.Now()
	database.DB.Save(&payment)

	// Активируем подписку
	activateSubscription(payment.UserID, payment.Plan)

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Вебхук для платежных систем
func PaymentWebhookHandler(c *gin.Context) {
	var webhookData map[string]interface{}
	if err := c.ShouldBindJSON(&webhookData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Обработка в зависимости от провайдера
	provider := c.Query("provider")
	
	switch provider {
	case "yookassa":
		handleYooKassaWebhook(webhookData)
	case "crypto":
		handleCryptoWebhook(webhookData)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Вспомогательные функции
func generateOrderID() string {
	return fmt.Sprintf("ORDER_%d", time.Now().UnixNano())
}

func activateSubscription(userID string, plan string) {
	// Здесь логика активации подписки
	println("Активирована подписка", plan, "для пользователя", userID)
}

func handleYooKassaWebhook(data map[string]interface{}) {
	// Обработка вебхука от ЮKassa
	println("Получен вебхук от ЮKassa")
}

func handleCryptoWebhook(data map[string]interface{}) {
	// Обработка вебхука от CryptoBot
	println("Получен вебхук от CryptoBot")
}