package handlers

import (
    "log"
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "subscription-system/database"
)

func HRDashboardHandler(c *gin.Context) {
    log.Println("=== HR Dashboard called ===")
    c.HTML(http.StatusOK, "dashboard.html", gin.H{
        "Title": "HR-модуль | SaaSPro",
    })
}

func GetEmployeesHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, first_name, last_name, email, phone, position, department, 
               hire_date, salary, status, created_at 
        FROM hr_employees WHERE tenant_id = $1 ORDER BY created_at DESC
    `, tenantID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var employees []gin.H
    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName, email, phone, position, department, status string
        var hireDate time.Time
        var salary float64
        var createdAt time.Time

        rows.Scan(&id, &firstName, &lastName, &email, &phone, &position, &department,
            &hireDate, &salary, &status, &createdAt)

        employees = append(employees, gin.H{
            "id":         id.String(),
            "first_name": firstName,
            "last_name":  lastName,
            "full_name":  firstName + " " + lastName,
            "email":      email,
            "phone":      phone,
            "position":   position,
            "department": department,
            "hire_date":  hireDate.Format("02.01.2006"),
            "salary":     salary,
            "status":     status,
            "created_at": createdAt.Format("02.01.2006"),
        })
    }

    c.JSON(http.StatusOK, gin.H{"employees": employees})
}

func AddEmployeeHandler(c *gin.Context) {
    var req struct {
        FirstName  string  `json:"first_name" binding:"required"`
        LastName   string  `json:"last_name" binding:"required"`
        Email      string  `json:"email" binding:"required,email"`
        Phone      string  `json:"phone"`
        Position   string  `json:"position" binding:"required"`
        Department string  `json:"department" binding:"required"`
        Salary     float64 `json:"salary" binding:"required"`
        HireDate   string  `json:"hire_date"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    hireDate := time.Now()
    if req.HireDate != "" {
        if parsed, err := time.Parse("2006-01-02", req.HireDate); err == nil {
            hireDate = parsed
        }
    }

    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO hr_employees (first_name, last_name, email, phone, position, department, 
        hire_date, salary, status, tenant_id)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'active', $9)
        RETURNING id
    `, req.FirstName, req.LastName, req.Email, req.Phone, req.Position, req.Department,
        hireDate, req.Salary, tenantID).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create employee"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "id": id.String()})
}

func UpdateEmployeeHandler(c *gin.Context) {
    id := c.Param("id")
    var req struct {
        FirstName  string  `json:"first_name"`
        LastName   string  `json:"last_name"`
        Email      string  `json:"email"`
        Phone      string  `json:"phone"`
        Position   string  `json:"position"`
        Department string  `json:"department"`
        Salary     float64 `json:"salary"`
        Status     string  `json:"status"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    query := `UPDATE hr_employees SET updated_at = NOW()`
    args := []interface{}{}
    argIdx := 1

    if req.FirstName != "" {
        query += ", first_name = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.FirstName)
        argIdx++
    }
    if req.LastName != "" {
        query += ", last_name = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.LastName)
        argIdx++
    }
    if req.Email != "" {
        query += ", email = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.Email)
        argIdx++
    }
    if req.Phone != "" {
        query += ", phone = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.Phone)
        argIdx++
    }
    if req.Position != "" {
        query += ", position = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.Position)
        argIdx++
    }
    if req.Department != "" {
        query += ", department = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.Department)
        argIdx++
    }
    if req.Salary > 0 {
        query += ", salary = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.Salary)
        argIdx++
    }
    if req.Status != "" {
        query += ", status = $" + strconv.Itoa(argIdx+1)
        args = append(args, req.Status)
        argIdx++
    }
    query += " WHERE id = $1"
    args = append([]interface{}{id}, args...)

    _, err := database.Pool.Exec(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteEmployeeHandler(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), `DELETE FROM hr_employees WHERE id = $1`, id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetVacationRequestsHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT r.id, r.employee_id, r.start_date, r.end_date, r.days, r.type, r.status, r.comment, r.created_at,
               e.first_name, e.last_name, e.position, e.department
        FROM hr_vacation_requests r
        JOIN hr_employees e ON r.employee_id = e.id
        WHERE r.tenant_id = $1
        ORDER BY r.created_at DESC
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var requests []gin.H
    for rows.Next() {
        var id, employeeID uuid.UUID
        var startDate, endDate, createdAt time.Time
        var days int
        var reqType, status, comment, firstName, lastName, position, department string

        rows.Scan(&id, &employeeID, &startDate, &endDate, &days, &reqType, &status, &comment, &createdAt,
            &firstName, &lastName, &position, &department)

        requests = append(requests, gin.H{
            "id":          id.String(),
            "employee_id": employeeID.String(),
            "employee":    firstName + " " + lastName,
            "position":    position,
            "department":  department,
            "start_date":  startDate.Format("02.01.2006"),
            "end_date":    endDate.Format("02.01.2006"),
            "days":        days,
            "type":        reqType,
            "status":      status,
            "comment":     comment,
            "created_at":  createdAt.Format("02.01.2006 15:04"),
        })
    }

    c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func AddVacationRequestHandler(c *gin.Context) {
    var req struct {
        EmployeeID string `json:"employee_id" binding:"required"`
        StartDate  string `json:"start_date" binding:"required"`
        EndDate    string `json:"end_date" binding:"required"`
        Type       string `json:"type"`
        Comment    string `json:"comment"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    startDate, _ := time.Parse("2006-01-02", req.StartDate)
    endDate, _ := time.Parse("2006-01-02", req.EndDate)
    days := int(endDate.Sub(startDate).Hours()/24) + 1

    if req.Type == "" {
        req.Type = "vacation"
    }

    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO hr_vacation_requests (employee_id, start_date, end_date, days, type, status, comment, tenant_id)
        VALUES ($1, $2, $3, $4, $5, 'pending', $6, $7)
        RETURNING id
    `, req.EmployeeID, startDate, endDate, days, req.Type, req.Comment, tenantID).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "id": id.String()})
}

