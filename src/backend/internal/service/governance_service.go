package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type GovernanceService interface {
	SubscribeAsset(userID, assetID uint64) (*model.AssetPermission, error)
	CreateApproval(userID uint64, req model.CreateApprovalRequest) (*model.ApprovalRequest, error)
	RequestJoinOrg(userID, orgID uint64, reason string) (*model.ApprovalRequest, error)
	ListMyRequests(userID uint64) ([]model.ApprovalRequestView, error)
	ListPendingRequests(userID uint64, isSuperAdmin bool) ([]model.ApprovalRequestView, error)
	ListHandledRequests(userID uint64) ([]model.ApprovalRequestView, error)
	HandleApproval(userID uint64, isSuperAdmin bool, requestID uint64, req model.HandleApprovalRequest) (*model.ApprovalRequest, error)
	SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository)
	SetCrossRefRepo(crossRefRepo repository.CrossRefRepository)
}

type governanceService struct {
	govRepo      repository.GovernanceRepository
	assetRepo    repository.AssetRepository
	domainRepo   repository.DomainRepository
	orgRepo      repository.OrgRepository
	crossRefRepo repository.CrossRefRepository
	auditRepo    repository.AuditLogRepository
	userRepo     repository.UserRepository
}

func (s *governanceService) SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository) {
	s.auditRepo = auditRepo
	s.userRepo = userRepo
}

func (s *governanceService) SetCrossRefRepo(crossRefRepo repository.CrossRefRepository) {
	s.crossRefRepo = crossRefRepo
}

func NewGovernanceService(
	govRepo repository.GovernanceRepository,
	assetRepo repository.AssetRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
) GovernanceService {
	return &governanceService{
		govRepo:    govRepo,
		assetRepo:  assetRepo,
		domainRepo: domainRepo,
		orgRepo:    orgRepo,
	}
}

func generateRequestNo() string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	suffix := 0
	if err == nil {
		suffix = int(n.Int64())
	}
	return fmt.Sprintf("APR-%s-%04d", time.Now().Format("20060102150405"), suffix)
}

func (s *governanceService) SubscribeAsset(userID, assetID uint64) (*model.AssetPermission, error) {
	asset, err := s.assetRepo.FindByID(assetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("asset not found")
		}
		return nil, err
	}

	if asset.Status != model.AssetStatusPublished {
		return nil, NewForbidden("需发起权限申请")
	}

	allowed := asset.Visibility == model.VisibilityPublic || asset.OwnerID == userID
	if !allowed && asset.Visibility == model.VisibilityDomain {
		allowed, err = hasDomainAccess(s.domainRepo, s.orgRepo, asset.DomainID, userID)
		if err != nil {
			return nil, err
		}
	}
	if !allowed {
		return nil, NewForbidden("需发起权限申请")
	}

	perm := &model.AssetPermission{
		AssetID:     assetID,
		GranteeType: model.GranteeTypeUser,
		GranteeID:   userID,
		PermLevel:   model.PermLevelInvoke,
		Source:      model.PermSourceSubscribe,
		Status:      model.PermStatusActive,
	}
	if err := s.govRepo.UpsertPermission(perm); err != nil {
		return nil, err
	}
	return s.govRepo.GetActiveUserPermission(assetID, userID)
}

func (s *governanceService) CreateApproval(userID uint64, req model.CreateApprovalRequest) (*model.ApprovalRequest, error) {
	asset, err := s.assetRepo.FindByID(req.AssetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("asset not found")
		}
		return nil, err
	}
	domain, err := s.domainRepo.FindByID(asset.DomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("domain not found")
		}
		return nil, err
	}

	targetType := req.TargetType
	if targetType == "" {
		targetType = model.ApprovalTargetAccess
	}
	requestedPerm := req.RequestedPerm
	if requestedPerm == "" {
		requestedPerm = model.PermLevelInvoke
	}
	durationDays := req.DurationDays
	if durationDays <= 0 {
		durationDays = 30
	}

	var applicantDomainID uint64
	if domainIDs, err := s.domainRepo.GetUserDomainIDs(userID); err == nil && len(domainIDs) > 0 {
		applicantDomainID = domainIDs[0]
	}

	requestNo := generateRequestNo()
	approvalReq := &model.ApprovalRequest{
		RequestNo:         requestNo,
		ApplicantID:       userID,
		ApplicantDomainID: applicantDomainID,
		TargetType:        targetType,
		AssetID:           asset.ID,
		RequestedPerm:     requestedPerm,
		DurationDays:      durationDays,
		Reason:            req.Reason,
		CurrentStep:       1,
		TotalSteps:        2,
		Status:            model.ApprovalStatusPending,
	}

	records := []model.ApprovalRecord{
		{
			StepNo:     1,
			StepName:   "资产负责人审批",
			ApproverID: asset.OwnerID,
			Action:     model.ApprovalActionPending,
		},
		{
			StepNo:     2,
			StepName:   "域管理员审批",
			ApproverID: domain.OwnerID,
			Action:     model.ApprovalActionPending,
		},
	}

	if err := s.govRepo.CreateApprovalWithRecords(approvalReq, records); err != nil {
		return nil, err
	}
	return approvalReq, nil
}

