package handlers

import (
    "context"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "subscription-system/database"
)

// ReferralProgram представляет партнёрскую программу
type ReferralProgram struct {
    ID                string    `json:"id"`
    UserID            string    `json:"user_id"`
    ReferralLink      string    `json:"referral_link"`
    CommissionPercent int       `json:"commission_percent"` // 20% например
    TotalEarned       int64     `json:"total_earned"`       // в Stars
    TotalReferred     int       `json:"total_referred"`
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}

// ReferralCommission представляет комиссию за реферала
type ReferralCommission struct {
    ID          string    `json:"id"`
    ReferrerID  string    `json:"referrer_id"`
    ReferredID  string    `json:"referred_id"`
    Amount      int64     `json:"amount"`      // в Stars
    Status      string    `json:"status"`      // pending, paid
    CreatedAt   time.Time `json:"created_at"`
    PaidAt      *time.Time `json:"paid_at,omitempty"`
}

// CreateReferralProgram создаёт партнёрскую программу для пользователя
func CreateReferralProgram(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    var req struct {
        CommissionPercent int `json:"commission_percent" binding:"required,min=5,max=50"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Генерируем уникальную ссылку
    referralLink := generateReferralLink(userID.(string))

    // Сохраняем в БД
    var program ReferralProgram
    err := database.Pool.QueryRow(c.Request.Context(),
        `INSERT INTO referral_programs (user_id, referral_link, commission_percent, created_at, updated_at)
         VALUES ($1, $2, $3, NOW(), NOW())
         ON CONFLICT (user_id) DO UPDATE 
         SET commission_percent = $3, updated_at = NOW(), referral_link = $2
         RETURNING id, user_id, referral_link, commission_percent, COALESCE(total_earned, 0), COALESCE(total_referred, 0), created_at, updated_at`,
        userID, referralLink, req.CommissionPercent).Scan(
        &program.ID, &program.UserID, &program.ReferralLink, &program.CommissionPercent,
        &program.TotalEarned, &program.TotalReferred, &program.CreatedAt, &program.UpdatedAt)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create referral program"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "program": program,
        "message": "Партнёрская программа создана! Делитесь ссылкой и зарабатывайте Stars",
    })
}

// GetReferralProgram возвращает информацию о партнёрской программе пользователя
func GetReferralProgram(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    var program ReferralProgram
    err := database.Pool.QueryRow(c.Request.Context(),
        `SELECT id, user_id, referral_link, commission_percent, total_earned, total_referred, created_at, updated_at
         FROM referral_programs WHERE user_id = $1`,
        userID).Scan(
        &program.ID, &program.UserID, &program.ReferralLink, &program.CommissionPercent,
        &program.TotalEarned, &program.TotalReferred, &program.CreatedAt, &program.UpdatedAt)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "No referral program found"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "program": program,
    })
}

// GetReferralCommissions возвращает историю комиссий
func GetReferralCommissions(c *gin.Context) {
    userID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(),
        `SELECT rc.id, rc.referrer_id, rc.referred_id, rc.amount, rc.status, rc.created_at, rc.paid_at,
                u.email, u.name
         FROM referral_commissions rc
         JOIN users u ON rc.referred_id = u.id
         WHERE rc.referrer_id = $1
         ORDER BY rc.created_at DESC`,
        userID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    var commissions []gin.H
    for rows.Next() {
        var id, referrerID, referredID, email, name string
        var amount int64
        var status string
        var createdAt, paidAt *time.Time

        rows.Scan(&id, &referrerID, &referredID, &amount, &status, &createdAt, &paidAt, &email, &name)

        commissions = append(commissions, gin.H{
            "id":           id,
            "referred":     name + " (" + email + ")",
            "amount":       amount,
            "status":       status,
            "created_at":   createdAt,
            "paid_at":      paidAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "success":     true,
        "commissions": commissions,
    })
}

// ProcessReferral обрабатывает переход по реферальной ссылке
func ProcessReferral(c *gin.Context) {
    referralCode := c.Query("ref")
    if referralCode == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "referral code required"})
        return
    }

    // Сохраняем referrer в куки или сессию
    c.SetCookie("referrer", referralCode, 30*24*60*60, "/", "", false, true)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Referral tracked",
    })
}

// RecordCommission записывает комиссию при покупке
func RecordCommission(referrerID, referredID string, amount int64) error {
    // Получаем комиссию реферрера
    var commissionPercent int
    err := database.Pool.QueryRow(context.Background(),
        "SELECT commission_percent FROM referral_programs WHERE user_id = $1",
        referrerID).Scan(&commissionPercent)

    if err != nil {
        return err
    }

    commissionAmount := amount * int64(commissionPercent) / 100

    _, err = database.Pool.Exec(context.Background(),
        `INSERT INTO referral_commissions (referrer_id, referred_id, amount, status, created_at)
         VALUES ($1, $2, $3, 'pending', NOW())`,
        referrerID, referredID, commissionAmount)

    return err
}

// PayCommission выплачивает комиссию (имитация отправки Stars)
func PayCommission(c *gin.Context) {
    var req struct {
        CommissionID string `json:"commission_id" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    now := time.Now()
    _, err := database.Pool.Exec(c.Request.Context(),
        `UPDATE referral_commissions SET status = 'paid', paid_at = $1 WHERE id = $2`,
        now, req.CommissionID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pay commission"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Commission paid",
    })
}

// generateReferralLink создаёт уникальную реферальную ссылку
func generateReferralLink(userID string) string {
    // В реальности можно использовать хеш или код
    return "https://t.me/AgentServer_bot/app?start=ref_" + userID[:8]
}