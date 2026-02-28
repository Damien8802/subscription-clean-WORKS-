package handlers

import (
    "net/http"
    "subscription-system/models"

    "github.com/gin-gonic/gin"
)

// GetPlansHandler возвращает список всех тарифов (API)
func GetPlansHandler(c *gin.Context) {
    plans, err := models.GetAllPlans()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "plans":   plans,
    })
}