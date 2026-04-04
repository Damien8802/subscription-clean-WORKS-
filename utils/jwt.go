package utils

import (
    "errors"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "subscription-system/config"
)

var cfg = config.Load()

type Claims struct {
    UserID   string `json:"user_id"`
    Email    string `json:"email"`
    Role     string `json:"role"`
    TenantID string `json:"tenant_id"`
    jwt.RegisteredClaims
}

// GenerateTokens создаёт access и refresh токены (стандартные сроки)
func GenerateTokens(userID, role string) (string, string, error) {
    return GenerateTokensWithExpiry(userID, "", role, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)
}

// GenerateTokensWithExpiry создаёт токены с указанными сроками
func GenerateTokensWithExpiry(userID, email, role string, accessExpiry, refreshExpiry time.Duration) (string, string, error) {
    // Access token
    accessClaims := Claims{
        UserID: userID,
        Email:  email,
        Role:   role,
        TenantID: "11111111-1111-1111-1111-111111111111",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    accessString, err := accessToken.SignedString([]byte(cfg.JWTSecret))
    if err != nil {
        return "", "", err
    }

    // Refresh token
    refreshClaims := Claims{
        UserID: userID,
        Email:  email,
        Role:   role,
        TenantID: "11111111-1111-1111-1111-111111111111",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(refreshExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
    refreshString, err := refreshToken.SignedString([]byte(cfg.JWTRefreshSecret))
    if err != nil {
        return "", "", err
    }

    return accessString, refreshString, nil
}

// ValidateToken проверяет токен
func ValidateToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(cfg.JWTSecret), nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, errors.New("invalid token")
}

// RefreshToken обновляет access token
func RefreshToken(refreshToken string) (string, error) {
    token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
        return []byte(cfg.JWTRefreshSecret), nil
    })

    if err != nil || !token.Valid {
        return "", errors.New("invalid refresh token")
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return "", errors.New("invalid claims")
    }

    userID, _ := claims["user_id"].(string)
    email, _ := claims["email"].(string)
    role, _ := claims["role"].(string)

    accessClaims := Claims{
        UserID: userID,
        Email:  email,
        Role:   role,
        TenantID: "11111111-1111-1111-1111-111111111111",
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.JWTAccessExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
    return newToken.SignedString([]byte(cfg.JWTSecret))
}
