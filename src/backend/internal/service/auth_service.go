package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
	"unicode"

	"ascp-backend/internal/config"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/jwt"
	"ascp-backend/internal/pkg/mailer"
	"ascp-backend/internal/pkg/password"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

const (
	maxLoginFailures   = 5
	loginLockDuration  = 15 * time.Minute
)

type AuthService interface {
	SetEmailService(emailSvc EmailService)
	Register(req model.RegisterRequest) (*model.UserInfo, error)
	Login(req model.LoginRequest) (*model.LoginResponse, error)
	LoginWithIP(req model.LoginRequest, clientIP string) (*model.LoginResponse, error)
	Logout(userID uint64) error
	GetProfile(userID uint64) (*model.UserInfo, error)
	UpdateProfile(userID uint64, req model.UpdateProfileRequest) (*model.UserInfo, error)
	BindEmail(userID uint64, req model.BindEmailRequest) (*model.UserInfo, error)
	UpdatePassword(userID uint64, req model.UpdatePasswordRequest) error
	SendResetPasswordCode(ctx context.Context, account, clientIP string) (string, error)
	ResetPasswordByEmail(ctx context.Context, req model.ResetPasswordRequest) error
}

type loginAttemptState struct {
	failures    int
	lockedUntil time.Time
}

type authService struct {
	userRepo      repository.UserRepository
	jwtCfg        config.JWTConfig
	emailSvc      EmailService
	loginMu       sync.Mutex
	loginAttempts map[string]*loginAttemptState
	nowFn         func() time.Time
}

func NewAuthService(userRepo repository.UserRepository, jwtCfg config.JWTConfig) AuthService {
	return &authService{
		userRepo:      userRepo,
		jwtCfg:        jwtCfg,
		loginAttempts: make(map[string]*loginAttemptState),
		nowFn:         time.Now,
	}
}

func (s *authService) SetEmailService(emailSvc EmailService) {
	s.emailSvc = emailSvc
}

func validateRegisterUsername(raw string) (string, error) {
	username := strings.TrimSpace(raw)
	if len(username) < 3 || len(username) > 64 {
		return "", NewBadRequest("用户名长度需在 3 到 64 个字符之间")
	}
	if strings.EqualFold(username, "system") {
		return "", NewBadRequest("该用户名不可使用")
	}
	for _, r := range username {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isUpper && !isDigit && r != '_' {
			return "", NewBadRequest("用户名只能由大小写英文字母、数字和下划线组成")
		}
	}
	return username, nil
}

func (s *authService) Register(req model.RegisterRequest) (*model.UserInfo, error) {
	username, err := validateRegisterUsername(req.Username)
	if err != nil {
		return nil, err
	}
	req.Username = username

	if err := validatePasswordComplexity(req.Password); err != nil {
		return nil, err
	}

	normalizedEmail, err := validateAndCheckDisposableEmail(req.Email)
	if err != nil {
		return nil, err
	}

	if _, err := s.userRepo.FindByUsername(req.Username); err == nil {
		return nil, NewConflict("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if _, err := s.userRepo.FindByEmail(normalizedEmail); err == nil {
		return nil, NewConflict("email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if s.emailSvc == nil {
		return nil, &AppError{HTTPStatus: 503, Code: 50301, Message: "邮箱验证服务未初始化"}
	}
	if err := s.emailSvc.VerifyCode(normalizedEmail, model.EmailPurposeRegister, req.EmailCode); err != nil {
		return nil, err
	}

	hash, err := password.Hash(req.Password)
	if err != nil {
		return nil, err
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	user := &model.SysUser{
		Username:      req.Username,
		PasswordHash:  hash,
		Nickname:      nickname,
		Email:         &normalizedEmail,
		EmailVerified: true,
		Status:        model.UserStatusActive,
		TokenVersion:  1,
	}

	if err := s.userRepo.CreateWithRole(user, model.RoleDeveloper); err != nil {
		return nil, err
	}

	info := model.ToUserInfo(user, []string{model.RoleDeveloper})
	return &info, nil
}

func (s *authService) Login(req model.LoginRequest) (*model.LoginResponse, error) {
	return s.LoginWithIP(req, "")
}

func (s *authService) LoginWithIP(req model.LoginRequest, clientIP string) (*model.LoginResponse, error) {
	if strings.EqualFold(strings.TrimSpace(req.Username), "system") {
		return nil, NewForbidden("系统内置账号禁止登录")
	}
	attemptKey := strings.ToLower(strings.TrimSpace(req.Username)) + "|" + strings.TrimSpace(clientIP)
	if s.isLoginLocked(attemptKey) {
		return nil, NewTooManyRequests("登录失败次数过多，请 15 分钟后再试")
	}

	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.recordLoginFailure(attemptKey)
			return nil, NewUnauthorized("invalid username or password")
		}
		return nil, err
	}

	if user.Status != model.UserStatusActive {
		return nil, NewForbidden("user account is disabled")
	}

	if !password.Verify(req.Password, user.PasswordHash) {
		s.recordLoginFailure(attemptKey)
		return nil, NewUnauthorized("invalid username or password")
	}

	s.clearLoginFailures(attemptKey)

	roles, err := s.userRepo.GetUserRoles(user.ID)
	if err != nil {
		return nil, err
	}

	token, expiresIn, err := jwt.GenerateToken(
		s.jwtCfg.Secret,
		s.jwtCfg.ExpireHours,
		user.ID,
		user.Username,
		roles,
		user.TokenVersion,
	)
	if err != nil {
		return nil, err
	}

	_ = s.userRepo.UpdateLastLogin(user.ID, time.Now())

	info := model.ToUserInfo(user, roles)
	info.Status = 0
	info.CreatedAt = time.Time{}

	return &model.LoginResponse{
		Token:     token,
		ExpiresIn: expiresIn,
		User:      info,
	}, nil
}

func (s *authService) Logout(userID uint64) error {
	return s.userRepo.IncrementTokenVersion(userID)
}

func (s *authService) GetProfile(userID uint64) (*model.UserInfo, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	roles, err := s.userRepo.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}
	info := model.ToUserInfo(user, roles)
	return &info, nil
}

