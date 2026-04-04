package handlers

import (
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// MarketplacePageHandler - главная страница маркетплейса
func MarketplacePageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "marketplace_index.html", gin.H{
        "Title": "Маркетплейс | SaaSPro",
    })
}

// GetMarketplaceApps - получить каталог приложений
func GetMarketplaceApps(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    category := c.Query("category")
    search := c.Query("search")
    featured := c.Query("featured") == "true"

    query := `SELECT id, name, slug, description, icon_url, category, price_monthly, 
              price_yearly, rating, reviews_count, downloads_count, featured, status
              FROM marketplace_apps 
              WHERE tenant_id = $1 AND status = 'active'`
    args := []interface{}{tenantID}
    argIdx := 2

    if category != "" && category != "all" {
        query += " AND category = $" + strconv.Itoa(argIdx)
        args = append(args, category)
        argIdx++
    }
    if featured {
        query += " AND featured = true"
    }
    if search != "" {
        query += " AND (name ILIKE $" + strconv.Itoa(argIdx) + " OR description ILIKE $" + strconv.Itoa(argIdx) + ")"
        args = append(args, "%"+search+"%")
    }
    query += " ORDER BY featured DESC, rating DESC, downloads_count DESC"

    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var apps []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, slug, description, iconUrl, category string
        var priceMonthly, priceYearly float64
        var rating float64
        var reviewsCount, downloadsCount int
        var featured bool
        var status string

        rows.Scan(&id, &name, &slug, &description, &iconUrl, &category, &priceMonthly, &priceYearly,
            &rating, &reviewsCount, &downloadsCount, &featured, &status)

        var purchased bool
        database.Pool.QueryRow(c.Request.Context(), `
            SELECT EXISTS(SELECT 1 FROM marketplace_purchases 
            WHERE company_id = $1 AND app_id = $2 AND status = 'active')
        `, tenantID, id).Scan(&purchased)

        apps = append(apps, gin.H{
            "id":            id.String(),
            "name":          name,
            "slug":          slug,
            "description":   description,
            "icon_url":      iconUrl,
            "category":      category,
            "price_monthly": priceMonthly,
            "price_yearly":  priceYearly,
            "rating":        rating,
            "reviews_count": reviewsCount,
            "downloads":     downloadsCount,
            "featured":      featured,
            "purchased":     purchased,
        })
    }

    c.JSON(http.StatusOK, gin.H{"apps": apps})
}

// PurchaseApp - покупка приложения
func PurchaseApp(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    userID := c.GetString("user_id")

    var req struct {
        AppID         string `json:"app_id" binding:"required"`
        BillingPeriod string `json:"billing_period"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.BillingPeriod == "" {
        req.BillingPeriod = "monthly"
    }

    var appID uuid.UUID
    var price float64
    var appName string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, CASE WHEN $1 = 'yearly' THEN price_yearly ELSE price_monthly END, name
        FROM marketplace_apps WHERE id = $2 AND tenant_id = $3 AND status = 'active'
    `, req.BillingPeriod, req.AppID, tenantID).Scan(&appID, &price, &appName)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
        return
    }

    var exists bool
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT EXISTS(SELECT 1 FROM marketplace_purchases 
        WHERE company_id = $1 AND app_id = $2 AND status = 'active')
    `, tenantID, appID).Scan(&exists)

    if exists {
        c.JSON(http.StatusBadRequest, gin.H{"error": "App already purchased"})
        return
    }

    expiresAt := time.Now().AddDate(0, 1, 0)
    if req.BillingPeriod == "yearly" {
        expiresAt = time.Now().AddDate(1, 0, 0)
    }

    purchaseID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO marketplace_purchases (id, company_id, app_id, user_id, amount, expires_at, auto_renew, tenant_id)
        VALUES ($1, $2, $3, $4, $5, $6, true, $7)
    `, purchaseID, tenantID, appID, userID, price, expiresAt, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create purchase"})
        return
    }

    database.Pool.Exec(c.Request.Context(), `
        UPDATE marketplace_apps SET downloads_count = downloads_count + 1 WHERE id = $1
    `, appID)

    c.JSON(http.StatusOK, gin.H{
        "success":     true,
        "purchase_id": purchaseID.String(),
        "message":     fmt.Sprintf("Приложение %s успешно активировано!", appName),
        "expires_at":  expiresAt.Format("02.01.2006"),
    })
}

