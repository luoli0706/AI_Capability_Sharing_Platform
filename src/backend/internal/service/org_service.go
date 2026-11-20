package service

import (
	"errors"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type OrgService interface {
	ListOrgs(userID uint64) ([]model.OrgListItem, error)
	CreateOrg(userID uint64, req model.CreateOrgRequest) (*model.OrgListItem, error)
	GetOrgDetail(userID uint64, orgID uint64) (*model.OrgDetail, error)
	ListMembers(orgID uint64) ([]model.OrgMemberItem, error)
	AddMember(operatorID uint64, orgID uint64, req model.AddOrgMemberRequest) error
	RemoveMember(operatorID uint64, orgID, targetUserID uint64) error
}

type orgService struct {
	orgRepo  repository.OrgRepository
	userRepo repository.UserRepository
}

func NewOrgService(orgRepo repository.OrgRepository, userRepo repository.UserRepository) OrgService {
	return &orgService{
		orgRepo:  orgRepo,
		userRepo: userRepo,
	}
}

func (s *orgService) ListOrgs(userID uint64) ([]model.OrgListItem, error) {
	return s.orgRepo.ListOrgs(userID)
}

func (s *orgService) CreateOrg(userID uint64, req model.CreateOrgRequest) (*model.OrgListItem, error) {
	if _, err := s.orgRepo.FindByCode(req.OrgCode); err == nil {
		return nil, NewConflict("organization code already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	org := &model.BizOrganization{
		OrgCode:     req.OrgCode,
		OrgName:     req.OrgName,
		Description: req.Description,
		OwnerID:     userID,
		Status:      1,
	}
	if err := s.orgRepo.CreateWithOwner(org); err != nil {
		return nil, err
	}

	return &model.OrgListItem{
		ID:          org.ID,
		OrgCode:     org.OrgCode,
		OrgName:     org.OrgName,
		Description: org.Description,
		OwnerID:     org.OwnerID,
		Status:      org.Status,
		MyRole:      model.OrgRoleOwner,
		MemberCount: 1,
		DomainCount: 0,
		CreatedAt:   org.CreatedAt,
	}, nil
}

func (s *orgService) GetOrgDetail(userID uint64, orgID uint64) (*model.OrgDetail, error) {
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("organization not found")
		}
		return nil, err
	}

	myRole, err := s.orgRepo.GetMemberRole(orgID, userID)
	if err != nil {
		return nil, err
	}
	members, err := s.orgRepo.ListMembers(orgID)
	if err != nil {
		return nil, err
	}
	publicDomains, err := s.orgRepo.ListPublicDomainsByOrg(orgID, userID)
	if err != nil {
		return nil, err
	}

	return &model.OrgDetail{
		OrgListItem: model.OrgListItem{
			ID:          org.ID,
			OrgCode:     org.OrgCode,
			OrgName:     org.OrgName,
			Description: org.Description,
			OwnerID:     org.OwnerID,
			Status:      org.Status,
			MyRole:      myRole,
			MemberCount: int64(len(members)),
			DomainCount: int64(len(publicDomains)),
			CreatedAt:   org.CreatedAt,
		},
		PublicDomains: publicDomains,
	}, nil
}

func (s *orgService) ListMembers(orgID uint64) ([]model.OrgMemberItem, error) {
	if _, err := s.orgRepo.FindByID(orgID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("organization not found")
		}
		return nil, err
	}
	return s.orgRepo.ListMembers(orgID)
}

func (s *orgService) AddMember(operatorID uint64, orgID uint64, req model.AddOrgMemberRequest) error {
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("organization not found")
		}
		return err
	}

	opRole, err := s.orgRepo.GetMemberRole(orgID, operatorID)
	if err != nil {
		return err
	}
	if org.OwnerID != operatorID && opRole != model.OrgRoleOwner && opRole != model.OrgRoleAdmin {
		return NewForbidden("only organization owner or admin can manage members")
	}

	var targetUser *model.SysUser
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

	if targetUser.ID == org.OwnerID {
		return NewForbidden("cannot modify organization owner role")
	}

	return s.orgRepo.UpsertMember(&model.OrganizationMember{
		OrgID:   orgID,
		UserID:  targetUser.ID,
		OrgRole: req.OrgRole,
	})
}

func (s *orgService) RemoveMember(operatorID uint64, orgID, targetUserID uint64) error {
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("organization not found")
		}
		return err
	}

	opRole, err := s.orgRepo.GetMemberRole(orgID, operatorID)
	if err != nil {
		return err
	}
	if org.OwnerID != operatorID && opRole != model.OrgRoleOwner && opRole != model.OrgRoleAdmin {
		return NewForbidden("only organization owner or admin can remove members")
	}
	if targetUserID == org.OwnerID {
		return NewForbidden("cannot remove organization owner")
	}

	err = s.orgRepo.RemoveMember(orgID, targetUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewNotFound("organization member not found")
	}
	return err
}
