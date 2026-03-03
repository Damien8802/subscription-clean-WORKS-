package handlers

import (
    "database/sql"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/config"
    "subscription-system/database"
    "subscription-system/services"
)

var notifier *services.NotificationService

// InitNotifier инициализирует сервис уведомлений (вызывается из main)
func InitNotifier(cfg *config.Config) {
    notifier = services.NewNotificationService(cfg)
}

// Customer представляет клиента в CRM
type Customer struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Email       string    `json:"email"`
    Phone       string    `json:"phone"`
    Company     string    `json:"company"`
    Status      string    `json:"status"`
    Responsible string    `json:"responsible"`
    Source      string    `json:"source"`
    Comment     string    `json:"comment"`
    CreatedAt   time.Time `json:"created_at"`
    LastSeen    time.Time `json:"last_seen"`
}

// Deal представляет сделку в CRM
type Deal struct {
    ID            string     `json:"id"`
    CustomerID    string     `json:"customer_id"`
    Title         string     `json:"title"`
    Value         float64    `json:"value"`
    Stage         string     `json:"stage"`
    Probability   int        `json:"probability"`
    Responsible   string     `json:"responsible"`
    Source        string     `json:"source"`
    Comment       string     `json:"comment"`
    ExpectedClose *time.Time `json:"expected_close,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    ClosedAt      *time.Time `json:"closed_at,omitempty"`
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

// getPaginationParams извлекает page и page_size из запроса с значениями по умолчанию
func getPaginationParams(c *gin.Context) (page, pageSize int) {
    page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
    if err != nil || page < 1 {
        page = 1
    }
    pageSize, err = strconv.Atoi(c.DefaultQuery("page_size", "20"))
    if err != nil || pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }
    return page, pageSize
}

// ========== СТРАНИЦА CRM ==========

// CRMHandler отображает страницу CRM
func CRMHandler(c *gin.Context) {
    c.HTML(http.StatusOK, "crm.html", gin.H{
        "Title": "CRM система - SaaSPro",
    })
}

// CRMHealthHandler возвращает статус CRM
func CRMHealthHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "operational",
        "crm":    "online",
        "time":   time.Now().Unix(),
    })
}

// ========== КЛИЕНТЫ ==========

// GetCustomers возвращает список клиентов с пагинацией, поиском и фильтрацией по дате создания
func GetCustomers(c *gin.Context) {
    status := c.Query("status")
    search := c.Query("search")
    createdFrom := c.Query("created_from")
    createdTo := c.Query("created_to")
    page, pageSize := getPaginationParams(c)
    offset := (page - 1) * pageSize

    // ---- Подсчёт общего количества записей с учётом всех фильтров ----
    countQuery := `SELECT COUNT(*) FROM crm_customers`
    countArgs := []interface{}{}
    whereClause := ""

    if status != "" {
        whereClause += " status = $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, status)
    }
    if search != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " (name ILIKE '%' || $" + strconv.Itoa(len(countArgs)+1) + " || '%' OR email ILIKE '%' || $" + strconv.Itoa(len(countArgs)+1) + " || '%')"
        countArgs = append(countArgs, search)
    }
    if createdFrom != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " created_at >= $" + strconv.Itoa(len(countArgs)+1) + "::date"
        countArgs = append(countArgs, createdFrom)
    }
    if createdTo != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " created_at < ($" + strconv.Itoa(len(countArgs)+1) + "::date + '1 day'::interval)"
        countArgs = append(countArgs, createdTo)
    }

    if whereClause != "" {
        countQuery += " WHERE" + whereClause
    }

    var total int
    err := database.Pool.QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // ---- Запрос данных с теми же фильтрами и пагинацией ----
    query := `SELECT id, name, email, phone, company, status, responsible, source, comment, created_at, last_seen
              FROM crm_customers`
    args := []interface{}{}
    whereData := ""

    if status != "" {
        whereData += " status = $" + strconv.Itoa(len(args)+1)
        args = append(args, status)
    }
    if search != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " (name ILIKE '%' || $" + strconv.Itoa(len(args)+1) + " || '%' OR email ILIKE '%' || $" + strconv.Itoa(len(args)+1) + " || '%')"
        args = append(args, search)
    }
    if createdFrom != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " created_at >= $" + strconv.Itoa(len(args)+1) + "::date"
        args = append(args, createdFrom)
    }
    if createdTo != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " created_at < ($" + strconv.Itoa(len(args)+1) + "::date + '1 day'::interval)"
        args = append(args, createdTo)
    }

    if whereData != "" {
        query += " WHERE" + whereData
    }
    query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
    args = append(args, pageSize, offset)

    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    customers := make([]Customer, 0)
    for rows.Next() {
        var cst Customer
        err := rows.Scan(&cst.ID, &cst.Name, &cst.Email, &cst.Phone, &cst.Company, &cst.Status,
            &cst.Responsible, &cst.Source, &cst.Comment, &cst.CreatedAt, &cst.LastSeen)
        if err != nil {
            continue
        }
        customers = append(customers, cst)
    }

    c.JSON(http.StatusOK, gin.H{
        "data":        customers,
        "total":       total,
        "page":        page,
        "page_size":   pageSize,
        "total_pages": (total + pageSize - 1) / pageSize,
    })
}

// CreateCustomer создаёт нового клиента
func CreateCustomer(c *gin.Context) {
    var req Customer
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var id string
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO crm_customers (name, email, phone, company, status, responsible, source, comment, created_at, last_seen)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
        RETURNING id
    `, req.Name, req.Email, req.Phone, req.Company, req.Status,
        req.Responsible, req.Source, req.Comment).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // NOTIFY: уведомление о создании клиента
    if notifier != nil {
        notifier.NotifyCustomerCreated(req.Name, req.Email, req.Phone, req.Company, req.Responsible)
    }

    c.JSON(http.StatusCreated, gin.H{"id": id})
}

