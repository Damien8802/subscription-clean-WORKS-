package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAccountID получает account_id из контекста
func GetAccountID(c *gin.Context) string {
	accountID, exists := c.Get("accountID")
	if !exists {
		return ""
	}
	return accountID.(string)
}

// CreateAgent - создание нового ИИ-агента
func CreateAgent(c *gin.Context) {
	accountID := GetAccountID(c)
	
	c.JSON(http.StatusOK, gin.H{
		"message":    "Агент будет создан",
		"account_id": accountID,
	})
}

// GetAgents - список агентов
func GetAgents(c *gin.Context) {
	accountID := GetAccountID(c)
	
	c.JSON(http.StatusOK, gin.H{
		"agents":     []gin.H{},
		"account_id": accountID,
	})
}

// UpdateAgent - обновление агента
func UpdateAgent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "агент обновлен"})
}

// DeleteAgent - удаление агента
func DeleteAgent(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "агент удален"})
}

// AddAgentAction - добавление действия
func AddAgentAction(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "действие добавлено"})
}

// GetAgentLogs - логи агентов
func GetAgentLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []gin.H{}})
}

// GetAgentStats - статистика
func GetAgentStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"total_actions":   0,
		"success_actions": 0,
		"error_actions":   0,
		"deals_created":   0,
	})
}

// AIAgentsPage - отображение страницы с агентами
func AIAgentsPage(c *gin.Context) {
	userEmail := c.GetString("userEmail")
	if userEmail == "" {
		userEmail = "admin@example.com"
	}

	userName := c.GetString("userName")
	if userName == "" {
		userName = "Администратор"
	}

	userRole := c.GetString("userRole")
	if userRole == "" {
		userRole = "admin"
	}

	c.HTML(http.StatusOK, "ai_agents.html", gin.H{
		"Title":     "ИИ-агенты - SaaSPro",
		"Version":   "3.0",
		"UserEmail": userEmail,
		"UserName":  userName,
		"IsAdmin":   userRole == "admin",
	})
}