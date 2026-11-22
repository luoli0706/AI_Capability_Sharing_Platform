package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type DomainService interface {
	ListDomains(userID uint64, q model.DomainQuery) ([]model.DomainListItem, int64, error)
	CreateDomain(userID uint64, req model.CreateDomainRequest) (*model.DomainListItem, error)
	UpdateDomain(operatorID uint64, domainID uint64, req model.UpdateDomainRequest) (*model.DomainListItem, error)
	DeleteDomain(operatorID uint64, domainID uint64) error
	ListMembers(domainID uint64) ([]model.DomainMemberItem, error)
	AddMember(operatorID uint64, domainID uint64, req model.AddDomainMemberRequest) error
	UpdateMemberRole(operatorID uint64, domainID, targetUserID uint64, req model.UpdateDomainMemberRoleRequest) error
	RemoveMember(operatorID uint64, domainID, targetUserID uint64) error
	CanManageDomain(domainID, userID uint64) (bool, *model.BizDomain, string, error)
	SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository)
	SetCrossRefRepo(crossRefRepo repository.CrossRefRepository)
}

type domainService struct {
	domainRepo   repository.DomainRepository
	userRepo     repository.UserRepository
	orgRepo      repository.OrgRepository
	crossRefRepo repository.CrossRefRepository
	auditRepo    repository.AuditLogRepository
}

func NewDomainService(domainRepo repository.DomainRepository, userRepo repository.UserRepository, orgRepo repository.OrgRepository) DomainService {
	return &domainService{
		domainRepo: domainRepo,
		userRepo:   userRepo,
		orgRepo:    orgRepo,
	}
}

func (s *domainService) SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository) {
	s.auditRepo = auditRepo
	if userRepo != nil {
		s.userRepo = userRepo
	}
}

func (s *domainService) SetCrossRefRepo(crossRefRepo repository.CrossRefRepository) {
	s.crossRefRepo = crossRefRepo
}

func (s *domainService) ListDomains(userID uint64, q model.DomainQuery) ([]model.DomainListItem, int64, error) {
	var userOrgIDs, adminOrgIDs []uint64
	if s.orgRepo != nil {
		var err error
		userOrgIDs, err = s.orgRepo.GetUserOrgIDs(userID)
		if err != nil {
			return nil, 0, err
		}
		adminOrgIDs, err = s.orgRepo.GetUserAdminOrgIDs(userID)
		if err != nil {
			return nil, 0, err
		}
	}
	return s.domainRepo.ListDomains(userID, q, userOrgIDs, adminOrgIDs)
}

func (s *domainService) CreateDomain(userID uint64, req model.CreateDomainRequest) (*model.DomainListItem, error) {
	if _, err := s.domainRepo.FindByCode(req.DomainCode); err == nil {
		return nil, NewConflict("domain code already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = model.DomainVisibilityPublic
	}

	orgID := req.OrgID
	orgName := "个人"
	if visibility == model.DomainVisibilityOrgPrivate && orgID == 0 {
		return nil, NewForbidden("仅组织创建者或组织管理员可创建组织私有域")
	}

	if orgID > 0 {
		if s.orgRepo == nil {
			return nil, NewBadRequest("organization module not configured")
		}
		org, err := s.orgRepo.FindByID(orgID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("organization not found")
			}
			return nil, err
		}
		orgRole, err := s.orgRepo.GetMemberRole(orgID, userID)
		if err != nil {
			return nil, err
		}
		if visibility == model.DomainVisibilityOrgPrivate {
			if org.OwnerID != userID && orgRole != model.OrgRoleOwner && orgRole != model.OrgRoleAdmin {
				return nil, NewForbidden("仅组织创建者或组织管理员可创建组织私有域")
			}
		} else if orgRole == "" && org.OwnerID != userID {
			return nil, NewForbidden("you must be a member of the organization to create a domain under it")
		}
		orgName = org.OrgName
	}

	uid, err := generateDomainUID()
	if err != nil {
		return nil, err
	}

	domain := &model.BizDomain{
		DomainUID:   uid,
		DomainCode:  req.DomainCode,
		DomainName:  req.DomainName,
		Description: req.Description,
		Visibility:  visibility,
		OrgID:       orgID,
		OrgName:     orgName,
		OwnerID:     userID,
		Status:      1,
	}

	if err := s.domainRepo.CreateWithOwner(domain); err != nil {
		return nil, err
	}

	recordAudit(s.auditRepo, s.userRepo, userID, domain.ID, domain.DomainName, domain.ID, domain.DomainCode, domain.DomainName, "domain", model.AuditActionDomainCreate, domain.DomainName, fmt.Sprintf("创建域 %s (%s)", domain.DomainName, domain.Visibility))

	return &model.DomainListItem{
		ID:           domain.ID,
		DomainUID:    domain.DomainUID,
		DomainCode:   domain.DomainCode,
		DomainName:   domain.DomainName,
		Description:  domain.Description,
		Visibility:   domain.Visibility,
		OrgID:        domain.OrgID,
		OrgName:      domain.OrgName,
		OwnerID:      domain.OwnerID,
		Status:       domain.Status,
		MyRole:       model.DomainRoleOwner,
		CanManage:    true,
		HasOrgAccess: domain.Visibility == model.DomainVisibilityPublic && domain.OrgID > 0,
		MemberCount:  1,
		CreatedAt:    domain.CreatedAt,
	}, nil
}