// UpdateCustomer обновляет данные клиента
func UpdateCustomer(c *gin.Context) {
    id := c.Param("id")
    var req Customer
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE crm_customers
        SET name = $1, email = $2, phone = $3, company = $4, status = $5,
            responsible = $6, source = $7, comment = $8, last_seen = NOW()
        WHERE id = $9
    `, req.Name, req.Email, req.Phone, req.Company, req.Status,
        req.Responsible, req.Source, req.Comment, id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // NOTIFY: уведомление об изменении клиента
    if notifier != nil {
        notifier.NotifyCustomerUpdated(id, req.Name, req.Email, req.Phone)
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteCustomer удаляет клиента
func DeleteCustomer(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), "DELETE FROM crm_customers WHERE id = $1", id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== СДЕЛКИ ==========

// GetDeals возвращает список сделок с пагинацией, поиском и фильтрацией по сумме и дате закрытия
func GetDeals(c *gin.Context) {
    stage := c.Query("stage")
    search := c.Query("search")
    valueMin := c.Query("value_min")
    valueMax := c.Query("value_max")
    closeFrom := c.Query("close_from")
    closeTo := c.Query("close_to")
    page, pageSize := getPaginationParams(c)
    offset := (page - 1) * pageSize

    // ---- Подсчёт общего количества с учётом всех фильтров ----
    countQuery := `SELECT COUNT(*) FROM crm_deals`
    countArgs := []interface{}{}
    whereClause := ""

    if stage != "" {
        whereClause += " stage = $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, stage)
    }
    if search != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " title ILIKE '%' || $" + strconv.Itoa(len(countArgs)+1) + " || '%'"
        countArgs = append(countArgs, search)
    }
    if valueMin != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " value >= $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, valueMin)
    }
    if valueMax != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " value <= $" + strconv.Itoa(len(countArgs)+1)
        countArgs = append(countArgs, valueMax)
    }
    if closeFrom != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " expected_close >= $" + strconv.Itoa(len(countArgs)+1) + "::date"
        countArgs = append(countArgs, closeFrom)
    }
    if closeTo != "" {
        if whereClause != "" {
            whereClause += " AND"
        }
        whereClause += " expected_close < ($" + strconv.Itoa(len(countArgs)+1) + "::date + '1 day'::interval)"
        countArgs = append(countArgs, closeTo)
    }

    if whereClause != "" {
        countQuery += " WHERE" + whereClause
    }

    var total int
    err := database.Pool.QueryRow(c.Request.Context(), countQuery, countArgs...).Scan(&total)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // ---- Запрос данных с теми же фильтрами ----
    query := `SELECT id, customer_id, title, value, stage, probability, responsible, source, comment, expected_close, created_at, closed_at
              FROM crm_deals`
    args := []interface{}{}
    whereData := ""

    if stage != "" {
        whereData += " stage = $" + strconv.Itoa(len(args)+1)
        args = append(args, stage)
    }
    if search != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " title ILIKE '%' || $" + strconv.Itoa(len(args)+1) + " || '%'"
        args = append(args, search)
    }
    if valueMin != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " value >= $" + strconv.Itoa(len(args)+1)
        args = append(args, valueMin)
    }
    if valueMax != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " value <= $" + strconv.Itoa(len(args)+1)
        args = append(args, valueMax)
    }
    if closeFrom != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " expected_close >= $" + strconv.Itoa(len(args)+1) + "::date"
        args = append(args, closeFrom)
    }
    if closeTo != "" {
        if whereData != "" {
            whereData += " AND"
        }
        whereData += " expected_close < ($" + strconv.Itoa(len(args)+1) + "::date + '1 day'::interval)"
        args = append(args, closeTo)
    }

    if whereData != "" {
        query += " WHERE" + whereData
    }
    query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2)
    args = append(args, pageSize, offset)

    rows, err := database.Pool.Query(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    deals := make([]Deal, 0)
    for rows.Next() {
        var d Deal
        err := rows.Scan(&d.ID, &d.CustomerID, &d.Title, &d.Value, &d.Stage, &d.Probability,
            &d.Responsible, &d.Source, &d.Comment, &d.ExpectedClose, &d.CreatedAt, &d.ClosedAt)
        if err != nil {
            continue
        }
        deals = append(deals, d)
    }

    c.JSON(http.StatusOK, gin.H{
        "data":        deals,
        "total":       total,
        "page":        page,
        "page_size":   pageSize,
        "total_pages": (total + pageSize - 1) / pageSize,
    })
}

// CreateDeal создаёт новую сделку
func CreateDeal(c *gin.Context) {
    var d Deal
    if err := c.ShouldBindJSON(&d); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO crm_deals (customer_id, title, value, stage, probability, responsible, source, comment, expected_close, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
        RETURNING id
    `, d.CustomerID, d.Title, d.Value, d.Stage, d.Probability,
        d.Responsible, d.Source, d.Comment, d.ExpectedClose).Scan(&d.ID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // NOTIFY: уведомление о создании сделки
    if notifier != nil {
        notifier.NotifyDealCreated(d.Title, d.Value, d.Stage, d.Responsible, d.CustomerID)
    }

    c.JSON(http.StatusCreated, d)
}

