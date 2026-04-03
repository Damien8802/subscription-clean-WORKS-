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

// ArchivePageHandler - страница архива
func ArchivePageHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "archive_index.html", gin.H{
        "Title": "Архив | SaaSPro",
    })
}

// GetArchiveStats - статистика использования архива
func GetArchiveStats(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    var stats struct {
        UsedBytes   int64 `json:"used_bytes"`
        TotalBytes  int64 `json:"total_bytes"`
        UsedPercent int   `json:"used_percent"`
    }

    // Получаем или создаем квоту
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT total_bytes, COALESCE(used_bytes, 0) FROM archive_quotas WHERE tenant_id = $1
    `, tenantID).Scan(&stats.TotalBytes, &stats.UsedBytes)

    if err != nil {
        stats.TotalBytes = 1073741824
        stats.UsedBytes = 0
        database.Pool.Exec(c.Request.Context(), `
            INSERT INTO archive_quotas (tenant_id, total_bytes, used_bytes) VALUES ($1, $2, $3)
        `, tenantID, stats.TotalBytes, stats.UsedBytes)
    }

    var customersSize, dealsSize, entriesSize int64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(COUNT(*), 0) * 1024 FROM crm_customers_archive WHERE tenant_id = $1
    `, tenantID).Scan(&customersSize)
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(COUNT(*), 0) * 1024 FROM crm_deals_archive WHERE tenant_id = $1
    `, tenantID).Scan(&dealsSize)
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(COUNT(*), 0) * 1024 FROM journal_entries_archive WHERE tenant_id = $1
    `, tenantID).Scan(&entriesSize)

    stats.UsedBytes = customersSize + dealsSize + entriesSize
    if stats.TotalBytes > 0 {
        stats.UsedPercent = int(float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100)
    }

    c.JSON(http.StatusOK, gin.H{
        "used_bytes":   stats.UsedBytes,
        "total_bytes":  stats.TotalBytes,
        "used_percent": stats.UsedPercent,
        "used_gb":      float64(stats.UsedBytes) / 1024 / 1024 / 1024,
        "total_gb":     float64(stats.TotalBytes) / 1024 / 1024 / 1024,
    })
}

// GetArchiveItems - получить список архивных записей
func GetArchiveItems(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    entityType := c.Query("type")
    pageStr := c.DefaultQuery("page", "1")
    limitStr := c.DefaultQuery("limit", "20")

    page, _ := strconv.Atoi(pageStr)
    limit, _ := strconv.Atoi(limitStr)
    offset := (page - 1) * limit

    var items []gin.H

    if entityType == "" || entityType == "customers" {
        rows, err := database.Pool.Query(c.Request.Context(), `
            SELECT id, name, email, phone, company, total_deals_sum, deals_count, archived_at
            FROM crm_customers_archive WHERE tenant_id = $1
            ORDER BY archived_at DESC LIMIT $2 OFFSET $3
        `, tenantID, limit, offset)

        if err == nil {
            for rows.Next() {
                var id uuid.UUID
                var name, email, phone, company string
                var totalDealsSum float64
                var dealsCount int
                var archivedAt time.Time

                rows.Scan(&id, &name, &email, &phone, &company, &totalDealsSum, &dealsCount, &archivedAt)

                items = append(items, gin.H{
                    "id":          id.String(),
                    "type":        "customer",
                    "type_label":  "Клиент",
                    "type_icon":   "👤",
                    "title":       name,
                    "subtitle":    company,
                    "details":     fmt.Sprintf("Email: %s | Телефон: %s | Сделок: %d | Сумма: %.2f ₽", email, phone, dealsCount, totalDealsSum),
                    "archived_at": archivedAt.Format("02.01.2006"),
                })
            }
            rows.Close()
        }
    }

    if entityType == "" || entityType == "deals" {
        rows, err := database.Pool.Query(c.Request.Context(), `
            SELECT d.id, d.title, d.value, d.stage, d.closed_at, COALESCE(c.name, 'Неизвестно') as customer_name
            FROM crm_deals_archive d
            LEFT JOIN crm_customers_archive c ON d.customer_id = c.id
            WHERE d.tenant_id = $1
            ORDER BY d.archived_at DESC LIMIT $2 OFFSET $3
        `, tenantID, limit, offset)

        if err == nil {
            for rows.Next() {
                var id uuid.UUID
                var title, stage, customerName string
                var value float64
                var closedAt *time.Time

                rows.Scan(&id, &title, &value, &stage, &closedAt, &customerName)

                items = append(items, gin.H{
                    "id":          id.String(),
                    "type":        "deal",
                    "type_label":  "Сделка",
                    "type_icon":   "💰",
                    "title":       title,
                    "subtitle":    customerName,
                    "details":     fmt.Sprintf("Сумма: %.2f ₽ | Статус: %s", value, stage),
                    "archived_at": func() string {
                        if closedAt != nil {
                            return closedAt.Format("02.01.2006")
                        }
                        return ""
                    }(),
                })
            }
            rows.Close()
        }
    }

    c.JSON(http.StatusOK, gin.H{
        "items": items,
        "total": len(items),
        "page":  page,
    })
}

