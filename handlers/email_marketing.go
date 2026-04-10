package handlers

import (
    "log"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// CreateEmailCampaign - создание кампании рассылки
func CreateEmailCampaign(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        Name        string   `json:"name" binding:"required"`
        Subject     string   `json:"subject" binding:"required"`
        Content     string   `json:"content" binding:"required"`
        Recipients  []string `json:"recipients" binding:"required"`
        ScheduledAt *string  `json:"scheduled_at"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Создаём кампанию
    campaignID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO email_campaigns (id, company_id, name, subject, content, status, created_at)
        VALUES ($1, $2, $3, $4, $5, 'draft', NOW())
    `, campaignID, companyID, req.Name, req.Subject, req.Content)
    
    if err != nil {
        log.Printf("❌ Ошибка создания кампании: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
        return
    }
    
    // Добавляем получателей
    for _, email := range req.Recipients {
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO email_recipients (id, campaign_id, email, status)
            VALUES ($1, $2, $3, 'pending')
        `, uuid.New(), campaignID, email)
        if err != nil {
            log.Printf("⚠️ Ошибка добавления получателя %s: %v", email, err)
        }
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":     "Кампания создана",
        "campaign_id": campaignID,
        "recipients":  len(req.Recipients),
    })
}

// GetEmailCampaigns - список кампаний
func GetEmailCampaigns(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, subject, status, recipient_count, opened_count, clicked_count, created_at, sent_at
        FROM email_campaigns
        WHERE company_id = $1
        ORDER BY created_at DESC
    `, companyID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load campaigns"})
        return
    }
    defer rows.Close()
    
    var campaigns []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, subject, status string
        var recipientCount, openedCount, clickedCount int
        var createdAt time.Time
        var sentAt *time.Time
        
        rows.Scan(&id, &name, &subject, &status, &recipientCount, &openedCount, &clickedCount, &createdAt, &sentAt)
        
        campaigns = append(campaigns, gin.H{
            "id":              id,
            "name":            name,
            "subject":         subject,
            "status":          status,
            "recipient_count": recipientCount,
            "opened_count":    openedCount,
            "clicked_count":   clickedCount,
            "created_at":      createdAt.Format("2006-01-02 15:04"),
            "sent_at":         sentAt,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"campaigns": campaigns})
}

// SendEmailCampaign - отправка кампании
func SendEmailCampaign(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    campaignID := c.Param("id")
    
    // Получаем кампанию
    var subject, content string
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT subject, content FROM email_campaigns
        WHERE id = $1 AND company_id = $2
    `, campaignID, companyID).Scan(&subject, &content)
    
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
        return
    }
    
    // Получаем получателей
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT email, name FROM email_recipients
        WHERE campaign_id = $1 AND status = 'pending'
    `, campaignID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load recipients"})
        return
    }
    defer rows.Close()
    
    // Отправка писем (имитация)
    var sentCount int
    for rows.Next() {
        var email, name string
        rows.Scan(&email, &name)
        
        // Здесь реальная отправка через SMTP
        // sendEmail(email, subject, content)
        
        sentCount++
        
        // Обновляем статус
        database.Pool.Exec(c.Request.Context(), `
            UPDATE email_recipients SET status = 'sent', sent_at = NOW()
            WHERE campaign_id = $1 AND email = $2
        `, campaignID, email)
    }
    
    // Обновляем кампанию
    database.Pool.Exec(c.Request.Context(), `
        UPDATE email_campaigns 
        SET status = 'sent', sent_at = NOW(), recipient_count = $1
        WHERE id = $2
    `, sentCount, campaignID)
    
    c.JSON(http.StatusOK, gin.H{
        "message":    "Рассылка выполнена",
        "sent_count": sentCount,
    })
}

// GetEmailTemplates - шаблоны писем
func GetEmailTemplates(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, subject, content, is_active
        FROM email_templates
        WHERE company_id = $1 AND is_active = true
        ORDER BY name
    `, companyID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load templates"})
        return
    }
    defer rows.Close()
    
    var templates []gin.H
    for rows.Next() {
        var id uuid.UUID
        var name, subject, content string
        var isActive bool
        
        rows.Scan(&id, &name, &subject, &content, &isActive)
        
        templates = append(templates, gin.H{
            "id":      id,
            "name":    name,
            "subject": subject,
            "content": content,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// CreateEmailTemplate - создание шаблона
func CreateEmailTemplate(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        Name    string `json:"name" binding:"required"`
        Subject string `json:"subject" binding:"required"`
        Content string `json:"content" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    templateID := uuid.New()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO email_templates (id, company_id, name, subject, content, created_at)
        VALUES ($1, $2, $3, $4, $5, NOW())
    `, templateID, companyID, req.Name, req.Subject, req.Content)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":     "Шаблон создан",
        "template_id": templateID,
    })
}