// UpdateDeal обновляет сделку
func UpdateDeal(c *gin.Context) {
    id := c.Param("id")
    var d Deal
    if err := c.ShouldBindJSON(&d); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE crm_deals
        SET title = $1, value = $2, stage = $3, probability = $4,
            responsible = $5, source = $6, comment = $7, expected_close = $8, updated_at = NOW()
        WHERE id = $9
    `, d.Title, d.Value, d.Stage, d.Probability,
        d.Responsible, d.Source, d.Comment, d.ExpectedClose, id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    // NOTIFY: уведомление об изменении сделки
    if notifier != nil {
        notifier.NotifyDealUpdated(id, d.Title, d.Value, d.Stage)
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// UpdateDealStage обновляет только стадию сделки
func UpdateDealStage(c *gin.Context) {
    id := c.Param("id")
    var req struct {
        Stage       string `json:"stage"`
        Probability int    `json:"probability"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE crm_deals
        SET stage = $1, probability = $2, updated_at = NOW()
        WHERE id = $3
    `, req.Stage, req.Probability, id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteDeal удаляет сделку
func DeleteDeal(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), "DELETE FROM crm_deals WHERE id = $1", id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== АНАЛИТИКА ==========

// GetCRMStats возвращает статистику для графиков CRM
func GetCRMStats(c *gin.Context) {
    ctx := c.Request.Context()

    // Количество сделок по стадиям
    rows, err := database.Pool.Query(ctx, `
        SELECT stage, COUNT(*) as count, COALESCE(SUM(value), 0) as total_value
        FROM crm_deals
        GROUP BY stage
        ORDER BY 
            CASE stage
                WHEN 'lead' THEN 1
                WHEN 'negotiation' THEN 2
                WHEN 'proposal' THEN 3
                WHEN 'closed_won' THEN 4
                WHEN 'closed_lost' THEN 5
                ELSE 6
            END
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    type stageStat struct {
        Stage      string  `json:"stage"`
        Count      int     `json:"count"`
        TotalValue float64 `json:"total_value"`
    }
    var stageStats []stageStat
    for rows.Next() {
        var s stageStat
        if err := rows.Scan(&s.Stage, &s.Count, &s.TotalValue); err != nil {
            continue
        }
        stageStats = append(stageStats, s)
    }

    // Динамика создания сделок по месяцам (последние 12 месяцев)
    rows, err = database.Pool.Query(ctx, `
        SELECT 
            TO_CHAR(date_trunc('month', created_at), 'YYYY-MM') as month,
            COUNT(*) as deals_created,
            COALESCE(SUM(value), 0) as total_value
        FROM crm_deals
        WHERE created_at >= NOW() - INTERVAL '12 months'
        GROUP BY date_trunc('month', created_at)
        ORDER BY month
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    type monthlyStat struct {
        Month        string  `json:"month"`
        DealsCreated int     `json:"deals_created"`
        TotalValue   float64 `json:"total_value"`
    }
    var monthlyStats []monthlyStat
    for rows.Next() {
        var m monthlyStat
        if err := rows.Scan(&m.Month, &m.DealsCreated, &m.TotalValue); err != nil {
            continue
        }
        monthlyStats = append(monthlyStats, m)
    }

    // Общая статистика
    var totalDeals, totalCustomers int
    var totalValue float64
    database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM crm_deals`).Scan(&totalDeals)
    database.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM crm_customers`).Scan(&totalCustomers)
    database.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(value), 0) FROM crm_deals`).Scan(&totalValue)

    c.JSON(http.StatusOK, gin.H{
        "stage_stats":     stageStats,
        "monthly_stats":   monthlyStats,
        "total_deals":     totalDeals,
        "total_customers": totalCustomers,
        "total_value":     totalValue,
    })
}

