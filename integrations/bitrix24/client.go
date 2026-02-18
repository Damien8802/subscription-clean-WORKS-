package bitrixintegration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Битрикс24 Клиент
type Bitrix24Client struct {
	webhookURL   string
	clientID     string
	clientSecret string
	portal       string
	httpClient   *http.Client
	accessToken  string
	tokenExpiry  time.Time
}

// Создаем клиент
func NewClient(webhookURL, clientID, clientSecret, portal string) *Bitrix24Client {
	return &Bitrix24Client{
		webhookURL:   webhookURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		portal:       portal,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Метод для вызова API Битрикс24
func (b *Bitrix24Client) CallMethod(method string, params map[string]interface{}) (map[string]interface{}, error) {
	// Формируем URL
	apiURL := fmt.Sprintf("https://%s/rest/%s", b.portal, method)
	if b.webhookURL != "" {
		apiURL = fmt.Sprintf("%s/%s", b.webhookURL, method)
	}

	// Добавляем access token если есть
	if b.accessToken != "" && time.Now().Before(b.tokenExpiry) {
		if params == nil {
			params = make(map[string]interface{})
		}
		params["auth"] = b.accessToken
	}

	// Преобразуем параметры в JSON
	jsonData, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	// Создаем запрос
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Отправляем запрос
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Парсим JSON ответ
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Проверяем на ошибки Битрикс
	if errorMsg, ok := result["error"]; ok {
		return nil, fmt.Errorf("bitrix error: %v", errorMsg)
	}

	return result, nil
}

// Создание сделки в Битрикс24
func (b *Bitrix24Client) CreateDeal(title string, contactID int, amount float64, stageID string) (int, error) {
	params := map[string]interface{}{
		"fields": map[string]interface{}{
			"TITLE":       title,
			"CONTACT_ID":  contactID,
			"OPPORTUNITY": amount,
			"CURRENCY_ID": "RUB",
			"CATEGORY_ID": 0, // Воронка по умолчанию
			"STAGE_ID":    stageID,
			"SOURCE_ID":   "WEB",
			"COMMENTS":    "Создано из подписочного сервиса",
		},
	}

	result, err := b.CallMethod("crm.deal.add", params)
	if err != nil {
		return 0, err
	}

	// Возвращаем ID созданной сделки
	if dealID, ok := result["result"].(float64); ok {
		return int(dealID), nil
	}

	return 0, fmt.Errorf("failed to get deal ID")
}

// Добавление контакта
func (b *Bitrix24Client) AddContact(name, email, phone string) (int, error) {
	params := map[string]interface{}{
		"fields": map[string]interface{}{
			"NAME":      name,
			"EMAIL":     []map[string]string{{"VALUE": email, "VALUE_TYPE": "WORK"}},
			"PHONE":     []map[string]string{{"VALUE": phone, "VALUE_TYPE": "WORK"}},
			"SOURCE_ID": "WEB",
		},
	}

	result, err := b.CallMethod("crm.contact.add", params)
	if err != nil {
		return 0, err
	}

	if contactID, ok := result["result"].(float64); ok {
		return int(contactID), nil
	}

	return 0, fmt.Errorf("failed to get contact ID")
}
