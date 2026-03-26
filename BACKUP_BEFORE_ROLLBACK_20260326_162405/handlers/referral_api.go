package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

type ReferralStats struct {
    Invited   int     `json:"invited"`
    Active    int     `json:"active"`
    Earned    float64 `json:"earned"`
    Available float64 `json:"available"`
}

type ReferralFriend struct {
    Date   string  `json:"date"`
    Email  string  `json:"email"`
    Status string  `json:"status"`
    Bonus  float64 `json:"bonus"`
}

func GetReferralStatsHandler(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    // В реальном проекте здесь запрос к БД
    stats := ReferralStats{
        Invited:   5,
        Active:    3,
        Earned:    1500,
        Available: 1200,
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "stats":   stats,
    })
}

func GetReferralFriendsHandler(c *gin.Context) {
    userID := c.Query("user_id")
    if userID == "" {
        userID = "test-user-123"
    }

    friends := []ReferralFriend{
        {Date: "15.03.2024", Email: "alex@example.com", Status: "active", Bonus: 300},
        {Date: "10.03.2024", Email: "maria@example.com", Status: "active", Bonus: 300},
        {Date: "05.03.2024", Email: "ivan@example.com", Status: "pending", Bonus: 0},
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "friends": friends,
    })
}