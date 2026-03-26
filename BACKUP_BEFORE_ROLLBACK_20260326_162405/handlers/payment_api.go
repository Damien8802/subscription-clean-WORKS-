package handlers

import (
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

// PaymentRequest - запрос на создание платежа
type PaymentRequest struct {
    Plan   string  `json:"plan"`
    Method string  `json:"method"`
    Amount float64 `json:"amount"`
    UserID string  `json:"userId"`
}

// PaymentResponse - ответ на создание платежа
type PaymentResponse struct {
    Success    bool        `json:"success"`
    PaymentID  string      `json:"paymentId,omitempty"`
    Address    string      `json:"address,omitempty"`
    Amount     float64     `json:"amount,omitempty"`
    Currency   string      `json:"currency,omitempty"`
    QRCode     string      `json:"qrCode,omitempty"`
    Invoice    interface{} `json:"invoice,omitempty"`
    Message    string      `json:"message,omitempty"`
    Error      string      `json:"error,omitempty"`
}

// Константы для кошельков
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

var usdtAmounts = map[string]float64{
    "basic":      33.22,
    "pro":        332.22,
    "family":     110.00,
    "enterprise": 544.44,
}

var btcAmounts = map[string]float64{
    "basic":      0.00066,
    "pro":        0.00664,
    "family":     0.00220,
    "enterprise": 0.01088,
}

// CreatePaymentHandler - создание платежа
func CreatePaymentHandler(c *gin.Context) {
    var req PaymentRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, PaymentResponse{
            Success: false,
            Error:   "Неверный формат запроса",
        })
        return
    }

    amount := req.Amount
    if amount == 0 {
        if val, ok := planPrices[req.Plan]; ok {
            amount = val
        }
    }

    paymentID := fmt.Sprintf("PAY_%d", time.Now().UnixNano())

    switch req.Method {
    case "usdt":
        c.JSON(http.StatusOK, PaymentResponse{
            Success:  true,
            PaymentID: paymentID,
            Address:  USDTAddress,
            Amount:   usdtAmounts[req.Plan],
            Currency: "USDT",
            Message:  "Отправьте USDT (TRC-20)",
        })
    case "btc":
        c.JSON(http.StatusOK, PaymentResponse{
            Success:  true,
            PaymentID: paymentID,
            Address:  BTCAddress,
            Amount:   btcAmounts[req.Plan],
            Currency: "BTC",
            Message:  "Отправьте Bitcoin",
        })
    case "sbp":
        qrURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=СБП %d руб", int(amount))
        c.JSON(http.StatusOK, PaymentResponse{
            Success:  true,
            PaymentID: paymentID,
            QRCode:   qrURL,
            Amount:   amount,
            Currency: "RUB",
            Message:  "QR-код для СБП",
        })
    case "crypto":
        invoice := map[string]interface{}{
            "pay_url":    "https://t.me/CryptoBot",
            "invoice_id": paymentID,
            "amount":     usdtAmounts[req.Plan],
        }
        c.JSON(http.StatusOK, PaymentResponse{
            Success:  true,
            PaymentID: paymentID,
            Invoice:  invoice,
            Message:  "Счет в CryptoBot",
        })
    case "card":
        c.JSON(http.StatusOK, PaymentResponse{
            Success:  true,
            PaymentID: paymentID,
            Message:  "Оплата картой (в разработке)",
        })
    default:
        c.JSON(http.StatusOK, PaymentResponse{
            Success:  true,
            PaymentID: paymentID,
            Message:  "Платёж создан",
        })
    }
}