func (s *governanceService) RequestJoinOrg(userID, orgID uint64, reason string) (*model.ApprovalRequest, error) {
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
	if org.Visibility == model.OrgVisibilityPrivate {
		return nil, NewForbidden("私密组织仅支持管理员邀请加入")
	}

	orgRole, err := s.orgRepo.GetMemberRole(orgID, userID)
	if err != nil {
		return nil, err
	}
	if orgRole != "" || org.OwnerID == userID {
		return nil, NewConflict("您已是该组织成员")
	}

	pending, err := s.govRepo.HasPendingOrgJoin(orgID, userID)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, NewConflict("您已提交过加入申请，请等待审批")
	}

	if reason == "" {
		reason = "申请加入组织 " + org.OrgName
	}
	requestNo := generateRequestNo()
	approvalReq := &model.ApprovalRequest{
		RequestNo:     requestNo,
		ApplicantID:   userID,
		TargetType:    model.ApprovalTargetOrgJoin,
		AssetID:       org.ID,
		RequestedPerm: model.OrgRoleMember,
		DurationDays:  0,
		Reason:        reason,
		CurrentStep:   1,
		TotalSteps:    1,
		Status:        model.ApprovalStatusPending,
	}
	records := []model.ApprovalRecord{
		{
			StepNo:     1,
			StepName:   "组织所有者/管理员审批",
			ApproverID: org.OwnerID,
			Action:     model.ApprovalActionPending,
		},
	}
	if err := s.govRepo.CreateApprovalWithRecords(approvalReq, records); err != nil {
		return nil, err
	}
	return approvalReq, nil
}

func (s *governanceService) ListMyRequests(userID uint64) ([]model.ApprovalRequestView, error) {
	return s.govRepo.ListMyRequests(userID)
}

func (s *governanceService) ListPendingRequests(userID uint64, isSuperAdmin bool) ([]model.ApprovalRequestView, error) {
	adminDomainIDs, err := s.domainRepo.GetUserAdminDomainIDs(userID)
	if err != nil {
		return nil, err
	}
	var adminOrgIDs []uint64
	if s.orgRepo != nil {
		adminOrgIDs, err = s.orgRepo.GetUserAdminOrgIDs(userID)
		if err != nil {
			return nil, err
		}
	}
	return s.govRepo.ListPendingRequests(userID, isSuperAdmin, adminDomainIDs, adminOrgIDs)
}

func (s *governanceService) ListHandledRequests(userID uint64) ([]model.ApprovalRequestView, error) {
	return s.govRepo.ListHandledRequests(userID)
}