func (s *authService) UpdateProfile(userID uint64, req model.UpdateProfileRequest) (*model.UserInfo, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = user.Nickname
	}
	avatar := user.Avatar
	if req.Avatar != nil {
		avatar = *req.Avatar
	}
	bio := user.Bio
	if req.Bio != nil {
		bio = *req.Bio
	}
	favoritesPublic := user.FavoritesPublic
	if req.FavoritesPublic != nil {
		favoritesPublic = *req.FavoritesPublic
	}

	if err := s.userRepo.UpdateProfile(userID, nickname, avatar, bio, favoritesPublic); err != nil {
		return nil, err
	}

	return s.GetProfile(userID)
}

func (s *authService) BindEmail(userID uint64, req model.BindEmailRequest) (*model.UserInfo, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	if !password.Verify(req.CurrentPassword, user.PasswordHash) {
		return nil, NewBadRequest("current password is incorrect")
	}

	normalizedEmail, err := validateAndCheckDisposableEmail(req.Email)
	if err != nil {
		return nil, err
	}

	existing, err := s.userRepo.FindByEmail(normalizedEmail)
	if err == nil && existing.ID != userID {
		return nil, NewConflict("email already in use")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if s.emailSvc == nil {
		return nil, &AppError{HTTPStatus: 503, Code: 50301, Message: "邮箱验证服务未初始化"}
	}
	if err := s.emailSvc.VerifyCode(normalizedEmail, model.EmailPurposeBindEmail, req.EmailCode); err != nil {
		return nil, err
	}

	if err := s.userRepo.UpdateEmail(userID, normalizedEmail, true); err != nil {
		return nil, err
	}
	return s.GetProfile(userID)
}

func (s *authService) UpdatePassword(userID uint64, req model.UpdatePasswordRequest) error {
	if err := validatePasswordComplexity(req.NewPassword); err != nil {
		return err
	}

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return err
	}

	if !password.Verify(req.OldPassword, user.PasswordHash) {
		return NewBadRequest("old password is incorrect")
	}

	hash, err := password.Hash(req.NewPassword)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePasswordAndTokenVersion(userID, hash)
}

func (s *authService) SendResetPasswordCode(ctx context.Context, account, clientIP string) (string, error) {
	if s.emailSvc == nil {
		return "", &AppError{HTTPStatus: 503, Code: 50301, Message: "邮箱验证服务未初始化"}
	}
	return s.emailSvc.SendResetPasswordCode(ctx, account, clientIP)
}

func (s *authService) ResetPasswordByEmail(ctx context.Context, req model.ResetPasswordRequest) error {
	if s.emailSvc == nil {
		return &AppError{HTTPStatus: 503, Code: 50301, Message: "邮箱验证服务未初始化"}
	}

	user, boundEmail, err := resolveUserByAccount(s.userRepo, req.Account)
	if err != nil {
		return err
	}

	if err := s.emailSvc.VerifyCode(boundEmail, mailer.PurposeResetPassword, req.EmailCode); err != nil {
		return err
	}

	hash, err := password.Hash("000000")
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePasswordAndTokenVersion(user.ID, hash); err != nil {
		return err
	}

	if err := s.emailSvc.SendPasswordResetNotice(ctx, boundEmail, user.Username); err != nil {
		log.Printf("failed to send password reset notice email to %s (user=%s): %v", boundEmail, user.Username, err)
	}
	return nil
}

func validatePasswordComplexity(pw string) error {
	if len(pw) < 8 || len(pw) > 64 {
		return NewBadRequest("密码长度需在 8 到 64 位之间")
	}
	var hasLetter, hasDigit bool
	for _, r := range pw {
		if unicode.IsLetter(r) {
			hasLetter = true
		} else if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return NewBadRequest("密码必须同时包含字母和数字")
	}
	return nil
}

func (s *authService) isLoginLocked(key string) bool {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	state, ok := s.loginAttempts[key]
	if !ok {
		return false
	}
	now := s.nowFn()
	if !state.lockedUntil.IsZero() && now.Before(state.lockedUntil) {
		return true
	}
	if !state.lockedUntil.IsZero() && !now.Before(state.lockedUntil) {
		delete(s.loginAttempts, key)
	}
	return false
}

func (s *authService) recordLoginFailure(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	now := s.nowFn()
	state, ok := s.loginAttempts[key]
	if !ok || (!state.lockedUntil.IsZero() && !now.Before(state.lockedUntil)) {
		state = &loginAttemptState{}
		s.loginAttempts[key] = state
	}
	state.failures++
	if state.failures >= maxLoginFailures {
		state.lockedUntil = now.Add(loginLockDuration)
	}
}

func (s *authService) clearLoginFailures(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	delete(s.loginAttempts, key)
}
