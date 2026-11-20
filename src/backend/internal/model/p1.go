package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	PermLevelInvoke = "invoke"
	PermLevelEdit   = "edit"
	PermLevelAdmin  = "admin"

	PermSourceSubscribe = "subscribe"
	PermSourceApproval  = "approval"
	PermSourceDirect    = "direct"

	PermStatusActive  = "active"
	PermStatusRevoked = "revoked"

	GranteeTypeUser   = "user"
	GranteeTypeDomain = "domain"

	ParseStatusPending    = "pending"
	ParseStatusProcessing = "processing"
	ParseStatusCompleted  = "completed"
	ParseStatusFailed     = "failed"

	ApprovalTargetAccess  = "asset_access"
	ApprovalTargetPublish = "asset_publish"

	ApprovalStatusPending   = "pending"
	ApprovalStatusApproved  = "approved"
	ApprovalStatusRejected  = "rejected"
	ApprovalStatusCancelled = "cancelled"

	ApprovalActionPending = "pending"
	ApprovalActionApprove = "approve"
	ApprovalActionReject  = "reject"

	AuthModeJWT    = "jwt"
	AuthModeAPIKey = "api_key"
)

type UserAPIKey struct {
	ID         uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint64         `gorm:"index:idx_user_id;not null" json:"user_id"`
	KeyName    string         `gorm:"type:varchar(64);not null" json:"key_name"`
	KeyPrefix  string         `gorm:"type:varchar(16);not null" json:"key_prefix"`
	KeyHash    string         `gorm:"type:varchar(128);uniqueIndex:uk_key_hash;not null" json:"-"`
	ExpiresAt  *time.Time     `gorm:"type:datetime" json:"expires_at,omitempty"`
	Status     int8           `gorm:"type:tinyint;not null;default:1" json:"status"`
	LastUsedAt *time.Time     `gorm:"type:datetime" json:"last_used_at,omitempty"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

func (UserAPIKey) TableName() string {
	return "user_api_key"
}

type AIAssetVersion struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID      uint64    `gorm:"uniqueIndex:uk_asset_version;not null" json:"asset_id"`
	Version      string    `gorm:"type:varchar(32);uniqueIndex:uk_asset_version;not null" json:"version"`
	Changelog    string    `gorm:"type:varchar(512);not null;default:''" json:"changelog"`
	SnapshotJSON JSONRaw   `gorm:"type:json;not null" json:"snapshot_json"`
	PublisherID  uint64    `gorm:"not null" json:"publisher_id"`
	IsActive     int8      `gorm:"type:tinyint;not null;default:1" json:"is_active"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AIAssetVersion) TableName() string {
	return "ai_asset_version"
}

type KBDocument struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	KBAssetID   uint64         `gorm:"column:kb_asset_id;index:idx_kb_asset;not null" json:"kb_asset_id"`
	FolderPath  string         `gorm:"type:varchar(512);index:idx_kb_folder;not null;default:''" json:"folder_path"`
	IsFolder    bool           `gorm:"type:tinyint(1);not null;default:0" json:"is_folder"`
	FileName    string         `gorm:"type:varchar(255);not null" json:"file_name"`
	FileExt     string         `gorm:"type:varchar(16);not null" json:"file_ext"`
	FileSize    uint64         `gorm:"not null;default:0" json:"file_size"`
	FilePath    string         `gorm:"type:varchar(512);not null" json:"file_path"`
	FileHash    string         `gorm:"type:varchar(128);not null;default:''" json:"file_hash"`
	ParseStatus string         `gorm:"type:varchar(32);not null;default:'pending'" json:"parse_status"`
	ErrorMsg    string         `gorm:"type:varchar(512);not null;default:''" json:"error_msg"`
	ChunkCount  int            `gorm:"type:int;not null;default:0" json:"chunk_count"`
	UploaderID  uint64         `gorm:"not null" json:"uploader_id"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (KBDocument) TableName() string {
	return "kb_document"
}

type KBChunk struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	KBAssetID  uint64    `gorm:"column:kb_asset_id;index:idx_kb_doc;not null" json:"kb_asset_id"`
	DocID      uint64    `gorm:"index:idx_kb_doc;not null" json:"doc_id"`
	ChunkIndex int       `gorm:"type:int;not null" json:"chunk_index"`
	Content    string    `gorm:"type:mediumtext;not null" json:"content"`
	TokenCount int       `gorm:"type:int;not null;default:0" json:"token_count"`
	Metadata   JSONRaw   `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (KBChunk) TableName() string {
	return "kb_chunk"
}

type AssetPermission struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID     uint64     `gorm:"index:idx_perm_lookup;not null" json:"asset_id"`
	GranteeType string     `gorm:"type:varchar(16);index:idx_perm_lookup;not null" json:"grantee_type"`
	GranteeID   uint64     `gorm:"index:idx_perm_lookup;not null" json:"grantee_id"`
	PermLevel   string     `gorm:"type:varchar(16);not null;default:'invoke'" json:"perm_level"`
	Source      string     `gorm:"type:varchar(16);not null;default:'subscribe'" json:"source"`
	ExpiresAt   *time.Time `gorm:"type:datetime" json:"expires_at,omitempty"`
	Status      string     `gorm:"type:varchar(16);index:idx_perm_lookup;not null;default:'active'" json:"status"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AssetPermission) TableName() string {
	return "asset_permission"
}

type ApprovalRequest struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RequestNo         string    `gorm:"type:varchar(64);uniqueIndex:uk_request_no;not null" json:"request_no"`
	ApplicantID       uint64    `gorm:"index:idx_applicant_status;not null" json:"applicant_id"`
	ApplicantDomainID uint64    `gorm:"not null;default:0" json:"applicant_domain_id"`
	TargetType        string    `gorm:"type:varchar(32);not null" json:"target_type"`
	AssetID           uint64    `gorm:"not null" json:"asset_id"`
	RequestedPerm     string    `gorm:"type:varchar(16);not null;default:'invoke'" json:"requested_perm"`
	DurationDays      int       `gorm:"type:int;not null;default:30" json:"duration_days"`
	Reason            string    `gorm:"type:varchar(512);not null;default:''" json:"reason"`
	CurrentStep       int       `gorm:"type:int;not null;default:1" json:"current_step"`
	TotalSteps        int       `gorm:"type:int;not null;default:2" json:"total_steps"`
	Status            string    `gorm:"type:varchar(16);index:idx_applicant_status;not null;default:'pending'" json:"status"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ApprovalRequest) TableName() string {
	return "approval_request"
}

