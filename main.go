package main

import (
    "context"
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    swaggerFiles "github.com/swaggo/files"
    ginSwagger "github.com/swaggo/gin-swagger"
    
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/handlers"
    "subscription-system/middleware"
    "subscription-system/services"
    _ "subscription-system/docs"

)

// //go:embed templates/*.html templates/hr/*.html
// var templateFS embed.FS

type ServiceOrder struct {
    Name        string `json:"name"`
    Contact     string `json:"contact"`
    Description string `json:"description"`
}

func serviceOrderHandler(c *gin.Context) {
    var order ServiceOrder
    if err := c.ShouldBindJSON(&order); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные"})
        return
    }

    if order.Name == "" || order.Contact == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Имя и контакт обязательны"})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO service_requests (name, contact, description, created_at)
        VALUES ($1, $2, $3, NOW())
    `, order.Name, order.Contact, order.Description)
    if err != nil {
        log.Printf("Ошибка сохранения заявки: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка базы данных"})
        return
    }

    log.Printf("📦 Новая заявка на услуги: %s (%s): %s", order.Name, order.Contact, order.Description)
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func main() {
    if err := godotenv.Load(); err != nil {
        log.Println("⚠️ .env file not found, using system environment")
    } else {
        fmt.Println("✅ .env file loaded and applied")
    }
    cfg := config.Load()

    if err := database.InitDB(cfg); err != nil {
        log.Fatalf("❌ Ошибка подключения к БД: %v", err)
    }
    defer database.CloseDB()

    // ========== СОЗДАНИЕ ТАБЛИЦ VPN ==========
    ctx := context.Background()
    
    _, err := database.Pool.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS vpn_plans (
            id SERIAL PRIMARY KEY,
            name VARCHAR(50) NOT NULL,
            price DECIMAL(10,2) NOT NULL,
            days INTEGER NOT NULL,
            speed VARCHAR(50),
            devices INTEGER DEFAULT 1,
            created_at TIMESTAMP DEFAULT NOW()
        )
    `)
    if err != nil {
        log.Printf("⚠️ Ошибка создания vpn_plans: %v", err)
    } else {
        log.Println("✅ Таблица vpn_plans готова")
    }
    
   _, err = database.Pool.Exec(ctx, `
    INSERT INTO vpn_plans (name, price, days, speed, devices, tenant_id) 
    VALUES ($1, $2, $3, $4, $5, $6),
           ($7, $8, $9, $10, $11, $12),
           ($13, $14, $15, $16, $17, $18),
           ($19, $20, $21, $22, $23, $24)
    ON CONFLICT (id) DO NOTHING
`,
    "Пробный", 0, 3, "10 Mbps", 1, "11111111-1111-1111-1111-111111111111",
    "Старт", 299, 30, "50 Mbps", 2, "11111111-1111-1111-1111-111111111111",
    "Про", 999, 90, "100 Mbps", 5, "11111111-1111-1111-1111-111111111111",
    "Премиум", 2999, 365, "1 Gbps", 10, "11111111-1111-1111-1111-111111111111",
)
    if err != nil {
        log.Printf("⚠️ Ошибка вставки тарифов: %v", err)
    } else {
        log.Println("✅ VPN тарифы загружены")
    }
    
    handlers.InitVPNWithDB(database.Pool)

    handlers.InitAuthHandler(cfg)
    handlers.InitNotifier(cfg)

    var yandexService *services.YandexAdapter
    var aiAgentService *services.AIAgentService
    var speechKitService *services.SpeechKitService

    yandexService = services.NewYandexService(cfg)
    aiAgentService = services.NewAIAgentService(yandexService)
    aiAgentService.StartAgentScheduler()
    log.Println("🤖 Сервис ИИ-агентов запущен с YandexGPT")

    speechKitService = services.NewSpeechKitService(cfg)
    _ = speechKitService
    log.Println("🎙️ Сервис транскрибации SpeechKit инициализирован")

    if cfg.Env == "release" {
        gin.SetMode(gin.ReleaseMode)
    }

    r := gin.New()

// ========== МЕГА-БЕЗОПАСНОСТЬ ==========
r.Use(middleware.MegaSecurityMiddleware())
// ========================================

r.Use(middleware.AuditMiddleware())          // Аудит действий
r.Use(middleware.Fail2BanMiddleware())       // Блокировка IP
r.Use(middleware.ForcePasswordChangeMiddleware()) // Принудительная смена пароля

