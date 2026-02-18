package handlers

import (
    "log"
    "net/http"
    "subscription-system/database"
    "time"

    "github.com/gin-gonic/gin"
)

type StatsResponse struct {
    TotalUsers         int `json:"total_users"`
    ActiveSubscriptions int `json:"active_subscriptions"`
    TotalAIRequests    int `json:"total_ai_requests"`
    TotalAPIKeys       int `json:"total_api_keys"`
}

func AdminStatsHandler(c *gin.Context) {
    var stats StatsResponse

    _ = database.Pool.QueryRow(c.Request.Context(),
        `SELECT COUNT(*) FROM users`).Scan(&stats.TotalUsers)

    _ = database.Pool.QueryRow(c.Request.Context(),
        `SELECT COUNT(*) FROM user_subscriptions WHERE status = 'active'`).Scan(&stats.ActiveSubscriptions)

    _ = database.Pool.QueryRow(c.Request.Context(),
        `SELECT COUNT(*) FROM ai_requests`).Scan(&stats.TotalAIRequests)

    _ = database.Pool.QueryRow(c.Request.Context(),
        `SELECT COUNT(*) FROM api_keys`).Scan(&stats.TotalAPIKeys)

    c.JSON(http.StatusOK, stats)
}

func AdminUsersHandler(c *gin.Context) {
    rows, err := database.Pool.Query(c.Request.Context(),
        `SELECT id, email, name, role, telegram_id, telegram_username, is_active, created_at
         FROM users
         ORDER BY created_at DESC
         LIMIT 20`)
    if err != nil {
        log.Printf("AdminUsersHandler query error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    defer rows.Close()

    type UserInfo struct {
        ID               string     `json:"id"`
        Email            string     `json:"email"`
        Name             *string    `json:"name"`
        Role             string     `json:"role"`
        TelegramID       *int64     `json:"telegram_id"`
        TelegramUsername *string    `json:"telegram_username"`
        IsActive         bool       `json:"is_active"`
        CreatedAt        time.Time  `json:"created_at"` // используем time.Time
    }

    var users []UserInfo
    for rows.Next() {
        var u UserInfo
        if err := rows.Scan(
            &u.ID,
            &u.Email,
            &u.Name,
            &u.Role,
            &u.TelegramID,
            &u.TelegramUsername,
            &u.IsActive,
            &u.CreatedAt,
        ); err != nil {
            log.Printf("AdminUsersHandler scan error: %v", err) // выводим ошибку в консоль
            c.JSON(http.StatusInternalServerError, gin.H{"error": "scan error"})
            return
        }
        users = append(users, u)
    }
    c.JSON(http.StatusOK, gin.H{"users": users})
}

func AdminToggleUserBlockHandler(c *gin.Context) {
    userID := c.Param("id")
    var req struct {
        IsActive bool `json:"is_active"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(),
        `UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2`,
        req.IsActive, userID)
    if err != nil {
        log.Printf("AdminToggleUserBlockHandler exec error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "user updated"})
}

func AdminBroadcastHandler(c *gin.Context) {
    var req struct {
        Message string `json:"message" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(),
        `SELECT telegram_id FROM users WHERE telegram_id IS NOT NULL`)
    if err != nil {
        log.Printf("AdminBroadcastHandler query error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
        return
    }
    defer rows.Close()

    var telegramIDs []int64
    for rows.Next() {
        var tid int64
        if err := rows.Scan(&tid); err != nil {
            log.Printf("AdminBroadcastHandler scan error: %v", err)
            continue
        }
        telegramIDs = append(telegramIDs, tid)
    }

    c.JSON(http.StatusOK, gin.H{
        "message":    "Broadcast prepared",
        "recipients": telegramIDs,
    })
}
