package onecintegration

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Обработчики для API

// Проверка соединения с 1С
func HandleTestConnection(client *OneCClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ok, err := client.TestConnection()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"connected": ok,
			"service":   "1C",
			"timestamp": time.Now().Unix(),
		})
	}
}

// Принудительная синхронизация
func HandleForceSync(syncManager *SyncManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		go syncManager.SyncAll()

		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"message":   "Синхронизация запущена",
			"startedAt": time.Now().Format("2006-01-02 15:04:05"),
		})
	}
}

// Получение статуса синхронизации
func HandleSyncStatus(syncManager *SyncManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"status":   "active",              // или paused, error
			"lastSync": "2024-01-01 12:00:00", // нужно хранить время
			"nextSync": time.Now().Add(syncManager.interval).Format("2006-01-02 15:04:05"),
		})
	}
}
