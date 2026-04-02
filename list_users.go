package main

import (
    "context"
    "fmt"
    "log"
    "subscription-system/database"
    "subscription-system/config"
)

func main() {
    cfg := config.Load()
    
    err := database.InitDB(cfg)
    if err != nil {
        log.Fatal("DB init failed:", err)
    }
    defer database.CloseDB()
    
    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, email, name, role FROM users ORDER BY created_at
    `)
    if err != nil {
        log.Fatal("Query failed:", err)
    }
    defer rows.Close()
    
    fmt.Println("=== СПИСОК ПОЛЬЗОВАТЕЛЕЙ ===\n")
    for rows.Next() {
        var id, email, name, role string
        rows.Scan(&id, &email, &name, &role)
        fmt.Printf("ID: %s\nEmail: %s\nName: %s\nRole: %s\n---\n", id, email, name, role)
    }
}
