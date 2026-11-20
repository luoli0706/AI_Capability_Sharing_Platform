package repository

import (
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type GovernanceRepository interface {
	GetActiveUserPermission(assetID, userID uint64) (*model.AssetPermission, error)
	GetUserPermittedAssetIDs(userID uint64) ([]uint64, error)
	GetUserSubscribedAssetIDs(userID uint64) ([]uint64, error)
	UpsertPermission(perm *model.AssetPermission) error
	CreateApprovalWithRecords(req *model.ApprovalRequest, records []model.ApprovalRecord) error
	FindApprovalByID(id uint64) (*model.ApprovalRequest, error)
	ListApprovalRecords(requestID uint64) ([]model.ApprovalRecordView, error)
	ListMyRequests(applicantID uint64) ([]model.ApprovalRequestView, error)
	ListPendingRequests(approverID uint64, isSuperAdmin bool, adminDomainIDs []uint64) ([]model.ApprovalRequestView, error)
	ListHandledRequests(approverID uint64) ([]model.ApprovalRequestView, error)
	HandleApprovalStep(req *model.ApprovalRequest, recordID uint64, action, comment string, grantPerm *model.AssetPermission) error
	HandlePublishApproval(req *model.ApprovalRequest, recordID, approverID uint64, action, comment string) error
}

type governanceRepository struct {
	db *gorm.DB
}

func NewGovernanceRepository(db *gorm.DB) GovernanceRepository {
	return &governanceRepository{db: db}
}

func (r *governanceRepository) GetActiveUserPermission(assetID, userID uint64) (*model.AssetPermission, error) {
	var perm model.AssetPermission
	now := time.Now()
	err := r.db.Where("asset_id = ? AND grantee_type = ? AND grantee_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)",
		assetID, model.GranteeTypeUser, userID, model.PermStatusActive, now).
		First(&perm).Error
	if err != nil {
		return nil, err
	}
	return &perm, nil
}

func (r *governanceRepository) GetUserPermittedAssetIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	now := time.Now()
	err := r.db.Model(&model.AssetPermission{}).
		Where("grantee_type = ? AND grantee_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)",
			model.GranteeTypeUser, userID, model.PermStatusActive, now).
		Pluck("asset_id", &ids).Error
	return ids, err
}

func (r *governanceRepository) GetUserSubscribedAssetIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	now := time.Now()
	err := r.db.Model(&model.AssetPermission{}).
		Where("grantee_type = ? AND grantee_id = ? AND source = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)",
			model.GranteeTypeUser, userID, model.PermSourceSubscribe, model.PermStatusActive, now).
		Pluck("asset_id", &ids).Error
	return ids, err
}

func (r *governanceRepository) UpsertPermission(perm *model.AssetPermission) error {
	return upsertPermissionTx(r.db, perm)
}

func upsertPermissionTx(tx *gorm.DB, perm *model.AssetPermission) error {
	var existing model.AssetPermission
	err := tx.Where("asset_id = ? AND grantee_type = ? AND grantee_id = ?",
		perm.AssetID, perm.GranteeType, perm.GranteeID).First(&existing).Error
	if err == nil {
		return tx.Model(&model.AssetPermission{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"perm_level": perm.PermLevel,
			"source":     perm.Source,
			"expires_at": perm.ExpiresAt,
			"status":     model.PermStatusActive,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return tx.Create(perm).Error
}

func (r *governanceRepository) CreateApprovalWithRecords(req *model.ApprovalRequest, records []model.ApprovalRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(req).Error; err != nil {
			return err
		}
		for i := range records {
			records[i].RequestID = req.ID
		}
		return tx.Create(&records).Error
	})
}

func (r *governanceRepository) FindApprovalByID(id uint64) (*model.ApprovalRequest, error) {
	var req model.ApprovalRequest
	if err := r.db.First(&req, id).Error; err != nil {
		return nil, err
	}
	return &req, nil
}

func (r *governanceRepository) ListApprovalRecords(requestID uint64) ([]model.ApprovalRecordView, error) {
	var records []model.ApprovalRecordView
	err := r.db.Table("approval_record").
		Select("approval_record.*, COALESCE(NULLIF(sys_user.nickname, ''), sys_user.username, '') AS approver_name").
		Joins("LEFT JOIN sys_user ON sys_user.id = approval_record.approver_id AND sys_user.deleted_at IS NULL").
		Where("approval_record.request_id = ?", requestID).
		Order("approval_record.step_no ASC").
		Scan(&records).Error
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []model.ApprovalRecordView{}
	}
	return records, nil
}

