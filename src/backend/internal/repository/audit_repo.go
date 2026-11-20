package repository

import (
	"context"
	"strings"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type AuditLogRepository interface {
	Create(ctx context.Context, log *model.AssetAuditLog) error
	ListManagedLogs(ctx context.Context, userID uint64, managedDomainIDs, managedOrgIDs []uint64, q model.AuditLogQuery) ([]model.AssetAuditLog, int64, error)
}

type auditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) Create(ctx context.Context, log *model.AssetAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *auditLogRepository) ListManagedLogs(ctx context.Context, userID uint64, managedDomainIDs, managedOrgIDs []uint64, q model.AuditLogQuery) ([]model.AssetAuditLog, int64, error) {
	tx := r.db.WithContext(ctx).Table("asset_audit_log").
		Joins("LEFT JOIN ai_asset ON ai_asset.id = asset_audit_log.asset_id")

	conds := []string{"asset_audit_log.operator_id = ?", "ai_asset.owner_id = ?"}
	args := []interface{}{userID, userID}
	if len(managedDomainIDs) > 0 {
		conds = append(conds, "asset_audit_log.domain_id IN ?")
		args = append(args, managedDomainIDs)
	}
	if len(managedOrgIDs) > 0 {
		conds = append(conds, "(asset_audit_log.asset_type = 'org' AND asset_audit_log.asset_id IN ?)")
		args = append(args, managedOrgIDs)
	}
	tx = tx.Where("("+strings.Join(conds, " OR ")+")", args...)

	if q.AssetType != "" {
		tx = tx.Where("asset_audit_log.asset_type = ?", q.AssetType)
	}
	if q.Action != "" {
		tx = tx.Where("asset_audit_log.action = ?", q.Action)
	}
	if q.DomainID > 0 {
		tx = tx.Where("asset_audit_log.domain_id = ?", q.DomainID)
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		tx = tx.Where("(asset_audit_log.asset_name LIKE ? OR asset_audit_log.asset_code LIKE ? OR asset_audit_log.target_name LIKE ? OR asset_audit_log.detail LIKE ?)",
			kw, kw, kw, kw)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var list []model.AssetAuditLog
	offset := (page - 1) * pageSize
	if err := tx.Select("asset_audit_log.*").Order("asset_audit_log.id DESC").Offset(offset).Limit(pageSize).Scan(&list).Error; err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []model.AssetAuditLog{}
	}
	return list, total, nil
}
