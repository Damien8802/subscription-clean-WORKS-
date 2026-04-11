package handlers

import (
    "fmt"
    "log"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

// GetChatbotSettings - получить настройки чат-бота
func GetChatbotSettings(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var id string
    var isEnabled bool
    var widgetPosition, primaryColor, welcomeMessage, aiModel string
    var temperature float64
    var knowledgeBase string
    var domains []string
    var workingHours []byte

    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT id, is_enabled, widget_position, primary_color, welcome_message, 
               ai_model, temperature, knowledge_base, domains, working_hours
        FROM chatbot_settings
        WHERE company_id = $1
    `, companyID).Scan(
        &id, &isEnabled, &widgetPosition, &primaryColor, &welcomeMessage,
        &aiModel, &temperature, &knowledgeBase, &domains, &workingHours,
    )

    if err != nil {
        // Создаём настройки по умолчанию
        newID := uuid.New().String()
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO chatbot_settings (id, company_id, is_enabled, widget_position, primary_color, welcome_message, ai_model, temperature, created_at)
            VALUES ($1, $2, true, 'bottom-right', '#8b5cf6', 'Здравствуйте! Чем я могу вам помочь?', 'yandex-gpt-lite', 0.7, NOW())
        `, newID, companyID)
        
        if err != nil {
            c.JSON(http.StatusOK, gin.H{
                "is_enabled": true,
                "widget_position": "bottom-right",
                "primary_color": "#8b5cf6",
                "welcome_message": "Здравствуйте! Чем я могу вам помочь?",
                "ai_model": "yandex-gpt-lite",
                "temperature": 0.7,
            })
            return
        }
        
        c.JSON(http.StatusOK, gin.H{
            "id": newID,
            "is_enabled": true,
            "widget_position": "bottom-right",
            "primary_color": "#8b5cf6",
            "welcome_message": "Здравствуйте! Чем я могу вам помочь?",
            "ai_model": "yandex-gpt-lite",
            "temperature": 0.7,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "id": id,
        "is_enabled": isEnabled,
        "widget_position": widgetPosition,
        "primary_color": primaryColor,
        "welcome_message": welcomeMessage,
        "ai_model": aiModel,
        "temperature": temperature,
        "knowledge_base": knowledgeBase,
        "domains": domains,
    })
}

// UpdateChatbotSettings - обновить настройки чат-бота
func UpdateChatbotSettings(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        IsEnabled       bool    `json:"is_enabled"`
        WidgetPosition  string  `json:"widget_position"`
        PrimaryColor    string  `json:"primary_color"`
        WelcomeMessage  string  `json:"welcome_message"`
        AiModel         string  `json:"ai_model"`
        Temperature     float64 `json:"temperature"`
        KnowledgeBase   string  `json:"knowledge_base"`
        Domains         []string `json:"domains"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE chatbot_settings 
        SET is_enabled = $1, widget_position = $2, primary_color = $3, 
            welcome_message = $4, ai_model = $5, temperature = $6, 
            knowledge_base = $7, domains = $8, updated_at = NOW()
        WHERE company_id = $9
    `, req.IsEnabled, req.WidgetPosition, req.PrimaryColor, req.WelcomeMessage,
        req.AiModel, req.Temperature, req.KnowledgeBase, req.Domains, companyID)

    if err != nil {
        log.Printf("❌ Ошибка обновления настроек: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Настройки сохранены"})
}

