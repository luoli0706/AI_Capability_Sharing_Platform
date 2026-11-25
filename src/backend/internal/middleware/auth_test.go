package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/jwt"
	"ascp-backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type mockUserRepo struct {
	users map[uint64]*model.SysUser
	roles map[uint64][]string
}

func (m *mockUserRepo) CreateWithRole(user *model.SysUser, roleCode string) error { return nil }
func (m *mockUserRepo) FindByID(id uint64) (*model.SysUser, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockUserRepo) FindByUsername(username string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockUserRepo) FindByEmail(email string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockUserRepo) GetUserRoles(userID uint64) ([]string, error) {
	return m.roles[userID], nil
}
func (m *mockUserRepo) UpdateProfile(userID uint64, nickname, avatar, bio string, favoritesPublic bool) error {
	return nil
}
func (m *mockUserRepo) UpdateEmail(userID uint64, email string, verified bool) error {
	return nil
}
func (m *mockUserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	return nil
}
func (m *mockUserRepo) IncrementTokenVersion(userID uint64) error          { return nil }
func (m *mockUserRepo) UpdateLastLogin(userID uint64, t time.Time) error { return nil }
func (m *mockUserRepo) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	return []model.UserBrief{}, nil
}
func (m *mockUserRepo) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	return map[uint64]model.UserBrief{}, nil
}

func TestAuthMiddleware(t *testing.T) {
	secret := "unit-test-secret"
	repo := &mockUserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "alice", Status: model.UserStatusActive, TokenVersion: 2},
			2: {ID: 2, Username: "bob", Status: model.UserStatusDisabled, TokenVersion: 1},
		},
		roles: map[uint64][]string{
			1: {model.RoleDeveloper},
			2: {model.RoleDeveloper},
		},
	}

	e := echo.New()
	mw := AuthMiddleware(secret, repo)
	protectedHandler := mw(func(c echo.Context) error {
		return response.Success(c, map[string]interface{}{
			"user_id":  GetUserID(c),
			"username": GetUsername(c),
			"roles":    GetRoles(c),
		})
	})

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := protectedHandler(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}

		var res response.Result
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if res.Code != response.CodeUnauthorized {
			t.Fatalf("expected code %d, got %d", response.CodeUnauthorized, res.Code)
		}
	})

	t.Run("stale token_version", func(t *testing.T) {
		tok, _, err := jwt.GenerateToken(secret, 1, 1, "alice", []string{model.RoleDeveloper}, 1)
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := protectedHandler(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("disabled user", func(t *testing.T) {
		tok, _, err := jwt.GenerateToken(secret, 1, 2, "bob", []string{model.RoleDeveloper}, 1)
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := protectedHandler(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		tok, _, err := jwt.GenerateToken(secret, 1, 1, "alice", []string{model.RoleDeveloper}, 2)
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		if err := protectedHandler(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var res response.Result
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if res.Code != response.CodeSuccess {
			t.Fatalf("expected code %d, got %d", response.CodeSuccess, res.Code)
		}
	})
}
