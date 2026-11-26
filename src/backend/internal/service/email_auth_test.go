package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/password"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type memAuthUserRepo struct {
	mu     sync.Mutex
	nextID uint64
	users  map[uint64]*model.SysUser
	roles  map[uint64][]string
}

func newMemAuthUserRepo() *memAuthUserRepo {
	return &memAuthUserRepo{
		nextID: 1,
		users:  make(map[uint64]*model.SysUser),
		roles:  make(map[uint64][]string),
	}
}

func (m *memAuthUserRepo) CreateWithRole(user *model.SysUser, roleCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user.ID = m.nextID
	m.nextID++
	cp := *user
	m.users[user.ID] = &cp
	m.roles[user.ID] = []string{roleCode}
	return nil
}

func (m *memAuthUserRepo) FindByID(id uint64) (*model.SysUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *memAuthUserRepo) FindByUsername(username string) (*model.SysUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memAuthUserRepo) FindByEmail(email string) (*model.SysUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email != nil && *u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *memAuthUserRepo) GetUserRoles(userID uint64) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.roles[userID], nil
}

func (m *memAuthUserRepo) UpdateProfile(userID uint64, nickname, avatar, bio string, favoritesPublic bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	u.Nickname = nickname
	u.Avatar = avatar
	u.Bio = bio
	u.FavoritesPublic = favoritesPublic
	return nil
}

func (m *memAuthUserRepo) UpdateEmail(userID uint64, email string, verified bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	cp := email
	u.Email = &cp
	u.EmailVerified = verified
	return nil
}

func (m *memAuthUserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	u.PasswordHash = passwordHash
	u.TokenVersion++
	return nil
}
func (m *memAuthUserRepo) IncrementTokenVersion(userID uint64) error          { return nil }
func (m *memAuthUserRepo) UpdateLastLogin(userID uint64, t time.Time) error { return nil }
func (m *memAuthUserRepo) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	return nil, nil
}
func (m *memAuthUserRepo) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	return nil, nil
}

type memEmailRepo struct {
	mu     sync.Mutex
	nextID uint64
	codes  []*model.EmailVerificationCode
	quotas map[string]int
}

func newMemEmailRepo() *memEmailRepo {
	return &memEmailRepo{
		nextID: 1,
		quotas: make(map[string]int),
	}
}

func (m *memEmailRepo) CheckCooldown(email, purpose, clientIP string, window time.Duration, now time.Time) (bool, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := now.Add(-window)
	var emailLimited, ipLimited bool
	for _, c := range m.codes {
		if c.Purpose != purpose || !c.CreatedAt.After(cutoff) {
			continue
		}
		if c.Email == email {
			emailLimited = true
		}
		if clientIP != "" && c.ClientIP == clientIP {
			ipLimited = true
		}
	}
	return emailLimited, ipLimited, nil
}

func (m *memEmailRepo) TryAcquireDailyQuota(bizDate, purpose string, limit int) (bool, error) {
	ok, _, err := m.TryAcquireDailyQuotaWithGlobal(bizDate, purpose, limit, 0)
	return ok, err
}

func (m *memEmailRepo) TryAcquireDailyQuotaWithGlobal(bizDate, purpose string, purposeLimit, globalLimit int) (bool, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	totalSent := 0
	prefix := bizDate + "|"
	for k, v := range m.quotas {
		if strings.HasPrefix(k, prefix) {
			totalSent += v
		}
	}
	if globalLimit > 0 && totalSent >= globalLimit {
		return false, "global", nil
	}
	key := prefix + purpose
	if purposeLimit > 0 && m.quotas[key] >= purposeLimit {
		return false, "purpose", nil
	}
	m.quotas[key]++
	return true, "", nil
}

func (m *memEmailRepo) ReleaseDailyQuota(bizDate, purpose string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := bizDate + "|" + purpose
	if m.quotas[key] > 0 {
		m.quotas[key]--
	}
	return nil
}

func (m *memEmailRepo) CreateCode(code *model.EmailVerificationCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	code.ID = m.nextID
	m.nextID++
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now()
	}
	cp := *code
	m.codes = append(m.codes, &cp)
	return nil
}

