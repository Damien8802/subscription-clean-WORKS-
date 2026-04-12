package handlers

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "time"
    "log"
    "subscription-system/database"
)

// ==================== ДАШБОРДЫ ====================

func DashboardImprovedHandler(c *gin.Context) {
    userID := c.GetString("user_id")
    userName := c.GetString("user_name")
    isDeveloper := false
    
    log.Printf("DEBUG: userID=%s, userName=%s", userID, userName)
    
    // Если SKIP_AUTH=true, используем тестового разработчика
    if userID == "" {
        userID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
        userName = "Разработчик"
        isDeveloper = true
        log.Printf("DEBUG: SKIP_AUTH режим, установлен разработчик")
    } else {
        // Проверяем в БД, является ли пользователь разработчиком
        if userID != "" {
            err := database.Pool.QueryRow(c.Request.Context(), 
                "SELECT is_developer FROM users WHERE id = $1", userID).Scan(&isDeveloper)
            if err != nil {
                log.Printf("Ошибка проверки is_developer: %v", err)
            }
        }
    }
    
    if userName == "" {
        userName = "Пользователь"
    }
    
    c.HTML(http.StatusOK, "dashboard-improved.html", gin.H{
        "Title":       "Улучшенный дашборд - SaaSPro",
        "Version":     "3.0",
        "Time":        time.Now().Format("2006-01-02 15:04:05"),
        "UserName":    userName,
        "IsDeveloper": isDeveloper,
        "Stats": gin.H{
            "ClientsCount": 0,
            "DealsCount":   0,
            "Revenue":      "0 ₽",
            "VPNUsers":     0,
        },
    })
}

func RealtimeDashboardHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "realtime-dashboard.html", gin.H{
        "Title":   "Дашборд реального времени - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func RevenueDashboardHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "revenue-dashboard.html", gin.H{
        "Title":   "Дашборд выручки - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func PartnerDashboardHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "partner-dashboard.html", gin.H{
        "Title":   "Партнерский дашборд - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func UnifiedDashboardHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "unified-dashboard.html", gin.H{
        "Title":   "Унифицированный дашборд - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}