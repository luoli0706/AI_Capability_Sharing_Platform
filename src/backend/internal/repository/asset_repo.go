package repository

import (
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssetRepository interface {
	ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs, permittedAssetIDs []uint64) ([]model.AssetListItem, int64, error)
	ListUserAssets(q model.AssetQuery) ([]model.AssetListItem, int64, error)
	FindByID(id uint64) (*model.AIAsset, error)
	FindByCode(code string) (*model.AIAsset, error)
	GetDetailByID(id uint64) (*model.AssetDetail, error)
	GetDetailByCode(code string) (*model.AssetDetail, error)
	CreateAsset(asset *model.AIAsset, ext interface{}) error
	UpdateAsset(asset *model.AIAsset, ext interface{}) error
	DeleteAsset(id uint64) error
	UpdateStatus(id uint64, status string, version string) error
	PublishWithSnapshot(assetID uint64, version, changelog string, snapshot model.JSONRaw, publisherID uint64) error
	ListVersions(assetID uint64) ([]model.AIAssetVersion, error)
	FindVersionByID(assetID, versionID uint64) (*model.AIAssetVersion, error)
	RestoreFromSnapshot(asset *model.AIAsset, ext interface{}, versionID uint64, version string) error
	UpdateMCPHealth(assetID uint64, healthStatus string, checkedAt time.Time) error
	UpdateMCPTools(assetID uint64, toolsSchema model.JSONRaw) error
	UpdateSkillPackage(assetID uint64, packagePath string, packageFiles model.StringSlice, content string) error
	IncrementInvokeCount(assetID uint64) error
}

type assetRepository struct {
	db *gorm.DB
}

func NewAssetRepository(db *gorm.DB) AssetRepository {
	return &assetRepository{db: db}
}

func (r *assetRepository) baseAssetListQuery() *gorm.DB {
	return r.db.Table("ai_asset").
		Select("ai_asset.*, COALESCE(biz_domain.domain_name, '') AS domain_name, COALESCE(NULLIF(sys_user.nickname, ''), sys_user.username, '') AS owner_name").
		Joins("LEFT JOIN biz_domain ON biz_domain.id = ai_asset.domain_id AND biz_domain.deleted_at IS NULL").
		Joins("LEFT JOIN sys_user ON sys_user.id = ai_asset.owner_id AND sys_user.deleted_at IS NULL").
		Where("ai_asset.deleted_at IS NULL")
}

func (r *assetRepository) applyCommonFilters(tx *gorm.DB, q model.AssetQuery) *gorm.DB {
	if q.AssetType != "" {
		tx = tx.Where("ai_asset.asset_type = ?", q.AssetType)
	}
	if q.DomainID > 0 {
		tx = tx.Where("ai_asset.domain_id = ?", q.DomainID)
	}
	if q.Visibility != "" {
		tx = tx.Where("ai_asset.visibility = ?", q.Visibility)
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		tx = tx.Where("(ai_asset.name LIKE ? OR ai_asset.summary LIKE ? OR ai_asset.asset_code LIKE ?)", kw, kw, kw)
	}
	return tx
}

func (r *assetRepository) paginateAndScan(tx *gorm.DB, page, pageSize int) ([]model.AssetListItem, int64, error) {
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.AssetListItem
	offset := (page - 1) * pageSize
	if err := tx.Order("ai_asset.id DESC").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []model.AssetListItem{}
	}
	return items, total, nil
}

