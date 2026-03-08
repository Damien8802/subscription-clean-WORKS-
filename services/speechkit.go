package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"subscription-system/config"
)

// SpeechKitService - сервис для работы с Yandex SpeechKit
type SpeechKitService struct {
	APIKey     string
	FolderID   string
	HTTPClient *http.Client
}

// NewSpeechKitService - конструктор
func NewSpeechKitService(cfg *config.Config) *SpeechKitService {
	return &SpeechKitService{
		APIKey:   cfg.YandexAPIKey,
		FolderID: cfg.YandexFolderID,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute, // Для аудио нужно больше времени
		},
	}
}

// TranscriptionResult - результат распознавания
type TranscriptionResult struct {
	Result string `json:"result"`
}

// TranscribeAudio - распознавание аудиофайла через Yandex SpeechKit
func (s *SpeechKitService) TranscribeAudio(ctx context.Context, audioData []byte, filename string) (string, error) {
	url := "https://transcribe.api.cloud.yandex.net/speech/stt/v2/longRunningRecognize"

	// Подготавливаем запрос для длительного распознавания
	request := map[string]interface{}{
		"config": map[string]interface{}{
			"specification": map[string]interface{}{
				"languageCode":          "ru-RU",
				"model":                 "general",
				"audioEncoding":         "MP3",
				"sampleRateHertz":       48000,
				"profanityFilter":       false,
				"literatureText":        false,
				"audioChannelCount":     1,
			},
		},
		"audio": map[string]interface{}{
			"content": string(audioData),
		},
	}

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("ошибка маршалинга запроса: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Api-Key "+s.APIKey)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка отправки запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SpeechKit вернул ошибку %d: %s", resp.StatusCode, string(body))
	}

	var operation struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &operation); err != nil {
		return "", fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	// Ждем завершения операции
	return s.waitForOperation(ctx, operation.ID)
}

// waitForOperation - ожидание завершения операции распознавания
func (s *SpeechKitService) waitForOperation(ctx context.Context, operationID string) (string, error) {
	url := fmt.Sprintf("https://operation.api.cloud.yandex.net/operations/%s", operationID)

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("операция прервана")
		case <-time.After(2 * time.Second):
			req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
			req.Header.Set("Authorization", "Api-Key "+s.APIKey)

			resp, err := s.HTTPClient.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			var result struct {
				Done   bool `json:"done"`
				Error  struct {
					Message string `json:"message"`
				} `json:"error"`
				Response struct {
					Chunks []struct {
						Alternatives []struct {
							Text string `json:"text"`
						} `json:"alternatives"`
					} `json:"chunks"`
				} `json:"response"`
			}

			json.Unmarshal(body, &result)

			if result.Error.Message != "" {
				return "", fmt.Errorf("ошибка операции: %s", result.Error.Message)
			}

			if result.Done {
				// Собираем все чанки в один текст
				var fullText string
				for _, chunk := range result.Response.Chunks {
					if len(chunk.Alternatives) > 0 {
						fullText += chunk.Alternatives[0].Text + " "
					}
				}
				return fullText, nil
			}
		}
	}
}

// AnalyzeSentiment - анализ тональности текста через YandexGPT
func (s *SpeechKitService) AnalyzeSentiment(ctx context.Context, text string) (string, error) {
	prompt := fmt.Sprintf(`Проанализируй тональность этого текста. Ответь только одним словом: positive, neutral или negative.

Текст: %s`, text)

	yandexService := NewYandexAIService(&config.Config{
		YandexFolderID: s.FolderID,
		YandexAPIKey:   s.APIKey,
	})

	return yandexService.Ask(ctx, prompt)
}

// GenerateSummary - создание краткого содержания
func (s *SpeechKitService) GenerateSummary(ctx context.Context, text string) (string, error) {
	prompt := fmt.Sprintf(`Сделай краткое содержание этого разговора. Выдели основные темы и договоренности.

Текст: %s`, text)

	yandexService := NewYandexAIService(&config.Config{
		YandexFolderID: s.FolderID,
		YandexAPIKey:   s.APIKey,
	})

	return yandexService.Ask(ctx, prompt)
}

// ExtractKeyPoints - извлечение ключевых моментов
func (s *SpeechKitService) ExtractKeyPoints(ctx context.Context, text string) ([]string, error) {
	prompt := fmt.Sprintf(`Выдели ключевые моменты из этого разговора в виде списка.

Текст: %s

Формат ответа: каждый пункт с новой строки, начинается с дефиса.`, text)

	yandexService := NewYandexAIService(&config.Config{
		YandexFolderID: s.FolderID,
		YandexAPIKey:   s.APIKey,
	})

	result, err := yandexService.Ask(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Простая реализация - разделяем по строкам
	var points []string
	// TODO: добавить парсинг result в массив строк
	_ = result // заглушка
	
	return points, nil
}

// GenerateActionItems - создание задач по звонку
func (s *SpeechKitService) GenerateActionItems(ctx context.Context, text string) ([]string, error) {
	prompt := fmt.Sprintf(`Какие задачи нужно выполнить после этого разговора? Составь список.

Текст: %s

Формат ответа: каждый пункт с новой строки, начинается с дефиса.`, text)

	yandexService := NewYandexAIService(&config.Config{
		YandexFolderID: s.FolderID,
		YandexAPIKey:   s.APIKey,
	})

	result, err := yandexService.Ask(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var items []string
	_ = result // заглушка
	
	return items, nil
}