func ApproveRequestHandler(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), `UPDATE hr_vacation_requests SET status = 'approved' WHERE id = $1`, id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func RejectRequestHandler(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), `UPDATE hr_vacation_requests SET status = 'rejected' WHERE id = $1`, id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetCandidatesHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT id, first_name, last_name, email, phone, position, status, source, match_score, interview_date, created_at
        FROM hr_candidates WHERE tenant_id = $1 ORDER BY created_at DESC
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    candidates := gin.H{
        "new":        []gin.H{},
        "interview":  []gin.H{},
        "offer":      []gin.H{},
        "hired":      []gin.H{},
    }

    for rows.Next() {
        var id uuid.UUID
        var firstName, lastName, email, phone, position, status, source string
        var matchScore int
        var interviewDate *time.Time
        var createdAt time.Time

        rows.Scan(&id, &firstName, &lastName, &email, &phone, &position, &status, &source, &matchScore, &interviewDate, &createdAt)

        cand := gin.H{
            "id":          id.String(),
            "first_name":  firstName,
            "last_name":   lastName,
            "full_name":   firstName + " " + lastName,
            "email":       email,
            "phone":       phone,
            "position":    position,
            "source":      source,
            "match_score": matchScore,
            "created_at":  createdAt.Format("02.01.2006"),
        }

        if interviewDate != nil {
            cand["interview_date"] = interviewDate.Format("02.01.2006 15:04")
        }

        switch status {
        case "new":
            candidates["new"] = append(candidates["new"].([]gin.H), cand)
        case "interview":
            candidates["interview"] = append(candidates["interview"].([]gin.H), cand)
        case "offer":
            candidates["offer"] = append(candidates["offer"].([]gin.H), cand)
        case "hired":
            candidates["hired"] = append(candidates["hired"].([]gin.H), cand)
        }
    }

    c.JSON(http.StatusOK, gin.H{"candidates": candidates})
}