func (r *governanceRepository) ListMyRequests(applicantID uint64) ([]model.ApprovalRequestView, error) {
	var reqs []model.ApprovalRequest
	if err := r.db.Where("applicant_id = ?", applicantID).Order("id DESC").Find(&reqs).Error; err != nil {
		return nil, err
	}
	return r.hydrateApprovalViews(reqs)
}

func (r *governanceRepository) ListPendingRequests(approverID uint64, isSuperAdmin bool, adminDomainIDs []uint64) ([]model.ApprovalRequestView, error) {
	var reqs []model.ApprovalRequest
	tx := r.db.Table("approval_request").
		Select("approval_request.*").
		Joins("INNER JOIN approval_record ON approval_record.request_id = approval_request.id AND approval_record.step_no = approval_request.current_step").
		Joins("LEFT JOIN ai_asset ON ai_asset.id = approval_request.asset_id").
		Where("approval_request.status = ? AND approval_record.action = ?", model.ApprovalStatusPending, model.ApprovalActionPending)

	var accessCond string
	var args []interface{}
	if isSuperAdmin {
		accessCond = "approval_request.target_type <> ?"
		args = append(args, model.ApprovalTargetPublish)
	} else {
		accessCond = "(approval_request.target_type <> ? AND approval_record.approver_id = ?)"
		args = append(args, model.ApprovalTargetPublish, approverID)
	}

	if len(adminDomainIDs) > 0 {
		tx = tx.Where("("+accessCond+" OR (approval_request.target_type = ? AND ai_asset.domain_id IN ?))",
			append(args, model.ApprovalTargetPublish, adminDomainIDs)...)
	} else {
		tx = tx.Where(accessCond, args...)
	}

	if err := tx.Order("approval_request.id DESC").Find(&reqs).Error; err != nil {
		return nil, err
	}
	return r.hydrateApprovalViews(reqs)
}

func (r *governanceRepository) ListHandledRequests(approverID uint64) ([]model.ApprovalRequestView, error) {
	var reqs []model.ApprovalRequest
	err := r.db.Table("approval_request").
		Select("DISTINCT approval_request.*").
		Joins("INNER JOIN approval_record ON approval_record.request_id = approval_request.id").
		Where("approval_record.approver_id = ? AND approval_record.action IN ?", approverID, []string{model.ApprovalActionApprove, model.ApprovalActionReject}).
		Order("approval_request.id DESC").
		Find(&reqs).Error
	if err != nil {
		return nil, err
	}
	return r.hydrateApprovalViews(reqs)
}

