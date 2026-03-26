package handlers

import (
    "bytes"
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// BitrixSettings структура настроек
type BitrixSettings struct {
    ID          uuid.UUID       `json:"id"`
    WebhookURL  string          `json:"webhook_url"`
    Domain      string          `json:"domain"`
    MemberID    string          `json:"member_id"`
    AccessToken string          `json:"access_token"`
    LastSync    *time.Time      `json:"last_sync"`
    SyncStatus  string          `json:"sync_status"`
    Settings    json.RawMessage `json:"settings"`
}

// GetBitrixSettings - получить настройки Bitrix
func GetBitrixSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var settings BitrixSettings
    var lastSync sql.NullTime
    var settingsJSON []byte
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, webhook_url, domain, member_id, access_token, 
               last_sync, sync_status, settings
        FROM bitrix_settings
        WHERE user_id = $1
    `, userID).Scan(
        &settings.ID, &settings.WebhookURL, &settings.Domain, &settings.MemberID,
        &settings.AccessToken, &lastSync, &settings.SyncStatus, &settingsJSON,
    )
    
    if err != nil {
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "settings": map[string]interface{}{
                "webhook_url":   "",
                "domain":        "",
                "sync_status":   "idle",
                "auto_sync":     false,
                "sync_interval": 3600,
            },
        })
        return
    }
    
    if lastSync.Valid {
        settings.LastSync = &lastSync.Time
    }
    
    var settingsMap map[string]interface{}
    if len(settingsJSON) > 0 {
        json.Unmarshal(settingsJSON, &settingsMap)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "settings": settings,
        "extra":    settingsMap,
    })
}

// SaveBitrixSettings - сохранить настройки Bitrix
func SaveBitrixSettings(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        WebhookURL string          `json:"webhook_url"`
        Domain     string          `json:"domain"`
        MemberID   string          `json:"member_id"`
        Settings   json.RawMessage `json:"settings"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO bitrix_settings (user_id, webhook_url, domain, member_id, settings, sync_status, updated_at)
        VALUES ($1, $2, $3, $4, $5, 'idle', NOW())
        ON CONFLICT (user_id) DO UPDATE SET
            webhook_url = EXCLUDED.webhook_url,
            domain = EXCLUDED.domain,
            member_id = EXCLUDED.member_id,
            settings = EXCLUDED.settings,
            updated_at = NOW()
    `, userID, req.WebhookURL, req.Domain, req.MemberID, req.Settings)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить настройки"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Настройки сохранены",
    })
}

// ExportLeadToBitrix - экспорт лида в Bitrix24
func ExportLeadToBitrix(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Title       string  `json:"title" binding:"required"`
        Name        string  `json:"name"`
        Phone       string  `json:"phone"`
        Email       string  `json:"email"`
        Description string  `json:"description"`
        Price       float64 `json:"price"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    leadData := map[string]interface{}{
        "fields": map[string]interface{}{
            "TITLE":       req.Title,
            "NAME":        req.Name,
            "COMMENTS":    req.Description,
            "OPPORTUNITY": req.Price,
            "CURRENCY_ID": "RUB",
        },
    }
    
    if req.Phone != "" {
        leadData["fields"].(map[string]interface{})["PHONE"] = []map[string]string{
            {"VALUE": req.Phone, "VALUE_TYPE": "WORK"},
        }
    }
    
    if req.Email != "" {
        leadData["fields"].(map[string]interface{})["EMAIL"] = []map[string]string{
            {"VALUE": req.Email, "VALUE_TYPE": "WORK"},
        }
    }
    
    jsonData, _ := json.Marshal(leadData)
    resp, err := http.Post(webhookURL+"/crm.lead.add", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка отправки в Bitrix"})
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    logID := uuid.New()
    bitrixID := ""
    if id, ok := result["result"]; ok {
        bitrixID = fmt.Sprintf("%v", id)
    }
    
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, bitrix_id, action, status, response, created_at)
        VALUES ($1, $2, 'export', 'lead', $3, 'create', 'completed', $4, NOW())
    `, logID, userID, bitrixID, string(jsonData))
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "bitrix_id": bitrixID,
        "message":   "Лид экспортирован в Bitrix24",
    })
}

// ImportLeadsFromBitrix - импорт лидов из Bitrix24
func ImportLeadsFromBitrix(c *gin.Context) {
    userID := getUserID(c)
    
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    resp, err := http.Get(webhookURL + "/crm.lead.list?select[]=ID&select[]=TITLE&select[]=NAME&select[]=PHONE&select[]=EMAIL&select[]=COMMENTS")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения лидов из Bitrix"})
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    leads, ok := result["result"].([]interface{})
    if !ok {
        c.JSON(http.StatusOK, gin.H{
            "success":  true,
            "imported": 0,
            "message":  "Нет новых лидов",
        })
        return
    }
    
    imported := 0
    for _, lead := range leads {
        leadMap, ok := lead.(map[string]interface{})
        if !ok {
            continue
        }
        
        name := ""
        if n, ok := leadMap["NAME"]; ok {
            name = fmt.Sprintf("%v", n)
        }
        
        email := ""
        if e, ok := leadMap["EMAIL"]; ok {
            email = fmt.Sprintf("%v", e)
        }
        
        phone := ""
        if p, ok := leadMap["PHONE"]; ok {
            phone = fmt.Sprintf("%v", p)
        }
        
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO crm_customers (user_id, name, email, phone, lead_score, created_at)
            VALUES ($1, $2, $3, $4, 50, NOW())
            ON CONFLICT (email) DO NOTHING
        `, userID, name, email, phone)
        
        if err == nil {
            imported++
        }
        
        logID := uuid.New()
        bitrixID := fmt.Sprintf("%v", leadMap["ID"])
        database.Pool.Exec(c.Request.Context(), `
            INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, bitrix_id, action, status, created_at)
            VALUES ($1, $2, 'import', 'lead', $3, 'create', 'completed', NOW())
        `, logID, userID, bitrixID)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "imported": imported,
        "message":  fmt.Sprintf("Импортировано %d лидов", imported),
    })
}

