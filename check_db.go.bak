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

    // Проверяем таблицы
    tables := []string{"users", "subscription_plans", "user_subscriptions", "api_keys", "referrals"}
    
    for _, table := range tables {
        var exists bool
        err := pool.QueryRow(context.Background(),
            "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
        if err != nil {
            fmt.Printf("❌ Ошибка проверки %s: %v\n", table, err)
            continue
        }
        if exists {
            fmt.Printf("✅ Таблица %s существует\n", table)
        } else {
            fmt.Printf("❌ Таблица %s НЕ существует\n", table)
        }
    }

    // Проверяем колонку ai_capabilities в subscription_plans
    var hasColumn bool
    err = pool.QueryRow(context.Background(),
        "SELECT EXISTS (SELECT FROM information_schema.columns WHERE table_name='subscription_plans' AND column_name='ai_capabilities')").Scan(&hasColumn)
    if err != nil {
        fmt.Printf("❌ Ошибка проверки колонки: %v\n", err)
    } else if hasColumn {
        fmt.Println("✅ Колонка ai_capabilities добавлена")
    } else {
        fmt.Println("❌ Колонки ai_capabilities нет — нужно добавить")
    }
}

