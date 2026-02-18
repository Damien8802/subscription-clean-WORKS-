package models

import (
"context"
"time"

"subscription-system/database"
"golang.org/x/crypto/bcrypt"
)

type User struct {
ID        string    `json:"id" db:"id"`
Email     string    `json:"email" db:"email"`
Password  string    `json:"-" db:"password_hash"`
Name      string    `json:"name" db:"name"`
Role      string    `json:"role" db:"role"`
CreatedAt time.Time `json:"created_at" db:"created_at"`
UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func HashPassword(password string) (string, error) {
bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
return err == nil
}

func FindUserByEmail(email string) (*User, error) {
var user User
query := `SELECT id, email, password_hash, name, role, created_at, updated_at 
  FROM users WHERE email = $1`
err := database.Pool.QueryRow(context.Background(), query, email).Scan(
&user.ID, &user.Email, &user.Password, &user.Name, &user.Role,
&user.CreatedAt, &user.UpdatedAt,
)
if err != nil {
return nil, err
}
return &user, nil
}

func CreateUser(email, password, name string) (*User, error) {
hash, err := HashPassword(password)
if err != nil {
return nil, err
}

var user User
query := `INSERT INTO users (email, password_hash, name, role, created_at, updated_at)
  VALUES ($1, $2, $3, 'user', NOW(), NOW())
  RETURNING id, email, name, role, created_at, updated_at`
err = database.Pool.QueryRow(context.Background(), query, email, hash, name).Scan(
&user.ID, &user.Email, &user.Name, &user.Role,
&user.CreatedAt, &user.UpdatedAt,
)
if err != nil {
return nil, err
}
return &user, nil
}

func GetUserByID(id string) (*User, error) {
var user User
query := `SELECT id, email, name, role, created_at, updated_at 
  FROM users WHERE id = $1`
err := database.Pool.QueryRow(context.Background(), query, id).Scan(
&user.ID, &user.Email, &user.Name, &user.Role,
&user.CreatedAt, &user.UpdatedAt,
)
if err != nil {
return nil, err
}
return &user, nil
}

// UpdateUser обновляет имя и email пользователя
func UpdateUser(id, name, email string) error {
    query := `UPDATE users SET name = $1, email = $2, updated_at = NOW() WHERE id = $3`
    _, err := database.Pool.Exec(context.Background(), query, name, email, id)
    return err
}

// UpdatePassword обновляет пароль пользователя
func UpdatePassword(id, newPassword string) error {
    hash, err := HashPassword(newPassword)
    if err != nil {
        return err
    }
    query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
    _, err = database.Pool.Exec(context.Background(), query, hash, id)
    return err
}