// ChatbotWidget - публичный эндпоинт для виджета
func ChatbotWidget(c *gin.Context) {
    companyID := c.Query("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    // Получаем настройки
    var isEnabled bool
    var widgetPosition, primaryColor, welcomeMessage, aiModel string
    var temperature float64

    database.Pool.QueryRow(c.Request.Context(), `
        SELECT is_enabled, widget_position, primary_color, welcome_message, ai_model, temperature
        FROM chatbot_settings
        WHERE company_id = $1
    `, companyID).Scan(&isEnabled, &widgetPosition, &primaryColor, &welcomeMessage, &aiModel, &temperature)

    c.HTML(http.StatusOK, "chatbot_widget.html", gin.H{
        "company_id": companyID,
        "is_enabled": isEnabled,
        "widget_position": widgetPosition,
        "primary_color": primaryColor,
        "welcome_message": welcomeMessage,
        "api_endpoint": "/api/chatbot/message",
    })
}

// SendChatbotMessage - отправка сообщения и получение ответа от AI
func SendChatbotMessage(c *gin.Context) {
    var req struct {
        CompanyID   string `json:"company_id" binding:"required"`
        SessionID   string `json:"session_id"`
        Message     string `json:"message" binding:"required"`
        VisitorName string `json:"visitor_name"`
        VisitorEmail string `json:"visitor_email"`
        PageURL     string `json:"page_url"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if req.CompanyID == "" {
        req.CompanyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    // Создаём или получаем сессию
    if req.SessionID == "" {
        req.SessionID = uuid.New().String()
        
        conversationID := uuid.New().String()
        _, err := database.Pool.Exec(c.Request.Context(), `
            INSERT INTO chatbot_conversations (id, company_id, session_id, visitor_name, visitor_email, page_url, ip_address, user_agent, started_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
        `, conversationID, req.CompanyID, req.SessionID, req.VisitorName, req.VisitorEmail, req.PageURL, c.ClientIP(), c.Request.UserAgent())
        
        if err != nil {
            log.Printf("Ошибка сохранения сессии: %v", err)
        }
    }

    // Получаем историю диалога
    var conversationID string
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT id FROM chatbot_conversations WHERE session_id = $1
    `, req.SessionID).Scan(&conversationID)
    
    // Сохраняем сообщение пользователя
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO chatbot_messages (id, conversation_id, sender, message, created_at)
        VALUES ($1, $2, 'visitor', $3, NOW())
    `, uuid.New().String(), conversationID, req.Message)

    // Генерация ответа
    response := generateAIResponse(req.Message)

    // Сохраняем ответ бота
    database.Pool.Exec(c.Request.Context(), `
        INSERT INTO chatbot_messages (id, conversation_id, sender, message, is_ai_generated, created_at)
        VALUES ($1, $2, 'bot', $3, true, NOW())
    `, uuid.New().String(), conversationID, response)

    c.JSON(http.StatusOK, gin.H{
        "response": response,
        "session_id": req.SessionID,
    })
}

// generateAIResponse - генерация ответа через AI
func generateAIResponse(message string) string {
    // TODO: Здесь реальный вызов YandexGPT API
    return fmt.Sprintf("Спасибо за ваш вопрос! Я передал его специалистам. Они свяжутся с вами в ближайшее время.\n\nВаше сообщение: %s", message)
}

// GetChatbotConversations - список диалогов
func GetChatbotConversations(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, session_id, visitor_name, visitor_email, page_url, status, 
               started_at, ended_at, created_at
        FROM chatbot_conversations
        WHERE company_id = $1
        ORDER BY created_at DESC
        LIMIT 100
    `, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load conversations"})
        return
    }
    defer rows.Close()

    var conversations []gin.H
    for rows.Next() {
        var id, sessionID, visitorName, visitorEmail, pageURL, status string
        var startedAt, endedAt, createdAt time.Time

        rows.Scan(&id, &sessionID, &visitorName, &visitorEmail, &pageURL, &status, &startedAt, &endedAt, &createdAt)

        conversations = append(conversations, gin.H{
            "id": id,
            "session_id": sessionID,
            "visitor_name": visitorName,
            "visitor_email": visitorEmail,
            "page_url": pageURL,
            "status": status,
            "started_at": startedAt.Format("2006-01-02 15:04:05"),
            "created_at": createdAt.Format("2006-01-02 15:04:05"),
        })
    }

    c.JSON(http.StatusOK, gin.H{"conversations": conversations})
}

// GetChatbotMessages - сообщения диалога
func GetChatbotMessages(c *gin.Context) {
    conversationID := c.Param("id")

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT sender, message, is_ai_generated, created_at
        FROM chatbot_messages
        WHERE conversation_id = $1
        ORDER BY created_at ASC
    `, conversationID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
        return
    }
    defer rows.Close()

    var messages []gin.H
    for rows.Next() {
        var sender, message string
        var isAIGenerated bool
        var createdAt time.Time

        rows.Scan(&sender, &message, &isAIGenerated, &createdAt)

        messages = append(messages, gin.H{
            "sender": sender,
            "message": message,
            "is_ai_generated": isAIGenerated,
            "created_at": createdAt.Format("2006-01-02 15:04:05"),
        })
    }

    c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// GetChatbotLeads - лиды из чата
func GetChatbotLeads(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, email, phone, message, status, created_at
        FROM chatbot_leads
        WHERE company_id = $1
        ORDER BY created_at DESC
        LIMIT 100
    `, companyID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load leads"})
        return
    }
    defer rows.Close()

    var leads []gin.H
    for rows.Next() {
        var id, name, email, phone, message, status string
        var createdAt time.Time

        rows.Scan(&id, &name, &email, &phone, &message, &status, &createdAt)

        leads = append(leads, gin.H{
            "id": id,
            "name": name,
            "email": email,
            "phone": phone,
            "message": message,
            "status": status,
            "created_at": createdAt.Format("2006-01-02 15:04:05"),
        })
    }

    c.JSON(http.StatusOK, gin.H{"leads": leads})
}

// CreateChatbotLead - создание лида из чата
func CreateChatbotLead(c *gin.Context) {
    companyID := c.GetString("company_id")
    if companyID == "" {
        companyID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }

    var req struct {
        ConversationID string `json:"conversation_id"`
        Name           string `json:"name"`
        Email          string `json:"email"`
        Phone          string `json:"phone"`
        Message        string `json:"message"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    leadID := uuid.New().String()
    _, err := database.Pool.Exec(c.Request.Context(), `
        INSERT INTO chatbot_leads (id, company_id, conversation_id, name, email, phone, message, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, 'new', NOW())
    `, leadID, companyID, req.ConversationID, req.Name, req.Email, req.Phone, req.Message)

    if err != nil {
        log.Printf("❌ Ошибка создания лида: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create lead"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Заявка отправлена",
        "lead_id": leadID,
    })
}