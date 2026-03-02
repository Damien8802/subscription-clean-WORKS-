package handlers

import (
"fmt"
"net/http"
"subscription-system/auth"
"subscription-system/config"
"subscription-system/models"

"github.com/gin-gonic/gin"
)

var cfg *config.Config

func InitAuthHandler(c *config.Config) {
cfg = c
}

type RegisterRequest struct {
Email    string `json:"email" binding:"required,email"`
Password string `json:"password" binding:"required,min=6"`
Name     string `json:"name"`
}

type LoginRequest struct {
Email    string `json:"email" binding:"required,email"`
Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
RefreshToken string `json:"refresh_token" binding:"required"`
}

// RegisterHandler – регистрация нового пользователя
func RegisterHandler(c *gin.Context) {
var req RegisterRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

existing, _ := models.FindUserByEmail(req.Email)
if existing != nil {
c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
return
}

user, err := models.CreateUser(req.Email, req.Password, req.Name)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
return
}

// === AI: добавляем информацию о пользователе в базу знаний ===
metadata := map[string]interface{}{
"user_id": user.ID,
"email":   user.Email,
"name":    user.Name,
"role":    user.Role,
}
content := fmt.Sprintf("Пользователь %s (%s) зарегистрировался с ролью %s", user.Name, user.Email, user.Role)
_ = models.AddDocument(user.ID, "user_profile", content, metadata)
// === конец блока AI ===

accessToken, refreshToken, err := auth.GenerateTokenPair(cfg, user.ID, user.Email, user.Role)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
return
}

c.JSON(http.StatusCreated, gin.H{
"user": gin.H{
"id":    user.ID,
"email": user.Email,
"name":  user.Name,
"role":  user.Role,
},
"access_token":  accessToken,
"refresh_token": refreshToken,
"token_type":    "Bearer",
"expires_in":    int(cfg.JWTAccessExpiry.Seconds()),
})
}

// LoginHandler – аутентификация пользователя
func LoginHandler(c *gin.Context) {
var req LoginRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

user, err := models.FindUserByEmail(req.Email)
if err != nil {
c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
return
}

if !models.CheckPasswordHash(req.Password, user.Password) {
c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
return
}

accessToken, refreshToken, err := auth.GenerateTokenPair(cfg, user.ID, user.Email, user.Role)
if err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate tokens"})
return
}

c.JSON(http.StatusOK, gin.H{
"user": gin.H{
"id":    user.ID,
"email": user.Email,
"name":  user.Name,
"role":  user.Role,
},
"access_token":  accessToken,
"refresh_token": refreshToken,
"token_type":    "Bearer",
"expires_in":    int(cfg.JWTAccessExpiry.Seconds()),
})
}

// RefreshHandler – обновление токенов
func RefreshHandler(c *gin.Context) {
var req RefreshRequest
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
return
}

newAccess, newRefresh, err := auth.RefreshTokens(cfg, req.RefreshToken)
if err != nil {
c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
return
}

c.JSON(http.StatusOK, gin.H{
"access_token":  newAccess,
"refresh_token": newRefresh,
"token_type":    "Bearer",
"expires_in":    int(cfg.JWTAccessExpiry.Seconds()),
})
}