func (s *domainService) UpdateDomain(operatorID uint64, domainID uint64, req model.UpdateDomainRequest) (*model.DomainListItem, error) {
	allowed, domain, myRole, err := s.CanManageDomain(domainID, operatorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, NewForbidden("no permission to manage this domain")
	}

	if req.Visibility == model.DomainVisibilityOrgPrivate {
		if domain.OrgID == 0 || !s.isOrgAdminOrOwner(domain.OrgID, operatorID) {
			return nil, NewForbidden("仅组织创建者或组织管理员可设置组织私有域")
		}
	}

	if req.DomainName != "" {
		domain.DomainName = req.DomainName
	}
	if req.Description != nil {
		domain.Description = *req.Description
	}
	if req.Visibility != "" {
		domain.Visibility = req.Visibility
	}

	if err := s.domainRepo.UpdateDomain(domain); err != nil {
		return nil, err
	}

	recordAudit(s.auditRepo, s.userRepo, operatorID, domain.ID, domain.DomainName, domain.ID, domain.DomainCode, domain.DomainName, "domain", model.AuditActionDomainUpdate, domain.DomainName, "更新域信息 "+domain.DomainName)

	members, _ := s.domainRepo.ListMembers(domainID)
	return &model.DomainListItem{
		ID:          domain.ID,
		DomainUID:   domain.DomainUID,
		DomainCode:  domain.DomainCode,
		DomainName:  domain.DomainName,
		Description: domain.Description,
		Visibility:  domain.Visibility,
		OrgID:       domain.OrgID,
		OrgName:     domain.OrgName,
		OwnerID:     domain.OwnerID,
		Status:      domain.Status,
		MyRole:      myRole,
		CanManage:   true,
		MemberCount: int64(len(members)),
		CreatedAt:   domain.CreatedAt,
	}, nil
}

