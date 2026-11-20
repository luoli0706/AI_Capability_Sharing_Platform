package repository

import (
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type APIKeyRepository interface {
	ListByUser(userID uint64) ([]model.UserAPIKey, error)
	Create(key *model.UserAPIKey) error
	FindActiveByHash(keyHash string) (*model.UserAPIKey, error)
	Revoke(id, userID uint64) error
	TouchLastUsed(id uint64, t time.Time) error
}

type apiKeyRepository struct {
	db *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) APIKeyRepository {
	return &apiKeyRepository{db: db}
}

func (r *apiKeyRepository) ListByUser(userID uint64) ([]model.UserAPIKey, error) {
	var keys []model.UserAPIKey
	if err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []model.UserAPIKey{}
	}
	return keys, nil
}

func (r *apiKeyRepository) Create(key *model.UserAPIKey) error {
	return r.db.Create(key).Error
}

func (r *apiKeyRepository) FindActiveByHash(keyHash string) (*model.UserAPIKey, error) {
	var key model.UserAPIKey
	if err := r.db.Where("key_hash = ? AND status = 1", keyHash).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepository) Revoke(id, userID uint64) error {
	res := r.db.Model(&model.UserAPIKey{}).Where("id = ? AND user_id = ?", id, userID).Update("status", 0)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *apiKeyRepository) TouchLastUsed(id uint64, t time.Time) error {
	return r.db.Model(&model.UserAPIKey{}).Where("id = ?", id).UpdateColumn("last_used_at", t).Error
}

type InvokeLogRepository interface {
	Create(log *model.AssetInvokeLog) error
	ListByCaller(callerUserID uint64, assetType string, isSuccess *int8, page, pageSize int) ([]model.InvokeLogItem, int64, error)
}

type invokeLogRepository struct {
	db *gorm.DB
}

func NewInvokeLogRepository(db *gorm.DB) InvokeLogRepository {
	return &invokeLogRepository{db: db}
}

func (r *invokeLogRepository) Create(log *model.AssetInvokeLog) error {
	return r.db.Create(log).Error
}

func (r *invokeLogRepository) ListByCaller(callerUserID uint64, assetType string, isSuccess *int8, page, pageSize int) ([]model.InvokeLogItem, int64, error) {
	tx := r.db.Table("asset_invoke_log").
		Select("asset_invoke_log.*, COALESCE(ai_asset.name, '') AS asset_name, COALESCE(ai_asset.asset_code, '') AS asset_code").
		Joins("LEFT JOIN ai_asset ON ai_asset.id = asset_invoke_log.asset_id").
		Where("asset_invoke_log.caller_user_id = ?", callerUserID)

	if assetType != "" {
		tx = tx.Where("asset_invoke_log.asset_type = ?", assetType)
	}
	if isSuccess != nil {
		tx = tx.Where("asset_invoke_log.is_success = ?", *isSuccess)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.InvokeLogItem
	offset := (page - 1) * pageSize
	if err := tx.Order("asset_invoke_log.id DESC").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	if items == nil {
		items = []model.InvokeLogItem{}
	}
	return items, total, nil
}
