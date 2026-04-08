package models

import "time"

type Employee struct {
    ID         string    `json:"id"`
    FirstName  string    `json:"first_name"`
    LastName   string    `json:"last_name"`
    Email      string    `json:"email"`
    Phone      string    `json:"phone"`
    Position   string    `json:"position"`
    Department string    `json:"department"`
    Salary     float64   `json:"salary"`
    HireDate   time.Time `json:"hire_date"`
    Status     string    `json:"status"`
}

type VacationRequest struct {
    ID         string    `json:"id"`
    EmployeeID string    `json:"employee_id"`
    StartDate  time.Time `json:"start_date"`
    EndDate    time.Time `json:"end_date"`
    Status     string    `json:"status"`
}

type Candidate struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Position string `json:"position"`
    Status   string `json:"status"`
    Score    int    `json:"score"`
}