func (m *memEmailRepo) InvalidateCode(id uint64, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.codes {
		if c.ID == id && c.UsedAt == nil {
			t := now
			c.UsedAt = &t
		}
	}
	return nil
}

func (m *memEmailRepo) VerifyAndConsumeCode(email, purpose, plainCode, hmacSecret string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *model.EmailVerificationCode
	for i := len(m.codes) - 1; i >= 0; i-- {
		c := m.codes[i]
		if c.Email == email && c.Purpose == purpose && c.UsedAt == nil && c.ExpiresAt.After(now) {
			latest = c
			break
		}
	}
	if latest == nil {
		return repository.ErrVerificationCodeInvalidOrExpired
	}
	if latest.Attempts >= latest.MaxAttempts {
		t := now
		latest.UsedAt = &t
		return repository.ErrVerificationCodeMaxAttempts
	}
	expected := repository.HashVerificationCode(plainCode, hmacSecret)
	if subtle.ConstantTimeCompare([]byte(latest.CodeHash), []byte(expected)) != 1 {
		latest.Attempts++
		if latest.Attempts >= latest.MaxAttempts {
			t := now
			latest.UsedAt = &t
			return repository.ErrVerificationCodeMaxAttempts
		}
		return repository.ErrVerificationCodeInvalidOrExpired
	}
	t := now
	latest.UsedAt = &t
	return nil
}

type stubMailer struct {
	mu       sync.Mutex
	sent     []struct{ Email, Purpose, Code string }
	notices  []struct{ Email, Username string }
	failNext bool
}

func (s *stubMailer) SendVerificationCode(_ context.Context, toEmail, purpose, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failNext {
		return errors.New("smtp error")
	}
	s.sent = append(s.sent, struct{ Email, Purpose, Code string }{toEmail, purpose, code})
	return nil
}

func (s *stubMailer) SendPasswordResetNotice(_ context.Context, toEmail, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failNext {
		return errors.New("smtp error")
	}
	s.notices = append(s.notices, struct{ Email, Username string }{toEmail, username})
	return nil
}

func TestEmailCooldownAndDisposableAndPlusAlias(t *testing.T) {
	userRepo := newMemAuthUserRepo()
	emailRepo := newMemEmailRepo()
	mailer := &stubMailer{}
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

	svc := NewEmailService(emailRepo, userRepo, mailer, "test-hmac").(*emailService)
	svc.nowFn = func() time.Time { return now }

	// Disposable email blocked
	err := svc.SendPublicRegisterCode(context.Background(), "bad@yopmail.com", "1.2.3.4")
	assertAppErrStatus(t, err, http.StatusBadRequest)

	// '+' alias blocked
	err = svc.SendPublicRegisterCode(context.Background(), "alice+tag@example.com", "1.2.3.4")
	assertAppErrStatus(t, err, http.StatusBadRequest)

	// First send succeeds
	if err := svc.SendPublicRegisterCode(context.Background(), "Alice@Example.com", "1.2.3.4"); err != nil {
		t.Fatalf("expected send success, got %v", err)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].Email != "alice@example.com" {
		t.Fatalf("unexpected sent record: %+v", mailer.sent)
	}

	// Same email within 60s -> 429
	now = now.Add(30 * time.Second)
	err = svc.SendPublicRegisterCode(context.Background(), "alice@example.com", "5.6.7.8")
	assertAppErrStatus(t, err, http.StatusTooManyRequests)

	// Same IP within 60s -> 429
	err = svc.SendPublicRegisterCode(context.Background(), "bob@example.com", "1.2.3.4")
	assertAppErrStatus(t, err, http.StatusTooManyRequests)

	// After 61s -> succeeds
	now = now.Add(31 * time.Second)
	if err := svc.SendPublicRegisterCode(context.Background(), "bob@example.com", "1.2.3.4"); err != nil {
		t.Fatalf("expected send success after cooldown, got %v", err)
	}
}

