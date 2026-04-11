package handlers

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// WhatsAppConfig - конфигурация
type WhatsAppConfig struct {
    AccessToken     string
    PhoneNumberID   string
    BusinessID      string
    VerifyToken     string
}

// ConnectWhatsApp - подключение WhatsApp Business
func ConnectWhatsApp(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        PhoneNumberID   string `json:"phone_number_id" binding:"required"`
        BusinessID      string `json:"business_account_id"`
        AccessToken     string `json:"access_token" binding:"required"`
        VerifyToken     string `json:"verify_token"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    integrationID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_integrations (id, company_id, phone_number_id, business_account_id, access_token, webhook_verify_token, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
    `, integrationID, companyID, req.PhoneNumberID, req.BusinessID, req.AccessToken, req.VerifyToken)

    if err != nil {
        log.Printf("❌ Ошибка подключения WhatsApp: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect WhatsApp"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "WhatsApp Business подключён",
        "integration_id": integrationID,
    })
}

// SendWhatsAppMessage - отправка сообщения
func SendWhatsAppMessage(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        To      string `json:"to" binding:"required"`
        Message string `json:"message" binding:"required"`
        Type    string `json:"type"` // text, template
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Получаем настройки интеграции
    var phoneNumberID, accessToken string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT phone_number_id, access_token FROM whatsapp_integrations
        WHERE company_id = $1 AND is_active = true
    `, companyID).Scan(&phoneNumberID, &accessToken)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "WhatsApp not connected"})
        return
    }

    // Отправка через WhatsApp Cloud API
    url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)
    
    payload := map[string]interface{}{
        "messaging_product": "whatsapp",
        "to": req.To,
        "type": "text",
        "text": map[string]string{"body": req.Message},
    }
    
    payloadBytes, _ := json.Marshal(payload)
    
    httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
    httpReq.Header.Set("Authorization", "Bearer "+accessToken)
    httpReq.Header.Set("Content-Type", "application/json")
    
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(httpReq)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
        return
    }
    defer resp.Body.Close()
    
    body, _ := io.ReadAll(resp.Body)
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Сообщение отправлено",
        "response": string(body),
    })
}

// CreateWhatsAppTemplate - создание шаблона
func CreateWhatsAppTemplate(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Name        string `json:"name" binding:"required"`
        Category    string `json:"category"`
        Content     string `json:"content" binding:"required"`
        Variables   []string `json:"variables"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    variablesJSON, _ := json.Marshal(req.Variables)
    
    templateID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_templates (id, company_id, name, category, content, variables, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
    `, templateID, companyID, req.Name, req.Category, req.Content, variablesJSON)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Шаблон создан",
        "template_id": templateID,
    })
}

// GetWhatsAppTemplates - список шаблонов
func GetWhatsAppTemplates(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, category, content, is_active, created_at
        FROM whatsapp_templates
        WHERE company_id = $1
        ORDER BY created_at DESC
    `, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load templates"})
        return
    }
    defer rows.Close()

    var templates []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, category, content string
        var isActive bool
        var createdAt time.Time

        rows.Scan(&id, &name, &category, &content, &isActive, &createdAt)

        templates = append(templates, gin.H{
            "id": id,
            "name": name,
            "category": category,
            "content": content,
            "is_active": isActive,
            "created_at": createdAt.Format("2006-01-02"),
        })
    }

    c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// CreateWhatsAppBroadcast - создание рассылки
func CreateWhatsAppBroadcast(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        Name        string   `json:"name" binding:"required"`
        TemplateID  string   `json:"template_id"`
        Message     string   `json:"message"`
        Recipients  []string `json:"recipients" binding:"required"`
        ScheduledAt *string  `json:"scheduled_at"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    recipientsJSON, _ := json.Marshal(req.Recipients)
    
    broadcastID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO whatsapp_broadcasts (id, company_id, template_id, name, recipients, status, scheduled_at, created_at)
        VALUES ($1, $2, $3, $4, $5, 'pending', $6, NOW())
    `, broadcastID, companyID, req.TemplateID, req.Name, recipientsJSON, req.ScheduledAt)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create broadcast"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "Рассылка создана",
        "broadcast_id": broadcastID,
    })
}

// SendWhatsAppBroadcast - отправка рассылки
func SendWhatsAppBroadcast(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    broadcastID := c.Param("id")

    // Получаем данные рассылки
    var templateID *string
    var message string
    var recipientsJSON []byte
    var name string
    
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT template_id, message, recipients, name FROM whatsapp_broadcasts
        WHERE id = $1 AND company_id = $2
    `, broadcastID, companyID).Scan(&templateID, &message, &recipientsJSON, &name)

    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Broadcast not found"})
        return
    }

    var recipients []string
    json.Unmarshal(recipientsJSON, &recipients)

    // Получаем настройки WhatsApp
    var phoneNumberID, accessToken string
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT phone_number_id, access_token FROM whatsapp_integrations
        WHERE company_id = $1 AND is_active = true
    `, companyID).Scan(&phoneNumberID, &accessToken)

    // Отправка сообщений
    sentCount := 0
    for _, to := range recipients {
        url := fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneNumberID)
        
        var payload []byte
        if templateID != nil && *templateID != "" {
            // Используем шаблон
            payload, _ = json.Marshal(map[string]interface{}{
                "messaging_product": "whatsapp",
                "to": to,
                "type": "template",
                "template": map[string]interface{}{
                    "name": *templateID,
                    "language": map[string]string{"code": "ru"},
                },
            })
        } else {
            // Обычный текст
            payload, _ = json.Marshal(map[string]interface{}{
                "messaging_product": "whatsapp",
                "to": to,
                "type": "text",
                "text": map[string]string{"body": message},
            })
        }
        
        httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
        httpReq.Header.Set("Authorization", "Bearer "+accessToken)
        httpReq.Header.Set("Content-Type", "application/json")
        
        client := &http.Client{Timeout: 30 * time.Second}
        resp, err := client.Do(httpReq)
        if err == nil && resp.StatusCode == 200 {
            sentCount++
        }
        if resp != nil {
            resp.Body.Close()
        }
    }

    // Обновляем статус рассылки
    database.Pool.Exec(c.Request.Context(), `
        UPDATE whatsapp_broadcasts 
        SET status = 'sent', sent_count = $1, sent_at = NOW()
        WHERE id = $2
    `, sentCount, broadcastID)

    c.JSON(http.StatusOK, gin.H{
        "message": fmt.Sprintf("Рассылка '%s' отправлена", name),
        "sent": sentCount,
        "total": len(recipients),
    })
}

// WhatsAppWebhook - вебхук для входящих сообщений
func WhatsAppWebhook(c *gin.Context) {
    if c.Request.Method == "GET" {
        // Подтверждение вебхука
        mode := c.Query("hub.mode")
        token := c.Query("hub.verify_token")
        challenge := c.Query("hub.challenge")
        
        if mode == "subscribe" && token == "your_verify_token" {
            c.String(http.StatusOK, challenge)
            return
        }
        c.String(http.StatusForbidden, "Forbidden")
        return
    }

    // Обработка входящих сообщений
    var body map[string]interface{}
    c.ShouldBindJSON(&body)
    
    // Сохраняем входящее сообщение в БД
    log.Printf("📨 Входящее WhatsApp сообщение: %v", body)
    
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}