package handlers

import (
"net/http"
"subscription-system/models"

"github.com/gin-gonic/gin"
"subscription-system/config"
"subscription-system/database"
)

func GetUserAIUsageHandler(c *gin.Context) {
userID, exists := c.Get("userID")
if !exists {
cfg := config.Load()
if cfg.SkipAuth {
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

usage, err := models.GetUserAIUsageByModel(userID.(string))
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get usage"})
return
}
c.JSON(http.StatusOK, gin.H{"usage": usage})
}