func TestEmailDailyQuotaClampingAndRollback(t *testing.T) {
	userRepo := newMemAuthUserRepo()
	emailRepo := newMemEmailRepo()
	mailer := &stubMailer{}
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

	svc := NewEmailService(emailRepo, userRepo, mailer, "test-hmac").(*emailService)
	svc.nowFn = func() time.Time { return now }

	// Send failure rolls back quota
	mailer.failNext = true
	err := svc.SendPublicRegisterCode(context.Background(), "fail@example.com", "9.9.9.9")
	assertAppErrStatus(t, err, http.StatusBadGateway)
	bizDate := now.In(svc.loc).Format("2006-01-02")
	if got := emailRepo.quotas[bizDate+"|"+model.EmailPurposeRegister]; got != 0 {
		t.Fatalf("expected quota rolled back to 0, got %d", got)
	}
	mailer.failNext = false

	// Register limit 100
	emailRepo.quotas[bizDate+"|"+model.EmailPurposeRegister] = DailyQuotaRegister
	err = svc.SendPublicRegisterCode(context.Background(), "over@example.com", "2.2.2.2")
	assertAppErrStatus(t, err, http.StatusTooManyRequests)
	if !strings.Contains(err.Error(), "注册名额已耗尽") {
		t.Fatalf("unexpected register quota error message: %v", err)
	}

	// Bind email limit 75
	emailRepo.quotas[bizDate+"|"+model.EmailPurposeBindEmail] = DailyQuotaBindEmail
	err = svc.SendBindEmailCode(context.Background(), 1, "bind@example.com", "3.3.3.3")
	assertAppErrStatus(t, err, http.StatusTooManyRequests)
	if !strings.Contains(err.Error(), "今日修改绑定邮箱名额已耗尽") {
		t.Fatalf("unexpected bind quota error message: %v", err)
	}
}

