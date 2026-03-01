package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// TrustedDevicesHandler отображает страницу доверенных устройств
func TrustedDevicesHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "trusted_devices.html", gin.H{
        "Title": "Доверенные устройства | SaaSPro",
    })
}