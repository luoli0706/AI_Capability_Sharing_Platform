package service

import (
	"errors"
	"fmt"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type GovernanceService interface {
	SubscribeAsset(userID, assetID uint64) (*model.AssetPermission, error)
	CreateApproval(userID uint64, req model.CreateApprovalRequest) (*model.ApprovalRequest, error)
	ListMyRequests(userID uint64) ([]model.ApprovalRequestView, error)
	ListPendingRequests(userID uint64, isSuperAdmin bool) ([]model.ApprovalRequestView, error)
	ListHandledRequests(userID uint64) ([]model.ApprovalRequestView, error)
	HandleApproval(userID uint64, isSuperAdmin bool, requestID uint64, req model.HandleApprovalRequest) (*model.ApprovalRequest, error)
}

type governanceService struct {
	govRepo    repository.GovernanceRepository
	assetRepo  repository.AssetRepository
	domainRepo repository.DomainRepository
	orgRepo    repository.OrgRepository
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

	requestNo := fmt.Sprintf("APR-%s-%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
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

func (s *governanceService) ListMyRequests(userID uint64) ([]model.ApprovalRequestView, error) {
	return s.govRepo.ListMyRequests(userID)
}

func (s *governanceService) ListPendingRequests(userID uint64, isSuperAdmin bool) ([]model.ApprovalRequestView, error) {
	return s.govRepo.ListPendingRequests(userID, isSuperAdmin)
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
