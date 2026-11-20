package repository

import (
	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssetRepository interface {
	ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs []uint64) ([]model.AssetListItem, int64, error)
	ListUserAssets(q model.AssetQuery) ([]model.AssetListItem, int64, error)
	FindByID(id uint64) (*model.AIAsset, error)
	FindByCode(code string) (*model.AIAsset, error)
	GetDetailByID(id uint64) (*model.AssetDetail, error)
	CreateAsset(asset *model.AIAsset, ext interface{}) error
	UpdateAsset(asset *model.AIAsset, ext interface{}) error
	DeleteAsset(id uint64) error
	UpdateStatus(id uint64, status string, version string) error
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

func (r *assetRepository) ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs []uint64) ([]model.AssetListItem, int64, error) {
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
		// Marketplace mode: published assets visible to current user
		tx = tx.Where("ai_asset.status = ?", model.AssetStatusPublished)
		if len(memberDomainIDs) > 0 {
			tx = tx.Where("(ai_asset.visibility = ? OR ai_asset.owner_id = ? OR (ai_asset.visibility = ? AND ai_asset.domain_id IN ?))",
				model.VisibilityPublic, userID, model.VisibilityDomain, memberDomainIDs)
		} else {
			tx = tx.Where("(ai_asset.visibility = ? OR ai_asset.owner_id = ?)",
				model.VisibilityPublic, userID)
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

	detail := &model.AssetDetail{
		AIAsset:    listItem.AIAsset,
		DomainName: listItem.DomainName,
		OwnerName:  listItem.OwnerName,
	}

	switch listItem.AssetType {
	case model.AssetTypeMCP:
		var cfg model.AIAssetMCP
		if err := r.db.Where("asset_id = ?", id).First(&cfg).Error; err == nil {
			detail.MCPConfig = &cfg
		}
	case model.AssetTypeSkill:
		var cfg model.AIAssetSkill
		if err := r.db.Where("asset_id = ?", id).First(&cfg).Error; err == nil {
			detail.SkillConfig = &cfg
		}
	case model.AssetTypeAgent:
		var cfg model.AIAssetAgent
		if err := r.db.Where("asset_id = ?", id).First(&cfg).Error; err == nil {
			detail.AgentConfig = &cfg
		}
	case model.AssetTypeKB:
		var cfg model.KBKnowledgeBase
		if err := r.db.Where("asset_id = ?", id).First(&cfg).Error; err == nil {
			detail.KBConfig = &cfg
		}
	}

	return detail, nil
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
