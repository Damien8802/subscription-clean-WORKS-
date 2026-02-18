package onecintegration

import (
	"log"
	"time"
)

// Структура для синхронизации
type SyncManager struct {
	client   *OneCClient
	interval time.Duration
	stopChan chan bool
}

// Создаем менеджер синхронизации
func NewSyncManager(client *OneCClient, intervalMinutes int) *SyncManager {
	return &SyncManager{
		client:   client,
		interval: time.Duration(intervalMinutes) * time.Minute,
		stopChan: make(chan bool),
	}
}

// Запускаем синхронизацию в фоне
func (sm *SyncManager) Start() {
	log.Println("[1C] Starting synchronization service...")

	// Сначала синхронизируем сразу
	sm.SyncAll()

	// Потом по расписанию
	ticker := time.NewTicker(sm.interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				sm.SyncAll()
			case <-sm.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Останавливаем синхронизацию
func (sm *SyncManager) Stop() {
	sm.stopChan <- true
	log.Println("[1C] Synchronization service stopped")
}

// Основная синхронизация
func (sm *SyncManager) SyncAll() {
	log.Println("[1C] Starting sync cycle...")

	// 1. Синхронизация пользователей
	if err := sm.SyncUsers(); err != nil {
		log.Printf("[1C] Error syncing users: %v", err)
	}

	// 2. Синхронизация платежей
	if err := sm.SyncPayments(); err != nil {
		log.Printf("[1C] Error syncing payments: %v", err)
	}

	// 3. Синхронизация подписок
	if err := sm.SyncSubscriptions(); err != nil {
		log.Printf("[1C] Error syncing subscriptions: %v", err)
	}

	log.Println("[1C] Sync cycle completed")
}

// Синхронизация пользователей
func (sm *SyncManager) SyncUsers() error {
	log.Println("[1C] Syncing users...")

	// Здесь получаем пользователей из вашей БД
	// users := db.GetUsersForSync()

	// Отправляем в 1С
	// data := map[string]interface{}{"users": users}
	// _, err := sm.client.SendData("sync/users", data, "json")

	return nil // временно
}

// Синхронизация платежей
func (sm *SyncManager) SyncPayments() error {
	log.Println("[1C] Syncing payments...")
	// Реализация
	return nil
}

// Синхронизация подписок
func (sm *SyncManager) SyncSubscriptions() error {
	log.Println("[1C] Syncing subscriptions...")
	// Реализация
	return nil
}