// GetBitrixSyncLogs - получить логи синхронизации
func GetBitrixSyncLogs(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, direction, entity_type, bitrix_id, action, status, error_message, created_at, synced_at
        FROM bitrix_sync_logs
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 50
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var logs []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var direction, entityType, bitrixID, action, status, errorMsg string
        var createdAt, syncedAt time.Time
        var syncedAtPtr *time.Time
        
        rows.Scan(&id, &direction, &entityType, &bitrixID, &action, &status, &errorMsg, &createdAt, &syncedAt)
        
        if !syncedAt.IsZero() {
            syncedAtPtr = &syncedAt
        }
        
        logs = append(logs, map[string]interface{}{
            "id":          id,
            "direction":   direction,
            "entity_type": entityType,
            "bitrix_id":   bitrixID,
            "action":      action,
            "status":      status,
            "error":       errorMsg,
            "created_at":  createdAt,
            "synced_at":   syncedAtPtr,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "logs":    logs,
    })
}

// SyncBitrixContacts - синхронизация контактов
func SyncBitrixContacts(c *gin.Context) {
    userID := getUserID(c)
    
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    _, err = database.Pool.Exec(c.Request.Context(), `
        ALTER TABLE crm_customers ADD COLUMN IF NOT EXISTS bitrix_synced BOOLEAN DEFAULT false;
        ALTER TABLE crm_customers ADD COLUMN IF NOT EXISTS bitrix_synced_at TIMESTAMP;
        ALTER TABLE crm_customers ADD COLUMN IF NOT EXISTS bitrix_id VARCHAR(100);
    `)
    if err != nil {
        // Игнорируем
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, email, phone, company
        FROM crm_customers
        WHERE user_id = $1 AND (bitrix_synced = false OR bitrix_synced IS NULL)
        LIMIT 100
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var synced int
    for rows.Next() {
        var id uuid.UUID
        var name, email, phone, company string
        
        rows.Scan(&id, &name, &email, &phone, &company)
        
        contactData := map[string]interface{}{
            "fields": map[string]interface{}{
                "NAME":         name,
                "COMPANY_TITLE": company,
            },
        }
        
        if email != "" {
            contactData["fields"].(map[string]interface{})["EMAIL"] = []map[string]string{
                {"VALUE": email, "VALUE_TYPE": "WORK"},
            }
        }
        
        if phone != "" {
            contactData["fields"].(map[string]interface{})["PHONE"] = []map[string]string{
                {"VALUE": phone, "VALUE_TYPE": "WORK"},
            }
        }
        
        jsonData, _ := json.Marshal(contactData)
        resp, err := http.Post(webhookURL+"/crm.contact.add", "application/json", bytes.NewBuffer(jsonData))
        if err == nil {
            resp.Body.Close()
            synced++
            
            database.Pool.Exec(c.Request.Context(), `
                UPDATE crm_customers SET bitrix_synced = true, bitrix_synced_at = NOW()
                WHERE id = $1
            `, id)
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "synced":  synced,
        "message": fmt.Sprintf("Синхронизировано %d контактов", synced),
    })
}

// ========== АВТОМАТИЧЕСКАЯ СИНХРОНИЗАЦИЯ ==========

// BitrixSyncScheduler - планировщик синхронизации с Bitrix24
type BitrixSyncScheduler struct {
    ticker *time.Ticker
    stop   chan bool
}

var bitrixSyncScheduler *BitrixSyncScheduler

// StartBitrixSyncScheduler - запуск планировщика
func StartBitrixSyncScheduler() {
    if bitrixSyncScheduler != nil {
        return
    }
    
    bitrixSyncScheduler = &BitrixSyncScheduler{
        stop: make(chan bool),
    }
    
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                checkAndRunBitrixSync()
            case <-bitrixSyncScheduler.stop:
                return
            }
        }
    }()
    
    log.Println("🤖 Планировщик синхронизации с Bitrix24 запущен")
}

