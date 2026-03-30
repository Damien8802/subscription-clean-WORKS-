package models

import (
    "time"
    "github.com/google/uuid"
    "github.com/lib/pq"
)

type WebAuthnCredential struct {
    ID              uuid.UUID      `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
    UserID          uuid.UUID      `gorm:"not null;type:uuid"`
    CredentialID    string         `gorm:"unique;not null"`
    PublicKey       []byte         `gorm:"not null"`
    AttestationType string         `gorm:"column:attestation_type"`
    Transport       pq.StringArray `gorm:"type:text[]"`
    AAGUID          string
    CreatedAt       time.Time
    LastUsed        *time.Time
    Name            string
}