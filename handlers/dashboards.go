package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// ==================== ДАШБОРДЫ ====================
func DashboardImprovedHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard-improved.html", gin.H{
		"Title":   "Улучшенный дашборд - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func RealtimeDashboardHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "realtime-dashboard.html", gin.H{
		"Title":   "Дашборд реального времени - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func RevenueDashboardHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "revenue-dashboard.html", gin.H{
		"Title":   "Дашборд выручки - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func PartnerDashboardHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "partner-dashboard.html", gin.H{
		"Title":   "Партнерский дашборд - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func UnifiedDashboardHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "unified-dashboard.html", gin.H{
		"Title":   "Унифицированный дашборд - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}