// StopBitrixSyncScheduler - остановка планировщика
func StopBitrixSyncScheduler() {
    if bitrixSyncScheduler != nil {
        bitrixSyncScheduler.stop <- true
        log.Println("🛑 Планировщик синхронизации с Bitrix24 остановлен")
    }
}

// checkAndRunBitrixSync - проверка настроек и запуск синхронизации
func checkAndRunBitrixSync() {
    ctx := context.Background()
    
    rows, err := database.Pool.Query(ctx, `
        SELECT user_id, settings
        FROM bitrix_settings
        WHERE settings->>'auto_sync' = 'true'
    `)
    if err != nil {
        log.Printf("⚠️ Ошибка проверки настроек синхронизации Bitrix: %v", err)
        return
    }
    defer rows.Close()
    
    for rows.Next() {
        var userID uuid.UUID
        var settingsJSON []byte
        rows.Scan(&userID, &settingsJSON)
        
        var settings map[string]interface{}
        json.Unmarshal(settingsJSON, &settings)
        
        interval := 3600
        if val, ok := settings["sync_interval"]; ok {
            if v, ok := val.(float64); ok {
                interval = int(v)
            }
        }
        
        var lastSync time.Time
        database.Pool.QueryRow(ctx, `
            SELECT COALESCE(MAX(created_at), '1970-01-01')
            FROM bitrix_sync_logs
            WHERE user_id = $1 AND direction = 'export' AND status = 'completed'
        `, userID).Scan(&lastSync)
        
        if time.Since(lastSync) > time.Duration(interval)*time.Second {
            go func(uid uuid.UUID) {
                log.Printf("🔄 Запуск автоматической синхронизации Bitrix для пользователя %s", uid)
                syncLeadsToBitrix(uid)
                importLeadsFromBitrix(uid)
            }(userID)
        }
    }
}