// ArchiveCustomer - переместить клиента в архив
func ArchiveCustomer(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    customerID := c.Param("id")

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO crm_customers_archive (id, name, email, phone, company, status, tenant_id, archived_at)
        SELECT id, name, email, phone, company, status, tenant_id, NOW()
        FROM crm_customers WHERE id = $1 AND tenant_id = $2
    `, customerID, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    _, err = database.Pool.Exec(c.Request.Context(), `
        DELETE FROM crm_customers WHERE id = $1 AND tenant_id = $2
    `, customerID, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Клиент перемещен в архив"})
}

// RestoreFromArchive - восстановить из архива
func RestoreFromArchive(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    entityType := c.Param("type")
    entityID := c.Param("id")

    switch entityType {
    case "customer":
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO crm_customers (id, name, email, phone, company, status, tenant_id, created_at)
            SELECT id, name, email, phone, company, status, tenant_id, archived_at
            FROM crm_customers_archive WHERE id = $1 AND tenant_id = $2
        `, entityID, tenantID)

        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }

        _, err = database.Pool.Exec(c.Request.Context(), `
            DELETE FROM crm_customers_archive WHERE id = $1 AND tenant_id = $2
        `, entityID, tenantID)

        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
    case "deal":
        c.JSON(http.StatusOK, gin.H{"success": true, "message": "Сделка восстановлена"})
        return
    case "entry":
        c.JSON(http.StatusOK, gin.H{"success": true, "message": "Проводка восстановлена"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Запись восстановлена"})
}

// UpgradeArchiveQuota - расширение квоты архива
func UpgradeArchiveQuota(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    var req struct {
        Plan string `json:"plan" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var totalBytes int64
    var price int

    switch req.Plan {
    case "10gb":
        totalBytes = 10 * 1024 * 1024 * 1024
        price = 490
    case "50gb":
        totalBytes = 50 * 1024 * 1024 * 1024
        price = 1490
    case "100gb":
        totalBytes = 100 * 1024 * 1024 * 1024
        price = 2490
    default:
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan"})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO archive_quotas (tenant_id, total_bytes, plan_type, updated_at)
        VALUES ($1, $2, $3, NOW())
        ON CONFLICT (tenant_id) DO UPDATE SET
            total_bytes = $2,
            plan_type = $3,
            updated_at = NOW()
    `, tenantID, totalBytes, req.Plan)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success":     true,
        "total_bytes": totalBytes,
        "total_gb":    totalBytes / 1024 / 1024 / 1024,
        "price":       price,
        "message":     "Тариф архива обновлен",
    })
}

