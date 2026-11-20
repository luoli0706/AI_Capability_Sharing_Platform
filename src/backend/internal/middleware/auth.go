package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/jwt"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/repository"

	"github.com/labstack/echo/v4"
)

const (
	ContextKeyUserID   = "user_id"
	ContextKeyUsername = "username"
	ContextKeyRoles    = "roles"
	ContextKeyAuthMode = "auth_mode"
)

// AuthMiddleware validates the Bearer JWT, checks user status and token_version,
// and injects user_id, username, and roles into echo.Context.
func AuthMiddleware(jwtSecret string, userRepo repository.UserRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if ok := authenticateJWT(c, jwtSecret, userRepo); !ok {
				return response.Unauthorized(c, "unauthorized")
			}
			return next(c)
		}
	}
}

// UnifiedAuthMiddleware accepts either Authorization: Bearer <JWT> or X-API-Key: <key>.
func UnifiedAuthMiddleware(
	jwtSecret string,
	userRepo repository.UserRepository,
	keyRepo repository.APIKeyRepository,
) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Header.Get("Authorization") != "" {
				if authenticateJWT(c, jwtSecret, userRepo) {
					return next(c)
				}
				return response.Unauthorized(c, "unauthorized")
			}

			apiKey := strings.TrimSpace(c.Request().Header.Get("X-API-Key"))
			if apiKey != "" && keyRepo != nil {
				if authenticateAPIKey(c, apiKey, userRepo, keyRepo) {
					return next(c)
				}
				return response.Unauthorized(c, "invalid or expired api key")
			}

			return response.Unauthorized(c, "missing authentication credentials")
		}
	}
}

func authenticateJWT(c echo.Context, jwtSecret string, userRepo repository.UserRepository) bool {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return false
	}

	claims, err := jwt.ParseToken(jwtSecret, strings.TrimSpace(parts[1]))
	if err != nil {
		return false
	}

	user, err := userRepo.FindByID(claims.UserID)
	if err != nil || user.Status != model.UserStatusActive || user.TokenVersion != claims.TokenVersion {
		return false
	}

	roles, err := userRepo.GetUserRoles(user.ID)
	if err != nil {
		roles = claims.Roles
	}

	c.Set(ContextKeyUserID, user.ID)
	c.Set(ContextKeyUsername, user.Username)
	c.Set(ContextKeyRoles, roles)
	c.Set(ContextKeyAuthMode, model.AuthModeJWT)
	return true
}

func authenticateAPIKey(
	c echo.Context,
	plainKey string,
	userRepo repository.UserRepository,
	keyRepo repository.APIKeyRepository,
) bool {
	sum := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(sum[:])

	keyRecord, err := keyRepo.FindActiveByHash(keyHash)
	if err != nil || keyRecord.Status != 1 {
		return false
	}
	if keyRecord.ExpiresAt != nil && keyRecord.ExpiresAt.Before(time.Now()) {
		return false
	}

	user, err := userRepo.FindByID(keyRecord.UserID)
	if err != nil || user.Status != model.UserStatusActive {
		return false
	}

	roles, err := userRepo.GetUserRoles(user.ID)
	if err != nil {
		roles = []string{}
	}

	_ = keyRepo.TouchLastUsed(keyRecord.ID, time.Now())

	c.Set(ContextKeyUserID, user.ID)
	c.Set(ContextKeyUsername, user.Username)
	c.Set(ContextKeyRoles, roles)
	c.Set(ContextKeyAuthMode, model.AuthModeAPIKey)
	return true
}

func GetUserID(c echo.Context) uint64 {
	if v, ok := c.Get(ContextKeyUserID).(uint64); ok {
		return v
	}
	return 0
}

func GetUsername(c echo.Context) string {
	if v, ok := c.Get(ContextKeyUsername).(string); ok {
		return v
	}
	return ""
}

func GetRoles(c echo.Context) []string {
	if v, ok := c.Get(ContextKeyRoles).([]string); ok {
		return v
	}
	return nil
}

func GetAuthMode(c echo.Context) string {
	if v, ok := c.Get(ContextKeyAuthMode).(string); ok {
		return v
	}
	return model.AuthModeJWT
}

func HasRole(c echo.Context, role string) bool {
	for _, r := range GetRoles(c) {
		if r == role {
			return true
		}
	}
	return false
}
