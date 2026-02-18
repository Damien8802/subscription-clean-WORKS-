package onecintegration

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"
)

// 1C Клиент
type OneCClient struct {
	baseURL    string
	login      string
	password   string
	httpClient *http.Client
}

// Новая структура для ответа 1С
type OneCResponse struct {
	Success bool        `json:"success" xml:"success"`
	Data    interface{} `json:"data" xml:"data"`
	Error   string      `json:"error,omitempty" xml:"error,omitempty"`
}

// Создаем клиент
func NewClient(baseURL, login, password string) *OneCClient {
	return &OneCClient{
		baseURL:  baseURL,
		login:    login,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Метод для отправки данных в 1С
func (c *OneCClient) SendData(endpoint string, data interface{}, format string) (*OneCResponse, error) {
	var body []byte
	var contentType string

	// Подготовка данных в нужном формате
	switch format {
	case "json":
		contentType = "application/json"
		body, _ = json.Marshal(data)
	case "xml":
		contentType = "application/xml"
		body, _ = xml.Marshal(data)
	default:
		contentType = "application/json"
		body, _ = json.Marshal(data)
	}

	// Формируем URL
	url := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	// Создаем запрос
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// Добавляем заголовки и авторизацию
	req.Header.Set("Content-Type", contentType)
	req.SetBasicAuth(c.login, c.password)

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Читаем ответ
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Парсим ответ
	var onecResp OneCResponse
	if format == "xml" {
		xml.Unmarshal(respBody, &onecResp)
	} else {
		json.Unmarshal(respBody, &onecResp)
	}

	return &onecResp, nil
}

// Тестовое соединение с 1С
func (c *OneCClient) TestConnection() (bool, error) {
	_, err := c.SendData("test", map[string]string{"action": "test"}, "json")
	if err != nil {
		return false, err
	}
	return true, nil
}
