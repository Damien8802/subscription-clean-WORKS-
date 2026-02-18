package handlers

import (
	"fmt"
	"net/http"
	"subscription-system/config"
	"subscription-system/database"
	"subscription-system/models"

	"github.com/gin-gonic/gin"
)

func MySubscriptionsPageHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		var id string
		rows, err := database.Pool.Query(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1")
		if err == nil && rows.Next() {
			rows.Scan(&id)
			userID = id
		}
		rows.Close()
		if userID == nil || userID == "" {
			c.HTML(http.StatusOK, "my-subscriptions.html", gin.H{
				"Title":         "Мои подписки - SaaSPro",
				"Version":       "3.0",
				"Subscriptions": []models.Subscription{},
			})
			return
		}
	}

	subs, err := models.GetUserSubscriptions(userID.(string))
	if err != nil {
		c.HTML(http.StatusOK, "my-subscriptions.html", gin.H{
			"Title":         "Мои подписки - SaaSPro",
			"Version":       "3.0",
			"Subscriptions": []models.Subscription{},
		})
		return
	}
	c.HTML(http.StatusOK, "my-subscriptions.html", gin.H{
		"Title":         "Мои подписки - SaaSPro",
		"Version":       "3.0",
		"Subscriptions": subs,
	})
}

func GetPlansHandler(c *gin.Context) {
	plans, err := models.GetAllActivePlans()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"plans": []models.Plan{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

type CreateSubscriptionRequest struct {
	PlanID      int `json:"plan_id" binding:"required"`
	PeriodMonth int `json:"period_month" binding:"required,oneof=1 12"`
}

func CreateSubscriptionHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		cfg := config.Load()
		if cfg.SkipAuth {
			// В режиме разработки берём первого пользователя
			var id string
			err := database.Pool.QueryRow(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1").Scan(&id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "no users found"})
				return
			}
			userID = id
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
	}

	var req CreateSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan, err := models.GetPlanByID(req.PlanID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}
	sub, err := models.CreateSubscription(userID.(string), req.PlanID, req.PeriodMonth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create subscription"})
		return
	}

	// AI: добавляем информацию о подписке в базу знаний
	metadata := map[string]interface{}{
		"subscription_id": sub.ID,
		"plan_id":         plan.ID,
		"plan_name":       plan.Name,
		"plan_price":      plan.PriceMonthly,
		"period_months":   req.PeriodMonth,
		"end_date":        sub.CurrentPeriodEnd,
	}
	content := fmt.Sprintf("Подписка на тариф '%s' активирована до %s", plan.Name, sub.CurrentPeriodEnd.Format("2006-01-02"))
	_ = models.AddDocument(userID.(string), "subscription", content, metadata)

	c.JSON(http.StatusCreated, gin.H{
		"subscription": sub,
		"plan":         plan,
	})
}

func GetUserSubscriptionsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		cfg := config.Load()
		if cfg.SkipAuth {
			// В режиме разработки берём первого пользователя
			var id string
			err := database.Pool.QueryRow(c.Request.Context(), "SELECT id FROM users ORDER BY created_at LIMIT 1").Scan(&id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "no users found"})
				return
			}
			userID = id
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
	}

	subs, err := models.GetUserSubscriptions(userID.(string))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"subscriptions": []models.Subscription{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}