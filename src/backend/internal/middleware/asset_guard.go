package middleware

import (
	"errors"
	"net/http"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

const ContextKeyAssetDetail = "asset_detail"

// AssetAccessChecker validates whether the caller may invoke the target asset.
type AssetAccessChecker interface {
	CheckAccess(userID uint64, isSuperAdmin bool, assetCode string) (*model.AssetDetail, error)
	LogDenied(userID uint64, assetCode, authMode, clientIP, traceID string, err error)
}

// AssetGuardMiddleware enforces published status, domain isolation, and asset_permission on invoke routes.
func AssetGuardMiddleware(checker AssetAccessChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			assetCode := c.Param("assetCode")
			if assetCode == "" {
				return response.BadRequest(c, "missing assetCode")
			}

			userID := GetUserID(c)
			detail, err := checker.CheckAccess(userID, HasRole(c, model.RoleSuperAdmin), assetCode)
			if err != nil {
				traceID := c.Response().Header().Get(echo.HeaderXRequestID)
				checker.LogDenied(userID, assetCode, GetAuthMode(c), c.RealIP(), traceID, err)
				var statusErr interface{ Error() string }
				if errors.As(err, &statusErr) && err.Error() == "asset not found" {
					return response.NotFound(c, err.Error())
				}
				return response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
			}

			c.Set(ContextKeyAssetDetail, detail)
			return next(c)
		}
	}
}
