package handlers

import (
"net/http"
"time"
"github.com/gin-gonic/gin"
)

// LogisticsHandler – страница логистики
func LogisticsHandler(c *gin.Context) {
c.HTML(http.StatusOK, "logistics.html", gin.H{
"Title":   "Логистика - SaaSPro",
"Version": "3.0",
"Time":    time.Now().Format("2006-01-02 15:04:05"),
})
}

// DeliveryHandler – страница доставки
func DeliveryHandler(c *gin.Context) {
c.HTML(http.StatusOK, "delivery.html", gin.H{
"Title":   "Доставка - SaaSPro",
"Version": "3.0",
"Time":    time.Now().Format("2006-01-02 15:04:05"),
})
}

// TrackHandler – страница отслеживания
func TrackHandler(c *gin.Context) {
c.HTML(http.StatusOK, "track.html", gin.H{
"Title":   "Отслеживание - SaaSPro",
"Version": "3.0",
"Time":    time.Now().Format("2006-01-02 15:04:05"),
})
}

// TrackAPIHandler – API для отслеживания
func TrackAPIHandler(c *gin.Context) {
trackingNumber := c.Param("trackingNumber")
c.JSON(http.StatusOK, gin.H{
"tracking_number": trackingNumber,
"status":          "in_transit",
"estimated_delivery": time.Now().Add(48 * time.Hour).Format("2006-01-02"),
})
}