// Для админских маршрутов добавляем 2FA
admin := r.Group("/admin")
admin.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg), handlers.AdminRequire2FA())
{
    admin.GET("/", handlers.AdminDashboardHandler)
    r.Use(gin.Logger())
    r.Use(gin.Recovery())
    r.Use(middleware.Logger())
    r.SetTrustedProxies(cfg.TrustedProxies)
    r.Use(middleware.SetupCORS(cfg))
    r.Use(middleware.TenantMiddleware(database.Pool))



    rateLimiter := middleware.NewRateLimiter(30, time.Minute)
    r.Use(middleware.SecurityMonitor())
    authLimiter := middleware.NewRateLimiter(3, time.Minute)

    // ========== ЗАГРУЗКА ШАБЛОНОВ ==========
        // Загружаем шаблоны из файловой системы
    tmpl, err := template.New("").Funcs(template.FuncMap{
        "jsonParse": func(s json.RawMessage) []interface{} {
            var arr []interface{}
            json.Unmarshal(s, &arr)
            return arr
        },
        "firstLetter": func(s string) string {
            if len(s) == 0 {
                return "?"
            }
            return strings.ToUpper(string(s[0]))
        },
        "sub": func(a, b int) int { return a - b },
        "add": func(a, b int) int { return a + b },
        "seq": func(n int) []int {
            s := make([]int, n)
            for i := 0; i < n; i++ {
                s[i] = i + 1
            }
            return s
        },
        "float": func(i int64) float64 { return float64(i) },
        "mul":   func(a, b float64) float64 { return a * b },
        "div": func(a, b float64) float64 {
            if b == 0 {
                return 0
            }
            return a / b
        },
        "default": func(defaultVal, val interface{}) interface{} {
            if val == nil {
                return defaultVal
            }
            if str, ok := val.(string); ok && str == "" {
                return defaultVal
            }
            return val
        },
    }).ParseGlob("templates/*.html")
    if err != nil {
        log.Fatalf("❌ Не удалось загрузить шаблоны: %v", err)
    }

    // Добавляем HR шаблоны
    hrTmpl, err := template.ParseGlob("templates/hr/*.html")
    if err == nil && hrTmpl != nil {
        for _, t := range hrTmpl.Templates() {
            tmpl.AddParseTree(t.Name(), t.Tree)
        }
    }

    // Добавляем MARKETPLACE шаблоны
    marketplaceTmpl, err := template.ParseGlob("templates/marketplace/*.html")
    if err == nil && marketplaceTmpl != nil {
        for _, t := range marketplaceTmpl.Templates() {
            tmpl.AddParseTree(t.Name(), t.Tree)
        }
    }

    r.SetHTMLTemplate(tmpl)
    log.Println("✅ Шаблоны загружены из файловой системы")



    // Загружаем шаблоны из файловой системы
    

    // ========== СТАТИКА, РЕДИРЕКТЫ ==========
    r.Static("/static", cfg.StaticPath)
    r.Static("/frontend", cfg.FrontendPath)
    r.Static("/app", "C:/Projects/subscription-clean-WORKS/telegram-mini-app")
    r.GET("/telegram/manifest.json", func(c *gin.Context) { c.File("./telegram-mini-app/manifest.json") })
    r.GET("/telegram/sw.js", func(c *gin.Context) { c.File("./telegram-mini-app/service-worker.js") })
    r.GET("/app", func(c *gin.Context) { c.File("C:/Projects/subscription-clean-WORKS/telegram-mini-app/index.html") })
    r.GET("/dashboard_improved", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/dashboard-improved") })
    r.GET("/dashboard", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/dashboard-improved") })
    r.GET("/delivery", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/logistics") })
    r.GET("/ai", handlers.AIChatPageHandler)
    r.GET("/my-keys", handlers.MyKeysPageHandler)
    r.GET("/api-keys", handlers.APIKeysPageHandler)
    r.GET("/support", handlers.SupportPageHandler)
    r.GET("/referral", handlers.ReferralPageHandler)
    r.GET("/ai-settings", handlers.AISettingsPageHandler)
    r.GET("/transcriptions", handlers.TranscriptionsPage)
    r.GET("/ai-agents", handlers.AIAgentsPage)
    r.GET("/advanced-analytics", handlers.AdvancedAnalyticsPage)

    // QR код авторизация
    r.GET("/qr-login", handlers.QRLoginPageHandler)
    r.POST("/api/qr/generate", handlers.GenerateQRCode)
    r.GET("/api/qr/status", handlers.QRStatusWebSocket)
    r.POST("/api/qr/scan", handlers.ScanQRCode)
    r.POST("/api/qr/approve", handlers.ApproveQRLogin)

    // Телефонная авторизация
    r.POST("/api/auth/send-code", handlers.SendPhoneCode)
    r.POST("/api/auth/verify-code", handlers.VerifyPhoneCode)

    // Push уведомления
    r.POST("/api/push/register", handlers.RegisterPushDevice)
    r.GET("/api/push/devices", handlers.GetUserDevices)
    r.DELETE("/api/push/devices/:id", handlers.RemovePushDevice)
    
    r.GET("/api-sales", handlers.APISalesPageHandler)           
    r.GET("/api/user/plan", handlers.GetUserPlan)                
    r.POST("/api/create-key", handlers.CreateAPIKey)             
    r.POST("/api/upgrade-key", handlers.UpgradeAPIKey)           
    r.GET("/api/user/usage", handlers.GetAPIUsage)  

    // Инвентаризация
    r.GET("/inventory", handlers.InventoryPageHandler)
    r.GET("/api/inventory/products", handlers.GetProducts)
    r.POST("/api/inventory/products", handlers.CreateProduct)
    r.PUT("/api/inventory/products/:id", handlers.UpdateProduct)
    r.DELETE("/api/inventory/products/:id", handlers.DeleteProduct)
    r.GET("/api/inventory/orders", handlers.GetOrders)
    r.POST("/api/inventory/orders", handlers.CreateOrder)
    r.GET("/api/inventory/orders/:id", handlers.GetOrderDetails)
    r.GET("/api/inventory/stats", handlers.GetInventoryStats)
    r.GET("/api/inventory/products/export/csv", handlers.ExportProductsCSV)

    // Поставщики
    r.GET("/api/suppliers", handlers.GetSuppliers)
    r.GET("/api/suppliers/:id", handlers.GetSupplier)
    r.POST("/api/suppliers", handlers.CreateSupplier)
    r.PUT("/api/suppliers/:id", handlers.UpdateSupplier)
    r.DELETE("/api/suppliers/:id", handlers.DeleteSupplier)

    // Заказы поставщикам
    r.GET("/api/purchase-orders", handlers.GetPurchaseOrders)
    r.GET("/api/purchase-orders/:id", handlers.GetPurchaseOrder)
    r.POST("/api/purchase-orders", handlers.CreatePurchaseOrder)
    r.PUT("/api/purchase-orders/:id/status", handlers.UpdatePurchaseOrderStatus)
    r.DELETE("/api/purchase-orders/:id", handlers.DeletePurchaseOrder)

    // Страница приемки товаров
    r.GET("/goods-receipts", handlers.GoodsReceiptsPageHandler)
    r.GET("/api/goods-receipts", handlers.GetGoodsReceipts)
    r.GET("/api/goods-receipts/:id", handlers.GetGoodsReceipt)
    r.POST("/api/goods-receipts", handlers.CreateGoodsReceipt)

    // ========== ФИНАНСОВЫЙ УЧЕТ ==========
    r.GET("/api/chart-of-accounts", handlers.GetChartOfAccounts)
    r.POST("/api/chart-of-accounts", handlers.CreateChartOfAccount)
    r.PUT("/api/chart-of-accounts/:id", handlers.UpdateChartOfAccount)
    r.DELETE("/api/chart-of-accounts/:id", handlers.DeleteChartOfAccount)

    r.GET("/finance", func(c *gin.Context) {
        c.HTML(http.StatusOK, "finance.html", gin.H{
            "title": "Финансовый учет | SaaSPro",
        })
    })

    r.GET("/api/payments", handlers.GetFinancePayments)
    r.POST("/api/payments", handlers.CreateFinancePayment)
    r.PUT("/api/payments/:id/status", handlers.UpdateFinancePaymentStatus)

    r.GET("/api/cash-operations", handlers.GetCashOperations)
    r.POST("/api/cash-operations", handlers.CreateCashOperation)
    r.GET("/api/journal-entries", handlers.GetJournalEntries)
    r.GET("/api/journal-entries/:id", handlers.GetJournalEntry)
    r.POST("/api/journal-entries", handlers.CreateJournalEntry)
    r.POST("/api/journal-entries/:id/post", handlers.PostJournalEntry)
    r.DELETE("/api/journal-entries/:id", handlers.DeleteJournalEntry)

    // Страница поставщиков
    r.GET("/suppliers", func(c *gin.Context) {
        c.HTML(http.StatusOK, "suppliers.html", gin.H{
            "title": "Поставщики | SaaSPro",
        })
    })

    // Страница закупок
    r.GET("/purchases", func(c *gin.Context) {
        c.Header("Cache-Control", "no-cache, no-store, must-revalidate, private")
        c.Header("Pragma", "no-cache")
        c.Header("Expires", "0")
        c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
        c.Header("ETag", "")
        c.HTML(http.StatusOK, "purchases.html", gin.H{
            "title": "Закупки | SaaSPro",
            "cacheBuster": time.Now().UnixNano(),
        })
    })

    // Уведомления
    r.GET("/api/notifications", handlers.GetNotifications)
    r.PUT("/api/notifications/:id/read", handlers.MarkNotificationRead)
    r.GET("/api/notifications/unread", handlers.GetUnreadCount)

    // Экспорт отчетов
    r.GET("/api/reports/export/osv", handlers.ExportOSVToExcel)
    r.GET("/api/reports/export/profit-loss", handlers.ExportProfitLossToHTML)

    // Гант-диаграмма
    r.GET("/api/gantt", handlers.GetGanttData)

    // Обновление статуса заказа
    r.PUT("/api/inventory/orders/:id/status", handlers.UpdateOrderStatus)

    // Отчеты
    r.GET("/api/inventory/reports/sales", handlers.GetSalesReport)
    r.GET("/api/inventory/reports/top-products", handlers.GetTopProducts)

    // OAuth2 / OpenID Connect маршруты
    r.GET("/.well-known/openid-configuration", handlers.OIDCConfigurationHandler)
    r.GET("/oauth/jwks", handlers.JWKSHander)
    r.GET("/oauth/authorize", handlers.OAuthAuthorizeHandler)
    r.POST("/oauth/token", handlers.OAuthTokenHandler)
    r.GET("/oauth/userinfo", handlers.OAuthUserInfoHandler)
    r.GET("/identity-hub", handlers.IdentityHubPageHandler)

    // ========== ОТЧЕТЫ И АНАЛИТИКА ==========
    r.GET("/api/reports/turnover-balance", handlers.GetTurnoverBalanceSheet)
    r.GET("/api/reports/profit-loss", handlers.GetProfitAndLoss)
    r.GET("/api/reports/dashboard-stats", handlers.GetDashboardStats)
    r.GET("/api/reports/sales-chart", handlers.GetSalesChart)

    r.GET("/reports", func(c *gin.Context) {
        c.HTML(http.StatusOK, "reports.html", gin.H{
            "title": "Отчеты и аналитика | SaaSPro",
        })
    })

    // ========== ИНТЕГРАЦИЯ С 1С ==========
    r.GET("/api/1c/export/products", handlers.ExportProductsTo1C)
    r.GET("/api/1c/export/orders", handlers.ExportOrdersTo1C)
    r.POST("/api/1c/import/products", handlers.ImportProductsFrom1C)
    r.GET("/api/1c/logs", handlers.GetSyncLogs)
    r.GET("/api/1c/settings", handlers.GetSyncSettings)
    r.POST("/api/1c/settings", handlers.UpdateSyncSettings)
    r.GET("/integration/1c", func(c *gin.Context) {
        c.HTML(http.StatusOK, "integration_1c.html", gin.H{
            "title": "Интеграция с 1С | SaaSPro",
        })
    })
    r.POST("/api/1c/webhook", handlers.AddWebhookHandler)

    // ========== BITRIX24 ==========
    r.GET("/api/bitrix/settings", handlers.GetBitrixSettings)
    r.POST("/api/bitrix/settings", handlers.SaveBitrixSettings)
    r.POST("/api/bitrix/export/lead", handlers.ExportLeadToBitrix)
    r.GET("/api/bitrix/import/leads", handlers.ImportLeadsFromBitrix)
    r.POST("/api/bitrix/sync/contacts", handlers.SyncBitrixContacts)
    r.GET("/api/bitrix/logs", handlers.GetBitrixSyncLogs)
    r.GET("/integration/bitrix", func(c *gin.Context) {
        c.HTML(http.StatusOK, "integration_bitrix.html", gin.H{
            "title": "Интеграция с Bitrix24 | SaaSPro",
        })
    })
    r.POST("/api/bitrix/task", handlers.SyncTasksToBitrix)
    r.GET("/api/bitrix/tasks", handlers.GetBitrixTasks)
    r.POST("/api/bitrix/webhook", handlers.BitrixWebhookHandler)

 // TeamSphere - Bitrix24 Alternative
   r.GET("/teamsphere", func(c *gin.Context) {
    c.HTML(http.StatusOK, "teamsphere_welcome.html", gin.H{
        "title": "TeamSphere | Добро пожаловать",
    })
})

r.GET("/teamsphere/dashboard", handlers.TeamSphereDashboard)
    r.GET("/integrations", handlers.IntegrationsHandler)
   // Projects page
   r.GET("/projects", handlers.ProjectsPageHandler)

// HR маршруты
hr := r.Group("/hr")
{
    hr.GET("/", handlers.HRDashboardHandler)
    hr.GET("/api/employees", handlers.GetEmployeesHandler)
    hr.POST("/api/employees", handlers.AddEmployeeHandler)
    hr.PUT("/api/employees/:id", handlers.UpdateEmployeeHandler)
    hr.DELETE("/api/employees/:id", handlers.DeleteEmployeeHandler)
    hr.GET("/api/vacations", handlers.GetVacationRequestsHandler)
    hr.POST("/api/vacations", handlers.AddVacationRequestHandler)
    hr.POST("/api/vacations/:id/approve", handlers.ApproveRequestHandler)
    hr.POST("/api/vacations/:id/reject", handlers.RejectRequestHandler)
    hr.GET("/api/candidates", handlers.GetCandidatesHandler)
    hr.POST("/api/candidates", handlers.AddCandidateHandler)
    hr.PUT("/api/candidates/:id/status", handlers.UpdateCandidateStatusHandler)
    hr.DELETE("/api/candidates/:id", handlers.DeleteCandidateHandler)
    hr.GET("/api/statistics", handlers.GetStatisticsHandler)
    hr.POST("/api/candidates/:id/analyze", handlers.AnalyzeCandidateHandler)
    hr.POST("/api/ai/chat", handlers.AIChatHandler)
    hr.GET("/api/training/suggestions", handlers.SuggestTrainingHandler)
    hr.GET("/api/turnover/predict", handlers.PredictTurnoverHandler)
    hr.POST("/api/orders/generate", handlers.GenerateOrderHandler)
    hr.GET("/api/departments", handlers.GetDepartmentsHandler)
}

// ========== АРХИВ ==========
archiveGroup := r.Group("/archive")
archiveGroup.Use(middleware.AuthMiddleware(cfg))

archiveGroup.DELETE("/api/trash/:id", handlers.DeleteFromTrashPermanently)
archiveGroup.DELETE("/api/trash/clear", handlers.ClearTrashBin)
{
    archiveGroup.GET("/", handlers.ArchivePageHandler)
    archiveGroup.GET("/api/stats", handlers.GetArchiveStats)
    archiveGroup.GET("/api/items", handlers.GetArchiveItems)
    archiveGroup.POST("/api/restore/:type/:id", handlers.RestoreFromArchive)
    archiveGroup.POST("/api/upgrade", handlers.UpgradeArchiveQuota)

// В блоке archiveGroup добавь:
archiveGroup.GET("/api/notifications", handlers.GetNotifications)
archiveGroup.POST("/api/notifications/:id/read", handlers.MarkNotificationRead)
// Дополнительные маршруты
    archiveGroup.GET("/api/auto-settings", handlers.GetAutoArchiveSettings)
    archiveGroup.POST("/api/auto-settings", handlers.UpdateAutoArchiveSettings)
    archiveGroup.POST("/api/run-auto-archive", handlers.RunAutoArchive)
    archiveGroup.GET("/api/trash", handlers.GetTrashItems)
    archiveGroup.POST("/api/trash/:type/:id", handlers.MoveToTrash)
    archiveGroup.POST("/api/trash/restore/:id", handlers.RestoreFromTrash)
    archiveGroup.GET("/api/logs", handlers.GetArchiveLogs)
    archiveGroup.GET("/api/export", handlers.ExportArchiveToExcel)

archiveGroup.GET("/api/plan", handlers.GetCurrentPlan)
}

// ========== МАРКЕТПЛЕЙС ==========
marketplace := r.Group("/marketplace")
marketplace.Use(middleware.AuthMiddleware(cfg))
{
    marketplace.GET("/", handlers.MarketplacePageHandler)
    marketplace.GET("/api/apps", handlers.GetMarketplaceApps)
    marketplace.GET("/api/apps/:slug", handlers.GetMarketplaceApp)
    marketplace.POST("/api/purchase", handlers.PurchaseApp)
    marketplace.POST("/api/review", handlers.AddReview)
    marketplace.GET("/api/my-purchases", handlers.GetMyPurchases)
}
// API для архивации из CRM
crmArchive := r.Group("/api/crm")
crmArchive.Use(middleware.AuthMiddleware(cfg))
{
    crmArchive.POST("/customers/:id/archive", handlers.ArchiveCustomer)
}
    // ========== PWA И PUSH УВЕДОМЛЕНИЯ ==========
    r.GET("/service-worker.js", func(c *gin.Context) { c.File("./static/service-worker.js") })
    r.GET("/manifest.json", func(c *gin.Context) { c.File("./static/manifest.json") })
    r.GET("/api/pwa/info", handlers.GetPWAInfo)
    r.POST("/api/push/subscribe", handlers.SavePushSubscription)
    r.GET("/api/push/subscriptions", handlers.GetPushSubscriptions)

    // Админские маршруты для управления OAuth клиентами
    adminOAuth := r.Group("/admin/oauth")
    adminOAuth.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
    {
        adminOAuth.GET("/clients", handlers.OAuthClientsPageHandler)
        adminOAuth.POST("/clients", handlers.CreateOAuthClient)
    }
    
    // VPN маршруты
    r.GET("/vpn", handlers.VPNSalesPageHandler)
    r.POST("/api/vpn/create", handlers.CreateVPNKey)
    r.GET("/api/vpn/config/:client", handlers.GetVPNConfig)
    r.GET("/api/vpn/status/:client", handlers.CheckVPNKey)
    r.GET("/api/vpn/stats", handlers.GetVPNStats)
    r.POST("/api/vpn/renew/:client", handlers.RenewVPNKey)

    // Админ маршруты для VPN
    adminVPN := r.Group("/admin/vpn")
    adminVPN.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
    {
        adminVPN.GET("/keys", handlers.GetAllVPNKeys)
        adminVPN.GET("/stats", handlers.AdminVPNHandler)
    }

    // Публичные маршруты
    public := r.Group("/")
    {
        public.GET("/", handlers.HomeHandler)
        public.GET("/about", handlers.AboutHandler)
        public.GET("/contact", handlers.ContactHandler)
        public.GET("/info", handlers.InfoHandler)
        public.GET("/pricing", handlers.PricingPageHandler)
        public.GET("/partner", handlers.PartnerHandler)
    }

    r.POST("/api/service-order", serviceOrderHandler)

    // Страницы авторизации
    authPages := r.Group("/")
    {
        authPages.GET("/login", handlers.LoginPageHandler)
        authPages.GET("/register", handlers.RegisterPageHandler)
        authPages.GET("/forgot-password", handlers.ForgotPasswordHandler)
    }

    // API авторизации
    authAPI := r.Group("/api/auth")
    authAPI.Use(func(c *gin.Context) {
        ip := c.ClientIP()
        if authLimiter.Limit(ip) {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "Слишком много попыток входа. Попробуйте через минуту.",
            })
            c.Abort()
            return
        }
        c.Next()
    })
    {
        authAPI.POST("/register", handlers.RegisterHandler)
        authAPI.POST("/login", handlers.LoginHandler)
        authAPI.POST("/refresh", handlers.RefreshHandler)
        authAPI.POST("/logout", handlers.LogoutHandler)
        authAPI.POST("/trusted-devices/add", handlers.AddTrustedDevice)
        authAPI.POST("/trusted-devices/revoke", handlers.RevokeTrustedDevice)
        authAPI.GET("/trusted-devices/list", handlers.GetTrustedDevices)
    }

    // Реферальная программа
    referralAPI := r.Group("/api/referral")
    referralAPI.Use(middleware.AuthMiddleware(cfg))
    {
        referralAPI.POST("/program/create", handlers.CreateReferralProgram)
        referralAPI.GET("/program", handlers.GetReferralProgram)
        referralAPI.GET("/commissions", handlers.GetReferralCommissions)
        referralAPI.POST("/commissions/pay", handlers.PayCommission)
    }
    r.GET("/ref", handlers.ProcessReferral)

    // Верификация
    verificationAPI := r.Group("/api/verification")
    {
        verificationAPI.POST("/send-email", handlers.SendVerificationEmail)
        verificationAPI.POST("/send-telegram", handlers.SendVerificationTelegram)
        verificationAPI.POST("/verify", handlers.VerifyCode)
        verificationAPI.GET("/status", handlers.CheckVerificationStatus)
    }

    // Защищенные маршруты
    protected := r.Group("/")
    protected.Use(middleware.AuthMiddleware(cfg))
    {
        protected.GET("/settings", handlers.SettingsHandler)
        protected.GET("/my-subscriptions", handlers.MySubscriptionsPageHandler)
        protected.GET("/trusted-devices", handlers.TrustedDevicesHandler)
        protected.GET("/monetization", handlers.MonetizationHandler)
        protected.GET("/profile", handlers.ProfilePageHandler)
        protected.GET("/calendar", handlers.CalendarHandler)
    }

    // Админские маршруты
    admin := r.Group("/")
    admin.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
    {
        admin.GET("/admin", handlers.AdminDashboardHandler)
        admin.GET("/admin/users", handlers.AdminUsersHandler)
        admin.GET("/admin/subscriptions", handlers.AdminSubscriptionsHandler)
        admin.GET("/admin-fixed", handlers.AdminFixedHandler)
        admin.GET("/gold-admin", handlers.GoldAdminHandler)
        admin.GET("/database-admin", handlers.DatabaseAdminHandler)
        admin.GET("/users", handlers.UsersHandler)
        admin.GET("/subscriptions", handlers.SubscriptionsHandler)
        admin.GET("/crm", handlers.CRMHandler)
        admin.GET("/admin/api-keys", handlers.AdminAPIKeysHandler)


admin2FA := r.Group("/api/admin/2fa")
admin2FA.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
{
    admin2FA.POST("/enable", handlers.EnableAdmin2FA)
    admin2FA.POST("/verify", handlers.VerifyAdmin2FA)
}
    }

    // Дашборды
    dashboards := r.Group("/")
    dashboards.Use(middleware.AuthMiddleware(cfg))
    {
        dashboards.GET("/dashboard-improved", handlers.DashboardImprovedHandler)
        dashboards.GET("/realtime-dashboard", handlers.RealtimeDashboardHandler)
        dashboards.GET("/revenue-dashboard", handlers.RevenueDashboardHandler)
        dashboards.GET("/partner-dashboard", handlers.PartnerDashboardHandler)
        dashboards.GET("/unified-dashboard", handlers.UnifiedDashboardHandler)
        dashboards.GET("/dashboard-stats", handlers.DashboardStatsHandler)
    }

    // Платежи
    payments := r.Group("/")
    payments.Use(middleware.AuthMiddleware(cfg))
    {
        payments.GET("/payment", handlers.PaymentHandler)
        payments.GET("/bank_card_payment", handlers.BankCardPaymentHandler)
        payments.GET("/payment-success", handlers.PaymentSuccessHandler)
        payments.GET("/usdt-payment", handlers.USDTPaymentHandler)
        payments.GET("/rub-payment", handlers.RUBPaymentHandler)
    }

      // ========== ЛОГИСТИКА ==========
    // Страницы логистики (публичные или с авторизацией)
    logisticsGroup := r.Group("/logistics")
    logisticsGroup.Use(middleware.AuthMiddleware(cfg))
    {
        logisticsGroup.GET("/", handlers.LogisticsDashboardHandler)
        logisticsGroup.GET("/orders", handlers.LogisticsOrdersHandler)
        logisticsGroup.GET("/track", handlers.TrackHandler)
    }
    
    // API логистики
    logisticsAPI := r.Group("/api/logistics")
    logisticsAPI.Use(middleware.AuthMiddleware(cfg))
    {
        logisticsAPI.POST("/orders", handlers.APICreateOrder)
        logisticsAPI.GET("/orders", handlers.APIGetOrders)
        logisticsAPI.PUT("/orders/:id/status", handlers.APIUpdateOrderStatus)
        logisticsAPI.GET("/stats", handlers.APIGetStats)
        logisticsAPI.GET("/track/:trackingNumber", handlers.TrackAPIHandler)
    }
    
    // Доставка (оставляем для обратной совместимости)
    deliveryAPI := r.Group("/api/delivery")
    deliveryAPI.Use(middleware.AuthMiddleware(cfg))
    {
        deliveryAPI.GET("/track/:trackingNumber", handlers.TrackAPIHandler)
    }

    // Основное API
    api := r.Group("/api")
    api.Use(func(c *gin.Context) {
        ip := c.ClientIP()
        if rateLimiter.Limit(ip) {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error": "Слишком много запросов. Попробуйте позже.",
            })
            c.Abort()
            return
        }
        c.Next()
    })
    api.Use(middleware.AuthMiddleware(cfg))
    {
        api.GET("/health", handlers.HealthHandler)
        api.GET("/crm/health", handlers.CRMHealthHandler)
        api.GET("/system/stats", handlers.SystemStatsHandler)
        api.GET("/test", handlers.TestHandler)
        api.POST("/user/profile", handlers.UpdateProfileHandler)
        api.POST("/user/password", handlers.UpdatePasswordHandler)
        api.GET("/plans", handlers.GetPlansHandler)
        api.POST("/subscriptions", handlers.CreateSubscriptionHandler)
        api.POST("/ai/ask", handlers.AIAskHandler)
        api.POST("/ai/ask-with-file", handlers.AskWithFileHandler)
        api.GET("/user/subscriptions", handlers.GetUserSubscriptionsHandler)
        api.GET("/user/ai-usage", handlers.GetUserAIUsageHandler)
        api.POST("/telegram/ensure-key", handlers.EnsureAPIKeyForTelegram)
        api.POST("/webapp/auth", handlers.WebAppAuthHandler)
        api.POST("/chat/save", handlers.SaveChatMessage)
        api.GET("/chat/history", handlers.GetChatHistory)
        api.POST("/knowledge/upload", handlers.UploadKnowledgeHandler)
        api.GET("/knowledge/list", handlers.ListKnowledgeHandler)
        api.DELETE("/knowledge/delete/:id", handlers.DeleteKnowledgeHandler)
        api.POST("/notify", handlers.NotifyHandler)
        api.POST("/keys/create", handlers.CreateAPIKeyHandler)
        api.GET("/user/keys", handlers.GetUserAPIKeysHandler)
        api.POST("/keys/revoke", handlers.RevokeAPIKeyHandler)
        api.POST("/keys/validate", handlers.ValidateAPIKeyHandler)
        api.GET("/referral/stats", handlers.GetReferralStatsHandler)
        api.GET("/referral/friends", handlers.GetReferralFriendsHandler)
        api.GET("/2fa/status", handlers.GetTwoFAStatus)
        api.GET("/2fa/generate", handlers.GenerateTwoFASecret)
        api.POST("/2fa/verify", handlers.VerifyTwoFACode)
        api.POST("/2fa/disable", handlers.DisableTwoFA)
api.GET("/2fa/backup-codes", handlers.GetBackupCodes)
api.POST("/2fa/backup-codes", handlers.GenerateBackupCodes)
api.GET("/2fa/settings", handlers.Get2FASettings)
api.GET("/2fa/check-trust", handlers.CheckTrustedDevice)
api.POST("/2fa/trust-device", handlers.TrustDevice)
api.POST("/2fa/verify-backup", handlers.VerifyWithBackupCode)
        api.GET("/crm/customers", handlers.GetCustomers)
        api.POST("/crm/customers", handlers.CreateCustomer)
        api.PUT("/crm/customers/:id", handlers.UpdateCustomer)
        api.DELETE("/crm/customers/:id", handlers.DeleteCustomer)
        api.GET("/crm/deals", handlers.GetDeals)
        api.POST("/crm/deals", handlers.CreateDeal)
        api.PUT("/crm/deals/:id", handlers.UpdateDeal)
        api.DELETE("/crm/deals/:id", handlers.DeleteDeal)
        api.PUT("/crm/deals/:id/stage", handlers.UpdateDealStage)
        api.GET("/crm/stats", handlers.GetCRMStats)
        api.POST("/crm/deals/:id/attachments", handlers.UploadDealAttachment)
        api.GET("/crm/deals/:id/attachments", handlers.GetDealAttachments)
        api.GET("/crm/attachments/:attachment_id/download", handlers.DownloadDealAttachment)
        api.DELETE("/crm/attachments/:attachment_id", handlers.DeleteDealAttachment)
        api.GET("/crm/advanced-stats", handlers.GetCRMAdvancedStats)
        api.POST("/crm/customers/batch/delete", handlers.BatchDeleteCustomers)
        api.PUT("/crm/customers/batch/status", handlers.BatchUpdateCustomersStatus)
        api.POST("/crm/deals/batch/delete", handlers.BatchDeleteDeals)
        api.PUT("/crm/deals/batch/stage", handlers.BatchUpdateDealsStage)
        api.PUT("/crm/deals/batch/responsible", handlers.BatchUpdateDealsResponsible)
        api.GET("/crm/customers/export/csv", handlers.ExportCustomersCSV)
        api.GET("/crm/customers/export/excel", handlers.ExportCustomersExcel)
        api.GET("/crm/deals/export/csv", handlers.ExportDealsCSV)
        api.GET("/crm/deals/export/excel", handlers.ExportDealsExcel)
        api.GET("/crm/history/:type/:id", handlers.GetEntityHistory)
        api.GET("/crm/tags", handlers.GetTags)
        api.POST("/crm/tags", handlers.CreateTag)
        api.DELETE("/crm/tags/:id", handlers.DeleteTag)
        api.POST("/crm/activities", handlers.AddActivity)
        api.GET("/crm/activities/:type/:id", handlers.GetActivities)
        api.POST("/crm/ai/ask", handlers.AIAskHandler)

        api.POST("/transcription/upload", handlers.UploadAudio)
        api.GET("/transcriptions", handlers.GetTranscriptions)
        api.GET("/transcription/:id", handlers.GetTranscriptionByID)

        api.GET("/notifications/settings", handlers.GetNotificationSettings)
        api.PUT("/notifications/settings", handlers.UpdateNotificationSettings)
        api.GET("/crm/forecast", handlers.GetSalesForecast)
        api.GET("/crm/conversion", handlers.GetStageConversion)
        api.DELETE("/crm/activities/:id", handlers.DeleteActivity)
        api.PUT("/crm/tags/:id", handlers.UpdateTag)
        api.POST("/ai/consultant", handlers.AIConsultantHandler)

        api.GET("/ai/agents", handlers.GetAgents)
        api.POST("/ai/agents", handlers.CreateAgent)
        api.PUT("/ai/agents/:id", handlers.UpdateAgent)
        api.DELETE("/ai/agents/:id", handlers.DeleteAgent)
        api.POST("/ai/agents/:id/actions", handlers.AddAgentAction)
        api.GET("/ai/agents/logs", handlers.GetAgentLogs)
        api.GET("/ai/agents/stats", handlers.GetAgentStats)

        api.GET("/analytics/ltv", handlers.GetLTVPredictions)
        api.GET("/analytics/ltv/:id", handlers.GetCustomerLTV)
        api.GET("/analytics/insights", handlers.GetInsights)
        api.GET("/analytics/segments", handlers.GetSegmentSummary)
        api.GET("/analytics/cohorts/run", handlers.RunCohortAnalysis)
    }

    secureAPI := r.Group("/api")
    secureAPI.Use(middleware.AuthMiddleware(cfg))
    {
        secureAPI.GET("/user/profile", handlers.GetUserProfile)
        secureAPI.GET("/user/ai-history", handlers.GetUserAIHistoryHandler)
    }

    r.GET("/notify", handlers.NotifyPageHandler)

    userKeys := r.Group("/api/user/keys")
    userKeys.Use(middleware.AuthMiddleware(cfg))
    {
        userKeys.DELETE("/:id", handlers.RevokeAPIKeyHandler)
    }

    v1 := r.Group("/api/v1")
    v1.Use(middleware.APIKeyAuthMiddleware())
    {
        // Зарезервировано
    }

  adminAPI := r.Group("/api/admin")
