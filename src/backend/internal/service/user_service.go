package service

import (
	"errors"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type UserService interface {
	SearchUsers(currentUserID uint64, keyword string) ([]model.UserBrief, error)
	GetPublicProfile(targetUserID uint64) (*model.PublicUserProfile, error)
	GetUserFavorites(viewerID, targetUserID uint64) (isPublic bool, items []model.UserFavoriteItem, err error)
	GetMyFavorites(userID uint64) ([]model.UserFavoriteItem, error)
}

type userService struct {
	userRepo        repository.UserRepository
	interactionRepo repository.KBInteractionRepository
}

func NewUserService(userRepo repository.UserRepository, interactionRepo repository.KBInteractionRepository) UserService {
	return &userService{
		userRepo:        userRepo,
		interactionRepo: interactionRepo,
	}
}

func (s *userService) SearchUsers(currentUserID uint64, keyword string) ([]model.UserBrief, error) {
	return s.userRepo.SearchActiveUsers(keyword, currentUserID, 20)
}

func (s *userService) GetPublicProfile(targetUserID uint64) (*model.PublicUserProfile, error) {
	user, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("user not found")
		}
		return nil, err
	}
	return &model.PublicUserProfile{
		ID:              user.ID,
		Username:        user.Username,
		Nickname:        user.Nickname,
		Avatar:          user.Avatar,
		Bio:             user.Bio,
		FavoritesPublic: user.FavoritesPublic,
		CreatedAt:       user.CreatedAt,
	}, nil
}

func (s *userService) GetUserFavorites(viewerID, targetUserID uint64) (bool, []model.UserFavoriteItem, error) {
	target, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, NewNotFound("user not found")
		}
		return false, nil, err
	}
	if viewerID != targetUserID && !target.FavoritesPublic {
		return false, []model.UserFavoriteItem{}, nil
	}
	items, err := s.interactionRepo.ListUserFavorites(targetUserID)
	if err != nil {
		return false, nil, err
	}
	return true, items, nil
}

func (s *userService) GetMyFavorites(userID uint64) ([]model.UserFavoriteItem, error) {
	return s.interactionRepo.ListUserFavorites(userID)
}