// ========== ВЛОЖЕНИЯ К СДЕЛКАМ ==========

const uploadDir = "./uploads/crm"

// UploadDealAttachment загружает файл и прикрепляет к сделке
// @Summary Загрузить файл для сделки
// @Description Загружает файл и прикрепляет его к указанной сделке
// @Tags CRM
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "ID сделки"
// @Param file formData file true "Файл для загрузки"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/crm/deals/{id}/attachments [post]
func UploadDealAttachment(c *gin.Context) {
    dealID := c.Param("id")
    if dealID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "deal_id required"})
        return
    }

    // Проверяем существование сделки
    var exists bool
    err := database.Pool.QueryRow(c.Request.Context(), "SELECT EXISTS(SELECT 1 FROM crm_deals WHERE id = $1)", dealID).Scan(&exists)
    if err != nil || !exists {
        c.JSON(http.StatusNotFound, gin.H{"error": "Deal not found"})
        return
    }

    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
        return
    }

    // Создаём директорию для загрузок, если её нет
    if err := os.MkdirAll(uploadDir, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot create upload directory"})
        return
    }

    // Генерируем уникальное имя файла
    ext := filepath.Ext(file.Filename)
    newFileName := uuid.New().String() + ext
    filePath := filepath.Join(uploadDir, newFileName)

    // Сохраняем файл
    if err := c.SaveUploadedFile(file, filePath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
        return
    }

    // Получаем ID пользователя из контекста (если есть)
    userID, _ := c.Get("userID")
    userIDStr, _ := userID.(string)

    // Вставляем запись в БД
    var attachmentID string
    err = database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO deal_attachments (deal_id, file_name, file_path, file_size, mime_type, uploaded_by)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `, dealID, file.Filename, filePath, file.Size, file.Header.Get("Content-Type"), userIDStr).Scan(&attachmentID)

    if err != nil {
        // Если не удалось записать в БД, удаляем загруженный файл
        os.Remove(filePath)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "id":          attachmentID,
        "file_name":   file.Filename,
        "file_size":   file.Size,
        "mime_type":   file.Header.Get("Content-Type"),
        "uploaded_at": time.Now(),
    })
}

// GetDealAttachments возвращает список вложений для сделки
// @Summary Список вложений сделки
// @Description Возвращает все файлы, прикреплённые к сделке
// @Tags CRM
// @Produce json
// @Param id path string true "ID сделки"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/crm/deals/{id}/attachments [get]
func GetDealAttachments(c *gin.Context) {
    dealID := c.Param("id")
    if dealID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "deal_id required"})
        return
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, file_name, file_path, file_size, mime_type, uploaded_by, uploaded_at
        FROM deal_attachments
        WHERE deal_id = $1
        ORDER BY uploaded_at DESC
    `, dealID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    type Attachment struct {
        ID         string    `json:"id"`
        FileName   string    `json:"file_name"`
        FilePath   string    `json:"file_path"`
        FileSize   int64     `json:"file_size"`
        MimeType   string    `json:"mime_type"`
        UploadedBy *string   `json:"uploaded_by,omitempty"`
        UploadedAt time.Time `json:"uploaded_at"`
    }

    var attachments []Attachment
    for rows.Next() {
        var a Attachment
        var uploadedBy sql.NullString
        err := rows.Scan(&a.ID, &a.FileName, &a.FilePath, &a.FileSize, &a.MimeType, &uploadedBy, &a.UploadedAt)
        if err != nil {
            continue
        }
        if uploadedBy.Valid {
            a.UploadedBy = &uploadedBy.String
        }
        attachments = append(attachments, a)
    }

    c.JSON(http.StatusOK, attachments)
}

