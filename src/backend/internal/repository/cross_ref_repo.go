package repository

import (
	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type CrossRefRepository interface {
	Create(ref *model.DomainCrossRef) error
	UpdateApprovalRequestID(id, approvalRequestID uint64) error
	FindByID(id uint64) (*model.DomainCrossRef, error)
	ExistsPendingOrActive(targetDomainID uint64, refType string, sourceAssetID, sourceDocID uint64) (bool, error)
	ListByTargetDomain(targetDomainID uint64, includePending bool, pendingCreatorID uint64, allPending bool) ([]model.CrossRefView, error)
	DeleteByID(id uint64) error
	DeleteByAssetID(assetID uint64) error
	DeleteByKBDocIDs(docIDs []uint64) error
	DeleteByDomainID(domainID uint64) error
	ListSourceDomains(isSuperAdmin bool, accessibleDomainIDs []uint64, excludeDomainID uint64) ([]model.BizDomain, error)
	ListPublishedAssetsByDomain(domainID uint64) ([]model.AIAsset, error)
	ListKBFilesByAssetIDs(kbAssetIDs []uint64) ([]model.KBDocument, error)
}

type crossRefRepository struct {
	db *gorm.DB
}

func NewCrossRefRepository(db *gorm.DB) CrossRefRepository {
	return &crossRefRepository{db: db}
}

func (r *crossRefRepository) Create(ref *model.DomainCrossRef) error {
	return r.db.Create(ref).Error
}

func (r *crossRefRepository) UpdateApprovalRequestID(id, approvalRequestID uint64) error {
	return r.db.Model(&model.DomainCrossRef{}).
		Where("id = ?", id).
		Update("approval_request_id", approvalRequestID).Error
}

func (r *crossRefRepository) FindByID(id uint64) (*model.DomainCrossRef, error) {
	var ref model.DomainCrossRef
	if err := r.db.First(&ref, id).Error; err != nil {
		return nil, err
	}
	return &ref, nil
}

func (r *crossRefRepository) ExistsPendingOrActive(targetDomainID uint64, refType string, sourceAssetID, sourceDocID uint64) (bool, error) {
	var count int64
	err := r.db.Model(&model.DomainCrossRef{}).
		Where("target_domain_id = ? AND ref_type = ? AND source_asset_id = ? AND source_doc_id = ? AND status IN ?",
			targetDomainID, refType, sourceAssetID, sourceDocID,
			[]string{model.CrossRefStatusPending, model.CrossRefStatusActive}).
		Count(&count).Error
	return count > 0, err
}

func (r *crossRefRepository) ListByTargetDomain(targetDomainID uint64, includePending bool, pendingCreatorID uint64, allPending bool) ([]model.CrossRefView, error) {
	tx := r.db.Model(&model.DomainCrossRef{}).Where("target_domain_id = ?", targetDomainID)
	if !includePending {
		tx = tx.Where("status = ?", model.CrossRefStatusActive)
	} else if allPending {
		tx = tx.Where("status IN ?", []string{model.CrossRefStatusActive, model.CrossRefStatusPending})
	} else {
		tx = tx.Where("(status = ? OR (status = ? AND created_by = ?))",
			model.CrossRefStatusActive, model.CrossRefStatusPending, pendingCreatorID)
	}

	var refs []model.DomainCrossRef
	if err := tx.Order("id DESC").Find(&refs).Error; err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return []model.CrossRefView{}, nil
	}

	domainIDSet := make(map[uint64]struct{})
	userIDSet := make(map[uint64]struct{})
	assetIDSet := make(map[uint64]struct{})
	docIDSet := make(map[uint64]struct{})
	for _, ref := range refs {
		domainIDSet[ref.SourceDomainID] = struct{}{}
		userIDSet[ref.CreatedBy] = struct{}{}
		if ref.SourceAssetID > 0 {
			assetIDSet[ref.SourceAssetID] = struct{}{}
		}
		if ref.RefType == model.CrossRefTypeKBFile && ref.SourceDocID > 0 {
			docIDSet[ref.SourceDocID] = struct{}{}
		}
	}

	domainNames := make(map[uint64]string)
	if len(domainIDSet) > 0 {
		var rows []struct {
			ID         uint64
			DomainName string
		}
		if err := r.db.Table("biz_domain").Select("id, domain_name").
			Where("id IN ?", mapKeysToSlice(domainIDSet)).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			domainNames[row.ID] = row.DomainName
		}
	}

	userNames := make(map[uint64]string)
	if len(userIDSet) > 0 {
		var rows []struct {
			ID       uint64
			Username string
			Nickname string
		}
		if err := r.db.Table("sys_user").Select("id, username, nickname").
			Where("id IN ?", mapKeysToSlice(userIDSet)).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			name := row.Nickname
			if name == "" {
				name = row.Username
			}
			userNames[row.ID] = name
		}
	}

	assetTypes := make(map[uint64]string)
	if len(assetIDSet) > 0 {
		var rows []struct {
			ID        uint64
			AssetType string
		}
		if err := r.db.Table("ai_asset").Select("id, asset_type").
			Where("id IN ?", mapKeysToSlice(assetIDSet)).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			assetTypes[row.ID] = row.AssetType
		}
	}

	type docMeta struct {
		ID       uint64
		FileExt  string
		FileSize uint64
	}
	docMetas := make(map[uint64]docMeta)
	if len(docIDSet) > 0 {
		var rows []docMeta
		if err := r.db.Table("kb_document").Select("id, file_ext, file_size").
			Where("id IN ?", mapKeysToSlice(docIDSet)).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			docMetas[row.ID] = row
		}
	}

	views := make([]model.CrossRefView, 0, len(refs))
	for _, ref := range refs {
		view := model.CrossRefView{
			ID:                ref.ID,
			TargetDomainID:    ref.TargetDomainID,
			SourceDomainID:    ref.SourceDomainID,
			SourceDomainName:  domainNames[ref.SourceDomainID],
			RefType:           ref.RefType,
			SourceAssetID:     ref.SourceAssetID,
			SourceDocID:       ref.SourceDocID,
			Title:             ref.Title,
			Description:       ref.Description,
			CreatedBy:         ref.CreatedBy,
			CreatorName:       userNames[ref.CreatedBy],
			Status:            ref.Status,
			ApprovalRequestID: ref.ApprovalRequestID,
			CreatedAt:         ref.CreatedAt,
			AssetType:         assetTypes[ref.SourceAssetID],
		}
		if ref.RefType == model.CrossRefTypeKBFile {
			if meta, ok := docMetas[ref.SourceDocID]; ok {
				view.FileExt = meta.FileExt
				view.FileSize = meta.FileSize
			}
		}
		views = append(views, view)
	}
	return views, nil
}

