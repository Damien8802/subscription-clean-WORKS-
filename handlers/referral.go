package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

func ReferralPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "referral.html", gin.H{
        "Title": "Рефералы - SaaSPro",
    })
}