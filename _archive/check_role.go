package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/joho/godotenv"
)

func main() {
    if err := godotenv.Load(); err != nil {
        log.Fatal("Ошибка загрузки .env:", err)
    }
    connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
        os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_HOST"), os.Getenv("DB_PORT"),
        os.Getenv("DB_NAME"), os.Getenv("DB_SSLMODE"))

    db, err := pgxpool.New(context.Background(), connString)
    if err != nil {
        log.Fatal("Ошибка подключения:", err)
    }
    defer db.Close()

    telegramID := int64(1977550186) // ваш ID
    var role string
    err = db.QueryRow(context.Background(),
        "SELECT role FROM users WHERE telegram_id = $1", telegramID).Scan(&role)
    if err != nil {
        log.Fatal("Пользователь с таким telegram_id не найден:", err)
    }
    fmt.Printf("Роль пользователя: %s\n", role)
}
