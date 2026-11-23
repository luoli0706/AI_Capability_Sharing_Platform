package model

import "time"

const (
	AuditActionAssetCreate        = "asset.create"
	AuditActionAssetUpdate        = "asset.update"
	AuditActionAssetPublishSubmit = "asset.publish_submit"
	AuditActionAssetPublishApprove = "asset.publish_approve"
	AuditActionAssetPublishReject = "asset.publish_reject"
	AuditActionAssetOffline       = "asset.offline"
	AuditActionAssetRollback      = "asset.rollback"
	AuditActionAssetDelete        = "asset.delete"

	AuditActionKBFolderCreate = "kb.folder_create"
	AuditActionKBFileCreate   = "kb.file_create"
	AuditActionKBFileUpdate   = "kb.file_update"
	AuditActionKBFileUpload   = "kb.file_upload"
	AuditActionKBFileMove     = "kb.file_move"
	AuditActionKBFileCopy     = "kb.file_copy"
	AuditActionKBFileRename   = "kb.file_rename"
	AuditActionKBFileDelete   = "kb.file_delete"

	AuditActionDomainCreate       = "domain.create"
	AuditActionDomainUpdate       = "domain.update"
	AuditActionDomainDelete       = "domain.delete"
	AuditActionDomainMemberAdd    = "domain.member_add"
	AuditActionDomainMemberRemove = "domain.member_remove"

	AuditActionOrgCreate       = "org.create"
	AuditActionOrgUpdate       = "org.update"
	AuditActionOrgDelete       = "org.delete"
	AuditActionOrgMemberAdd    = "org.member_add"
	AuditActionOrgMemberRemove = "org.member_remove"

	AuditActionCrossRefCreate  = "cross_ref.create"
	AuditActionCrossRefApprove = "cross_ref.approve"
	AuditActionCrossRefReject  = "cross_ref.reject"
	AuditActionCrossRefDelete  = "cross_ref.delete"
)

type AssetAuditLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	OperatorID   uint64    `gorm:"index:idx_operator;not null" json:"operator_id"`
	OperatorName string    `gorm:"type:varchar(64);not null;default:''" json:"operator_name"`
	DomainID     uint64    `gorm:"index:idx_domain_time;not null;default:0" json:"domain_id"`
	DomainName   string    `gorm:"type:varchar(128);not null;default:''" json:"domain_name"`
	AssetID      uint64    `gorm:"index:idx_asset;not null;default:0" json:"asset_id"`
	AssetCode    string    `gorm:"type:varchar(64);not null;default:''" json:"asset_code"`
	AssetName    string    `gorm:"type:varchar(128);not null;default:''" json:"asset_name"`
	AssetType    string    `gorm:"type:varchar(16);index:idx_type;not null;default:''" json:"asset_type"`
	Action       string    `gorm:"type:varchar(64);index:idx_action;not null" json:"action"`
	TargetName   string    `gorm:"type:varchar(255);not null;default:''" json:"target_name"`
	Detail       string    `gorm:"type:varchar(512);not null;default:''" json:"detail"`
	CreatedAt    time.Time `gorm:"index:idx_domain_time;autoCreateTime" json:"created_at"`
}

func (AssetAuditLog) TableName() string {
	return "asset_audit_log"
}

type AuditLogQuery struct {
	AssetType string
	Action    string
	DomainID  uint64
	Keyword   string
	Page      int
	PageSize  int
}
