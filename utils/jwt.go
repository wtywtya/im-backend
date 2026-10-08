package utils

import (
	"errors"
	"github.com/golang-jwt/jwt/v4"
	"os"
	"strings"
	"time"
)

func jwtSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("未设置 JWT_SECRET，请先配置环境变量，详见 README.md")
	}
	return []byte(secret), nil
}

func ValidateJWTConfig() error {
	_, err := jwtSecret()
	return err
}

type CustomClaims struct {
	UID uint `json:"uid"`
	jwt.RegisteredClaims
}

func GenerateToken(uid uint) (string, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", err
	}
	claims := CustomClaims{UID: uid, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), IssuedAt: jwt.NewNumericDate(time.Now())}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}
func ParseToken(tokenStr string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(t *jwt.Token) (interface{}, error) { return jwtSecret() }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return 0, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims.UID, nil
	}
	return 0, errors.New("invalid token")
}