func AddCandidateHandler(c *gin.Context) {
    var req struct {
        FirstName string `json:"first_name" binding:"required"`
        LastName  string `json:"last_name" binding:"required"`
        Email     string `json:"email"`
        Phone     string `json:"phone"`
        Position  string `json:"position" binding:"required"`
        Source    string `json:"source"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    var id uuid.UUID
    err := database.Pool.QueryRow(c.Request.Context(), `
        INSERT INTO hr_candidates (first_name, last_name, email, phone, position, status, source, tenant_id)
        VALUES ($1, $2, $3, $4, $5, 'new', $6, $7)
        RETURNING id
    `, req.FirstName, req.LastName, req.Email, req.Phone, req.Position, req.Source, tenantID).Scan(&id)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create candidate"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true, "id": id.String()})
}

func UpdateCandidateStatusHandler(c *gin.Context) {
    id := c.Param("id")
    var req struct {
        Status        string  `json:"status"`
        InterviewDate *string `json:"interview_date"`
        MatchScore    *int    `json:"match_score"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    query := `UPDATE hr_candidates SET status = $2, updated_at = NOW()`
    args := []interface{}{id, req.Status}
    argIdx := 3

    if req.InterviewDate != nil && *req.InterviewDate != "" {
        if interviewDate, err := time.Parse("2006-01-02T15:04", *req.InterviewDate); err == nil {
            query += ", interview_date = $" + strconv.Itoa(argIdx)
            args = append(args, interviewDate)
            argIdx++
        }
    }
    if req.MatchScore != nil {
        query += ", match_score = $" + strconv.Itoa(argIdx)
        args = append(args, *req.MatchScore)
    }
    query += " WHERE id = $1"

    _, err := database.Pool.Exec(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteCandidateHandler(c *gin.Context) {
    id := c.Param("id")
    _, err := database.Pool.Exec(c.Request.Context(), `DELETE FROM hr_candidates WHERE id = $1`, id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetStatisticsHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    var totalEmployees, vacationCount, sickCount, pendingRequests, candidatesCount int
    var totalSalary, avgSalary float64

    database.Pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM hr_employees WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&totalEmployees)
    database.Pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM hr_employees WHERE tenant_id = $1 AND status = 'vacation'`, tenantID).Scan(&vacationCount)
    database.Pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM hr_employees WHERE tenant_id = $1 AND status = 'sick'`, tenantID).Scan(&sickCount)
    database.Pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM hr_vacation_requests WHERE tenant_id = $1 AND status = 'pending'`, tenantID).Scan(&pendingRequests)
    database.Pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM hr_candidates WHERE tenant_id = $1`, tenantID).Scan(&candidatesCount)
    database.Pool.QueryRow(c.Request.Context(), `SELECT COALESCE(SUM(salary), 0), COALESCE(AVG(salary), 0) FROM hr_employees WHERE tenant_id = $1 AND status = 'active'`, tenantID).Scan(&totalSalary, &avgSalary)

    c.JSON(http.StatusOK, gin.H{"statistics": gin.H{
        "totalEmployees":  totalEmployees,
        "vacationCount":   vacationCount,
        "sickCount":       sickCount,
        "pendingRequests": pendingRequests,
        "candidatesCount": candidatesCount,
        "totalSalary":     totalSalary,
        "avgSalary":       avgSalary,
    }})
}

func AnalyzeCandidateHandler(c *gin.Context) {
    id := c.Param("id")
    var firstName, lastName, position string

    err := database.Pool.QueryRow(c.Request.Context(), `SELECT first_name, last_name, position FROM hr_candidates WHERE id = $1`, id).Scan(&firstName, &lastName, &position)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Candidate not found"})
        return
    }

    matchScore := 75 + (len(firstName)+len(lastName))%25

    c.JSON(http.StatusOK, gin.H{
        "analysis": gin.H{
            "candidate":      firstName + " " + lastName,
            "position":       position,
            "match_score":    matchScore,
            "strengths":      "Хорошие коммуникативные навыки, опыт работы с командой",
            "weaknesses":     "Требуется дополнительное обучение",
            "recommendation": "Рекомендуется к собеседованию",
        },
    })
}