// syncLeadsToBitrix - синхронизация лидов в Bitrix24
func syncLeadsToBitrix(userID uuid.UUID) {
    ctx := context.Background()
    
    var webhookURL string
    err := database.Pool.QueryRow(ctx, `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        return
    }
    
    rows, err := database.Pool.Query(ctx, `
        SELECT id, name, email, phone, company, lead_score
        FROM crm_customers
        WHERE user_id = $1 AND (bitrix_synced = false OR bitrix_synced IS NULL)
        LIMIT 50
    `, userID)
    
    if err != nil {
        log.Printf("Ошибка получения лидов для синхронизации: %v", err)
        return
    }
    defer rows.Close()
    
    var leads []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var name, email, phone, company string
        var leadScore float64
        
        rows.Scan(&id, &name, &email, &phone, &company, &leadScore)
        
        leads = append(leads, map[string]interface{}{
            "id":         id.String(),
            "name":       name,
            "email":      email,
            "phone":      phone,
            "company":    company,
            "lead_score": leadScore,
        })
    }
    
    if len(leads) == 0 {
        return
    }
    
    synced := 0
    for _, lead := range leads {
        leadData := map[string]interface{}{
            "fields": map[string]interface{}{
                "TITLE":         lead["name"],
                "NAME":          lead["name"],
                "COMPANY_TITLE": lead["company"],
            },
        }
        
        if lead["email"] != "" {
            leadData["fields"].(map[string]interface{})["EMAIL"] = []map[string]string{
                {"VALUE": lead["email"].(string), "VALUE_TYPE": "WORK"},
            }
        }
        
        if lead["phone"] != "" {
            leadData["fields"].(map[string]interface{})["PHONE"] = []map[string]string{
                {"VALUE": lead["phone"].(string), "VALUE_TYPE": "WORK"},
            }
        }
        
        jsonData, _ := json.Marshal(leadData)
        resp, err := http.Post(webhookURL+"/crm.lead.add", "application/json", bytes.NewBuffer(jsonData))
        
        if err == nil {
            resp.Body.Close()
            synced++
            
            id, _ := uuid.Parse(lead["id"].(string))
            database.Pool.Exec(ctx, `
                UPDATE crm_customers SET bitrix_synced = true, bitrix_synced_at = NOW()
                WHERE id = $1
            `, id)
        }
    }
    
    if synced > 0 {
        logID := uuid.New()
        database.Pool.Exec(ctx, `
            INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, record_count, status, created_at)
            VALUES ($1, $2, 'export', 'leads', $3, 'completed', NOW())
        `, logID, userID, synced)
        
        log.Printf("✅ Синхронизировано %d лидов в Bitrix24 для пользователя %s", synced, userID)
    }
}

// importLeadsFromBitrix - импорт лидов из Bitrix24
func importLeadsFromBitrix(userID uuid.UUID) {
    ctx := context.Background()
    
    var webhookURL string
    err := database.Pool.QueryRow(ctx, `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        return
    }
    
    resp, err := http.Get(webhookURL + "/crm.lead.list?select[]=ID&select[]=TITLE&select[]=NAME&select[]=PHONE&select[]=EMAIL&select[]=COMPANY_TITLE")
    if err != nil {
        log.Printf("Ошибка получения лидов из Bitrix: %v", err)
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    leads, ok := result["result"].([]interface{})
    if !ok {
        return
    }
    
    imported := 0
    for _, lead := range leads {
        leadMap, ok := lead.(map[string]interface{})
        if !ok {
            continue
        }
        
        name := ""
        if n, ok := leadMap["NAME"]; ok {
            name = fmt.Sprintf("%v", n)
        }
        if name == "" {
            if t, ok := leadMap["TITLE"]; ok {
                name = fmt.Sprintf("%v", t)
            }
        }
        
        email := ""
        if e, ok := leadMap["EMAIL"]; ok {
            if emailArr, ok := e.([]interface{}); ok && len(emailArr) > 0 {
                if emailMap, ok := emailArr[0].(map[string]interface{}); ok {
                    email = fmt.Sprintf("%v", emailMap["VALUE"])
                }
            }
        }
        
        phone := ""
        if p, ok := leadMap["PHONE"]; ok {
            if phoneArr, ok := p.([]interface{}); ok && len(phoneArr) > 0 {
                if phoneMap, ok := phoneArr[0].(map[string]interface{}); ok {
                    phone = fmt.Sprintf("%v", phoneMap["VALUE"])
                }
            }
        }
        
        company := ""
        if c, ok := leadMap["COMPANY_TITLE"]; ok {
            company = fmt.Sprintf("%v", c)
        }
        
        _, err := database.Pool.Exec(ctx, `
            INSERT INTO crm_customers (user_id, name, email, phone, company, lead_score, created_at)
            VALUES ($1, $2, $3, $4, $5, 50, NOW())
            ON CONFLICT (email) DO NOTHING
        `, userID, name, email, phone, company)
        
        if err == nil {
            imported++
        }
    }
    
    if imported > 0 {
        logID := uuid.New()
        database.Pool.Exec(ctx, `
            INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, record_count, status, created_at)
            VALUES ($1, $2, 'import', 'leads', $3, 'completed', NOW())
        `, logID, userID, imported)
        
        log.Printf("✅ Импортировано %d лидов из Bitrix24 для пользователя %s", imported, userID)
    }
}

