package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port           string
	Env            string
	LogLevel       string
	StaticPath     string
	FrontendPath   string
	TemplatesPath  string
	Debug          bool
	TrustedProxies []string
	AllowedOrigins []string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	JWTSecret        string
	JWTRefreshSecret string
	JWTAccessExpiry  time.Duration
	JWTRefreshExpiry time.Duration

	SkipAuth bool // если true – отключает проверку JWT (для разработки)

	// API ключи для AI-агента
	OpenRouterAPIKey string // ключ для OpenRouter
	YandexFolderID   string
	YandexAPIKey     string
	GigaChatClientID string // для бизнес-версии (Client ID)
	GigaChatSecret   string // для бизнес-версии (Client Secret)
	GigaChatAuthKey  string // прямой ключ авторизации (для физических лиц)

	// SMTP для почтовых уведомлений
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	EmailFrom    string
}

func Load() *Config {
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		Env:            getEnv("GIN_MODE", "debug"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		StaticPath:     getEnv("STATIC_PATH", "./static"),
		FrontendPath:   getEnv("FRONTEND_PATH", "./frontend"),
		TemplatesPath:  getEnv("TEMPLATES_PATH", "./templates/*.html"),
		Debug:          getEnvAsBool("DEBUG", true),
		TrustedProxies: []string{},
		AllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "postgres"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		JWTSecret:        getEnv("JWT_ACCESS_SECRET", "default-access-secret"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "default-refresh-secret"),
		JWTAccessExpiry:  getEnvAsDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry: getEnvAsDuration("JWT_REFRESH_EXPIRY", 30*24*time.Hour),

		SkipAuth: getEnvAsBool("SKIP_AUTH", false),

		// AI ключи
		OpenRouterAPIKey: getEnv("OPENROUTER_API_KEY", ""),
		YandexFolderID:   getEnv("YANDEX_FOLDER_ID", ""),
		YandexAPIKey:     getEnv("YANDEX_API_KEY", ""),
		GigaChatClientID: getEnv("GIGACHAT_CLIENT_ID", ""),
		GigaChatSecret:   getEnv("GIGACHAT_CLIENT_SECRET", ""),
		GigaChatAuthKey:  getEnv("GIGACHAT_AUTH_KEY", ""),

		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.yandex.ru"),
		SMTPPort:     getEnvAsInt("SMTP_PORT", 587),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		EmailFrom:    getEnv("EMAIL_FROM", ""),
	}

	if proxies := getEnv("TRUSTED_PROXIES", ""); proxies != "" {
		cfg.TrustedProxies = strings.Split(proxies, ",")
	}

	log.Printf("📋 Конфигурация загружена: порт=%s, режим=%s, БД=%s, SkipAuth=%v, OpenRouterKeySet=%v",
		cfg.Port, cfg.Env, cfg.DBName, cfg.SkipAuth, cfg.OpenRouterAPIKey != "")
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	strVal := getEnv(key, "")
	if val, err := strconv.ParseBool(strVal); err == nil {
		return val
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	strVal := getEnv(key, "")
	if val, err := strconv.Atoi(strVal); err == nil {
		return val
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	strVal := getEnv(key, "")
	if val, err := time.ParseDuration(strVal); err == nil {
		return val
	}
	return defaultValue
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	val := getEnv(key, "")
	if val == "" {
		return defaultValue
	}
	parts := strings.Split(val, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}