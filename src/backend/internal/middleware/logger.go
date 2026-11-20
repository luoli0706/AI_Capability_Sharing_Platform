package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// ZapLogger logs HTTP requests with latency, status, request_id, and user_id.
func ZapLogger(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()
			logger.Info("http_request",
				zap.String("request_id", res.Header().Get(echo.HeaderXRequestID)),
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.Int("status", res.Status),
				zap.Duration("latency", time.Since(start)),
				zap.Uint64("user_id", GetUserID(c)),
				zap.String("remote_ip", c.RealIP()),
			)
			return nil
		}
	}
}