func (r *assetRepository) ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs, permittedAssetIDs []uint64) ([]model.AssetListItem, int64, error) {
	tx := r.baseAssetListQuery()
	tx = r.applyCommonFilters(tx, q)

	isWorkspace := q.Scope == "workspace" || (q.Scope == "" && q.Status != "" && q.Status != model.AssetStatusPublished)
	if isWorkspace {
		if q.Status != "" {
			tx = tx.Where("ai_asset.status = ?", q.Status)
		}
		if !isSuperAdmin {
			if len(adminDomainIDs) > 0 {
				tx = tx.Where("(ai_asset.owner_id = ? OR ai_asset.domain_id IN ?)", userID, adminDomainIDs)
			} else {
				tx = tx.Where("ai_asset.owner_id = ?", userID)
			}
		}
	} else {
		tx = tx.Where("ai_asset.status = ?", model.AssetStatusPublished)
		if !isSuperAdmin {
			cond := "(ai_asset.visibility = ? OR ai_asset.owner_id = ?"
			args := []interface{}{model.VisibilityPublic, userID}
			if len(memberDomainIDs) > 0 {
				cond += " OR (ai_asset.visibility = ? AND ai_asset.domain_id IN ?)"
				args = append(args, model.VisibilityDomain, memberDomainIDs)
			}
			if len(permittedAssetIDs) > 0 {
				cond += " OR ai_asset.id IN ?"
				args = append(args, permittedAssetIDs)
			}
			cond += ")"
			tx = tx.Where(cond, args...)
		}
	}

	return r.paginateAndScan(tx, q.Page, q.PageSize)
}

func (r *assetRepository) ListUserAssets(q model.AssetQuery) ([]model.AssetListItem, int64, error) {
	tx := r.baseAssetListQuery().Where("ai_asset.owner_id = ?", q.OwnerID)
	tx = r.applyCommonFilters(tx, q)
	if q.Status != "" {
		tx = tx.Where("ai_asset.status = ?", q.Status)
	}
	return r.paginateAndScan(tx, q.Page, q.PageSize)
}

func (r *assetRepository) FindByID(id uint64) (*model.AIAsset, error) {
	var asset model.AIAsset
	if err := r.db.First(&asset, id).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (r *assetRepository) FindByCode(code string) (*model.AIAsset, error) {
	var asset model.AIAsset
	if err := r.db.Where("asset_code = ?", code).First(&asset).Error; err != nil {
		return nil, err
	}
	return &asset, nil
}

func (r *assetRepository) GetDetailByID(id uint64) (*model.AssetDetail, error) {
	var listItem model.AssetListItem
	err := r.baseAssetListQuery().Where("ai_asset.id = ?", id).Take(&listItem).Error
	if err != nil {
		return nil, err
	}
	return r.attachExtension(&listItem), nil
}

func (r *assetRepository) GetDetailByCode(code string) (*model.AssetDetail, error) {
	var listItem model.AssetListItem
	err := r.baseAssetListQuery().Where("ai_asset.asset_code = ?", code).Take(&listItem).Error
	if err != nil {
		return nil, err
	}
	return r.attachExtension(&listItem), nil
}

func (r *assetRepository) attachExtension(listItem *model.AssetListItem) *model.AssetDetail {
	detail := &model.AssetDetail{
		AIAsset:    listItem.AIAsset,
		DomainName: listItem.DomainName,
		OwnerName:  listItem.OwnerName,
	}

	switch listItem.AssetType {
	case model.AssetTypeMCP:
		var cfg model.AIAssetMCP
		if err := r.db.Where("asset_id = ?", listItem.ID).First(&cfg).Error; err == nil {
			detail.MCPConfig = &cfg
		}
	case model.AssetTypeSkill:
		var cfg model.AIAssetSkill
		if err := r.db.Where("asset_id = ?", listItem.ID).First(&cfg).Error; err == nil {
			detail.SkillConfig = &cfg
		}
	case model.AssetTypeAgent:
		var cfg model.AIAssetAgent
		if err := r.db.Where("asset_id = ?", listItem.ID).First(&cfg).Error; err == nil {
			detail.AgentConfig = &cfg
		}
	case model.AssetTypeKB:
		var cfg model.KBKnowledgeBase
		if err := r.db.Where("asset_id = ?", listItem.ID).First(&cfg).Error; err == nil {
			detail.KBConfig = &cfg
		}
	}

	return detail
}

func (r *assetRepository) CreateAsset(asset *model.AIAsset, ext interface{}) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(asset).Error; err != nil {
			return err
		}
		if ext == nil {
			return nil
		}
		setAssetID(ext, asset.ID)
		return tx.Create(ext).Error
	})
}

func (r *assetRepository) UpdateAsset(asset *model.AIAsset, ext interface{}) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(asset).Error; err != nil {
			return err
		}
		if ext == nil {
			return nil
		}
		setAssetID(ext, asset.ID)
		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(ext).Error
	})
}