// GetMyPurchases - получить мои покупки
func GetMyPurchases(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.id, a.name, a.icon_url, p.amount, p.purchased_at, p.expires_at, p.status,
               a.slug, a.category
        FROM marketplace_purchases p
        JOIN marketplace_apps a ON p.app_id = a.id
        WHERE p.company_id = $1 AND p.status = 'active'
        ORDER BY p.purchased_at DESC
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var purchases []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, iconUrl, slug, category string
        var amount float64
        var purchasedAt, expiresAt time.Time
        var status string

        rows.Scan(&id, &name, &iconUrl, &amount, &purchasedAt, &expiresAt, &status, &slug, &category)

        purchases = append(purchases, gin.H{
            "id":           id.String(),
            "name":         name,
            "icon_url":     iconUrl,
            "slug":         slug,
            "category":     category,
            "amount":       amount,
            "purchased_at": purchasedAt.Format("02.01.2006"),
            "expires_at":   expiresAt.Format("02.01.2006"),
            "is_active":    expiresAt.After(time.Now()),
            "status":       status,
        })
    }

    c.JSON(http.StatusOK, gin.H{"purchases": purchases})
}

// AddReview - добавить отзыв на приложение
func AddReview(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }
    userID := c.GetString("user_id")

    var req struct {
        AppID  string `json:"app_id" binding:"required"`
        Rating int    `json:"rating" binding:"required,min=1,max=5"`
        Review string `json:"review"`
        Pros   string `json:"pros"`
        Cons   string `json:"cons"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var purchased bool
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT EXISTS(SELECT 1 FROM marketplace_purchases 
        WHERE company_id = $1 AND app_id = $2 AND status = 'active')
    `, tenantID, req.AppID).Scan(&purchased)

    if !purchased {
        c.JSON(http.StatusForbidden, gin.H{"error": "You can only review apps you purchased"})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO marketplace_reviews (company_id, app_id, user_id, rating, review, pros, cons, tenant_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, tenantID, req.AppID, userID, req.Rating, req.Review, req.Pros, req.Cons, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    database.Pool.Exec(c.Request.Context(), `
        UPDATE marketplace_apps 
        SET rating = (SELECT AVG(rating) FROM marketplace_reviews WHERE app_id = $1),
            reviews_count = (SELECT COUNT(*) FROM marketplace_reviews WHERE app_id = $1)
        WHERE id = $1
    `, req.AppID)

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Отзыв добавлен"})
}

// GetMarketplaceApp - детальная страница приложения
func GetMarketplaceApp(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    slug := c.Param("slug")

    var app struct {
        ID              uuid.UUID
        Name            string
        Slug            string
        Description     string
        LongDescription string
        IconUrl         string
        CoverImage      string
        Category        string
        Tags            []string
        PriceMonthly    float64
        PriceYearly     float64
        Rating          float64
        ReviewsCount    int
        DownloadsCount  int
        MinPlan         string
        DocumentationUrl string
        SupportEmail    string
        PartnerName     string
    }

    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT a.id, a.name, a.slug, a.description, COALESCE(a.long_description, ''),
               a.icon_url, COALESCE(a.cover_image, ''), a.category, a.tags,
               a.price_monthly, a.price_yearly, a.rating, a.reviews_count, a.downloads_count,
               a.min_plan, a.documentation_url, a.support_email, COALESCE(p.company_name, '')
        FROM marketplace_apps a
        LEFT JOIN marketplace_partners p ON a.partner_id = p.id
        WHERE a.slug = $1 AND a.tenant_id = $2 AND a.status = 'active'
    `, slug, tenantID).Scan(
        &app.ID, &app.Name, &app.Slug, &app.Description, &app.LongDescription,
        &app.IconUrl, &app.CoverImage, &app.Category, &app.Tags,
        &app.PriceMonthly, &app.PriceYearly, &app.Rating, &app.ReviewsCount, &app.DownloadsCount,
        &app.MinPlan, &app.DocumentationUrl, &app.SupportEmail, &app.PartnerName,
    )

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "App not found"})
        return
    }

    // Получаем отзывы
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT rating, review, pros, cons, created_at, u.name
        FROM marketplace_reviews r
        JOIN users u ON r.user_id = u.id
        WHERE r.app_id = $1
        ORDER BY r.created_at DESC LIMIT 10
    `, app.ID)
    
    var reviewsList []gin.H
    if err == nil {
        defer rows.Close()
        for rows.Next() {
            var rating int
            var review, pros, cons, userName string
            var createdAt time.Time
            rows.Scan(&rating, &review, &pros, &cons, &createdAt, &userName)
            reviewsList = append(reviewsList, gin.H{
                "rating":     rating,
                "review":     review,
                "pros":       pros,
                "cons":       cons,
                "user_name":  userName,
                "created_at": createdAt.Format("02.01.2006"),
            })
        }
    }

    // Проверяем, куплено ли приложение
    var purchased bool
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT EXISTS(SELECT 1 FROM marketplace_purchases 
        WHERE company_id = $1 AND app_id = $2 AND status = 'active')
    `, tenantID, app.ID).Scan(&purchased)

    c.JSON(http.StatusOK, gin.H{
        "app":       app,
        "reviews":   reviewsList,
        "purchased": purchased,
    })
}

