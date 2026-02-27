package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

func SecurityPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "security.html", gin.H{
        "Title": "Безопасность - SaaSPro",
    })
}