package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestRateLimitByUserOrIP(t *testing.T) {
	e := echo.New()
	mw := RateLimitByUserOrIP(2, time.Minute)
	handler := mw(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	doReq := func(userID uint64, ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tools/video-link/resolve", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if userID > 0 {
			c.Set(ContextKeyUserID, userID)
		}
		if err := handler(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return rec.Code
	}

	// User 10: 2 allowed, 3rd rate-limited
	if code := doReq(10, "1.2.3.4"); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if code := doReq(10, "1.2.3.4"); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if code := doReq(10, "1.2.3.4"); code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", code)
	}

	// Different user (20) on same IP still allowed
	if code := doReq(20, "1.2.3.4"); code != http.StatusOK {
		t.Fatalf("expected 200 for different user, got %d", code)
	}
}
