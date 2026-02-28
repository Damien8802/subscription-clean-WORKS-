package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

// ==================== ОСНОВНЫЕ СТРАНИЦЫ ====================
func HomeHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "index.html", gin.H{
        "Title":   "SaaSPro - Управление подписками",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func DashboardHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "dashboard.html", gin.H{
        "Title":   "Дашборд - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func AdminHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "admin.html", gin.H{
        "Title":   "Админ-панель - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func CRMHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "crm.html", gin.H{
        "Title":   "CRM система - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

// AnalyticsHandler УДАЛЁН ОТСЮДА — он теперь в analytics.go

func PaymentHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "payment.html", gin.H{
        "Title":   "Платежи - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func PricingHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "pricing.html", gin.H{
        "Title":   "Тарифы - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

// ==================== АВТОРИЗАЦИЯ (СТРАНИЦЫ) ====================
func LoginPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "login.html", gin.H{
        "Title":   "Вход - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func RegisterPageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "register.html", gin.H{
        "Title":   "Регистрация - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func ForgotPasswordHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "forgot-password.html", gin.H{
        "Title":   "Восстановление пароля - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

// ==================== СИСТЕМНЫЕ СТРАНИЦЫ ====================
func SettingsHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "settings.html", gin.H{
        "Title":   "Настройки - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func UsersHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "users.html", gin.H{
        "Title":   "Пользователи - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func MySubscriptionsHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "my-subscriptions.html", gin.H{
        "Title":   "Мои подписки - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

// ==================== ПАРТНЕРЫ И КОНТАКТЫ ====================
func PartnerHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "partner.html", gin.H{
        "Title":   "Партнерская программа - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func ContactHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "contact.html", gin.H{
        "Title":   "Контакты - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func ReferralHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "referral.html", gin.H{
        "Title":   "Реферальная программа - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

// ==================== БЕЗОПАСНОСТЬ ====================
func SecurityHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "security.html", gin.H{
        "Title":   "Безопасность - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func SecurityHubHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "security-hub.html", gin.H{
        "Title":   "Центр безопасности - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func SecurityPanelHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "security-panel.html", gin.H{
        "Title":   "Панель безопасности - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

// ==================== ДОПОЛНИТЕЛЬНЫЕ СТРАНИЦЫ ====================
func AboutHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "about.html", gin.H{
        "Title":   "О нас - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func InfoHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "info.html", gin.H{
        "Title":   "Информация - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func IntegrationsHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "integrations.html", gin.H{
        "Title":   "Интеграции - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func MonetizationHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "monetization.html", gin.H{
        "Title":   "Монетизация - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}

func SubscriptionsHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "subscriptions.html", gin.H{
        "Title":   "Подписки - SaaSPro",
        "Version": "3.0",
        "Time":    time.Now().Format("2006-01-02 15:04:05"),
    })
}