func TestVerificationCodeMaxAttemptsAndFullFlowsAndLoginLockout(t *testing.T) {
	userRepo := newMemAuthUserRepo()
	emailRepo := newMemEmailRepo()
	mailer := &stubMailer{}
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

	emailSvc := NewEmailService(emailRepo, userRepo, mailer, "test-hmac").(*emailService)
	emailSvc.nowFn = func() time.Time { return now }

	authSvc := NewAuthService(userRepo, config.JWTConfig{Secret: "jwt-secret", ExpireHours: 24}).(*authService)
	authSvc.SetEmailService(emailSvc)
	authSvc.nowFn = func() time.Time { return now }

	// 1. Send code & test 5 wrong attempts invalidate code
	if err := emailSvc.SendPublicRegisterCode(context.Background(), "alice@example.com", "10.0.0.1"); err != nil {
		t.Fatalf("send code failed: %v", err)
	}
	realCode := mailer.sent[0].Code

	for i := 0; i < 5; i++ {
		err := emailSvc.VerifyCode("alice@example.com", model.EmailPurposeRegister, "000000")
		assertAppErrStatus(t, err, http.StatusBadRequest)
	}
	// Now even realCode is rejected
	err := emailSvc.VerifyCode("alice@example.com", model.EmailPurposeRegister, realCode)
	assertAppErrStatus(t, err, http.StatusBadRequest)

	// 2. Advance past cooldown, send new code and register
	now = now.Add(65 * time.Second)
	if err := emailSvc.SendPublicRegisterCode(context.Background(), "alice@example.com", "10.0.0.1"); err != nil {
		t.Fatalf("resend code failed: %v", err)
	}
	realCode2 := mailer.sent[1].Code

	// Weak password rejected
	_, err = authSvc.Register(model.RegisterRequest{
		Username:  "alice",
		Password:  "12345678",
		Email:     "alice@example.com",
		EmailCode: realCode2,
	})
	assertAppErrStatus(t, err, http.StatusBadRequest)

	userInfo, err := authSvc.Register(model.RegisterRequest{
		Username:  "alice",
		Password:  "Alice1234",
		Email:     "alice@example.com",
		EmailCode: realCode2,
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if !userInfo.EmailVerified || userInfo.Email != "alice@example.com" {
		t.Fatalf("expected verified email, got %+v", userInfo)
	}

	// 3. Bind new email flow
	if err := emailSvc.SendBindEmailCode(context.Background(), userInfo.ID, "alice.new@example.com", "10.0.0.1"); err != nil {
		t.Fatalf("send bind code failed: %v", err)
	}
	bindCode := mailer.sent[2].Code

	// Wrong current password -> 400
	_, err = authSvc.BindEmail(userInfo.ID, model.BindEmailRequest{
		CurrentPassword: "WrongPassword1",
		Email:           "alice.new@example.com",
		EmailCode:       bindCode,
	})
	assertAppErrStatus(t, err, http.StatusBadRequest)

	updated, err := authSvc.BindEmail(userInfo.ID, model.BindEmailRequest{
		CurrentPassword: "Alice1234",
		Email:           "alice.new@example.com",
		EmailCode:       bindCode,
	})
	if err != nil {
		t.Fatalf("bind email failed: %v", err)
	}
	if updated.Email != "alice.new@example.com" || !updated.EmailVerified {
		t.Fatalf("expected updated verified email, got %+v", updated)
	}

	// 4. Login 5 failures lockout for 15 minutes
	for i := 0; i < 5; i++ {
		_, err := authSvc.LoginWithIP(model.LoginRequest{Username: "alice", Password: "wrong"}, "10.0.0.1")
		assertAppErrStatus(t, err, http.StatusUnauthorized)
	}
	// 6th attempt even with right password is locked (429)
	_, err = authSvc.LoginWithIP(model.LoginRequest{Username: "alice", Password: "Alice1234"}, "10.0.0.1")
	assertAppErrStatus(t, err, http.StatusTooManyRequests)

	// After 15 minutes + 1s, login succeeds and clears failures
	now = now.Add(15*time.Minute + time.Second)
	res, err := authSvc.LoginWithIP(model.LoginRequest{Username: "alice", Password: "Alice1234"}, "10.0.0.1")
	if err != nil || res.Token == "" {
		t.Fatalf("expected login success after lockout expired, got err=%v", err)
	}

	// 5. Legacy user without email can still log in normally
	hash, _ := password.Hash("Legacy123")
	_ = userRepo.CreateWithRole(&model.SysUser{
		Username:      "legacy_user",
		PasswordHash:  hash,
		Nickname:      "Legacy",
		Email:         nil,
		EmailVerified: false,
		Status:        model.UserStatusActive,
		TokenVersion:  1,
	}, model.RoleDeveloper)
	legacyRes, err := authSvc.LoginWithIP(model.LoginRequest{Username: "legacy_user", Password: "Legacy123"}, "10.0.0.2")
	if err != nil || legacyRes.Token == "" || legacyRes.User.Email != "" || legacyRes.User.EmailVerified {
		t.Fatalf("expected legacy user login success, got res=%+v err=%v", legacyRes, err)
	}
}

func TestResetPasswordFlowAndGlobalQuota200(t *testing.T) {
	userRepo := newMemAuthUserRepo()
	emailRepo := newMemEmailRepo()
	mailer := &stubMailer{}
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

	emailSvc := NewEmailService(emailRepo, userRepo, mailer, "test-hmac").(*emailService)
	emailSvc.nowFn = func() time.Time { return now }

	authSvc := NewAuthService(userRepo, config.JWTConfig{Secret: "jwt-secret", ExpireHours: 24}).(*authService)
	authSvc.SetEmailService(emailSvc)
	authSvc.nowFn = func() time.Time { return now }

	// 1. User without bound email -> returns bad request telling user to contact admin, and does not deduct quota
	hashNoEmail, _ := password.Hash("NoEmail123")
	_ = userRepo.CreateWithRole(&model.SysUser{
		Username:     "unbound_user",
		PasswordHash: hashNoEmail,
		Nickname:     "Unbound",
		Email:        nil,
		Status:       model.UserStatusActive,
		TokenVersion: 1,
	}, model.RoleDeveloper)

	_, err := authSvc.SendResetPasswordCode(context.Background(), "unbound_user", "10.0.0.10")
	assertAppErrStatus(t, err, http.StatusBadRequest)
	if !strings.Contains(err.Error(), "联系网站管理员") {
		t.Fatalf("expected contact admin message, got: %v", err)
	}
	bizDate := now.In(emailSvc.loc).Format("2006-01-02")
	if emailRepo.quotas[bizDate+"|"+model.EmailPurposeResetPassword] != 0 {
		t.Fatalf("expected 0 quota deducted for unbound user, got %d", emailRepo.quotas[bizDate+"|"+model.EmailPurposeResetPassword])
	}

	// 2. User with bound email -> send reset code, verify masked email, reset to 000000, old token version invalidated, login with 000000, notice sent
	boundEmail := "luna@gmail.com"
	hashBound, _ := password.Hash("OldPass123")
	_ = userRepo.CreateWithRole(&model.SysUser{
		Username:      "luna",
		PasswordHash:  hashBound,
		Nickname:      "Luna",
		Email:         &boundEmail,
		EmailVerified: true,
		Status:        model.UserStatusActive,
		TokenVersion:  1,
	}, model.RoleDeveloper)

	oldLogin, err := authSvc.LoginWithIP(model.LoginRequest{Username: "luna", Password: "OldPass123"}, "10.0.0.11")
	if err != nil || oldLogin.Token == "" {
		t.Fatalf("initial login failed: %v", err)
	}

	masked, err := authSvc.SendResetPasswordCode(context.Background(), "luna", "10.0.0.11")
	if err != nil {
		t.Fatalf("send reset code failed: %v", err)
	}
	if masked != "lu***@gmail.com" {
		t.Fatalf("expected masked email lu***@gmail.com, got %q", masked)
	}
	if len(mailer.sent) != 1 || mailer.sent[0].Purpose != model.EmailPurposeResetPassword {
		t.Fatalf("unexpected sent code record: %+v", mailer.sent)
	}

	if err := authSvc.ResetPasswordByEmail(context.Background(), model.ResetPasswordRequest{
		Account:   "luna",
		EmailCode: mailer.sent[0].Code,
	}); err != nil {
		t.Fatalf("reset password failed: %v", err)
	}

	// TokenVersion incremented (old token invalidated)
	uAfter, _ := userRepo.FindByUsername("luna")
	if uAfter.TokenVersion != 2 {
		t.Fatalf("expected TokenVersion 2 after reset, got %d", uAfter.TokenVersion)
	}

	// Notice email sent
	if len(mailer.notices) != 1 || mailer.notices[0].Email != boundEmail || mailer.notices[0].Username != "luna" {
		t.Fatalf("expected reset notice email sent, got %+v", mailer.notices)
	}

	// Old password fails, "000000" succeeds
	if _, err := authSvc.LoginWithIP(model.LoginRequest{Username: "luna", Password: "OldPass123"}, "10.0.0.12"); err == nil {
		t.Fatalf("expected old password to fail after reset")
	}
	newLogin, err := authSvc.LoginWithIP(model.LoginRequest{Username: "luna", Password: "000000"}, "10.0.0.13")
	if err != nil || newLogin.Token == "" {
		t.Fatalf("expected login with 000000 to succeed, got err=%v", err)
	}

	// 3. Global 200 cap: register (100) + bind (75) + reset (23) + 2 already sent = 200 -> next send blocked
	emailRepo.quotas[bizDate+"|"+model.EmailPurposeRegister] = 100
	emailRepo.quotas[bizDate+"|"+model.EmailPurposeBindEmail] = 75
	emailRepo.quotas[bizDate+"|"+model.EmailPurposeResetPassword] = 25 // total = 200

	now = now.Add(65 * time.Second)
	_, err = authSvc.SendResetPasswordCode(context.Background(), "luna@gmail.com", "10.0.0.14")
	assertAppErrStatus(t, err, http.StatusTooManyRequests)
	if !strings.Contains(err.Error(), "今日邮件发送额度已耗尽") {
		t.Fatalf("expected global quota exhausted message, got: %v", err)
	}
}
