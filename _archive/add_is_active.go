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

    _, err = db.Exec(context.Background(),
        `ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true`)
    if err != nil {
        log.Fatal("Ошибка добавления колонки is_active:", err)
    }
    fmt.Println("✅ Колонка is_active добавлена (если не существовала)")
}
