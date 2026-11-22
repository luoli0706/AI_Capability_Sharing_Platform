package model

import "time"

const (
	CrossRefTypeAsset  = "asset"
	CrossRefTypeKBFile = "kb_file"

	CrossRefStatusPending  = "pending"
	CrossRefStatusActive   = "active"
	CrossRefStatusRejected = "rejected"
)

type DomainCrossRef struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetDomainID    uint64    `gorm:"index:idx_cross_ref_target;not null" json:"target_domain_id"`
	SourceDomainID    uint64    `gorm:"index:idx_cross_ref_source;not null" json:"source_domain_id"`
	RefType           string    `gorm:"size:20;not null" json:"ref_type"`
	SourceAssetID     uint64    `gorm:"index:idx_cross_ref_asset;not null" json:"source_asset_id"`
	SourceDocID       uint64    `gorm:"index:idx_cross_ref_doc;not null;default:0" json:"source_doc_id"`
	Title             string    `gorm:"size:200;not null" json:"title"`
	Description       string    `gorm:"size:500;not null;default:''" json:"description"`
	CreatedBy         uint64    `gorm:"not null" json:"created_by"`
	Status            string    `gorm:"size:20;not null;default:'pending';index:idx_cross_ref_status" json:"status"`
	ApprovalRequestID uint64    `gorm:"not null;default:0" json:"approval_request_id"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DomainCrossRef) TableName() string {
	return "domain_cross_ref"
}

type CreateCrossRefRequest struct {
	SourceDomainID uint64 `json:"source_domain_id" validate:"required,gt=0"`
	RefType        string `json:"ref_type" validate:"required,oneof=asset kb_file"`
	SourceAssetID  uint64 `json:"source_asset_id" validate:"required,gt=0"`
	SourceDocID    uint64 `json:"source_doc_id"`
	Description    string `json:"description" validate:"max=500"`
}

type CrossRefView struct {
	ID                uint64    `json:"id"`
	TargetDomainID    uint64    `json:"target_domain_id"`
	SourceDomainID    uint64    `json:"source_domain_id"`
	SourceDomainName  string    `json:"source_domain_name"`
	RefType           string    `json:"ref_type"`
	SourceAssetID     uint64    `json:"source_asset_id"`
	SourceDocID       uint64    `json:"source_doc_id"`
	Title             string    `json:"title"`
	Description       string    `json:"description"`
	CreatedBy         uint64    `json:"created_by"`
	CreatorName       string    `json:"creator_name"`
	Status            string    `json:"status"`
	ApprovalRequestID uint64    `json:"approval_request_id"`
	CreatedAt         time.Time `json:"created_at"`
	FileExt           string    `json:"file_ext,omitempty"`
	FileSize          uint64    `json:"file_size,omitempty"`
	AssetType         string    `json:"asset_type,omitempty"`
}

type CrossRefSourceAssetItem struct {
	ID        uint64 `json:"id"`
	AssetCode string `json:"asset_code"`
	Name      string `json:"name"`
	AssetType string `json:"asset_type"`
	Summary   string `json:"summary"`
}

type CrossRefSourceKBFileItem struct {
	ID         uint64 `json:"id"`
	KBAssetID  uint64 `json:"kb_asset_id"`
	KBName     string `json:"kb_name"`
	FolderPath string `json:"folder_path"`
	FileName   string `json:"file_name"`
	FileExt    string `json:"file_ext"`
	FileSize   uint64 `json:"file_size"`
}