// DownloadDealAttachment скачивает файл
// @Summary Скачать файл
// @Description Скачивает файл по его ID
// @Tags CRM
// @Produce application/octet-stream
// @Param attachment_id path string true "ID вложения"
// @Success 200 {file} binary
// @Failure 404 {object} map[string]interface{}
// @Router /api/crm/attachments/{attachment_id}/download [get]
func DownloadDealAttachment(c *gin.Context) {
    attachmentID := c.Param("attachment_id")
    if attachmentID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "attachment_id required"})
        return
    }

    var filePath, fileName string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT file_path, file_name FROM deal_attachments WHERE id = $1
    `, attachmentID).Scan(&filePath, &fileName)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
        return
    }

    c.FileAttachment(filePath, fileName)
}

// DeleteDealAttachment удаляет вложение
// @Summary Удалить файл
// @Description Удаляет файл и запись о нём
// @Tags CRM
// @Produce json
// @Param attachment_id path string true "ID вложения"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/crm/attachments/{attachment_id} [delete]
func DeleteDealAttachment(c *gin.Context) {
    attachmentID := c.Param("attachment_id")
    if attachmentID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "attachment_id required"})
        return
    }

    var filePath string
    err := database.Pool.QueryRow(c.Request.Context(), `
        DELETE FROM deal_attachments WHERE id = $1
        RETURNING file_path
    `, attachmentID).Scan(&filePath)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Attachment not found"})
        return
    }

    // Удаляем файл с диска
    os.Remove(filePath)

    c.JSON(http.StatusOK, gin.H{"success": true})
}

// ========== РАСШИРЕННАЯ АНАЛИТИКА ==========

// GetCRMAdvancedStats возвращает расширенную статистику с возможностью фильтрации по дате
// @Summary Расширенная аналитика CRM
// @Description Возвращает статистику по ответственным, источникам и динамику за период
// @Tags CRM
// @Produce json
// @Param date_from query string false "Начальная дата (YYYY-MM-DD)"
// @Param date_to query string false "Конечная дата (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/crm/advanced-stats [get]
func GetCRMAdvancedStats(c *gin.Context) {
    dateFrom := c.Query("date_from")
    dateTo := c.Query("date_to")
    ctx := c.Request.Context()

    // Базовое условие для фильтрации по дате создания сделки
    dateFilter := ""
    args := []interface{}{}
    if dateFrom != "" {
        dateFilter += " AND created_at >= $" + strconv.Itoa(len(args)+1) + "::date"
        args = append(args, dateFrom)
    }
    if dateTo != "" {
        dateFilter += " AND created_at < ($" + strconv.Itoa(len(args)+1) + "::date + '1 day'::interval)"
        args = append(args, dateTo)
    }

    // 1. Статистика по ответственным
    responsibleQuery := `
        SELECT 
            COALESCE(responsible, 'Не назначен') as responsible,
            COUNT(*) as deals_count,
            COALESCE(SUM(value), 0) as total_value
        FROM crm_deals
        WHERE 1=1 ` + dateFilter + `
        GROUP BY responsible
        ORDER BY total_value DESC
    `
    rows, err := database.Pool.Query(ctx, responsibleQuery, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    responsibleStats := []gin.H{}
    for rows.Next() {
        var responsible string
        var dealsCount int
        var totalValue float64
        if err := rows.Scan(&responsible, &dealsCount, &totalValue); err != nil {
            continue
        }
        responsibleStats = append(responsibleStats, gin.H{
            "responsible": responsible,
            "deals_count": dealsCount,
            "total_value": totalValue,
        })
    }

    // 2. Статистика по источникам
    sourceQuery := `
        SELECT 
            COALESCE(source, 'Не указан') as source,
            COUNT(*) as deals_count,
            COALESCE(SUM(value), 0) as total_value
        FROM crm_deals
        WHERE 1=1 ` + dateFilter + `
        GROUP BY source
        ORDER BY total_value DESC
    `
    rows, err = database.Pool.Query(ctx, sourceQuery, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    sourceStats := []gin.H{}
    for rows.Next() {
        var source string
        var dealsCount int
        var totalValue float64
        if err := rows.Scan(&source, &dealsCount, &totalValue); err != nil {
            continue
        }
        sourceStats = append(sourceStats, gin.H{
            "source":      source,
            "deals_count": dealsCount,
            "total_value": totalValue,
        })
    }

    // 3. Ежемесячная динамика с фильтром
    monthlyQuery := `
        SELECT 
            TO_CHAR(date_trunc('month', created_at), 'YYYY-MM') as month,
            COUNT(*) as deals_created,
            COALESCE(SUM(value), 0) as total_value
        FROM crm_deals
        WHERE 1=1 ` + dateFilter + `
        GROUP BY date_trunc('month', created_at)
        ORDER BY month
    `
    rows, err = database.Pool.Query(ctx, monthlyQuery, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
        return
    }
    defer rows.Close()

    monthlyStats := []gin.H{}
    for rows.Next() {
        var month string
        var dealsCount int
        var totalValue float64
        if err := rows.Scan(&month, &dealsCount, &totalValue); err != nil {
            continue
        }
        monthlyStats = append(monthlyStats, gin.H{
            "month":       month,
            "deals_count": dealsCount,
            "total_value": totalValue,
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "responsible_stats": responsibleStats,
        "source_stats":      sourceStats,
        "monthly_stats":     monthlyStats,
    })
}