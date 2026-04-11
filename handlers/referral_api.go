package handlers

import (
    "fmt"
    "log"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

type ReferralStats struct {
    Invited   int     `json:"invited"`
    Active    int     `json:"active"`
    Earned    float64 `json:"earned"`
    Available float64 `json:"available"`
    Pending   float64 `json:"pending"`
}

type ReferralFriend struct {
    Date   string  `json:"date"`
    Email  string  `json:"email"`
    Status string  `json:"status"`
    Bonus  float64 `json:"bonus"`
}

type PayoutRequest struct {
    Amount  float64                `json:"amount" binding:"required"`
    Method  string                 `json:"method"`
    Details map[string]interface{} `json:"details"`
}

// GetReferralStatsHandler - получить статистику
func GetReferralStatsHandler(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var invited, active int
    var earned, available, pending float64

    // Количество приглашённых
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM referrals WHERE user_id = $1
    `, userID).Scan(&invited)

    // Количество активных
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM referrals WHERE user_id = $1 AND status = 'active'
    `, userID).Scan(&active)

    // Заработано всего
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(commission), 0) FROM referrals WHERE user_id = $1
    `, userID).Scan(&earned)

    // Выплачено
    var paid float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(amount), 0) FROM partner_payouts WHERE user_id = $1 AND status = 'completed'
    `, userID).Scan(&paid)
    available = earned - paid

    // В обработке
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(amount), 0) FROM partner_payouts WHERE user_id = $1 AND status = 'pending'
    `, userID).Scan(&pending)

    stats := ReferralStats{
        Invited:   invited,
        Active:    active,
        Earned:    earned,
        Available: available,
        Pending:   pending,
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "stats":   stats,
    })
}

// GetReferralFriendsHandler - список друзей
func GetReferralFriendsHandler(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT created_at, referred_email, status, commission
        FROM referrals
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)

    var friends []ReferralFriend
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var f ReferralFriend
            var createdAt time.Time
            rows.Scan(&createdAt, &f.Email, &f.Status, &f.Bonus)
            f.Date = createdAt.Format("02.01.2006")
            friends = append(friends, f)
        }
    }

    if len(friends) == 0 {
        friends = []ReferralFriend{
            {Date: time.Now().Format("02.01.2006"), Email: "friend@example.com", Status: "pending", Bonus: 0},
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "friends": friends,
    })
}

// RequestPayout - запрос на вывод средств
func RequestPayout(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req PayoutRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Проверяем доступную сумму
    var earned, paid float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(commission), 0) FROM referrals WHERE user_id = $1
    `, userID).Scan(&earned)

    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(amount), 0) FROM partner_payouts WHERE user_id = $1 AND status = 'completed'
    `, userID).Scan(&paid)

    available := earned - paid

    if req.Amount <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Сумма должна быть больше 0"})
        return
    }

    if req.Amount > available {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Недостаточно средств. Доступно: " + fmt.Sprintf("%.2f", available)})
        return
    }

    if req.Amount < 500 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Минимальная сумма вывода: 500 ₽"})
        return
    }

    payoutID := uuid.New().String()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO partner_payouts (id, user_id, amount, status, payment_method, payment_details, created_at)
        VALUES ($1, $2, $3, 'pending', $4, $5, NOW())
    `, payoutID, userID, req.Amount, req.Method, req.Details)

    if err != nil {
        log.Printf("❌ Ошибка создания заявки: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания заявки"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "message":   "Заявка на вывод средств создана",
        "payout_id": payoutID,
    })
}

// GetPayoutHistory - история выплат
func GetPayoutHistory(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, amount, status, payment_method, created_at, processed_at
        FROM partner_payouts
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)

    var payouts []gin.H
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var id, status, method string
            var amount float64
            var createdAt, processedAt *time.Time
            rows.Scan(&id, &amount, &status, &method, &createdAt, &processedAt)

            payouts = append(payouts, gin.H{
                "id":          id,
                "amount":      amount,
                "status":      status,
                "method":      method,
                "created_at":  createdAt.Format("02.01.2006"),
                "processed_at": processedAt,
            })
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "payouts": payouts,
    })
}

// GetReferralLink - получить реферальную ссылку
func GetReferralLink(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var code string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT referral_code FROM users WHERE id = $1
    `, userID).Scan(&code)

    if err != nil || code == "" {
        code = uuid.New().String()[:8]
        database.Pool.Exec(c.Request.Context(), `
            UPDATE users SET referral_code = $1 WHERE id = $2
        `, code, userID)
    }

    link := fmt.Sprintf("http://localhost:8080/ref?code=%s", code)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "link":    link,
        "code":    code,
    })
}