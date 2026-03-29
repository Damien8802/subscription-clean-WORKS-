package security

import (
    "crypto/rand"
    "encoding/base64"
    "log"
    "sync"
    "time"
    
    "github.com/pquerna/otp/totp"
    "github.com/skip2/go-qrcode"
    "golang.org/x/crypto/bcrypt"
)

type SecurityManager struct {
    mu            sync.RWMutex
    loginAttempts map[string]*LoginAttempt
}

type LoginAttempt struct {
    Count        int
    LastAttempt  time.Time
    BlockedUntil time.Time
}

var security = &SecurityManager{
    loginAttempts: make(map[string]*LoginAttempt),
}

func GenerateQRCode(email, issuer string) (string, string, error) {
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      issuer,
        AccountName: email,
        Period:      30,
        Digits:      6,
    })
    if err != nil {
        return "", "", err
    }
    
    qrBytes, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
    if err != nil {
        return "", "", err
    }
    
    return key.Secret(), base64.StdEncoding.EncodeToString(qrBytes), nil
}

func VerifyTOTP(secret, code string) bool {
    return totp.Validate(code, secret)
}

func GenerateBackupCodes(userID string) []string {
    codes := make([]string, 10)
    chars := []rune("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
    
    for i := 0; i < 10; i++ {
        code := make([]rune, 8)
        for j := range code {
            b := make([]byte, 1)
            rand.Read(b)
            code[j] = chars[int(b[0])%len(chars)]
        }
        codes[i] = string(code)
    }
    return codes
}

func CheckLoginAttempts(ip string) bool {
    security.mu.RLock()
    defer security.mu.RUnlock()
    
    attempt, exists := security.loginAttempts[ip]
    if !exists {
        return true
    }
    return time.Now().After(attempt.BlockedUntil)
}

func RecordFailedAttempt(ip string) {
    security.mu.Lock()
    defer security.mu.Unlock()
    
    attempt, exists := security.loginAttempts[ip]
    if !exists {
        security.loginAttempts[ip] = &LoginAttempt{Count: 1, LastAttempt: time.Now()}
        return
    }
    
    attempt.Count++
    attempt.LastAttempt = time.Now()
    
    if attempt.Count >= 5 {
        attempt.BlockedUntil = time.Now().Add(15 * time.Minute)
        log.Printf("🔒 IP %s заблокирован на 15 минут", ip)
    }
}

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
