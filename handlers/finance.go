package handlers

import (
    "database/sql"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)

// ChartOfAccount структура счета
type ChartOfAccount struct {
    ID          uuid.UUID  `json:"id"`
    Code        string     `json:"code"`
    Name        string     `json:"name"`
    AccountType string     `json:"account_type"`
    ParentID    *uuid.UUID `json:"parent_id"`
    Level       int        `json:"level"`
    IsGroup     bool       `json:"is_group"`
    Currency    string     `json:"currency"`
    Description string     `json:"description"`
    IsActive    bool       `json:"is_active"`
    CreatedAt   time.Time  `json:"created_at"`
}

// GetChartOfAccounts - получить план счетов
func GetChartOfAccounts(c *gin.Context) {
    userID := getUserID(c)
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, code, name, account_type, parent_id, level, is_group, currency, description, is_active, created_at
        FROM chart_of_accounts
        WHERE user_id = $1 AND is_active = true
        ORDER BY code
    `, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var accounts []ChartOfAccount
    for rows.Next() {
        var a ChartOfAccount
        var parentID sql.NullString
        err := rows.Scan(
            &a.ID, &a.Code, &a.Name, &a.AccountType, &parentID,
            &a.Level, &a.IsGroup, &a.Currency, &a.Description,
            &a.IsActive, &a.CreatedAt,
        )
        if err != nil {
            continue
        }
        if parentID.Valid {
            id, _ := uuid.Parse(parentID.String)
            a.ParentID = &id
        }
        accounts = append(accounts, a)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":  true,
        "accounts": accounts,
    })
}

// CreateChartOfAccount - создать счет
func CreateChartOfAccount(c *gin.Context) {
    userID := getUserID(c)
    
    var req struct {
        Code        string     `json:"code" binding:"required"`
        Name        string     `json:"name" binding:"required"`
        AccountType string     `json:"account_type" binding:"required"`
        ParentID    *uuid.UUID `json:"parent_id"`
        Level       int        `json:"level"`
        IsGroup     bool       `json:"is_group"`
        Currency    string     `json:"currency"`
        Description string     `json:"description"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if req.Currency == "" {
        req.Currency = "RUB"
    }
    
    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO chart_of_accounts (user_id, code, name, account_type, parent_id, level, is_group, currency, description, is_active, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, NOW(), NOW())
        RETURNING id
    `, userID, req.Code, req.Name, req.AccountType, req.ParentID, req.Level, req.IsGroup, req.Currency, req.Description).Scan(&id)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать счет"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "id":      id,
        "message": "Счет создан",
    })
}

// UpdateChartOfAccount - обновить счет
func UpdateChartOfAccount(c *gin.Context) {
    userID := getUserID(c)
    accountID := c.Param("id")
    
    var req struct {
        Code        string     `json:"code"`
        Name        string     `json:"name"`
        AccountType string     `json:"account_type"`
        ParentID    *uuid.UUID `json:"parent_id"`
        Level       int        `json:"level"`
        IsGroup     bool       `json:"is_group"`
        Currency    string     `json:"currency"`
        Description string     `json:"description"`
        IsActive    bool       `json:"is_active"`
    }
    
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE chart_of_accounts SET
            code = $1, name = $2, account_type = $3, parent_id = $4,
            level = $5, is_group = $6, currency = $7, description = $8,
            is_active = $9, updated_at = NOW()
        WHERE id = $10 AND user_id = $11
    `, req.Code, req.Name, req.AccountType, req.ParentID,
        req.Level, req.IsGroup, req.Currency, req.Description,
        req.IsActive, accountID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить счет"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Счет обновлен",
    })
}

// DeleteChartOfAccount - удалить счет
func DeleteChartOfAccount(c *gin.Context) {
    userID := getUserID(c)
    accountID := c.Param("id")
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE chart_of_accounts SET is_active = false, updated_at = NOW()
        WHERE id = $1 AND user_id = $2
    `, accountID, userID)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить счет"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Счет удален",
    })
}