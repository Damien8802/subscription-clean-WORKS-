package handlers

import (
	"net/http"
	"time"

	"subscription-system/database"
	"subscription-system/models"
	"subscription-system/services"

	"github.com/gin-gonic/gin"
)

// GetDashboardAnalytics - основная аналитика для дашборда
func GetDashboardAnalytics(c *gin.Context) {
	accountID := GetAccountID(c)
	
	// Получаем период из запроса (по умолчанию месяц)
	period := c.DefaultQuery("period", "month")
	
	var days int
	switch period {
	case "week":
		days = 7
	case "month":
		days = 30
	case "quarter":
		days = 90
	case "year":
		days = 365
	default:
		days = 30
	}
	
	// Доход за период
	revenueQuery := `
		SELECT COALESCE(SUM(amount), 0)
		FROM payments
		WHERE account_id = $1 
			AND status = 'completed'
			AND created_at >= NOW() - INTERVAL '1 day' * $2
	`
	var revenue float64
	database.Pool.QueryRow(c.Request.Context(), revenueQuery, accountID, days).Scan(&revenue)
	
	// Новые клиенты
	customersQuery := `
		SELECT COUNT(*)
		FROM customers
		WHERE account_id = $1 
			AND created_at >= NOW() - INTERVAL '1 day' * $2
	`
	var newCustomers int
	database.Pool.QueryRow(c.Request.Context(), customersQuery, accountID, days).Scan(&newCustomers)
	
	// Активные подписки
	subscriptionsQuery := `
		SELECT COUNT(*)
		FROM subscriptions
		WHERE account_id = $1 AND status = 'active'
	`
	var activeSubscriptions int
	database.Pool.QueryRow(c.Request.Context(), subscriptionsQuery, accountID).Scan(&activeSubscriptions)
	
	// Средний чек
	var avgCheck float64
	if newCustomers > 0 {
		avgCheck = revenue / float64(newCustomers)
	}
	
	// Данные для графика доходов
	chartQuery := `
		SELECT 
			DATE(created_at) as date,
			COALESCE(SUM(amount), 0) as daily_revenue
		FROM payments
		WHERE account_id = $1 
			AND status = 'completed'
			AND created_at >= NOW() - INTERVAL '1 day' * $2
		GROUP BY DATE(created_at)
		ORDER BY date
	`
	
	rows, err := database.Pool.Query(c.Request.Context(), chartQuery, accountID, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	var dates []string
	var revenues []float64
	
	for rows.Next() {
		var date time.Time
		var dailyRevenue float64
		
		if err := rows.Scan(&date, &dailyRevenue); err != nil {
			continue
		}
		
		dates = append(dates, date.Format("02.01"))
		revenues = append(revenues, dailyRevenue)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"revenue":             revenue,
		"new_customers":       newCustomers,
		"active_subscriptions": activeSubscriptions,
		"avg_check":           avgCheck,
		"chart_labels":        dates,
		"chart_data":          revenues,
		"period":              period,
	})
}

// GetRFMAnalysis - RFM-анализ клиентов
func GetRFMAnalysis(c *gin.Context) {
	accountID := GetAccountID(c)
	
	analyticsService := services.NewAnalyticsService()
	rfmData, err := analyticsService.CalculateRFMAnalysis(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Группируем по сегментам
	segments := make(map[string]map[string]interface{})
	for _, rfm := range rfmData {
		if _, exists := segments[rfm.Segment]; !exists {
			segments[rfm.Segment] = map[string]interface{}{
				"name":        rfm.Segment,
				"color":       rfm.SegmentColor,
				"count":       0,
				"total_value": 0.0,
				"customers":   []models.RFMAnalysis{},
			}
		}
		
		seg := segments[rfm.Segment]
		seg["count"] = seg["count"].(int) + 1
		seg["total_value"] = seg["total_value"].(float64) + rfm.Monetary
		seg["customers"] = append(seg["customers"].([]models.RFMAnalysis), rfm)
	}
	
	// Преобразуем в массив для фронтенда
	segmentArray := make([]map[string]interface{}, 0, len(segments))
	for _, seg := range segments {
		segmentArray = append(segmentArray, seg)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"segments": segmentArray,
		"total":    len(rfmData),
	})
}

// GetChurnPrediction - прогноз оттока
func GetChurnPrediction(c *gin.Context) {
	accountID := GetAccountID(c)
	
	query := `
		SELECT 
			c.id,
			c.name,
			c.email,
			cp.churn_probability,
			cp.risk_level,
			cp.factors,
			cp.predicted_date
		FROM analytics_churn_predictions cp
		JOIN customers c ON c.id = cp.customer_id
		WHERE cp.account_id = $1
		ORDER BY cp.churn_probability DESC
		LIMIT 20
	`
	
	rows, err := database.Pool.Query(c.Request.Context(), query, accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	var highRisk []gin.H
	var mediumRisk []gin.H
	var lowRisk []gin.H
	
	for rows.Next() {
		var id, name, email, riskLevel string
		var probability float64
		var factors map[string]interface{}
		var predictedDate time.Time
		
		if err := rows.Scan(&id, &name, &email, &probability, &riskLevel, &factors, &predictedDate); err != nil {
			continue
		}
		
		customer := gin.H{
			"id":              id,
			"name":            name,
			"email":           email,
			"probability":     probability,
			"probability_pct": int(probability * 100),
			"factors":         factors,
			"predicted_date":  predictedDate.Format("02.01.2006"),
		}
		
		switch riskLevel {
		case "high":
			highRisk = append(highRisk, customer)
		case "medium":
			mediumRisk = append(mediumRisk, customer)
		default:
			lowRisk = append(lowRisk, customer)
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"high_risk":  highRisk,
		"medium_risk": mediumRisk,
		"low_risk":   lowRisk,
	})
}

// GetCohortAnalysis - когортный анализ
func GetCohortAnalysis(c *gin.Context) {
	accountID := GetAccountID(c)
	
	query := `
		SELECT 
			cohort_date,
			period,
			cohort_size,
			retention_rate
		FROM analytics_cohorts
		WHERE account_id = $1
		ORDER BY cohort_date, period
	`
	
	rows, err := database.Pool.Query(c.Request.Context(), query, accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	
	type CohortData struct {
		Date     string    `json:"date"`
		Periods  []int     `json:"periods"`
		Retentions []float64 `json:"retentions"`
		Sizes    []int     `json:"sizes"`
	}
	
	cohorts := make(map[string]*CohortData)
	
	for rows.Next() {
		var cohortDate time.Time
		var period int
		var cohortSize int
		var retentionRate float64
		
		if err := rows.Scan(&cohortDate, &period, &cohortSize, &retentionRate); err != nil {
			continue
		}
		
		dateKey := cohortDate.Format("01.2006")
		if _, exists := cohorts[dateKey]; !exists {
			cohorts[dateKey] = &CohortData{
				Date:       dateKey,
				Periods:    []int{},
				Retentions: []float64{},
				Sizes:      []int{},
			}
		}
		
		cohorts[dateKey].Periods = append(cohorts[dateKey].Periods, period)
		cohorts[dateKey].Retentions = append(cohorts[dateKey].Retentions, retentionRate)
		cohorts[dateKey].Sizes = append(cohorts[dateKey].Sizes, cohortSize)
	}
	
	// Преобразуем в массив
	cohortArray := make([]*CohortData, 0, len(cohorts))
	for _, cohort := range cohorts {
		cohortArray = append(cohortArray, cohort)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"cohorts": cohortArray,
	})
}