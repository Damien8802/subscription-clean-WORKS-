package models

import (
    "time"
    "github.com/google/uuid"
)

type BankAccount struct {
    ID           uuid.UUID `json:"id" db:"id"`
    CompanyID    uuid.UUID `json:"company_id" db:"company_id"`
    BankName     string    `json:"bank_name" db:"bank_name"` // Тинькофф, Альфа, Сбер, ВТБ
    AccountNumber string   `json:"account_number" db:"account_number"`
    BIC          string    `json:"bic" db:"bic"`
    APIKey       string    `json:"api_key" db:"api_key"` // Токен для доступа
    IsActive     bool      `json:"is_active" db:"is_active"`
    LastSync     time.Time `json:"last_sync" db:"last_sync"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type BankTransaction struct {
    ID              uuid.UUID   `json:"id" db:"id"`
    AccountID       uuid.UUID   `json:"account_id" db:"account_id"`
    TransactionDate time.Time   `json:"transaction_date" db:"transaction_date"`
    Amount          float64     `json:"amount" db:"amount"`
    Description     string      `json:"description" db:"description"`
    Counterparty    string      `json:"counterparty" db:"counterparty"`
    Purpose         string      `json:"purpose" db:"purpose"`
    IsMatched       bool        `json:"is_matched" db:"is_matched"`
    MatchedEntryID  *uuid.UUID  `json:"matched_entry_id" db:"matched_entry_id"`
    CreatedAt       time.Time   `json:"created_at" db:"created_at"`
}

type BankSyncLog struct {
    ID          uuid.UUID `json:"id" db:"id"`
    AccountID   uuid.UUID `json:"account_id" db:"account_id"`
    Status      string    `json:"status" db:"status"` // success, error
    RecordsCount int      `json:"records_count" db:"records_count"`
    ErrorMsg    string    `json:"error_msg" db:"error_msg"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
}