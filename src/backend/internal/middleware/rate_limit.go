package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"ascp-backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

const CodeTooManyRequests = 42901

type rateWindowEntry struct {
	count   int
	resetAt time.Time
}

// RateLimitByUserOrIP limits requests per authenticated user ID (or client IP when unauthenticated)
// using a fixed time window in memory.
func RateLimitByUserOrIP(limit int, window time.Duration) echo.MiddlewareFunc {
	if limit <= 0 {
		limit = 20
	}
	if window <= 0 {
		window = time.Minute
	}

	var mu sync.Mutex
	entries := make(map[string]*rateWindowEntry)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var key string
			if uid := GetUserID(c); uid > 0 {
				key = fmt.Sprintf("user:%d", uid)
			} else {
				key = "ip:" + c.RealIP()
			}

			now := time.Now()
			mu.Lock()
			if len(entries) > 1024 {
				for k, v := range entries {
					if now.After(v.resetAt) {
						delete(entries, k)
					}
				}
			}
			entry, exists := entries[key]
			if !exists || now.After(entry.resetAt) {
				entry = &rateWindowEntry{
					count:   1,
					resetAt: now.Add(window),
				}
				entries[key] = entry
				mu.Unlock()
				return next(c)
			}

			entry.count++
			allowed := entry.count <= limit
			mu.Unlock()

			if !allowed {
				return response.Error(c, http.StatusTooManyRequests, CodeTooManyRequests, "请求过于频繁，请稍后再试")
			}
			return next(c)
		}
	}
}
