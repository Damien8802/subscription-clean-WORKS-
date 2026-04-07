package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

func GetCurrentUserID(c *gin.Context) {
    userID := getUserID(c)
    c.JSON(http.StatusOK, gin.H{
        "user_id": userID.String(),
        "is_admin": isAdmin(c),
    })
}