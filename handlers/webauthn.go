package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/go-webauthn/webauthn/webauthn"
    "your-project/config"
    "your-project/models"
    "log"
)

var webAuthn *webauthn.WebAuthn

func InitWebAuthn() {
    var err error
    webAuthn, err = webauthn.New(&webauthn.Config{
        RPDisplayName: "Your Service",
        RPID:          config.App.Domain, // например, localhost или ваш домен
        RPOrigins:     []string{config.App.BaseURL},
    })
    if err != nil {
        log.Fatal("Failed to init WebAuthn:", err)
    }
}

// WebAuthnUser — адаптер вашей модели User к интерфейсу webauthn.User
type WebAuthnUser struct {
    ID          []byte
    Name        string
    DisplayName string
    Credentials []webauthn.Credential
}

func (u *WebAuthnUser) WebAuthnID() []byte                { return u.ID }
func (u *WebAuthnUser) WebAuthnName() string              { return u.Name }
func (u *WebAuthnUser) WebAuthnDisplayName() string       { return u.DisplayName }
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

// BeginRegistration — начало регистрации ключа
func WebAuthnRegisterBegin(c *gin.Context) {
    if !config.App.Features.WebAuthnEnabled {
        c.JSON(http.StatusForbidden, gin.H{"error": "WebAuthn disabled"})
        return
    }
    user := GetCurrentUser(c) // ваша функция получения текущего пользователя
    if user == nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    webauthnUser := &WebAuthnUser{
        ID:          []byte(user.ID.String()),
        Name:        user.Email,
        DisplayName: user.Name,
        // загрузите существующие credentials из БД
        Credentials: []webauthn.Credential{}, // реализуйте загрузку
    }

    options, sessionData, err := webAuthn.BeginRegistration(webauthnUser)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // сохраните sessionData в сессии (например, в Redis или в cookie)
    // используйте любой механизм: cookie, JWT, context
    c.SetCookie("webauthn_registration", sessionData.RequestID, 300, "/", config.App.Domain, true, true)

    c.JSON(http.StatusOK, options)
}

// CompleteRegistration — завершение регистрации после ответа клиента
func WebAuthnRegisterComplete(c *gin.Context) {
    // аналогично: получите sessionData, user, вызовите webAuthn.FinishRegistration
    // сохраните новый credential в БД
}

// BeginLogin, CompleteLogin — аналогично