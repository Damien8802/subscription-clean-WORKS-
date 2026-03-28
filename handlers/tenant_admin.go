package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"

    "subscription-system/database"
)
// TenantAdminPage - страница управления компаниями
func TenantAdminPage(c *gin.Context) {
    c.HTML(http.StatusOK, "admin_tenants.html", gin.H{
        "title": "Управление компаниями | TeamSphere",
    })
}

// GetTenants - получить список всех компаний
func GetTenants(c *gin.Context) {
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, name, subdomain, logo_url, settings, status, created_at, updated_at
        FROM tenants
        ORDER BY created_at DESC
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var tenants []map[string]interface{}
    for rows.Next() {
        var id uuid.UUID
        var name, subdomain, logoURL, status string
        var settings []byte
        var createdAt, updatedAt time.Time

        rows.Scan(&id, &name, &subdomain, &logoURL, &settings, &status, &createdAt, &updatedAt)

        tenants = append(tenants, map[string]interface{}{
            "id":         id,
            "name":       name,
            "subdomain":  subdomain,
            "logo_url":   logoURL,
            "settings":   settings,
            "status":     status,
            "created_at": createdAt,
            "updated_at": updatedAt,
        })
    }

    c.JSON(http.StatusOK, gin.H{"tenants": tenants})
}

// CreateTenant - создать новую компанию
func CreateTenant(c *gin.Context) {
    var req struct {
        Name      string `json:"name" binding:"required"`
        Subdomain string `json:"subdomain" binding:"required"`
    }

    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO tenants (id, name, subdomain, status, created_at, updated_at)
        VALUES (gen_random_uuid(), $1, $2, 'active', NOW(), NOW())
        RETURNING id
    `, req.Name, req.Subdomain).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tenant"})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "id":      id,
        "message": "Компания создана",
    })
}

// UpdateTenant - обновить компанию
func UpdateTenant(c *gin.Context) {
    tenantID := c.Param("id")

    var req struct {
        Name   string `json:"name"`
        Status string `json:"status"`
    }

    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE tenants SET name = $1, status = $2, updated_at = NOW()
        WHERE id = $3
    `, req.Name, req.Status, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update tenant"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Компания обновлена"})
}

// DeleteTenant - удалить компанию
func DeleteTenant(c *gin.Context) {
    tenantID := c.Param("id")

    // Не даем удалить дефолтного tenant
    if tenantID == "11111111-1111-1111-1111-111111111111" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete default tenant"})
        return
    }

    _, err := database.Pool.Exec(c.Request.Context(), `
        DELETE FROM tenants WHERE id = $1
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete tenant"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "message": "Компания удалена"})
}

// SwitchTenant - переключить компанию (для UI)
func SwitchTenant(c *gin.Context) {
    tenantID := c.Param("id")

    // Сохраняем выбранный tenant в сессию/cookie
    c.SetCookie("selected_tenant", tenantID, 3600*24*30, "/", "", false, true)

    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "message": "Компания переключена",
    })
}