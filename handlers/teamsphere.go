package handlers

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "time"
)

func TeamSphereDashboard(c *gin.Context) {
    c.HTML(http.StatusOK, "teamsphere_dashboard.html", gin.H{
        "title":       "TeamSphere - Альтернатива Bitrix24",
        "description": "Полная платформа для управления командой и совместной работы",
        "version":     "3.12.0",
        "year":        time.Now().Year(),
        "features": []gin.H{
            {"icon": "🤝", "name": "Командная работа", "desc": "Общение и управление задачами в реальном времени"},
            {"icon": "📊", "name": "Управление проектами", "desc": "Отслеживание проектов, дедлайнов и этапов"},
            {"icon": "🔌", "name": "Совместимость с API", "desc": "100% совместимость с API Bitrix24"},
            {"icon": "🔒", "name": "Корпоративная безопасность", "desc": "2FA, логи безопасности, контроль доступа"},
            {"icon": "📱", "name": "Мобильное приложение", "desc": "PWA приложение, работа офлайн"},
            {"icon": "🤖", "name": "AI Ассистент", "desc": "Умные подсказки и автоматизация"},
        },
        "stats": gin.H{
            "customers": 9,
            "deals": 3,
            "revenue": 900000,
            "conversion": 75,
        },
    })
}

func TeamSphereIntegrations(c *gin.Context) {
    c.HTML(http.StatusOK, "teamsphere_integrations.html", gin.H{
        "title": "Интеграции TeamSphere",
        "integrations": []gin.H{
            {"name": "1С:Предприятие", "icon": "fas fa-building", "status": "active", "url": "/integration/1c", "desc": "Синхронизация товаров и заказов"},
            {"name": "Bitrix24", "icon": "fab fa-bitrix", "status": "active", "url": "/integration/bitrix", "desc": "Синхронизация лидов и контактов"},
            {"name": "Telegram Бот", "icon": "fab fa-telegram", "status": "active", "url": "/settings", "desc": "Уведомления и управление"},
            {"name": "Email (SMTP)", "icon": "fas fa-envelope", "status": "active", "url": "/settings", "desc": "Отправка email уведомлений"},
            {"name": "REST API", "icon": "fas fa-code", "status": "active", "url": "/swagger/index.html", "desc": "Полный доступ к API"},
            {"name": "Webhooks", "icon": "fas fa-webhook", "status": "beta", "url": "#", "desc": "События в реальном времени"},
        },
    })
}