func (s *domainService) DeleteDomain(operatorID uint64, domainID uint64) error {
	allowed, domain, _, err := s.CanManageDomain(domainID, operatorID)
	if err != nil {
		return err
	}
	if !allowed {
		return NewForbidden("no permission to delete this domain")
	}

	if err := s.domainRepo.DeleteDomain(domainID); err != nil {
		return err
	}
	if s.crossRefRepo != nil {
		_ = s.crossRefRepo.DeleteByDomainID(domainID)
	}
	recordAudit(s.auditRepo, s.userRepo, operatorID, domain.ID, domain.DomainName, domain.ID, domain.DomainCode, domain.DomainName, "domain", model.AuditActionDomainDelete, domain.DomainName, "删除域 "+domain.DomainName)
	return nil
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

func (s *domainService) AddMember(operatorID uint64, domainID uint64, req model.AddDomainMemberRequest) error {
	allowed, domain, _, err := s.CanManageDomain(domainID, operatorID)
	if err != nil {
		return err
	}
	if !allowed {
		return NewForbidden("no permission to manage domain members")
	}

	if req.DomainRole == model.DomainRoleOwner {
		return NewForbidden("cannot assign owner role")
	}
	if domain.Visibility != model.DomainVisibilityOrgPrivate && req.DomainRole == model.DomainRoleAdmin && domain.OwnerID != operatorID {
		return NewForbidden("only domain creator can assign domain admin")
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

	if targetUser.ID == domain.OwnerID {
		return NewForbidden("cannot modify domain creator role")
	}

	if domain.Visibility == model.DomainVisibilityOrgPrivate && domain.OrgID > 0 && s.orgRepo != nil {
		orgRole, err := s.orgRepo.GetMemberRole(domain.OrgID, targetUser.ID)
		if err != nil {
			return err
		}
		if orgRole == "" {
			return NewBadRequest("组织私有域只能添加当前组织内的成员")
		}
	}

	existingRole, err := s.domainRepo.GetMemberRole(domainID, targetUser.ID)
	if err != nil {
		return err
	}
	if domain.Visibility != model.DomainVisibilityOrgPrivate && existingRole == model.DomainRoleAdmin && domain.OwnerID != operatorID {
		return NewForbidden("only domain creator can modify domain admin")
	}

	if err := s.domainRepo.UpsertMember(&model.DomainMember{
		DomainID:   domainID,
		UserID:     targetUser.ID,
		DomainRole: req.DomainRole,
	}); err != nil {
		return err
	}

	recordAudit(s.auditRepo, s.userRepo, operatorID, domain.ID, domain.DomainName, domain.ID, domain.DomainCode, domain.DomainName, "domain", model.AuditActionDomainMemberAdd, targetUser.Username, fmt.Sprintf("添加域成员 %s (%s)", targetUser.Username, req.DomainRole))
	return nil
}

func (s *domainService) UpdateMemberRole(operatorID uint64, domainID, targetUserID uint64, req model.UpdateDomainMemberRoleRequest) error {
	allowed, domain, _, err := s.CanManageDomain(domainID, operatorID)
	if err != nil {
		return err
	}
	if !allowed {
		return NewForbidden("no permission to update domain member role")
	}
	if domain.Visibility != model.DomainVisibilityOrgPrivate && domain.OwnerID != operatorID {
		return NewForbidden("only domain creator can update member roles")
	}
	if targetUserID == domain.OwnerID {
		return NewForbidden("cannot demote domain creator")
	}

	existingRole, err := s.domainRepo.GetMemberRole(domainID, targetUserID)
	if err != nil {
		return err
	}
	if existingRole == "" {
		return NewNotFound("domain member not found")
	}

	return s.domainRepo.UpsertMember(&model.DomainMember{
		DomainID:   domainID,
		UserID:     targetUserID,
		DomainRole: req.DomainRole,
	})
}

func (s *domainService) RemoveMember(operatorID uint64, domainID, targetUserID uint64) error {
	allowed, domain, _, err := s.CanManageDomain(domainID, operatorID)
	if err != nil {
		return err
	}
	if !allowed {
		return NewForbidden("no permission to remove domain members")
	}
	if targetUserID == domain.OwnerID {
		return NewForbidden("cannot remove domain creator")
	}

	targetRole, err := s.domainRepo.GetMemberRole(domainID, targetUserID)
	if err != nil {
		return err
	}
	if targetRole == "" {
		return NewNotFound("domain member not found")
	}
	if domain.Visibility != model.DomainVisibilityOrgPrivate && targetRole == model.DomainRoleAdmin && domain.OwnerID != operatorID {
		return NewForbidden("only domain creator can remove a domain admin")
	}

	err = s.domainRepo.RemoveMember(domainID, targetUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewNotFound("domain member not found")
	}
	if err != nil {
		return err
	}

	targetName := fmt.Sprintf("user#%d", targetUserID)
	if s.userRepo != nil {
		if u, uErr := s.userRepo.FindByID(targetUserID); uErr == nil {
			targetName = u.Username
		}
	}
	recordAudit(s.auditRepo, s.userRepo, operatorID, domain.ID, domain.DomainName, domain.ID, domain.DomainCode, domain.DomainName, "domain", model.AuditActionDomainMemberRemove, targetName, "移除域成员 "+targetName)
	return nil
}

// CanManageDomain enforces strict domain management:
// - org_private: ONLY organization owner or organization admin of domain.OrgID.
// - public / private: ONLY domain creator or assigned domain admin.
func (s *domainService) CanManageDomain(domainID, userID uint64) (bool, *model.BizDomain, string, error) {
	domain, err := s.domainRepo.FindByID(domainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, "", NewNotFound("domain not found")
		}
		return false, nil, "", err
	}
	role, err := s.domainRepo.GetMemberRole(domainID, userID)
	if err != nil {
		return false, nil, "", err
	}

	if domain.Visibility == model.DomainVisibilityOrgPrivate && domain.OrgID > 0 {
		return s.isOrgAdminOrOwner(domain.OrgID, userID), domain, role, nil
	}

	allowed := domain.OwnerID == userID || role == model.DomainRoleOwner || role == model.DomainRoleAdmin
	return allowed, domain, role, nil
}

func (s *domainService) isOrgAdminOrOwner(orgID, userID uint64) bool {
	if s.orgRepo == nil || orgID == 0 {
		return false
	}
	org, err := s.orgRepo.FindByID(orgID)
	if err != nil {
		return false
	}
	if org.OwnerID == userID {
		return true
	}
	orgRole, err := s.orgRepo.GetMemberRole(orgID, userID)
	if err != nil {
		return false
	}
	return orgRole == model.OrgRoleOwner || orgRole == model.OrgRoleAdmin
}

func generateDomainUID() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, 8)
	for i := range buf {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		buf[i] = charset[n.Int64()]
	}
	return "DOM-" + string(buf), nil
}
