package models

import (
    "time"
)

type Referral struct {
    ID           string    `json:"id" db:"id"`
    UserID       string    `json:"user_id" db:"user_id"`                 // кто пригласил
    ReferredID   string    `json:"referred_id" db:"referred_id"`         // кто пришёл
    ReferredEmail string   `json:"referred_email" db:"referred_email"`
    Status       string    `json:"status" db:"status"`                   // pending, active, paid
    Commission   float64   `json:"commission" db:"commission"`           // сколько заработано
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
    ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`           // когда сгорает комиссия
}

type ReferralStats struct {
    TotalInvited   int     `json:"total_invited"`
    ActiveInvited  int     `json:"active_invited"`
    TotalEarned    float64 `json:"total_earned"`
    AvailableForWithdraw float64 `json:"available_for_withdraw"`
}