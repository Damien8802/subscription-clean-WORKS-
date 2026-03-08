package services

import (
	"context"

	"subscription-system/config"
)

// YandexAdapter - адаптер для совместимости YandexAIService с AIAgentService
type YandexAdapter struct {
	service *YandexAIService
}

// NewYandexService - конструктор для совместимости
func NewYandexService(cfg *config.Config) *YandexAdapter {
	return &YandexAdapter{
		service: NewYandexAIService(cfg),
	}
}

// Ask - реализация метода, совместимого с AIAgentService
func (a *YandexAdapter) Ask(prompt string, model string, temperature float64) (string, error) {
	ctx := context.Background()
	return a.service.Ask(ctx, prompt)
}