type ApprovalRecord struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	RequestID  uint64     `gorm:"index:idx_request_step;not null" json:"request_id"`
	StepNo     int        `gorm:"type:int;index:idx_request_step;not null" json:"step_no"`
	StepName   string     `gorm:"type:varchar(64);not null" json:"step_name"`
	ApproverID uint64     `gorm:"index:idx_approver_action;not null" json:"approver_id"`
	Action     string     `gorm:"type:varchar(16);index:idx_approver_action;not null;default:'pending'" json:"action"`
	Comment    string     `gorm:"type:varchar(512);not null;default:''" json:"comment"`
	HandledAt  *time.Time `gorm:"type:datetime" json:"handled_at,omitempty"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (ApprovalRecord) TableName() string {
	return "approval_record"
}

type AssetInvokeLog struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TraceID        string    `gorm:"type:varchar(64);not null" json:"trace_id"`
	AssetID        uint64    `gorm:"index:idx_asset_time;not null" json:"asset_id"`
	AssetType      string    `gorm:"type:varchar(16);not null" json:"asset_type"`
	AssetVersion   string    `gorm:"type:varchar(32);not null;default:''" json:"asset_version"`
	CallerUserID   uint64    `gorm:"index:idx_caller_time;not null" json:"caller_user_id"`
	CallerDomainID uint64    `gorm:"not null;default:0" json:"caller_domain_id"`
	AuthMode       string    `gorm:"type:varchar(16);not null" json:"auth_mode"`
	StatusCode     int       `gorm:"type:int;not null" json:"status_code"`
	IsSuccess      int8      `gorm:"type:tinyint;not null;default:1" json:"is_success"`
	LatencyMS      uint      `gorm:"column:latency_ms;type:int unsigned;not null;default:0" json:"latency_ms"`
	ErrorMessage   string    `gorm:"type:varchar(512);not null;default:''" json:"error_message"`
	ClientIP       string    `gorm:"type:varchar(64);not null;default:''" json:"client_ip"`
	CreatedAt      time.Time `gorm:"index:idx_asset_time;index:idx_caller_time;autoCreateTime" json:"created_at"`
}

func (AssetInvokeLog) TableName() string {
	return "asset_invoke_log"
}

// --- P1 DTOs ---

type RollbackAssetRequest struct {
	VersionID uint64 `json:"version_id" validate:"required,gt=0"`
}

type KBSearchRequest struct {
	Query string `json:"query" validate:"required,min=1"`
	TopK  int    `json:"top_k"`
}

type CreateKBFolderRequest struct {
	FolderPath string `json:"folder_path" validate:"required,max=512"`
}

type MoveKBDocumentRequest struct {
	TargetFolderPath string `json:"target_folder_path" validate:"max=512"`
}

type CopyKBDocumentRequest struct {
	TargetFolderPath string `json:"target_folder_path" validate:"max=512"`
	NewName          string `json:"new_name" validate:"max=255"`
}

type KBPreviewFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type KBDownloadPreview struct {
	Scope      string          `json:"scope"`
	Target     string          `json:"target"`
	FileCount  int             `json:"file_count"`
	TotalBytes int64           `json:"total_bytes"`
	Files      []KBPreviewFile `json:"files"`
}

type KBSearchHit struct {
	ChunkID    uint64  `json:"chunk_id"`
	DocID      uint64  `json:"doc_id"`
	FileName   string  `json:"file_name"`
	ChunkIndex int     `json:"chunk_index"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

type CreateApprovalRequest struct {
	AssetID       uint64 `json:"asset_id" validate:"required,gt=0"`
	TargetType    string `json:"target_type" validate:"omitempty,oneof=asset_access asset_publish"`
	RequestedPerm string `json:"requested_perm" validate:"omitempty,oneof=invoke edit admin"`
	DurationDays  int    `json:"duration_days"`
	Reason        string `json:"reason" validate:"max=512"`
}

type HandleApprovalRequest struct {
	Action  string `json:"action" validate:"required,oneof=approve reject"`
	Comment string `json:"comment" validate:"max=512"`
}

type ApprovalRecordView struct {
	ApprovalRecord
	ApproverName string `json:"approver_name"`
}

type ApprovalRequestView struct {
	ApprovalRequest
	ApplicantName string               `json:"applicant_name"`
	AssetName     string               `json:"asset_name"`
	AssetType     string               `json:"asset_type"`
	DomainName    string               `json:"domain_name"`
	Records       []ApprovalRecordView `json:"records"`
}

type CreateAPIKeyRequest struct {
	KeyName     string `json:"key_name" validate:"required,min=1,max=64"`
	ExpiresDays int    `json:"expires_days"`
}

type InvokeLogItem struct {
	AssetInvokeLog
	AssetName string `json:"asset_name"`
	AssetCode string `json:"asset_code"`
}