func (r *assetRepository) DeleteAsset(id uint64) error {
	return r.db.Delete(&model.AIAsset{}, id).Error
}

func (r *assetRepository) UpdateStatus(id uint64, status string, version string) error {
	updates := map[string]interface{}{"status": status}
	if version != "" {
		updates["current_version"] = version
	}
	return r.db.Model(&model.AIAsset{}).Where("id = ?", id).Updates(updates).Error
}

func (r *assetRepository) PublishWithSnapshot(assetID uint64, version, changelog string, snapshot model.JSONRaw, publisherID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AIAssetVersion{}).
			Where("asset_id = ?", assetID).
			Update("is_active", 0).Error; err != nil {
			return err
		}

		ver := model.AIAssetVersion{
			AssetID:      assetID,
			Version:      version,
			Changelog:    changelog,
			SnapshotJSON: snapshot,
			PublisherID:  publisherID,
			IsActive:     1,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "asset_id"}, {Name: "version"}},
			DoUpdates: clause.AssignmentColumns([]string{"changelog", "snapshot_json", "publisher_id", "is_active"}),
		}).Create(&ver).Error; err != nil {
			return err
		}

		return tx.Model(&model.AIAsset{}).Where("id = ?", assetID).Updates(map[string]interface{}{
			"status":          model.AssetStatusPublished,
			"current_version": version,
		}).Error
	})
}

func (r *assetRepository) ListVersions(assetID uint64) ([]model.AIAssetVersion, error) {
	var list []model.AIAssetVersion
	if err := r.db.Where("asset_id = ?", assetID).Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if list == nil {
		list = []model.AIAssetVersion{}
	}
	return list, nil
}

func (r *assetRepository) FindVersionByID(assetID, versionID uint64) (*model.AIAssetVersion, error) {
	var ver model.AIAssetVersion
	if err := r.db.Where("id = ? AND asset_id = ?", versionID, assetID).First(&ver).Error; err != nil {
		return nil, err
	}
	return &ver, nil
}

func (r *assetRepository) RestoreFromSnapshot(asset *model.AIAsset, ext interface{}, versionID uint64, version string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		asset.CurrentVersion = version
		if err := tx.Save(asset).Error; err != nil {
			return err
		}
		if ext != nil {
			setAssetID(ext, asset.ID)
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(ext).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.AIAssetVersion{}).Where("asset_id = ?", asset.ID).Update("is_active", 0).Error; err != nil {
			return err
		}
		return tx.Model(&model.AIAssetVersion{}).Where("id = ? AND asset_id = ?", versionID, asset.ID).Update("is_active", 1).Error
	})
}

func (r *assetRepository) UpdateMCPHealth(assetID uint64, healthStatus string, checkedAt time.Time) error {
	return r.db.Model(&model.AIAssetMCP{}).Where("asset_id = ?", assetID).Updates(map[string]interface{}{
		"health_status":   healthStatus,
		"last_checked_at": checkedAt,
	}).Error
}

func (r *assetRepository) UpdateMCPTools(assetID uint64, toolsSchema model.JSONRaw) error {
	return r.db.Model(&model.AIAssetMCP{}).Where("asset_id = ?", assetID).Update("tools_schema", toolsSchema).Error
}

func (r *assetRepository) UpdateSkillPackage(assetID uint64, packagePath string, packageFiles model.StringSlice, content string) error {
	updates := map[string]interface{}{
		"package_path":  packagePath,
		"package_files": packageFiles,
	}
	if content != "" {
		updates["content"] = content
	}
	return r.db.Model(&model.AIAssetSkill{}).Where("asset_id = ?", assetID).Updates(updates).Error
}

func (r *assetRepository) IncrementInvokeCount(assetID uint64) error {
	return r.db.Model(&model.AIAsset{}).Where("id = ?", assetID).
		UpdateColumn("invoke_count", gorm.Expr("invoke_count + 1")).Error
}

func setAssetID(ext interface{}, assetID uint64) {
	switch v := ext.(type) {
	case *model.AIAssetMCP:
		v.AssetID = assetID
	case *model.AIAssetSkill:
		v.AssetID = assetID
	case *model.AIAssetAgent:
		v.AssetID = assetID
	case *model.KBKnowledgeBase:
		v.AssetID = assetID
	}
}
