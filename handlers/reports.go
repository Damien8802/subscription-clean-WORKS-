package handlers

import (
    "context"
    "net/http"
    "time"
    
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    
    "subscription-system/database"
)
// BalanceItem структура строки ОСВ
type BalanceItem struct {
    AccountID   uuid.UUID `json:"account_id"`
    AccountCode string    `json:"account_code"`
    AccountName string    `json:"account_name"`
    AccountType string    `json:"account_type"`
    OpeningDebit  float64 `json:"opening_debit"`
    OpeningCredit float64 `json:"opening_credit"`
    PeriodDebit   float64 `json:"period_debit"`
    PeriodCredit  float64 `json:"period_credit"`
    ClosingDebit  float64 `json:"closing_debit"`
    ClosingCredit float64 `json:"closing_credit"`
}

// GetTurnoverBalanceSheet - Оборотно-сальдовая ведомость (ОСВ)
func GetTurnoverBalanceSheet(c *gin.Context) {
    userID := getUserID(c)
    
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    // Получаем все счета
    accounts, err := getAccounts(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Получаем проводки за период
    postings, err := getPostingsByPeriod(userID, startDate, endDate)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Формируем ОСВ
    var osv []BalanceItem
    for _, acc := range accounts {
        item := BalanceItem{
            AccountID:   acc.ID,
            AccountCode: acc.Code,
            AccountName: acc.Name,
            AccountType: acc.AccountType,
        }
        
        // Собираем обороты по счету
        for _, p := range postings {
            if p.AccountID == acc.ID {
                item.PeriodDebit += p.DebitAmount
                item.PeriodCredit += p.CreditAmount
            }
        }
        
        // Рассчитываем начальное сальдо (для упрощения берем 0)
        // В реальной системе нужно брать из предыдущего периода
        if acc.AccountType == "active" || acc.AccountType == "active_passive" {
            item.ClosingDebit = item.OpeningDebit + item.PeriodDebit - item.PeriodCredit
        } else {
            item.ClosingCredit = item.OpeningCredit + item.PeriodCredit - item.PeriodDebit
        }
        
        if item.ClosingDebit > 0 || item.ClosingCredit > 0 || item.PeriodDebit > 0 || item.PeriodCredit > 0 {
            osv = append(osv, item)
        }
    }
    
    // Подсчет итогов
    totals := struct {
        OpeningDebit  float64 `json:"opening_debit"`
        OpeningCredit float64 `json:"opening_credit"`
        PeriodDebit   float64 `json:"period_debit"`
        PeriodCredit  float64 `json:"period_credit"`
        ClosingDebit  float64 `json:"closing_debit"`
        ClosingCredit float64 `json:"closing_credit"`
    }{}
    
    for _, item := range osv {
        totals.OpeningDebit += item.OpeningDebit
        totals.OpeningCredit += item.OpeningCredit
        totals.PeriodDebit += item.PeriodDebit
        totals.PeriodCredit += item.PeriodCredit
        totals.ClosingDebit += item.ClosingDebit
        totals.ClosingCredit += item.ClosingCredit
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "start_date": startDate,
        "end_date":   endDate,
        "data":       osv,
        "totals":     totals,
    })
}

// GetProfitAndLoss - Отчет о прибылях и убытках
func GetProfitAndLoss(c *gin.Context) {
    userID := getUserID(c)
    
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    
    if startDate == "" {
        startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-01")
    }
    if endDate == "" {
        endDate = time.Now().Format("2006-01-02")
    }
    
    // Доходы (счета 90, 91)
    var revenue float64
    err := database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date BETWEEN $2 AND $3
        AND p.account_id IN (
            SELECT id FROM chart_of_accounts 
            WHERE user_id = $1 AND code IN ('90', '91')
        )
    `, userID, startDate, endDate).Scan(&revenue)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // Расходы (счета 20, 26, 44, 91)
    var expenses float64
    err = database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date BETWEEN $2 AND $3
        AND p.account_id IN (
            SELECT id FROM chart_of_accounts 
            WHERE user_id = $1 AND code IN ('20', '26', '44', '91')
        )
    `, userID, startDate, endDate).Scan(&expenses)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    profit := revenue - expenses
    
    c.JSON(http.StatusOK, gin.H{
        "success":    true,
        "start_date": startDate,
        "end_date":   endDate,
        "revenue":    revenue,
        "expenses":   expenses,
        "profit":     profit,
    })
}

