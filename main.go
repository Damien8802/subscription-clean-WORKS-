package main

import (
    "embed"
    "encoding/json"
    "fmt"
    "html/template"
    "io/fs"
    "log"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/handlers"
    "subscription-system/middleware"
)

//go:embed templates/*.html
var templateFS embed.FS

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

    handlers.InitAuthHandler(cfg)

    if cfg.Env == "release" {
        gin.SetMode(gin.ReleaseMode)
    }

    r := gin.New()
    r.Use(gin.Logger())
    r.Use(gin.Recovery())
    r.Use(middleware.Logger())
    r.SetTrustedProxies(cfg.TrustedProxies)
    r.Use(middleware.SetupCORS(cfg))

    // Загрузка шаблонов
    subFS, err := fs.Sub(templateFS, "templates")
    if err != nil {
        log.Fatalf("❌ Не удалось открыть встроенные шаблоны: %v", err)
    }
    tmpl := template.New("").Funcs(template.FuncMap{
        "jsonParse": func(s json.RawMessage) []interface{} {
            var arr []interface{}
            err := json.Unmarshal(s, &arr)
            if err != nil {
                return []interface{}{}
            }
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
            switch v := val.(type) {
            case nil:
                return defaultVal
            case string:
                if v == "" {
                    return defaultVal
                }
            }
            if val == nil {
                return defaultVal
            }
            return val
        },
    })
    tmpl = template.Must(tmpl.ParseFS(subFS, "*.html"))
    r.SetHTMLTemplate(tmpl)
    log.Println("✅ Шаблоны загружены из embed.FS")

        // ========== СТАТИКА ==========
    r.Static("/static", cfg.StaticPath)
    r.Static("/frontend", cfg.FrontendPath)
    r.Static("/app", "C:/Projects/subscription-clean-WORKS/telegram-mini-app")
        // Для PWA – отдаём манифест и service-worker из папки telegram-mini-app
    r.GET("/manifest.json", func(c *gin.Context) {
        c.File("./telegram-mini-app/manifest.json")
    })
    r.GET("/service-worker.js", func(c *gin.Context) {
        c.File("./telegram-mini-app/service-worker.js")
    })
