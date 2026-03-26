package handlers

import (
	"log"
	"net/http"
	"time"

	"subscription-system/database"
	"subscription-system/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetAccountID получает account_id из контекста
func GetAccountID(c *gin.Context) string {
    accountID, exists := c.Get("accountID")
    if !exists {
        // В режиме SkipAuth используем тестовый account_id
        log.Println("⚠️ accountID не найден в контексте, использую тестовый")
        return "00000000-0000-0000-0000-000000000001"
    }
    
    // Преобразуем в строку
    if str, ok := accountID.(string); ok {
        return str
    }
    
    log.Println("⚠️ accountID имеет неправильный тип, использую тестовый")
    return "00000000-0000-0000-0000-000000000001"
}

// CreateAgent - создание нового ИИ-агента
func CreateAgent(c *gin.Context) {
	accountID := GetAccountID(c)
	
	var agent models.AIAgent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Валидация
	if agent.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "имя агента обязательно"})
		return
	}
	
	// Заполняем обязательные поля
	agent.ID = uuid.New().String()
	agent.AccountID = accountID
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()
	
	if agent.Temperature == 0 {
		agent.Temperature = 0.7
	}
	if agent.Schedule == "" {
		agent.Schedule = "24/7"
	}
	if agent.Model == "" {
		agent.Model = "openrouter/auto"
	}
	
	// Сохраняем в БД
	_, err := database.Pool.Exec(c.Request.Context(), `
		INSERT INTO ai_agents (id, account_id, name, role, instructions, model, temperature, schedule, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, agent.ID, agent.AccountID, agent.Name, agent.Role, agent.Instructions,
		agent.Model, agent.Temperature, agent.Schedule, agent.IsActive,
		agent.CreatedAt, agent.UpdatedAt)
	
	if err != nil {
		log.Printf("❌ Ошибка создания агента: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Агент успешно создан",
		"agent":   agent,
	})
}

// GetAgents - список агентов
func GetAgents(c *gin.Context) {
	accountID := GetAccountID(c)
	
	rows, err := database.Pool.Query(c.Request.Context(), `
		SELECT id, name, role, instructions, model, temperature, schedule, is_active, created_at, updated_at
		FROM ai_agents
		WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	
	if err != nil {
		log.Printf("❌ Ошибка получения агентов: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	var agents []gin.H
	for rows.Next() {
		var id, name, role, instructions, model, schedule string
		var temperature float64
		var isActive bool
		var createdAt, updatedAt time.Time
		
		err := rows.Scan(&id, &name, &role, &instructions, &model, &temperature, &schedule, &isActive, &createdAt, &updatedAt)
		if err != nil {
			continue
		}
		
		agents = append(agents, gin.H{
			"id":           id,
			"name":         name,
			"role":         role,
			"instructions": instructions,
			"model":        model,
			"temperature":  temperature,
			"schedule":     schedule,
			"is_active":    isActive,
			"created_at":   createdAt,
			"updated_at":   updatedAt,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
	})
}

// UpdateAgent - обновление агента
func UpdateAgent(c *gin.Context) {
	agentID := c.Param("id")
	accountID := GetAccountID(c)
	
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Проверяем, что агент принадлежит аккаунту
	var exists bool
	err := database.Pool.QueryRow(c.Request.Context(), `
		SELECT EXISTS(SELECT 1 FROM ai_agents WHERE id = $1 AND account_id = $2)
	`, agentID, accountID).Scan(&exists)
	
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "агент не найден"})
		return
	}
	
	// Добавляем updated_at
	updates["updated_at"] = time.Now()
	
	// Строим запрос динамически
	query := "UPDATE ai_agents SET "
	args := []interface{}{}
	i := 1
	
	for key, value := range updates {
		if key == "id" || key == "account_id" || key == "created_at" {
			continue
		}
		if i > 1 {
			query += ", "
		}
		query += key + " = $" + string(rune('0'+i))
		args = append(args, value)
		i++
	}
	query += " WHERE id = $" + string(rune('0'+i)) + " AND account_id = $" + string(rune('0'+i+1))
	args = append(args, agentID, accountID)
	
	_, err = database.Pool.Exec(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "агент обновлен"})
}

