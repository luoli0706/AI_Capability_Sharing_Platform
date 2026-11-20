package service

import (
	"context"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"
)

type AuditService interface {
	ListManagedLogs(ctx context.Context, userID uint64, q model.AuditLogQuery) ([]model.AssetAuditLog, int64, error)
}

type auditService struct {
	auditRepo  repository.AuditLogRepository
	domainRepo repository.DomainRepository
	orgRepo    repository.OrgRepository
}

func NewAuditService(auditRepo repository.AuditLogRepository, domainRepo repository.DomainRepository, orgRepo repository.OrgRepository) AuditService {
	return &auditService{
		auditRepo:  auditRepo,
		domainRepo: domainRepo,
		orgRepo:    orgRepo,
	}
}

func (s *auditService) ListManagedLogs(ctx context.Context, userID uint64, q model.AuditLogQuery) ([]model.AssetAuditLog, int64, error) {
	managedDomainIDs, err := s.domainRepo.GetUserAdminDomainIDs(userID)
	if err != nil {
		return nil, 0, err
	}
	var managedOrgIDs []uint64
	if s.orgRepo != nil {
		managedOrgIDs, err = s.orgRepo.GetUserAdminOrgIDs(userID)
		if err != nil {
			return nil, 0, err
		}
	}
	return s.auditRepo.ListManagedLogs(ctx, userID, managedDomainIDs, managedOrgIDs, q)
}

// recordAudit writes an audit log entry if auditRepo is configured (nil-safe for unit tests).
func recordAudit(
	auditRepo repository.AuditLogRepository,
	userRepo repository.UserRepository,
	operatorID uint64,
	domainID uint64,
	domainName string,
	assetID uint64,
	assetCode, assetName, assetType, action, targetName, detail string,
) {
	if auditRepo == nil {
		return
	}

	operatorName := ""
	if userRepo != nil && operatorID > 0 {
		if u, err := userRepo.FindByID(operatorID); err == nil {
			if u.Nickname != "" {
				operatorName = u.Nickname
			} else {
				operatorName = u.Username
			}
		}
	}

	if len(detail) > 500 {
		detail = detail[:500]
	}

	_ = auditRepo.Create(context.Background(), &model.AssetAuditLog{
		OperatorID:   operatorID,
		OperatorName: operatorName,
		DomainID:     domainID,
		DomainName:   domainName,
		AssetID:      assetID,
		AssetCode:    assetCode,
		AssetName:    assetName,
		AssetType:    assetType,
		Action:       action,
		TargetName:   targetName,
		Detail:       detail,
	})
}