adminAPI.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
{
    adminAPI.PUT("/subscriptions/:id/cancel", handlers.AdminCancelSubscriptionHandler)
    adminAPI.PUT("/subscriptions/:id/reactivate", handlers.AdminReactivateSubscriptionHandler)
    adminAPI.GET("/plans", handlers.AdminGetPlansHandler)
    adminAPI.POST("/plans", handlers.AdminCreatePlanHandler)
    adminAPI.PUT("/plans/:id", handlers.AdminUpdatePlanHandler)
    adminAPI.DELETE("/plans/:id", handlers.AdminDeletePlanHandler)
    adminAPI.PUT("/api-keys/:id", handlers.AdminUpdateAPIKeyHandler)
    adminAPI.DELETE("/api-keys/:id", handlers.AdminDeleteAPIKeyHandler)
    adminAPI.GET("/stats", handlers.AdminStatsHandler)
    adminAPI.GET("/users", handlers.AdminUsersHandler)
    adminAPI.PUT("/users/:id/block", handlers.AdminToggleUserBlockHandler)
    adminAPI.GET("/payments", handlers.AdminPaymentsHandler)
    adminAPI.GET("/payment-stats", handlers.AdminPaymentStats)
    adminAPI.GET("/security-logs", handlers.AdminSecurityLogs)
    adminAPI.GET("/blocked-ips", handlers.AdminBlockedIPs)
    adminAPI.POST("/users/toggle-block", handlers.AdminToggleUserBlock)
    adminAPI.POST("/users/change-role", handlers.AdminChangeUserRole)
    adminAPI.POST("/users/delete", handlers.AdminDeleteUser)
    
    // ДОБАВЬ ЭТИ СТРОКИ:
    adminAPI.GET("/tenants", handlers.GetTenants)
    adminAPI.POST("/tenants", handlers.CreateTenant)
    adminAPI.PUT("/tenants/:id", handlers.UpdateTenant)
    adminAPI.DELETE("/tenants/:id", handlers.DeleteTenant)
    adminAPI.POST("/tenants/:id/switch", handlers.SwitchTenant)
}

