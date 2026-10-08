package utils

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestJWTMissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if ValidateJWTConfig() == nil {
		t.Fatal("missing secret must fail startup validation")
	}
	if _, err := GenerateToken(42); err == nil {
		t.Fatal("missing secret must not sign a token")
	}
}

func TestJWTSigningAndVerification(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-key-not-for-deployment")
	token, err := GenerateToken(42)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := ParseToken(token)
	if err != nil || uid != 42 {
		t.Fatalf("round trip: uid=%d err=%v", uid, err)
	}
	t.Setenv("JWT_SECRET", "different-test-only-key")
	if _, err := ParseToken(token); err == nil {
		t.Fatal("wrong signing key accepted")
	}
	t.Setenv("JWT_SECRET", "")
	if _, err := ParseToken(token); err == nil {
		t.Fatal("missing verification key accepted")
	}
}

func TestJWTRejectsInvalidTokens(t *testing.T) {
	secret := "test-only-key-not-for-deployment"
	t.Setenv("JWT_SECRET", secret)
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		expiry time.Time
	}{
		{"expired", jwt.SigningMethodHS256, time.Now().Add(-time.Hour)},
		{"unexpected-algorithm", jwt.SigningMethodHS384, time.Now().Add(time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, CustomClaims{UID: 42, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(tc.expiry)}}).SignedString([]byte(secret))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseToken(token); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
}
