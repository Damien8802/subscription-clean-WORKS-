package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// ==================== API МАРШРУТЫ ====================
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "3.0",
		"time":    time.Now().Unix(),
	})
}

func CRMHealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"crm":     "operational",
		"version": "3.0",
		"time":    time.Now().Unix(),
	})
}

func SystemStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"users":         1248,
		"subscriptions": 5342,
		"revenue":       245820,
		"uptime":        "99.9%",
		"timestamp":     time.Now().Unix(),
	})
}

func TestHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "Сервер работает!",
		"time":    time.Now().Format(time.RFC3339),
	})
}
