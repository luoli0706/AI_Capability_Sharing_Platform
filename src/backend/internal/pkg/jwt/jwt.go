package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Claims defines the JWT payload for ASCP authentication.
type Claims struct {
	UserID       uint64   `json:"user_id"`
	Username     string   `json:"username"`
	Roles        []string `json:"roles"`
	TokenVersion int      `json:"token_version"`
	jwtv5.RegisteredClaims
}

// GenerateToken signs a new HS256 JWT token with the given secret and expiration hours.
func GenerateToken(secret string, expireHours int, userID uint64, username string, roles []string, tokenVersion int) (string, int64, error) {
	if expireHours <= 0 {
		expireHours = 24
	}
	now := time.Now()
	expiresAt := now.Add(time.Duration(expireHours) * time.Hour)

	claims := Claims{
		UserID:       userID,
		Username:     username,
		Roles:        roles,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwtv5.RegisteredClaims{
			IssuedAt:  jwtv5.NewNumericDate(now),
			ExpiresAt: jwtv5.NewNumericDate(expiresAt),
		},
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", 0, err
	}
	return signed, int64(expireHours) * 3600, nil
}

// ParseToken validates the JWT signature and expiration, returning Claims if valid.
func ParseToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwtv5.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
