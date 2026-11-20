package jwt

import (
	"errors"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
	secret := "test-secret-key"
	token, expiresIn, err := GenerateToken(secret, 24, 42, "alice", []string{"developer"}, 3)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiresIn != 24*3600 {
		t.Fatalf("expected expiresIn %d, got %d", 24*3600, expiresIn)
	}

	claims, err := ParseToken(secret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "alice" || claims.TokenVersion != 3 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "developer" {
		t.Fatalf("unexpected roles: %+v", claims.Roles)
	}
}

func TestParseToken_InvalidSecret(t *testing.T) {
	token, _, err := GenerateToken("secret-1", 1, 1, "bob", []string{"developer"}, 1)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if _, err = ParseToken("secret-2", token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseToken_Expired(t *testing.T) {
	secret := "test-secret-key"
	claims := Claims{
		UserID:       1,
		Username:     "bob",
		Roles:        []string{"developer"},
		TokenVersion: 1,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString failed: %v", err)
	}

	if _, err = ParseToken(secret, signed); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