// Админская страница для управления компаниями (отдельно)
adminTenants := r.Group("/admin/tenants")
adminTenants.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
{
    adminTenants.GET("/", handlers.TenantAdminPage)
}
   // API Documentation with back button
r.GET("/api-docs", func(c *gin.Context) {
    c.HTML(http.StatusOK, "api_with_back.html", gin.H{
        "title": "API Documentation - TeamSphere",
    })
})

// Original Swagger (без кнопки)
r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

// Обработка запросов Chrome DevTools
r.GET("/.well-known/appspecific/com.chrome.devtools.json", func(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "app-specific": true,
    })
})

r.NoRoute(func(c *gin.Context) {
    c.HTML(http.StatusNotFound, "404.html", gin.H{
        "Title":   "Страница не найдена - SaaSPro",
        "Version": "3.0",
    })
})

    r.NoRoute(func(c *gin.Context) {
        c.HTML(http.StatusNotFound, "404.html", gin.H{
            "Title":   "Страница не найдена - SaaSPro",
            "Version": "3.0",
        })
    })

    port := ":" + cfg.Port
    baseURL := "http://localhost:" + cfg.Port
    fmt.Printf("   🔒 Безопасность     %s/security-center\n", baseURL)
    fmt.Printf("📍 ВСЕ ИНТЕРФЕЙСЫ ДОСТУПНЫ ПО ССЫЛКАМ:\n\n")
    fmt.Printf("   🔹 Главная           %s/\n", baseURL)
    fmt.Printf("   🔹 Дашборд          %s/dashboard-improved\n", baseURL)
    fmt.Printf("   🔹 Админка          %s/admin\n", baseURL)
    fmt.Printf("   🔹 CRM              %s/crm\n", baseURL)
    fmt.Printf("   🔹 Аналитика        %s/analytics\n", baseURL)
    fmt.Printf("   🔹 Платежи          %s/payment\n", baseURL)
    fmt.Printf("   🔹 Тарифы           %s/pricing\n", baseURL)
    fmt.Printf("   🔹 Партнёры         %s/partner\n", baseURL)
    fmt.Printf("   🔹 Контакты         %s/contact\n", baseURL)
    fmt.Printf("   🔹 Логистика        %s/logistics\n", baseURL)
    fmt.Printf("   🔹 Отслеживание     %s/track\n\n", baseURL)
    fmt.Printf("   🔐 Вход             %s/login\n", baseURL)
    fmt.Printf("   🔐 Регистрация      %s/register\n", baseURL)
    fmt.Printf("   🔐 Восстановление   %s/forgot-password\n\n", baseURL)
    fmt.Printf("   ⚙️  Настройки       %s/settings\n", baseURL)
    fmt.Printf("   ⚙️  Пользователи    %s/users\n", baseURL)
    fmt.Printf("   ⚙️  Подписки        %s/subscriptions\n", baseURL)
    fmt.Printf("   ⚙️  Мои подписки    %s/my-subscriptions\n", baseURL)
    fmt.Printf("   👤 Профиль          %s/profile\n\n", baseURL)
    fmt.Printf("   💳 Оплата картой    %s/bank_card_payment\n", baseURL)
    fmt.Printf("   💳 USDT             %s/usdt-payment\n", baseURL)
    fmt.Printf("   💳 RUB              %s/rub-payment\n", baseURL)
    fmt.Printf("   💳 Успешно          %s/payment-success\n\n", baseURL)
    fmt.Printf("   📊 Админ (Fixed)    %s/admin-fixed\n", baseURL)
    fmt.Printf("   📊 Gold Admin       %s/gold-admin\n", baseURL)
    fmt.Printf("   📊 Админ БД         %s/database-admin\n\n", baseURL)
    fmt.Printf("   📈 Дашборд улучш.   %s/dashboard-improved\n", baseURL)
    fmt.Printf("   📈 Real-time        %s/realtime-dashboard\n", baseURL)
    fmt.Printf("   📈 Выручка          %s/revenue-dashboard\n", baseURL)
    fmt.Printf("   📈 Партнёрский      %s/partner-dashboard\n", baseURL)
    fmt.Printf("   📈 Унифицированный  %s/unified-dashboard\n\n", baseURL)
    fmt.Printf("   📡 API Health       %s/api/health\n", baseURL)
    fmt.Printf("   📡 CRM Health       %s/api/crm/health\n", baseURL)
    fmt.Printf("   📡 Система          %s/api/system/stats\n", baseURL)
    fmt.Printf("   📡 Тест             %s/api/test\n", baseURL)
    fmt.Printf("   📡 Отслеживание API %s/api/delivery/track/:id\n\n", baseURL)
    fmt.Printf("============================================================\n")
    fmt.Printf("   ⚙️  Конфигурация: порт=%s, режим=%s, БД=%s\n", cfg.Port, cfg.Env, cfg.DBName)
    fmt.Printf("   🔒 SKIP_AUTH=%v – все защищённые страницы открыты без токена\n", cfg.SkipAuth)
    fmt.Printf("============================================================\n")

    log.Printf("🚀 Сервер запущен на порту %s", port)
    
   // Запуск планировщиков