r.GET("/app", func(c *gin.Context) {
        c.File("C:/Projects/subscription-clean-WORKS/telegram-mini-app/index.html")
    })// ========== РЕДИРЕКТЫ ==========
    r.GET("/dashboard_improved", func(c *gin.Context) {
        c.Redirect(http.StatusMovedPermanently, "/dashboard-improved")
    })
    r.GET("/dashboard", func(c *gin.Context) {
        c.Redirect(http.StatusMovedPermanently, "/dashboard-improved")
    })
    r.GET("/delivery", func(c *gin.Context) {
        c.Redirect(http.StatusMovedPermanently, "/logistics")
    })

    // ========== ГРУППЫ МАРШРУТОВ ==========
    public := r.Group("/")
    {
        public.GET("/", handlers.HomeHandler)
        public.GET("/about", handlers.AboutHandler)
        public.GET("/contact", handlers.ContactHandler)
        public.GET("/info", handlers.InfoHandler)
        public.GET("/pricing", handlers.PricingPageHandler)
        public.GET("/partner", handlers.PartnerHandler)
        public.GET("/referral", handlers.ReferralHandler)
    }

    authPages := r.Group("/")
    {
        authPages.GET("/login", handlers.LoginPageHandler)
        authPages.GET("/register", handlers.RegisterPageHandler)
        authPages.GET("/forgot-password", handlers.ForgotPasswordHandler)
    }

    protected := r.Group("/")
    protected.Use(middleware.AuthMiddleware(cfg))
    {
        protected.GET("/settings", handlers.SettingsHandler)
        protected.GET("/my-subscriptions", handlers.MySubscriptionsPageHandler)
        protected.GET("/security", handlers.SecurityHandler)
        protected.GET("/security-hub", handlers.SecurityHubHandler)
        protected.GET("/security-panel", handlers.SecurityPanelHandler)
        protected.GET("/integrations", handlers.IntegrationsHandler)
        protected.GET("/monetization", handlers.MonetizationHandler)
        protected.GET("/profile", handlers.ProfilePageHandler)
    }

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
        admin.GET("/analytics", handlers.AnalyticsHandler)
        admin.GET("/crm", handlers.CRMHandler)
        admin.GET("/admin/api-keys", handlers.AdminAPIKeysHandler)
    }

    dashboards := r.Group("/")
    dashboards.Use(middleware.AuthMiddleware(cfg))
    {
        dashboards.GET("/dashboard-improved", handlers.DashboardImprovedHandler)
        dashboards.GET("/realtime-dashboard", handlers.RealtimeDashboardHandler)
        dashboards.GET("/revenue-dashboard", handlers.RevenueDashboardHandler)
        dashboards.GET("/partner-dashboard", handlers.PartnerDashboardHandler)
        dashboards.GET("/unified-dashboard", handlers.UnifiedDashboardHandler)
    }

    payments := r.Group("/")
    payments.Use(middleware.AuthMiddleware(cfg))
    {
        payments.GET("/payment", handlers.PaymentHandler)
        payments.GET("/bank_card_payment", handlers.BankCardPaymentHandler)
        payments.GET("/payment-success", handlers.PaymentSuccessHandler)
        payments.GET("/usdt-payment", handlers.USDTPaymentHandler)
        payments.GET("/rub-payment", handlers.RUBPaymentHandler)
    }

    logistics := r.Group("/")
    logistics.Use(middleware.AuthMiddleware(cfg))
    {
        logistics.GET("/logistics", handlers.LogisticsHandler)
        logistics.GET("/track", handlers.TrackHandler)
    }

    deliveryAPI := r.Group("/api/delivery")
    deliveryAPI.Use(middleware.AuthMiddleware(cfg))
    {
        deliveryAPI.GET("/track/:trackingNumber", handlers.TrackAPIHandler)
    }

    // API (JSON)
    api := r.Group("/api")
    {
        api.GET("/health", handlers.HealthHandler)
        api.GET("/crm/health", handlers.CRMHealthHandler)
        api.GET("/system/stats", handlers.SystemStatsHandler)
        api.GET("/test", handlers.TestHandler)
        api.POST("/auth/register", handlers.RegisterHandler)
        api.POST("/auth/login", handlers.LoginHandler)
        api.POST("/auth/refresh", handlers.RefreshHandler)
        api.POST("/user/profile", handlers.UpdateProfileHandler)
        api.POST("/user/password", handlers.UpdatePasswordHandler)
        api.GET("/plans", handlers.GetPlansHandler)
        api.POST("/subscriptions", handlers.CreateSubscriptionHandler)
        api.POST("/ai/ask", handlers.AIAskHandler)
        api.GET("/user/subscriptions", handlers.GetUserSubscriptionsHandler)
        api.GET("/user/ai-usage", handlers.GetUserAIUsageHandler)
        api.POST("/telegram/ensure-key", handlers.EnsureAPIKeyForTelegram)
        api.POST("/webapp/auth", handlers.WebAppAuthHandler)
        api.POST("/chat/save", handlers.SaveChatMessage)
        api.GET("/chat/history", handlers.GetChatHistory)
        api.POST("/ai/ask-with-file", handlers.AskWithFileHandler)
        api.POST("/knowledge/upload", handlers.UploadKnowledgeHandler)
        api.GET("/knowledge/list", handlers.ListKnowledgeHandler)
        api.DELETE("/knowledge/delete/:id", handlers.DeleteKnowledgeHandler)

        // Защищённые эндпоинты API
        authAPI := api.Group("/")
        authAPI.Use(middleware.AuthMiddleware(cfg))
        {
            authAPI.GET("/user/profile", handlers.GetUserProfile)
            authAPI.GET("/user/ai-history", handlers.GetUserAIHistoryHandler)
        }
    }

    // ========== УПРАВЛЕНИЕ API-КЛЮЧАМИ ==========
    userKeys := r.Group("/api/user/keys")
    userKeys.Use(middleware.AuthMiddleware(cfg))
    {
        userKeys.GET("/", handlers.ListAPIKeysHandler)
        userKeys.POST("/", handlers.GenerateAPIKeyHandler)
        userKeys.DELETE("/:id", handlers.RevokeAPIKeyHandler)
    }

    // ========== AI GATEWAY ==========
    v1 := r.Group("/api/v1")
    v1.Use(middleware.APIKeyAuthMiddleware())
    {
        v1.POST("/chat/completions", handlers.ChatCompletionsHandler)
    }

    // Админские API
    adminAPI := r.Group("/api/admin")
    adminAPI.Use(middleware.AuthMiddleware(cfg), middleware.AdminMiddleware(cfg))
    {
        //adminAPI.PUT("/users/:id/role", handlers.AdminUpdateUserRoleHandler)
        //adminAPI.DELETE("/users/:id", handlers.AdminDeleteUserHandler)
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
        adminAPI.POST("/broadcast", handlers.AdminBroadcastHandler)
    }

    // 404
    r.NoRoute(func(c *gin.Context) {
        c.HTML(http.StatusNotFound, "404.html", gin.H{
            "Title":   "Страница не найдена - SaaSPro",
            "Version": "3.0",
        })
    })

    // Баннер
    port := ":" + cfg.Port
    baseURL := "http://localhost:" + cfg.Port

    fmt.Printf("\n============================================================\n")
    fmt.Printf("   🚀 SaaSPro - ПОЛНАЯ ВЕРСИЯ 3.0 (УНИФИЦИРОВАННАЯ)\n")
    fmt.Printf("============================================================\n\n")
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
    fmt.Printf("   🔒 Безопасность     %s/security\n", baseURL)
    fmt.Printf("   🔒 Центр безопасн.  %s/security-hub\n", baseURL)
    fmt.Printf("   🔒 Панель безопасн. %s/security-panel\n\n", baseURL)
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
    r.Run(port)
}