// DeleteAgent - удаление агента
func DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")
	accountID := GetAccountID(c)
	
	_, err := database.Pool.Exec(c.Request.Context(), `
		DELETE FROM ai_agents WHERE id = $1 AND account_id = $2
	`, agentID, accountID)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "агент удален"})
}

// AddAgentAction - добавление действия
func AddAgentAction(c *gin.Context) {
	agentID := c.Param("id")
	accountID := GetAccountID(c)
	
	var action struct {
		Action    string `json:"action"`
		Condition string `json:"condition"`
		Config    map[string]interface{} `json:"config"`
	}
	
	if err := c.ShouldBindJSON(&action); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Проверяем агента
	var exists bool
	err := database.Pool.QueryRow(c.Request.Context(), `
		SELECT EXISTS(SELECT 1 FROM ai_agents WHERE id = $1 AND account_id = $2)
	`, agentID, accountID).Scan(&exists)
	
	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "агент не найден"})
		return
	}
	
	_, err = database.Pool.Exec(c.Request.Context(), `
		INSERT INTO ai_agent_actions (id, agent_id, action, condition, config, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.New().String(), agentID, action.Action, action.Condition, action.Config, time.Now())
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "действие добавлено"})
}

// GetAgentLogs - логи агентов
func GetAgentLogs(c *gin.Context) {
	accountID := GetAccountID(c)
	
	rows, err := database.Pool.Query(c.Request.Context(), `
		SELECT l.id, l.action, l.result, l.status, l.created_at,
		       a.name as agent_name,
		       c.name as customer_name
		FROM ai_agent_logs l
		JOIN ai_agents a ON a.id = l.agent_id
		LEFT JOIN customers c ON c.id = l.customer_id
		WHERE a.account_id = $1
		ORDER BY l.created_at DESC
		LIMIT 100
	`, accountID)
	
	if err != nil {
		log.Printf("❌ Ошибка получения логов: %v", err)
		c.JSON(http.StatusOK, gin.H{"logs": []gin.H{}})
		return
	}
	defer rows.Close()
	
	var logs []gin.H
	for rows.Next() {
		var id, action, result, status, agentName, customerName string
		var createdAt time.Time
		
		err := rows.Scan(&id, &action, &result, &status, &createdAt, &agentName, &customerName)
		if err != nil {
			continue
		}
		
		logs = append(logs, gin.H{
			"id":            id,
			"action":        action,
			"result":        result,
			"status":        status,
			"created_at":    createdAt,
			"agent_name":    agentName,
			"customer_name": customerName,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// GetAgentStats - статистика
func GetAgentStats(c *gin.Context) {
	accountID := GetAccountID(c)
	
	var totalActions, successActions, errorActions, dealsCreated int
	
	err := database.Pool.QueryRow(c.Request.Context(), `
		SELECT 
			COALESCE(COUNT(*), 0) as total,
			COALESCE(COUNT(CASE WHEN status = 'success' THEN 1 END), 0) as success,
			COALESCE(COUNT(CASE WHEN status = 'error' THEN 1 END), 0) as error,
			COALESCE(COUNT(CASE WHEN action = 'create_deal' AND status = 'success' THEN 1 END), 0) as deals
		FROM ai_agent_logs l
		JOIN ai_agents a ON a.id = l.agent_id
		WHERE a.account_id = $1
	`, accountID).Scan(&totalActions, &successActions, &errorActions, &dealsCreated)
	
	if err != nil {
		log.Printf("❌ Ошибка получения статистики: %v", err)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"total_actions":   totalActions,
		"success_actions": successActions,
		"error_actions":   errorActions,
		"deals_created":   dealsCreated,
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