func (r *governanceRepository) hydrateApprovalViews(reqs []model.ApprovalRequest) ([]model.ApprovalRequestView, error) {
	if len(reqs) == 0 {
		return []model.ApprovalRequestView{}, nil
	}

	reqIDs := make([]uint64, 0, len(reqs))
	assetIDs := make([]uint64, 0, len(reqs))
	userIDSet := make(map[uint64]struct{})
	for _, req := range reqs {
		reqIDs = append(reqIDs, req.ID)
		assetIDs = append(assetIDs, req.AssetID)
		userIDSet[req.ApplicantID] = struct{}{}
	}

	type assetInfo struct {
		ID         uint64
		Name       string
		AssetType  string
		DomainName string
	}
	var assets []assetInfo
	if err := r.db.Table("ai_asset").
		Select("ai_asset.id, ai_asset.name, ai_asset.asset_type, COALESCE(biz_domain.domain_name, '') AS domain_name").
		Joins("LEFT JOIN biz_domain ON biz_domain.id = ai_asset.domain_id AND biz_domain.deleted_at IS NULL").
		Where("ai_asset.id IN ?", assetIDs).
		Scan(&assets).Error; err != nil {
		return nil, err
	}
	assetMap := make(map[uint64]assetInfo, len(assets))
	for _, a := range assets {
		assetMap[a.ID] = a
	}

	var allRecords []model.ApprovalRecord
	if err := r.db.Where("request_id IN ?", reqIDs).Order("step_no ASC").Find(&allRecords).Error; err != nil {
		return nil, err
	}
	for _, rec := range allRecords {
		userIDSet[rec.ApproverID] = struct{}{}
	}

	userIDs := make([]uint64, 0, len(userIDSet))
	for uid := range userIDSet {
		userIDs = append(userIDs, uid)
	}
	type userRow struct {
		ID       uint64
		Username string
		Nickname string
	}
	var users []userRow
	if len(userIDs) > 0 {
		if err := r.db.Table("sys_user").Select("id, username, nickname").Where("id IN ?", userIDs).Scan(&users).Error; err != nil {
			return nil, err
		}
	}
	userMap := make(map[uint64]string, len(users))
	for _, u := range users {
		name := u.Nickname
		if name == "" {
			name = u.Username
		}
		userMap[u.ID] = name
	}

	recMap := make(map[uint64][]model.ApprovalRecordView)
	for _, rec := range allRecords {
		recMap[rec.RequestID] = append(recMap[rec.RequestID], model.ApprovalRecordView{
			ApprovalRecord: rec,
			ApproverName:   userMap[rec.ApproverID],
		})
	}

	views := make([]model.ApprovalRequestView, 0, len(reqs))
	for _, req := range reqs {
		a := assetMap[req.AssetID]
		recs := recMap[req.ID]
		if recs == nil {
			recs = []model.ApprovalRecordView{}
		}
		views = append(views, model.ApprovalRequestView{
			ApprovalRequest: req,
			ApplicantName:   userMap[req.ApplicantID],
			AssetName:       a.Name,
			AssetType:       a.AssetType,
			DomainName:      a.DomainName,
			Records:         recs,
		})
	}
	return views, nil
}

func (r *governanceRepository) HandleApprovalStep(req *model.ApprovalRequest, recordID uint64, action, comment string, grantPerm *model.AssetPermission) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.ApprovalRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{
			"action":     action,
			"comment":    comment,
			"handled_at": now,
		}).Error; err != nil {
			return err
		}

		reqUpdates := map[string]interface{}{}
		if action == model.ApprovalActionReject {
			reqUpdates["status"] = model.ApprovalStatusRejected
		} else if req.CurrentStep < req.TotalSteps {
			reqUpdates["current_step"] = req.CurrentStep + 1
		} else {
			reqUpdates["status"] = model.ApprovalStatusApproved
		}

		if err := tx.Model(&model.ApprovalRequest{}).Where("id = ?", req.ID).Updates(reqUpdates).Error; err != nil {
			return err
		}

		if grantPerm != nil {
			return upsertPermissionTx(tx, grantPerm)
		}
		return nil
	})
}

func (r *governanceRepository) HandlePublishApproval(req *model.ApprovalRequest, recordID, approverID uint64, action, comment string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.ApprovalRecord{}).Where("id = ?", recordID).Updates(map[string]interface{}{
			"approver_id": approverID,
			"action":      action,
			"comment":     comment,
			"handled_at":  now,
		}).Error; err != nil {
			return err
		}

		if action == model.ApprovalActionReject {
			if err := tx.Model(&model.ApprovalRequest{}).Where("id = ?", req.ID).
				Update("status", model.ApprovalStatusRejected).Error; err != nil {
				return err
			}
			return tx.Model(&model.AIAsset{}).Where("id = ?", req.AssetID).
				Update("status", model.AssetStatusDraft).Error
		}

		if err := tx.Model(&model.ApprovalRequest{}).Where("id = ?", req.ID).
			Update("status", model.ApprovalStatusApproved).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.AIAssetVersion{}).Where("asset_id = ?", req.AssetID).
			Update("is_active", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.AIAssetVersion{}).
			Where("asset_id = ? AND version = ?", req.AssetID, req.RequestedPerm).
			Update("is_active", 1).Error; err != nil {
			return err
		}
		return tx.Model(&model.AIAsset{}).Where("id = ?", req.AssetID).Updates(map[string]interface{}{
			"status":          model.AssetStatusPublished,
			"current_version": req.RequestedPerm,
		}).Error
	})
}
