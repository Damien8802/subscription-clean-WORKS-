package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    // Берём параметры из переменных окружения или используем значения по умолчанию
    dbHost := getEnv("DB_HOST", "localhost")
    dbPort := getEnv("DB_PORT", "5432")
    dbUser := getEnv("DB_USER", "postgres")
    dbPass := getEnv("DB_PASSWORD", "6213110")
    dbName := getEnv("DB_NAME", "GO")
    dbSSLMode := getEnv("DB_SSLMODE", "disable")

    dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
        dbHost, dbPort, dbUser, dbPass, dbName, dbSSLMode)

    pool, err := pgxpool.New(context.Background(), dsn)
    if err != nil {
        log.Fatal("❌ Ошибка подключения к БД:", err)
    }
    defer pool.Close()

    if err := pool.Ping(context.Background()); err != nil {
        log.Fatal("❌ Не удалось пинговать БД:", err)
    }

    fmt.Println("✅ Подключились к БД")

    // Получаем все тарифы
    rows, err := pool.Query(context.Background(), `
        SELECT id, name, code, description, price_monthly, price_yearly, 
               currency, features, ai_capabilities, max_users, is_active, 
               sort_order, created_at, updated_at
        FROM subscription_plans
        WHERE is_active = true
        ORDER BY sort_order
    `)
    if err != nil {
        log.Fatal("❌ Ошибка загрузки тарифов:", err)
    }
    defer rows.Close()

    type Plan struct {
        ID              int             `json:"id"`
        Name            string          `json:"name"`
        Code            string          `json:"code"`
        Description     string          `json:"description"`
        PriceMonthly    float64         `json:"price_monthly"`
        PriceYearly     float64         `json:"price_yearly"`
        Currency        string          `json:"currency"`
        Features        json.RawMessage `json:"features"`
        AICapabilities  json.RawMessage `json:"ai_capabilities"`
        MaxUsers        int             `json:"max_users"`
        IsActive        bool            `json:"is_active"`
        SortOrder       int             `json:"sort_order"`
        CreatedAt       time.Time       `json:"created_at"`  // ← исправлено на time.Time
        UpdatedAt       time.Time       `json:"updated_at"`  // ← исправлено на time.Time
    }

    // Функция для получения AI-возможностей
    getAICapabilities := func(capsJSON json.RawMessage) map[string]interface{} {
        var caps map[string]interface{}
        if err := json.Unmarshal(capsJSON, &caps); err != nil {
            return map[string]interface{}{
                "model":        "basic",
                "max_requests": float64(50),
                "file_upload":  false,
                "voice_input":  false,
            }
        }
        return caps
    }

    var plans []Plan

    for rows.Next() {
        var p Plan
        err := rows.Scan(
            &p.ID, &p.Name, &p.Code, &p.Description,
            &p.PriceMonthly, &p.PriceYearly, &p.Currency,
            &p.Features, &p.AICapabilities, &p.MaxUsers,
            &p.IsActive, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt,
        )
        if err != nil {
            log.Printf("⚠️ Ошибка сканирования: %v", err)
            continue
        }
        plans = append(plans, p)
    }

    fmt.Printf("✅ Найдено тарифов: %d\n\n", len(plans))

    for _, p := range plans {
        fmt.Printf("=== %s ===\n", p.Name)
        fmt.Printf("Код: %s\n", p.Code)
        fmt.Printf("Цена: %.2f ₽/мес\n", p.PriceMonthly)

        caps := getAICapabilities(p.AICapabilities)
        capsJSON, _ := json.MarshalIndent(caps, "", "  ")
        fmt.Printf("AI возможности: %s\n", string(capsJSON))

        // Вспомогательные функции
        getModel := func() string {
            if val, ok := caps["model"]; ok {
                if str, ok := val.(string); ok {
                    return str
                }
            }
            return "basic"
        }

        getMaxRequests := func() int64 {
            if val, ok := caps["max_requests"]; ok {
                switch v := val.(type) {
                case float64:
                    return int64(v)
                case int64:
                    return v
                }
            }
            return 50
        }

        canUseFeature := func(feature string) bool {
            if val, ok := caps[feature]; ok {
                if boolVal, ok := val.(bool); ok {
                    return boolVal
                }
            }
            return false
        }

        fmt.Printf("Модель: %s\n", getModel())
        fmt.Printf("Макс. запросов: %d\n", getMaxRequests())
        fmt.Printf("Загрузка файлов: %v\n", canUseFeature("file_upload"))
        fmt.Printf("Голосовой ввод: %v\n", canUseFeature("voice_input"))
        fmt.Println()
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}