func AIChatHandler(c *gin.Context) {
    var req struct{ Message string }
    c.ShouldBindJSON(&req)

    reply := "Я AI ассистент HR-модуля. Чем могу помочь? Могу подобрать кандидатов, проанализировать резюме или дать рекомендации по подбору персонала."

    if req.Message != "" {
        reply = "Анализирую ваш запрос: \"" + req.Message + "\". Рекомендую обратить внимание на кандидатов с опытом работы от 3 лет и высокой мотивацией."
    }

    c.JSON(http.StatusOK, gin.H{"reply": reply})
}

func SuggestTrainingHandler(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "suggestions": []gin.H{
            {"name": "Курс по управлению персоналом", "duration": "40 часов", "level": "Продвинутый", "price": 15000},
            {"name": "Тренинг лидерства и мотивации", "duration": "24 часа", "level": "Средний", "price": 12000},
            {"name": "HR-аналитика и метрики", "duration": "32 часа", "level": "Продвинутый", "price": 18000},
        },
    })
}

func PredictTurnoverHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    var predictions []gin.H
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT first_name, last_name, position, department, hire_date, salary
        FROM hr_employees WHERE tenant_id = $1 AND status = 'active'
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    for rows.Next() {
        var firstName, lastName, position, department string
        var hireDate time.Time
        var salary float64
        rows.Scan(&firstName, &lastName, &position, &department, &hireDate, &salary)

        yearsWorked := time.Since(hireDate).Hours() / 24 / 365
        risk := "Низкий"
        riskColor := "green"
        if yearsWorked < 1 {
            risk = "Высокий"
            riskColor = "red"
        } else if yearsWorked < 2 {
            risk = "Средний"
            riskColor = "orange"
        }

        predictions = append(predictions, gin.H{
            "name":       firstName + " " + lastName,
            "position":   position,
            "department": department,
            "risk":       risk,
            "risk_color": riskColor,
            "years":      yearsWorked,
        })
    }

    c.JSON(http.StatusOK, gin.H{"predictions": predictions})
}

func GenerateOrderHandler(c *gin.Context) {
    var req struct {
        Type       string `json:"type"`
        EmployeeID string `json:"employee_id"`
    }
    c.ShouldBindJSON(&req)

    var employeeName, position string
    database.Pool.QueryRow(c.Request.Context(), `SELECT first_name || ' ' || last_name, position FROM hr_employees WHERE id = $1`, req.EmployeeID).Scan(&employeeName, &position)

    order := "ПРИКАЗ №" + time.Now().Format("20060102-1504") + "\n\n"
    if req.Type == "hire" {
        order += "О приеме на работу\n\nПринять " + employeeName + " на должность " + position + " с " + time.Now().Format("02.01.2006")
    } else if req.Type == "vacation" {
        order += "О предоставлении отпуска\n\nПредоставить " + employeeName + " ежегодный оплачиваемый отпуск с " + time.Now().Format("02.01.2006")
    } else {
        order += "О кадровом перемещении\n\nПеревести " + employeeName + " на новую должность"
    }

    c.JSON(http.StatusOK, gin.H{"order": order})
}

func GetDepartmentsHandler(c *gin.Context) {
    tenantID := c.GetString("tenant_id")
    if tenantID == "" {
        tenantID = "11111111-1111-1111-1111-111111111111"
    }

    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT department, COUNT(*) as count, SUM(salary) as budget
        FROM hr_employees 
        WHERE tenant_id = $1 AND status = 'active'
        GROUP BY department
        ORDER BY department
    `, tenantID)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()

    var departments []gin.H
    for rows.Next() {
        var department string
        var count int
        var budget float64
        rows.Scan(&department, &count, &budget)

        departments = append(departments, gin.H{
            "name":   department,
            "count":  count,
            "budget": budget,
        })
    }

    c.JSON(http.StatusOK, gin.H{"departments": departments})
}


