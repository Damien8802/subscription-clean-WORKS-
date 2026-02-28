package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "subscription-system/models"
)

// PricingPageHandler отображает страницу с тарифами
func PricingPageHandler(c *gin.Context) {
    plans, err := models.GetAllPlans()
    if err != nil {
        c.HTML(http.StatusInternalServerError, "pricing.html", gin.H{
            "Title": "Ошибка",
            "Error": "Не удалось загрузить тарифы",
        })
        return
    }

    c.HTML(http.StatusOK, "pricing.html", gin.H{
        "Title": "Тарифы - SaaSPro",
        "Plans": plans,
    })
}