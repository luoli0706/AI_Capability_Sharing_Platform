package service

import (
	"errors"
	"strings"
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/jwt"
	"ascp-backend/internal/pkg/password"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type AuthService interface {
	Register(req model.RegisterRequest) (*model.UserInfo, error)
	Login(req model.LoginRequest) (*model.LoginResponse, error)
	Logout(userID uint64) error
	GetProfile(userID uint64) (*model.UserInfo, error)
	UpdateProfile(userID uint64, req model.UpdateProfileRequest) (*model.UserInfo, error)
	UpdatePassword(userID uint64, req model.UpdatePasswordRequest) error
}

type authService struct {
	userRepo repository.UserRepository
	jwtCfg   config.JWTConfig
}

func NewAuthService(userRepo repository.UserRepository, jwtCfg config.JWTConfig) AuthService {
	return &authService{
		userRepo: userRepo,
		jwtCfg:   jwtCfg,
	}
}

func (s *authService) Register(req model.RegisterRequest) (*model.UserInfo, error) {
	if _, err := s.userRepo.FindByUsername(req.Username); err == nil {
		return nil, NewConflict("username already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var emailPtr *string
	trimmedEmail := strings.TrimSpace(req.Email)
	if trimmedEmail != "" {
		if _, err := s.userRepo.FindByEmail(trimmedEmail); err == nil {
			return nil, NewConflict("email already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		emailPtr = &trimmedEmail
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
		Username:     req.Username,
		PasswordHash: hash,
		Nickname:     nickname,
		Email:        emailPtr,
		Status:       model.UserStatusActive,
		TokenVersion: 1,
	}

	if err := s.userRepo.CreateWithRole(user, model.RoleDeveloper); err != nil {
		return nil, err
	}

	info := model.ToUserInfo(user, []string{model.RoleDeveloper})
	return &info, nil
}

func (s *authService) Login(req model.LoginRequest) (*model.LoginResponse, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewUnauthorized("invalid username or password")
		}
		return nil, err
	}

	if user.Status != model.UserStatusActive {
		return nil, NewForbidden("user account is disabled")
	}

	if !password.Verify(req.Password, user.PasswordHash) {
		return nil, NewUnauthorized("invalid username or password")
	}

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

	emailPtr := user.Email
	trimmedEmail := strings.TrimSpace(req.Email)
	if trimmedEmail != "" {
		currentEmail := ""
		if user.Email != nil {
			currentEmail = *user.Email
		}
		if trimmedEmail != currentEmail {
			existing, err := s.userRepo.FindByEmail(trimmedEmail)
			if err == nil && existing.ID != userID {
				return nil, NewConflict("email already in use")
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
		emailPtr = &trimmedEmail
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

	if err := s.userRepo.UpdateProfile(userID, nickname, emailPtr, avatar, bio); err != nil {
		return nil, err
	}

	return s.GetProfile(userID)
}

func (s *authService) UpdatePassword(userID uint64, req model.UpdatePasswordRequest) error {
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
