package handlers

import (
    "log"
    "time"
)

// StartTeamSphereScheduler - запускает планировщик TeamSphere
func StartTeamSphereScheduler() {
    log.Println("🤖 Планировщик синхронизации с TeamSphere запущен")
    
    // Запускаем периодическую синхронизацию каждые 5 минут
    ticker := time.NewTicker(5 * time.Minute)
    
    go func() {
        // Первый запуск сразу
        log.Println("🔄 TeamSphere: проверка задач...")
        
        for range ticker.C {
            log.Println("🔄 TeamSphere: проверка задач...")
            // Здесь будет логика синхронизации TeamSphere
        }
    }()
}