package service

import (
	"errors"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type DomainService interface {
	ListDomains(userID uint64) ([]model.DomainListItem, error)
	CreateDomain(userID uint64, req model.CreateDomainRequest) (*model.DomainListItem, error)
	ListMembers(domainID uint64) ([]model.DomainMemberItem, error)
	AddMember(operatorID uint64, isSuperAdmin bool, domainID uint64, req model.AddDomainMemberRequest) error
}

type domainService struct {
	domainRepo repository.DomainRepository
	userRepo   repository.UserRepository
}

func NewDomainService(domainRepo repository.DomainRepository, userRepo repository.UserRepository) DomainService {
	return &domainService{
		domainRepo: domainRepo,
		userRepo:   userRepo,
	}
}

func (s *domainService) ListDomains(userID uint64) ([]model.DomainListItem, error) {
	return s.domainRepo.ListDomains(userID)
}

func (s *domainService) CreateDomain(userID uint64, req model.CreateDomainRequest) (*model.DomainListItem, error) {
	if _, err := s.domainRepo.FindByCode(req.DomainCode); err == nil {
		return nil, NewConflict("domain code already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	domain := &model.BizDomain{
		DomainCode:  req.DomainCode,
		DomainName:  req.DomainName,
		Description: req.Description,
		OwnerID:     userID,
		Status:      1,
	}

	if err := s.domainRepo.CreateWithOwner(domain); err != nil {
		return nil, err
	}

	return &model.DomainListItem{
		ID:          domain.ID,
		DomainCode:  domain.DomainCode,
		DomainName:  domain.DomainName,
		Description: domain.Description,
		OwnerID:     domain.OwnerID,
		Status:      domain.Status,
		MyRole:      model.DomainRoleOwner,
		MemberCount: 1,
		CreatedAt:   domain.CreatedAt,
	}, nil
}

func (s *domainService) ListMembers(domainID uint64) ([]model.DomainMemberItem, error) {
	if _, err := s.domainRepo.FindByID(domainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("domain not found")
		}
		return nil, err
	}
	return s.domainRepo.ListMembers(domainID)
}

func (s *domainService) AddMember(operatorID uint64, isSuperAdmin bool, domainID uint64, req model.AddDomainMemberRequest) error {
	if _, err := s.domainRepo.FindByID(domainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("domain not found")
		}
		return err
	}

	if !isSuperAdmin {
		role, err := s.domainRepo.GetMemberRole(domainID, operatorID)
		if err != nil {
			return err
		}
		if role != model.DomainRoleOwner && role != model.DomainRoleAdmin {
			return NewForbidden("only domain owner, domain admin, or super_admin can manage members")
		}
	}

	var targetUser *model.SysUser
	var err error
	switch {
	case req.UserID > 0:
		targetUser, err = s.userRepo.FindByID(req.UserID)
	case req.Username != "":
		targetUser, err = s.userRepo.FindByUsername(req.Username)
	default:
		return NewBadRequest("username or user_id is required")
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("target user not found")
		}
		return err
	}

	member := &model.DomainMember{
		DomainID:   domainID,
		UserID:     targetUser.ID,
		DomainRole: req.DomainRole,
	}
	return s.domainRepo.UpsertMember(member)
}