func (r *crossRefRepository) DeleteByID(id uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var ref model.DomainCrossRef
		if err := tx.First(&ref, id).Error; err != nil {
			return err
		}
		if ref.ApprovalRequestID > 0 {
			if err := tx.Model(&model.ApprovalRequest{}).
				Where("id = ? AND status = ?", ref.ApprovalRequestID, model.ApprovalStatusPending).
				Update("status", model.ApprovalStatusCancelled).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.DomainCrossRef{}, id).Error
	})
}

func (r *crossRefRepository) DeleteByAssetID(assetID uint64) error {
	return r.deleteWhere(r.db.Where("source_asset_id = ?", assetID))
}

func (r *crossRefRepository) DeleteByKBDocIDs(docIDs []uint64) error {
	if len(docIDs) == 0 {
		return nil
	}
	return r.deleteWhere(r.db.Where("ref_type = ? AND source_doc_id IN ?", model.CrossRefTypeKBFile, docIDs))
}

func (r *crossRefRepository) DeleteByDomainID(domainID uint64) error {
	return r.deleteWhere(r.db.Where("source_domain_id = ? OR target_domain_id = ?", domainID, domainID))
}

func (r *crossRefRepository) deleteWhere(scope *gorm.DB) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var refs []model.DomainCrossRef
		if err := scope.Session(&gorm.Session{NewDB: false}).Find(&refs).Error; err != nil {
			return err
		}
		if len(refs) == 0 {
			return nil
		}
		var reqIDs []uint64
		var refIDs []uint64
		for _, ref := range refs {
			refIDs = append(refIDs, ref.ID)
			if ref.ApprovalRequestID > 0 {
				reqIDs = append(reqIDs, ref.ApprovalRequestID)
			}
		}
		if len(reqIDs) > 0 {
			if err := tx.Model(&model.ApprovalRequest{}).
				Where("id IN ? AND status = ?", reqIDs, model.ApprovalStatusPending).
				Update("status", model.ApprovalStatusCancelled).Error; err != nil {
				return err
			}
		}
		return tx.Where("id IN ?", refIDs).Delete(&model.DomainCrossRef{}).Error
	})
}

func (r *crossRefRepository) ListSourceDomains(isSuperAdmin bool, accessibleDomainIDs []uint64, excludeDomainID uint64) ([]model.BizDomain, error) {
	tx := r.db.Model(&model.BizDomain{})
	if excludeDomainID > 0 {
		tx = tx.Where("id <> ?", excludeDomainID)
	}
	if !isSuperAdmin {
		if len(accessibleDomainIDs) > 0 {
			tx = tx.Where("(visibility = ? OR id IN ?)", model.DomainVisibilityPublic, accessibleDomainIDs)
		} else {
			tx = tx.Where("visibility = ?", model.DomainVisibilityPublic)
		}
	}
	var domains []model.BizDomain
	if err := tx.Order("id DESC").Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

func (r *crossRefRepository) ListPublishedAssetsByDomain(domainID uint64) ([]model.AIAsset, error) {
	var assets []model.AIAsset
	err := r.db.Where("domain_id = ? AND status = ?", domainID, model.AssetStatusPublished).
		Order("id DESC").Find(&assets).Error
	return assets, err
}

func (r *crossRefRepository) ListKBFilesByAssetIDs(kbAssetIDs []uint64) ([]model.KBDocument, error) {
	if len(kbAssetIDs) == 0 {
		return []model.KBDocument{}, nil
	}
	var docs []model.KBDocument
	err := r.db.Where("kb_asset_id IN ? AND is_folder = ?", kbAssetIDs, false).
		Order("id DESC").Find(&docs).Error
	return docs, err
}

func mapKeysToSlice(m map[uint64]struct{}) []uint64 {
	out := make([]uint64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
