package models

import (
	"time"
)

// AnalyticsMetric - метрика
type AnalyticsMetric struct {
	ID         string                 `json:"id" db:"id"`
	AccountID  string                 `json:"account_id" db:"account_id"`
	MetricDate time.Time              `json:"metric_date" db:"metric_date"`
	MetricType string                 `json:"metric_type" db:"metric_type"`
	Value      float64                `json:"value" db:"value"`
	Metadata   map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt  time.Time              `json:"created_at" db:"created_at"`
}

// AnalyticsCohort - когорта
type AnalyticsCohort struct {
	ID            string    `json:"id" db:"id"`
	AccountID     string    `json:"account_id" db:"account_id"`
	CohortDate    time.Time `json:"cohort_date" db:"cohort_date"`
	CohortSize    int       `json:"cohort_size" db:"cohort_size"`
	Period        int       `json:"period" db:"period"`
	RetentionRate float64   `json:"retention_rate" db:"retention_rate"`
	Revenue       float64   `json:"revenue" db:"revenue"`
}

// ChurnPrediction - прогноз оттока
type ChurnPrediction struct {
	ID               string                 `json:"id" db:"id"`
	AccountID        string                 `json:"account_id" db:"account_id"`
	CustomerID       string                 `json:"customer_id" db:"customer_id"`
	ChurnProbability float64                `json:"churn_probability" db:"churn_probability"`
	RiskLevel        string                 `json:"risk_level" db:"risk_level"`
	Factors          map[string]interface{} `json:"factors" db:"factors"`
	PredictedDate    time.Time              `json:"predicted_date" db:"predicted_date"`
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`
}

// RFMAnalysis - RFM-анализ (Recency, Frequency, Monetary)
type RFMAnalysis struct {
	CustomerID    string  `json:"customer_id"`
	CustomerName  string  `json:"customer_name"`
	Recency       int     `json:"recency"`       // дней с последней покупки
	Frequency     int     `json:"frequency"`     // количество покупок
	Monetary      float64 `json:"monetary"`      // сумма покупок
	RFMScore      string  `json:"rfm_score"`     // 111, 112, ... 555
	Segment       string  `json:"segment"`       // Champions, Loyal, etc
	SegmentColor  string  `json:"segment_color"` // цвет для UI
}

// LTVPrediction - прогноз пожизненной ценности клиента
type LTVPrediction struct {
    ID              string                 `json:"id" db:"id"`
    AccountID       string                 `json:"account_id" db:"account_id"`
    CustomerID      string                 `json:"customer_id" db:"customer_id"`
    PredictedLTV    float64                `json:"predicted_ltv" db:"predicted_ltv"`
    Confidence      float64                `json:"confidence" db:"confidence"`
    Factors         map[string]interface{} `json:"factors" db:"factors"`
    PredictionDate  time.Time              `json:"prediction_date" db:"prediction_date"`
    CreatedAt       time.Time              `json:"created_at" db:"created_at"`
}

// CohortRevenue - доход по когортам
type CohortRevenue struct {
    ID                string    `json:"id" db:"id"`
    AccountID         string    `json:"account_id" db:"account_id"`
    CohortDate        time.Time `json:"cohort_date" db:"cohort_date"`
    Period            int       `json:"period" db:"period"`
    RevenuePerCustomer float64   `json:"revenue_per_customer" db:"revenue_per_customer"`
    TotalRevenue      float64   `json:"total_revenue" db:"total_revenue"`
    CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// ABTest - A/B тест
type ABTest struct {
    ID          string                 `json:"id" db:"id"`
    AccountID   string                 `json:"account_id" db:"account_id"`
    Name        string                 `json:"name" db:"name"`
    Description string                 `json:"description" db:"description"`
    VariantA    string                 `json:"variant_a" db:"variant_a"`
    VariantB    string                 `json:"variant_b" db:"variant_b"`
    Metric      string                 `json:"metric" db:"metric"`
    Results     map[string]interface{} `json:"results" db:"results"`
    Status      string                 `json:"status" db:"status"`
    StartedAt   *time.Time             `json:"started_at" db:"started_at"`
    EndedAt     *time.Time             `json:"ended_at" db:"ended_at"`
    CreatedAt   time.Time              `json:"created_at" db:"created_at"`
}

// ABTestVariant - вариант теста для создания
type ABTestVariant struct {
    Name        string `json:"name"`
    Description string `json:"description"`
    Traffic     int    `json:"traffic"` // процент трафика
}

// Dashboard - пользовательский дашборд
type Dashboard struct {
    ID        string                 `json:"id" db:"id"`
    AccountID string                 `json:"account_id" db:"account_id"`
    UserID    string                 `json:"user_id" db:"user_id"`
    Name      string                 `json:"name" db:"name"`
    Config    map[string]interface{} `json:"config" db:"config"`
    IsDefault bool                   `json:"is_default" db:"is_default"`
    CreatedAt time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt time.Time              `json:"updated_at" db:"updated_at"`
}

// DashboardWidget - виджет дашборда
type DashboardWidget struct {
    ID       string                 `json:"id"`
    Type     string                 `json:"type"` // chart, metric, table, list
    Title    string                 `json:"title"`
    Size     string                 `json:"size"` // small, medium, large
    Position int                    `json:"position"`
    Config   map[string]interface{} `json:"config"`
    Data     map[string]interface{} `json:"data,omitempty"`
}

// Insight - инсайт для бизнеса
type Insight struct {
    ID          string    `json:"id"`
    AccountID   string    `json:"account_id"`
    Type        string    `json:"type"` // opportunity, risk, trend
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Impact      string    `json:"impact"` // high, medium, low
    Metric      float64   `json:"metric"`
    Action      string    `json:"action"` // что можно сделать
    CreatedAt   time.Time `json:"created_at"`
}