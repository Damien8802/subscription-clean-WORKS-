package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jackc/pgx/v5/pgxpool"
)

func main() {
    connString := "postgres://postgres:6213110@localhost:5432/GO?sslmode=disable"
    pool, err := pgxpool.New(context.Background(), connString)
    if err != nil {
        log.Fatal("❌ Ошибка подключения:", err)
    }
    defer pool.Close()

    fmt.Println("✅ Подключились к БД")

    // Добавляем колонку ai_capabilities
    _, err = pool.Exec(context.Background(), `
        ALTER TABLE subscription_plans 
        ADD COLUMN IF NOT EXISTS ai_capabilities JSONB NOT NULL 
        DEFAULT '{"model": "basic", "max_requests": 50, "file_upload": false, "voice_input": false}';
    `)
    
    if err != nil {
        log.Fatal("❌ Ошибка добавления колонки:", err)
    }

    fmt.Println("✅ Колонка ai_capabilities успешно добавлена")

    // Обновляем существующие тарифы с разными AI-возможностями
    _, err = pool.Exec(context.Background(), `
        UPDATE subscription_plans SET 
        ai_capabilities = '{"model": "basic", "max_requests": 50, "file_upload": false, "voice_input": false}'
        WHERE code = 'basic';
    `)
    if err != nil {
        log.Printf("⚠️ Ошибка обновления basic: %v", err)
    }

    _, err = pool.Exec(context.Background(), `
        UPDATE subscription_plans SET 
        ai_capabilities = '{"model": "gpt-3.5", "max_requests": 500, "file_upload": true, "voice_input": true}'
        WHERE code = 'pro';
    `)
    if err != nil {
        log.Printf("⚠️ Ошибка обновления pro: %v", err)
    }

    _, err = pool.Exec(context.Background(), `
        UPDATE subscription_plans SET 
        ai_capabilities = '{"model": "gpt-4", "max_requests": 5000, "file_upload": true, "voice_input": true}'
        WHERE code = 'enterprise';
    `)
    if err != nil {
        log.Printf("⚠️ Ошибка обновления enterprise: %v", err)
    }

    fmt.Println("✅ AI-возможности обновлены для всех тарифов")
}
