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
	domainRepo      repository.DomainRepository
	orgRepo         repository.OrgRepository
	govRepo         repository.GovernanceRepository
}

func NewUserService(
	userRepo repository.UserRepository,
	interactionRepo repository.KBInteractionRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	govRepo repository.GovernanceRepository,
) UserService {
	return &userService{
		userRepo:        userRepo,
		interactionRepo: interactionRepo,
		domainRepo:      domainRepo,
		orgRepo:         orgRepo,
		govRepo:         govRepo,
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
	filtered, err := s.filterAccessibleFavorites(viewerID, items)
	if err != nil {
		return false, nil, err
	}
	return true, filtered, nil
}

func (s *userService) filterAccessibleFavorites(viewerID uint64, items []model.UserFavoriteItem) ([]model.UserFavoriteItem, error) {
	if len(items) == 0 {
		return items, nil
	}

	roles, err := s.userRepo.GetUserRoles(viewerID)
	if err != nil {
		return nil, err
	}
	for _, r := range roles {
		if r == model.RoleSuperAdmin {
			return items, nil
		}
	}

	var accessibleDomainSet, adminDomainSet, permittedAssetSet map[uint64]bool
	if s.domainRepo != nil {
		accessibleIDs, err := getAccessibleDomainIDs(s.domainRepo, s.orgRepo, viewerID)
		if err != nil {
			return nil, err
		}
		accessibleDomainSet = toUint64Set(accessibleIDs)

		adminIDs, err := s.domainRepo.GetUserAdminDomainIDs(viewerID)
		if err != nil {
			return nil, err
		}
		adminDomainSet = toUint64Set(adminIDs)
	}
	if s.govRepo != nil {
		permittedIDs, err := s.govRepo.GetUserPermittedAssetIDs(viewerID)
		if err != nil {
			return nil, err
		}
		permittedAssetSet = toUint64Set(permittedIDs)
	}

	out := make([]model.UserFavoriteItem, 0, len(items))
	for _, item := range items {
		if canAccessKBAsset(
			viewerID,
			false,
			item.DomainID,
			item.DomainVisibility,
			item.KBAssetID,
			item.AssetOwnerID,
			item.AssetVisibility,
			item.AssetStatus,
			accessibleDomainSet,
			adminDomainSet,
			permittedAssetSet,
		) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *userService) GetMyFavorites(userID uint64) ([]model.UserFavoriteItem, error) {
	return s.interactionRepo.ListUserFavorites(userID)
}
