package handlers

import (
"net/http"
"subscription-system/models"

"github.com/gin-gonic/gin"
)

func PricingPageHandler(c *gin.Context) {
plans, err := models.GetAllActivePlans()
if err != nil {
// Если ошибка – показываем пустой список, но не 500
c.HTML(http.StatusOK, "pricing.html", gin.H{
"Title":   "Тарифы - SaaSPro",
"Version": "3.0",
"Plans":   []models.Plan{},
"Error":   "Не удалось загрузить тарифы",
})
return
}
c.HTML(http.StatusOK, "pricing.html", gin.H{
"Title":   "Тарифы - SaaSPro",
"Version": "3.0",
"Plans":   plans,
})
}
