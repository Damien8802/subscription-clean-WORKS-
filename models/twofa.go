package models

import (
    "time"
)

type TwoFA struct {
    ID        string    `json:"id" db:"id"`
    UserID    string    `json:"user_id" db:"user_id"`
    Secret    string    `json:"-" db:"secret"` // Не отдаём в JSON
    Enabled   bool      `json:"enabled" db:"enabled"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func (TwoFA) TableName() string {
    return "twofa"
}