// SyncTasksToBitrix - синхронизация задач в Bitrix24
func SyncTasksToBitrix(c *gin.Context) {
    userID := getUserID(c)
    
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    var req struct {
        Title       string `json:"title" binding:"required"`
        Description string `json:"description"`
        Deadline    string `json:"deadline"`
        Responsible string `json:"responsible"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    taskData := map[string]interface{}{
        "fields": map[string]interface{}{
            "TITLE":         req.Title,
            "DESCRIPTION":   req.Description,
            "CREATED_BY":    1,
            "RESPONSIBLE_ID": req.Responsible,
        },
    }
    
    if req.Deadline != "" {
        taskData["fields"].(map[string]interface{})["DEADLINE"] = req.Deadline
    }
    
    jsonData, _ := json.Marshal(taskData)
    resp, err := http.Post(webhookURL+"/tasks.task.add", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка отправки в Bitrix"})
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    logID := uuid.New()
    taskID := ""
    if id, ok := result["result"]; ok {
        if taskMap, ok := id.(map[string]interface{}); ok {
            if t, ok := taskMap["task"]; ok {
                if task, ok := t.(map[string]interface{}); ok {
                    if tid, ok := task["id"]; ok {
                        taskID = fmt.Sprintf("%v", tid)
                    }
                }
            }
        }
    }
    
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO bitrix_sync_logs (id, user_id, direction, entity_type, bitrix_id, action, status, created_at)
        VALUES ($1, $2, 'export', 'task', $3, 'create', 'completed', NOW())
    `, logID, userID, taskID)
    
    c.JSON(http.StatusOK, gin.H{
        "success":   true,
        "task_id":   taskID,
        "message":   "Задача создана в Bitrix24",
    })
}

// GetBitrixTasks - получить задачи из Bitrix24
func GetBitrixTasks(c *gin.Context) {
    userID := getUserID(c)
    
    var webhookURL string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT webhook_url FROM bitrix_settings WHERE user_id = $1
    `, userID).Scan(&webhookURL)
    
    if err != nil || webhookURL == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrix24 не настроен"})
        return
    }
    
    resp, err := http.Get(webhookURL + "/tasks.task.list?select[]=ID&select[]=TITLE&select[]=STATUS&select[]=DEADLINE&select[]=CREATED_DATE")
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения задач"})
        return
    }
    defer resp.Body.Close()
    
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    
    tasks, ok := result["result"].(map[string]interface{})
    if !ok {
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "tasks":   []interface{}{},
        })
        return
    }
    
    tasksList, ok := tasks["tasks"].([]interface{})
    if !ok {
        c.JSON(http.StatusOK, gin.H{
            "success": true,
            "tasks":   []interface{}{},
        })
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "tasks":   tasksList,
    })
}

// BitrixWebhookHandler - обработчик webhook от Bitrix24
func BitrixWebhookHandler(c *gin.Context) {
    var req struct {
        Event   string                 `json:"event"`
        Data    map[string]interface{} `json:"data"`
        Auth    map[string]interface{} `json:"auth"`
        Ts      int64                  `json:"ts"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    log.Printf("📥 Webhook от Bitrix24:")
    log.Printf("   Event: %s", req.Event)
    log.Printf("   Data: %v", req.Data)
    log.Printf("   Timestamp: %d", req.Ts)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Webhook принят",
    })
}