handlers.StartSyncScheduler()
handlers.StartBitrixSyncScheduler()
handlers.StartTeamSphereScheduler() // Планировщик TeamSphere

// Favicon обработка
r.GET("/favicon.ico", func(c *gin.Context) {
    c.File("./static/favicon.ico")
})  
  r.GET("/team/team", func(c *gin.Context) {
    c.HTML(http.StatusOK, "team_page.html", gin.H{
        "title": "Команда | TeamSphere",
    })
})

  // Tasks page
    r.GET("/tasks", func(c *gin.Context) {
        c.HTML(http.StatusOK, "tasks.html", gin.H{
            "title": "Задачи - TeamSphere",
        })
    })
    
// Chat page
r.GET("/chat", func(c *gin.Context) {
    c.HTML(http.StatusOK, "chat.html", gin.H{
        "title": "Чат - TeamSphere",
    })
})
     // TeamSphere Calendar page
r.GET("/team-calendar", func(c *gin.Context) {
    c.HTML(http.StatusOK, "calendar.html", gin.H{
        "title": "Календарь - TeamSphere",
    })
})
    


r.GET("/security-center", func(c *gin.Context) {
    c.HTML(http.StatusOK, "security_universal.html", gin.H{
        "title": "Security Center | SaaSPro",
    })
})

 // Универсальная аналитика - новый путь
r.GET("/analytics-center", func(c *gin.Context) {
    c.HTML(http.StatusOK, "analytics_universal.html", gin.H{
        "title": "Analytics Center | SaaSPro",
    })
})
   r.Run(port)
}




}









