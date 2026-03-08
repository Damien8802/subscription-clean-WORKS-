package models

import (
	"time"
)

// AudioTranscription - аудиофайл и его транскрипция
type AudioTranscription struct {
	ID            string                 `json:"id" db:"id"`
	AccountID     string                 `json:"account_id" db:"account_id"`
	CustomerID    string                 `json:"customer_id" db:"customer_id"`
	DealID        string                 `json:"deal_id" db:"deal_id"`
	Filename      string                 `json:"filename" db:"filename"`
	FileSize      int64                  `json:"file_size" db:"file_size"`
	Duration      int                    `json:"duration" db:"duration"`
	AudioURL      string                 `json:"audio_url" db:"audio_url"`
	Transcription string                 `json:"transcription" db:"transcription"`
	Summary       string                 `json:"summary" db:"summary"`
	Sentiment     string                 `json:"sentiment" db:"sentiment"`
	KeyPoints     []string               `json:"key_points" db:"key_points"`
	ActionItems   []string               `json:"action_items" db:"action_items"`
	Status        string                 `json:"status" db:"status"`
	Metadata      map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt     time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at" db:"updated_at"`
}

// CallAnalytics - аналитика звонка
type CallAnalytics struct {
	ID               string                 `json:"id" db:"id"`
	TranscriptionID  string                 `json:"transcription_id" db:"transcription_id"`
	TalkRatio        map[string]float64     `json:"talk_ratio" db:"talk_ratio"` // manager, client
	SilencePercentage float64                `json:"silence_percentage" db:"silence_percentage"`
	Interruptions    int                    `json:"interruptions" db:"interruptions"`
	Keywords         []string               `json:"keywords" db:"keywords"`
	QuestionsAsked   int                    `json:"questions_asked" db:"questions_asked"`
	ObjectionsRaised int                    `json:"objections_raised" db:"objections_raised"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
}

// TranscriptionRequest - запрос на транскрибацию
type TranscriptionRequest struct {
	AudioURL   string `json:"audio_url" binding:"required"`
	CustomerID string `json:"customer_id"`
	DealID     string `json:"deal_id"`
	Language   string `json:"language" default:"ru-RU"`
}