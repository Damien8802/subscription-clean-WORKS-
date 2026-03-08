package services

import (
	"context"
	"log"
	"time"

	"subscription-system/database"
)

// OpenRouterServiceInterface - интерфейс для AI сервисов
type OpenRouterServiceInterface interface {
	Ask(prompt string, model string, temperature float64) (string, error)
}

// AIAgentService - сервис для работы с ИИ-агентами
type AIAgentService struct {
	AI OpenRouterServiceInterface
}

// NewAIAgentService - конструктор
func NewAIAgentService(ai OpenRouterServiceInterface) *AIAgentService {
	return &AIAgentService{
		AI: ai,
	}
}

// StartAgentScheduler - запуск планировщика
func (s *AIAgentService) StartAgentScheduler() {
	log.Println("🤖 ИИ-агенты: планировщик запущен")

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			log.Println("🔄 ИИ-агенты: проверка задач...")
			s.ProcessPendingTasks()
		}
	}()
}

// ProcessPendingTasks - обработка ожидающих задач
func (s *AIAgentService) ProcessPendingTasks() {
	ctx := context.Background()

	rows, err := database.Pool.Query(ctx, `
		SELECT id, agent_id, customer_id, task_type, prompt
		FROM ai_agent_tasks
		WHERE status = 'pending' AND scheduled_at <= NOW()
		LIMIT 5
	`)

	if err != nil {
		log.Printf("❌ Ошибка получения задач: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, agentID, customerID, taskType, prompt string
		err := rows.Scan(&id, &agentID, &customerID, &taskType, &prompt)
		if err != nil {
			log.Printf("❌ Ошибка сканирования: %v", err)
			continue
		}

		// Получаем ответ от AI
		response, err := s.AI.Ask(prompt, "", 0.7)
		if err != nil {
			log.Printf("❌ Ошибка AI: %v", err)
			database.Pool.Exec(ctx, "UPDATE ai_agent_tasks SET status = 'failed' WHERE id = $1", id)
			continue
		}

		// Обновляем задачу
		database.Pool.Exec(ctx, `
			UPDATE ai_agent_tasks
			SET status = 'completed', result = $1, executed_at = NOW()
			WHERE id = $2
		`, response, id)

		log.Printf("✅ Задача %s выполнена", id)
	}
}