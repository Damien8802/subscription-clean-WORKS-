package integrations

import (
	"os"
	"strconv"
)

// Config для всех интеграций
type IntegrationConfig struct {
	// 1C настройки
	OneCEnabled      bool
	OneCBaseURL      string
	OneCLogin        string
	OneCPassword     string
	OneCDatabase     string
	OneCSyncInterval int // минуты

	// Битрикс24 настройки
	BitrixEnabled      bool
	BitrixWebhookURL   string
	BitrixClientID     string
	BitrixClientSecret string
	BitrixPortal       string
}

// Загружаем конфиг из .env или переменных окружения
func LoadConfig() *IntegrationConfig {
	onecEnabled, _ := strconv.ParseBool(getEnv("ONEC_ENABLED", "false"))
	syncInterval, _ := strconv.Atoi(getEnv("ONEC_SYNC_INTERVAL", "60"))
	bitrixEnabled, _ := strconv.ParseBool(getEnv("BITRIX_ENABLED", "false"))

	return &IntegrationConfig{
		// 1C
		OneCEnabled:      onecEnabled,
		OneCBaseURL:      getEnv("ONEC_BASE_URL", "http://localhost:8088/ut_demo/ws/1c_ut8"),
		OneCLogin:        getEnv("ONEC_LOGIN", "admin"),
		OneCPassword:     getEnv("ONEC_PASSWORD", ""),
		OneCDatabase:     getEnv("ONEC_DATABASE", "UT_DEMO"),
		OneCSyncInterval: syncInterval,

		// Битрикс24
		BitrixEnabled:      bitrixEnabled,
		BitrixWebhookURL:   getEnv("BITRIX_WEBHOOK_URL", ""),
		BitrixClientID:     getEnv("BITRIX_CLIENT_ID", ""),
		BitrixClientSecret: getEnv("BITRIX_CLIENT_SECRET", ""),
		BitrixPortal:       getEnv("BITRIX_PORTAL", "yourcompany.bitrix24.ru"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