// GetDashboardStats - Статистика для дашборда
func GetDashboardStats(c *gin.Context) {
    userID := getUserID(c)
    
    // Общая выручка за месяц
    var revenue float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '90')
    `, userID).Scan(&revenue)
    
    // Общие расходы за месяц
    var expenses float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(debit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date >= DATE_TRUNC('month', NOW())
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code IN ('20', '26', '44'))
    `, userID).Scan(&expenses)
    
    // Количество проводок
    var entriesCount int
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COUNT(*) FROM journal_entries 
        WHERE user_id = $1 AND entry_status = 'posted'
    `, userID).Scan(&entriesCount)
    
    // Остаток на расчетном счете
    var bankBalance float64
    database.Pool.QueryRow(c.Request.Context(), `
        SELECT COALESCE(SUM(debit_amount - credit_amount), 0)
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '51')
    `, userID).Scan(&bankBalance)
    
    c.JSON(http.StatusOK, gin.H{
        "success":       true,
        "revenue":       revenue,
        "expenses":      expenses,
        "profit":        revenue - expenses,
        "entries_count": entriesCount,
        "bank_balance":  bankBalance,
    })
}

// GetSalesChart - Данные для графика продаж
func GetSalesChart(c *gin.Context) {
    userID := getUserID(c)
    
    period := c.DefaultQuery("period", "month") // month, quarter, year
    
    var interval string
    switch period {
    case "quarter":
        interval = "3 months"
    case "year":
        interval = "1 year"
    default:
        interval = "1 month"
    }
    
    rows, err := database.Pool.Query(c.Request.Context(), `
        SELECT 
            DATE_TRUNC('day', e.entry_date) as date,
            COALESCE(SUM(p.credit_amount), 0) as total
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date >= NOW() - $2::INTERVAL
        AND p.account_id IN (SELECT id FROM chart_of_accounts WHERE user_id = $1 AND code = '90')
        GROUP BY DATE_TRUNC('day', e.entry_date)
        ORDER BY date
    `, userID, interval)
    
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    defer rows.Close()
    
    var dates []string
    var values []float64
    
    for rows.Next() {
        var date time.Time
        var total float64
        rows.Scan(&date, &total)
        dates = append(dates, date.Format("2006-01-02"))
        values = append(values, total)
    }
    
    c.JSON(http.StatusOK, gin.H{
        "success": true,
        "period":  period,
        "labels":  dates,
        "data":    values,
    })
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

type accountInfo struct {
    ID          uuid.UUID
    Code        string
    Name        string
    AccountType string
}

type postingInfo struct {
    AccountID    uuid.UUID
    DebitAmount  float64
    CreditAmount float64
}

func getAccounts(userID uuid.UUID) ([]accountInfo, error) {
    rows, err := database.Pool.Query(context.Background(), `
        SELECT id, code, name, account_type 
        FROM chart_of_accounts 
        WHERE user_id = $1 AND is_active = true
        ORDER BY code
    `, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var accounts []accountInfo
    for rows.Next() {
        var a accountInfo
        rows.Scan(&a.ID, &a.Code, &a.Name, &a.AccountType)
        accounts = append(accounts, a)
    }
    return accounts, nil
}

func getPostingsByPeriod(userID uuid.UUID, startDate, endDate string) ([]postingInfo, error) {
    rows, err := database.Pool.Query(context.Background(), `
        SELECT p.account_id, p.debit_amount, p.credit_amount
        FROM journal_postings p
        JOIN journal_entries e ON p.entry_id = e.id
        WHERE e.user_id = $1 
        AND e.entry_status = 'posted'
        AND e.entry_date BETWEEN $2 AND $3
    `, userID, startDate, endDate)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var postings []postingInfo
    for rows.Next() {
        var p postingInfo
        rows.Scan(&p.AccountID, &p.DebitAmount, &p.CreditAmount)
        postings = append(postings, p)
    }
    return postings, nil
}