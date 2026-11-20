package service

import (
	"errors"
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

	if _, err := s.userRepo.FindByEmail(req.Email); err == nil {
		return nil, NewConflict("email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
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
		Username:     req.Username,
		PasswordHash: hash,
		Nickname:     nickname,
		Email:        req.Email,
		Status:       model.UserStatusActive,
		TokenVersion: 1,
	}

	if err := s.userRepo.CreateWithRole(user, model.RoleDeveloper); err != nil {
		return nil, err
	}

	return &model.UserInfo{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Status:    user.Status,
		Roles:     []string{model.RoleDeveloper},
		CreatedAt: user.CreatedAt,
	}, nil
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

	return &model.LoginResponse{
		Token:     token,
		ExpiresIn: expiresIn,
		User: model.UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Nickname: user.Nickname,
			Email:    user.Email,
			Avatar:   user.Avatar,
			Roles:    roles,
		},
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
	return &model.UserInfo{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Status:    user.Status,
		Roles:     roles,
		CreatedAt: user.CreatedAt,
	}, nil
}

func (s *authService) UpdateProfile(userID uint64, req model.UpdateProfileRequest) (*model.UserInfo, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}

	if req.Email != "" && req.Email != user.Email {
		existing, err := s.userRepo.FindByEmail(req.Email)
		if err == nil && existing.ID != userID {
			return nil, NewConflict("email already in use")
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = user.Nickname
	}
	email := req.Email
	if email == "" {
		email = user.Email
	}

	if err := s.userRepo.UpdateProfile(userID, nickname, email, req.Avatar); err != nil {
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