func (s *governanceService) HandleApproval(userID uint64, isSuperAdmin bool, requestID uint64, req model.HandleApprovalRequest) (*model.ApprovalRequest, error) {
	approvalReq, err := s.govRepo.FindApprovalByID(requestID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("approval request not found")
		}
		return nil, err
	}
	if approvalReq.Status != model.ApprovalStatusPending {
		return nil, NewBadRequest("approval request is already closed")
	}

	records, err := s.govRepo.ListApprovalRecords(requestID)
	if err != nil {
		return nil, err
	}

	var currentRecord *model.ApprovalRecordView
	for i := range records {
		if records[i].StepNo == approvalReq.CurrentStep {
			currentRecord = &records[i]
			break
		}
	}
	if currentRecord == nil {
		return nil, NewBadRequest("current approval step not found")
	}

	if approvalReq.TargetType == model.ApprovalTargetPublish {
		asset, err := s.assetRepo.FindByID(approvalReq.AssetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("asset not found")
			}
			return nil, err
		}
		domain, err := s.domainRepo.FindByID(asset.DomainID)
		if err != nil {
			return nil, err
		}
		role, err := s.domainRepo.GetMemberRole(asset.DomainID, userID)
		if err != nil {
			return nil, err
		}
		if domain.OwnerID != userID && role != model.DomainRoleOwner && role != model.DomainRoleAdmin {
			return nil, NewForbidden("only domain owner or domain admin can handle publish approval")
		}
		if err := s.govRepo.HandlePublishApproval(approvalReq, currentRecord.ID, userID, req.Action, req.Comment); err != nil {
			return nil, err
		}
		actionName := model.AuditActionAssetPublishApprove
		if req.Action == model.ApprovalActionReject {
			actionName = model.AuditActionAssetPublishReject
		}
		recordAudit(s.auditRepo, s.userRepo, userID, domain.ID, domain.DomainName, asset.ID, asset.AssetCode, asset.Name, asset.AssetType, actionName, approvalReq.RequestedPerm, req.Comment)
		return s.govRepo.FindApprovalByID(requestID)
	}

	if approvalReq.TargetType == model.ApprovalTargetOrgJoin {
		if s.orgRepo == nil {
			return nil, NewBadRequest("organization module not configured")
		}
		org, err := s.orgRepo.FindByID(approvalReq.AssetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("organization not found")
			}
			return nil, err
		}
		orgRole, err := s.orgRepo.GetMemberRole(org.ID, userID)
		if err != nil {
			return nil, err
		}
		if org.OwnerID != userID && orgRole != model.OrgRoleOwner && orgRole != model.OrgRoleAdmin {
			return nil, NewForbidden("only organization owner or admin can handle join approval")
		}
		if err := s.govRepo.HandleOrgJoinApproval(approvalReq, currentRecord.ID, userID, req.Action, req.Comment); err != nil {
			return nil, err
		}
		if req.Action == model.ApprovalActionApprove {
			applicantName := fmt.Sprintf("user#%d", approvalReq.ApplicantID)
			if s.userRepo != nil {
				if u, uErr := s.userRepo.FindByID(approvalReq.ApplicantID); uErr == nil {
					applicantName = u.Username
				}
			}
			recordAudit(s.auditRepo, s.userRepo, userID, 0, "", org.ID, org.OrgCode, org.OrgName, "org", model.AuditActionOrgMemberAdd, applicantName, "审批通过加入组织 "+applicantName)
		}
		return s.govRepo.FindApprovalByID(requestID)
	}

	if approvalReq.TargetType == model.ApprovalTargetCrossRef {
		if s.crossRefRepo == nil {
			return nil, NewBadRequest("cross reference module not configured")
		}
		crossRef, err := s.crossRefRepo.FindByID(approvalReq.AssetID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("cross-domain reference not found")
			}
			return nil, err
		}
		targetDomain, err := s.domainRepo.FindByID(crossRef.TargetDomainID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("target domain not found")
			}
			return nil, err
		}
		if !isSuperAdmin {
			role, err := s.domainRepo.GetMemberRole(crossRef.TargetDomainID, userID)
			if err != nil {
				return nil, err
			}
			if targetDomain.OwnerID != userID && role != model.DomainRoleOwner && role != model.DomainRoleAdmin {
				return nil, NewForbidden("only target domain owner, admin, or super_admin can handle cross-domain reference approval")
			}
		}
		if err := s.govRepo.HandleCrossRefApproval(approvalReq, currentRecord.ID, userID, req.Action, req.Comment); err != nil {
			return nil, err
		}
		actionName := model.AuditActionCrossRefApprove
		if req.Action == model.ApprovalActionReject {
			actionName = model.AuditActionCrossRefReject
		}
		recordAudit(s.auditRepo, s.userRepo, userID, targetDomain.ID, targetDomain.DomainName, crossRef.SourceAssetID, "", crossRef.Title, crossRef.RefType, actionName, crossRef.Title, req.Comment)
		return s.govRepo.FindApprovalByID(requestID)
	}

	if !isSuperAdmin && currentRecord.ApproverID != userID {
		return nil, NewForbidden("you are not the approver for the current step")
	}

	var grantPerm *model.AssetPermission
	if req.Action == model.ApprovalActionApprove && approvalReq.CurrentStep >= approvalReq.TotalSteps {
		exp := time.Now().Add(time.Duration(approvalReq.DurationDays) * 24 * time.Hour)
		grantPerm = &model.AssetPermission{
			AssetID:     approvalReq.AssetID,
			GranteeType: model.GranteeTypeUser,
			GranteeID:   approvalReq.ApplicantID,
			PermLevel:   approvalReq.RequestedPerm,
			Source:      model.PermSourceApproval,
			ExpiresAt:   &exp,
			Status:      model.PermStatusActive,
		}
	}

	if err := s.govRepo.HandleApprovalStep(approvalReq, currentRecord.ID, req.Action, req.Comment, grantPerm); err != nil {
		return nil, err
	}
	return s.govRepo.FindApprovalByID(requestID)
}
