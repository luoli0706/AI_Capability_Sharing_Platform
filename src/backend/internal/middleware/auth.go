package middleware

import (
	"strings"

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
)

// AuthMiddleware validates the Bearer JWT, checks user status and token_version,
// and injects user_id, username, and roles into echo.Context.
func AuthMiddleware(jwtSecret string, userRepo repository.UserRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return response.Unauthorized(c, "missing authorization header")
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				return response.Unauthorized(c, "invalid authorization header format")
			}

			claims, err := jwt.ParseToken(jwtSecret, strings.TrimSpace(parts[1]))
			if err != nil {
				return response.Unauthorized(c, "invalid or expired token")
			}

			user, err := userRepo.FindByID(claims.UserID)
			if err != nil || user.Status != model.UserStatusActive || user.TokenVersion != claims.TokenVersion {
				return response.Unauthorized(c, "token is no longer valid")
			}

			roles, err := userRepo.GetUserRoles(user.ID)
			if err != nil {
				roles = claims.Roles
			}

			c.Set(ContextKeyUserID, user.ID)
			c.Set(ContextKeyUsername, user.Username)
			c.Set(ContextKeyRoles, roles)

			return next(c)
		}
	}
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

func HasRole(c echo.Context, role string) bool {
	for _, r := range GetRoles(c) {
		if r == role {
			return true
		}
	}
	return false
}
