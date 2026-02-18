package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// ==================== АДМИН-ПАНЕЛИ ====================
func AdminFixedHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "admin-fixed.html", gin.H{
		"Title":   "Админ-панель (Fixed) - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func GoldAdminHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "gold-admin.html", gin.H{
		"Title":   "Gold Admin - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func DatabaseAdminHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "database-admin.html", gin.H{
		"Title":   "Админ базы данных - SaaSPro",
		"Version": "3.0",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}
