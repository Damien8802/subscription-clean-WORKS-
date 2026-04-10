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

// GetEmployeesForPayroll - список сотрудников для расчёта зарплаты
func GetEmployeesForPayroll(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, first_name, last_name, position, 
               COALESCE(salary, 0) as salary, 
               COALESCE(tax_rate, 13) as tax_rate
        FROM hr_employees 
        WHERE tenant_id = $1 AND status = 'active'
        ORDER BY last_name
    `, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка загрузки сотрудников: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load employees"})
        return
    }
    defer rows.Close()
    
    var employees []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName, position string
        var salary, taxRate float64
        
        err := rows.Scan(&id, &firstName, &lastName, &position, &salary, &taxRate)
        if err != nil {
            log.Printf("⚠️ Ошибка сканирования: %v", err)
            continue
        }
        
        tax := salary * taxRate / 100
        netAmount := salary - tax
        
        employees = append(employees, gin.H{
            "id":         id,
            "name":       firstName + " " + lastName,
            "position":   position,
            "salary":     salary,
            "tax_rate":   taxRate,
            "tax":        tax,
            "net_amount": netAmount,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"employees": employees})
}

// CalculatePayroll - расчёт зарплаты за период
func CalculatePayroll(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        Month int `json:"month" binding:"required"`
        Year  int `json:"year" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, first_name, last_name, salary, tax_rate
        FROM hr_employees 
        WHERE tenant_id = $1 AND status = 'active'
    `, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка загрузки сотрудников: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load employees"})
        return
    }
    defer rows.Close()
    
    var payrolls []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName string
        var salary, taxRate float64
        
        rows.Scan(&id, &firstName, &lastName, &salary, &taxRate)
        
        tax := salary * taxRate / 100
        netAmount := salary - tax
        
        _, err = database.Pool.Exec(c.Request.Context(), `
            INSERT INTO payroll (id, tenant_id, employee_id, period_month, period_year, salary, tax, net_amount, status, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'calculated', NOW())
            ON CONFLICT (employee_id, period_month, period_year) DO UPDATE
            SET salary = $6, tax = $7, net_amount = $8, status = 'calculated'
        `, uuid.New(), tenantID, id, req.Month, req.Year, salary, tax, netAmount)
        
        payrolls = append(payrolls, gin.H{
            "employee_id": id,
            "name":        firstName + " " + lastName,
            "salary":      salary,
            "tax":         tax,
            "net_amount":  netAmount,
        })
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":  "Расчёт выполнен",
        "payrolls": payrolls,
        "total":    len(payrolls),
        "month":    req.Month,
        "year":     req.Year,
    })
}

// GetPayrollHistory - история начислений
func GetPayrollHistory(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT p.id, e.first_name, e.last_name, p.period_month, p.period_year, 
               p.salary, p.tax, p.net_amount, p.status, p.created_at
        FROM payroll p
        JOIN hr_employees e ON p.employee_id = e.id
        WHERE p.tenant_id = $1
        ORDER BY p.period_year DESC, p.period_month DESC, e.last_name
        LIMIT 100
    `, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка загрузки истории: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load history"})
        return
    }
    defer rows.Close()
    
    var history []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName string
        var month, year int
        var salary, tax, netAmount float64
        var status string
        var createdAt time.Time
        
        rows.Scan(&id, &firstName, &lastName, &month, &year, &salary, &tax, &netAmount, &status, &createdAt)
        
        history = append(history, gin.H{
            "id":         id,
            "employee":   firstName + " " + lastName,
            "period":     fmt.Sprintf("%d/%d", month, year),
            "salary":     salary,
            "tax":        tax,
            "net_amount": netAmount,
            "status":     status,
            "created_at": createdAt.Format("2006-01-02"),
        })
    }
    
    c.JSON(http.StatusOK, gin.H{"history": history})
}

// ProcessPayrollPayment - выплата зарплаты
func ProcessPayrollPayment(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        PayrollID string `json:"payroll_id" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    _, err := database.Pool.Exec(c.Request.Context(), `
        UPDATE payroll 
        SET status = 'paid', paid_at = NOW()
        WHERE id = $1 AND tenant_id = $2
    `, req.PayrollID, tenantID)
    
    if err != nil {
        log.Printf("❌ Ошибка выплаты: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process payment"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Зарплата выплачена"})
}

// GenerateTaxReport - генерация налогового отчёта
func GenerateTaxReport(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "aa5f14e6-30e1-476c-ac42-8c11ced838a4"
    }
    
    var req struct {
        Month int `json:"month" binding:"required"`
        Year  int `json:"year" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // Собираем данные за период
    var totalIncome, totalTax float64
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT COALESCE(SUM(salary), 0), COALESCE(SUM(tax), 0)
        FROM payroll
        WHERE tenant_id = $1 AND period_month = $2 AND period_year = $3
    `, tenantID, req.Month, req.Year)
    
    if err == nil {
        defer rows.Close()
        if rows.Next() {
            rows.Scan(&totalIncome, &totalTax)
        }
    }
    
    // Сохраняем отчёт
    reportID := uuid.New()
    _, err = database.Pool.Exec(c.Request.Context(), `
        INSERT INTO tax_reports (id, tenant_id, report_type, period_month, period_year, total_income, total_tax, created_at)
        VALUES ($1, $2, '6-НДФЛ', $3, $4, $5, $6, NOW())
    `, reportID, tenantID, req.Month, req.Year, totalIncome, totalTax)
    
    if err != nil {
        log.Printf("❌ Ошибка генерации отчёта: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate report"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{
        "message":      "Отчёт сгенерирован",
        "report_id":    reportID,
        "total_income": totalIncome,
        "total_tax":    totalTax,
        "month":        req.Month,
        "year":         req.Year,
    })
}