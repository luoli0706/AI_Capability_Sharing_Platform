package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	AssetTypeMCP   = "mcp"
	AssetTypeSkill = "skill"
	AssetTypeAgent = "agent"
	AssetTypeKB    = "kb"

	VisibilityPrivate = "private"
	VisibilityDomain  = "domain"
	VisibilityPublic  = "public"

	AssetStatusDraft      = "draft"
	AssetStatusPending    = "pending"
	AssetStatusPublished  = "published"
	AssetStatusDeprecated = "deprecated"
)

type AIAsset struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetCode      string         `gorm:"type:varchar(64);uniqueIndex:uk_asset_code;not null" json:"asset_code"`
	AssetType      string         `gorm:"type:varchar(16);index:idx_domain_type_status;not null" json:"asset_type"`
	Name           string         `gorm:"type:varchar(128);not null" json:"name"`
	Summary        string         `gorm:"type:varchar(512);not null;default:''" json:"summary"`
	Tags           StringSlice    `gorm:"type:json" json:"tags"`
	DomainID       uint64         `gorm:"index:idx_domain_type_status;not null" json:"domain_id"`
	OwnerID        uint64         `gorm:"not null" json:"owner_id"`
	Visibility     string         `gorm:"type:varchar(16);not null;default:'domain'" json:"visibility"`
	CurrentVersion string         `gorm:"type:varchar(32);not null;default:'v0.1.0'" json:"current_version"`
	Status         string         `gorm:"type:varchar(32);index:idx_domain_type_status;not null;default:'draft'" json:"status"`
	InvokeCount    uint64         `gorm:"not null;default:0" json:"invoke_count"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

func (AIAsset) TableName() string {
	return "ai_asset"
}

type AIAssetMCP struct {
	AssetID       uint64     `gorm:"primaryKey;autoIncrement:false" json:"asset_id"`
	TransportType string     `gorm:"type:varchar(32);not null;default:'sse'" json:"transport_type"`
	EndpointURL   string     `gorm:"type:varchar(512);not null" json:"endpoint_url"`
	AuthType      string     `gorm:"type:varchar(32);not null;default:'none'" json:"auth_type"`
	AuthConfigEnc string     `gorm:"type:text" json:"auth_config_enc,omitempty"`
	ToolsSchema   JSONRaw    `gorm:"type:json" json:"tools_schema,omitempty"`
	HealthStatus  string     `gorm:"type:varchar(16);not null;default:'unknown'" json:"health_status"`
	LastCheckedAt *time.Time `gorm:"type:datetime" json:"last_checked_at,omitempty"`
}

func (AIAssetMCP) TableName() string {
	return "ai_asset_mcp"
}

type AIAssetSkill struct {
	AssetID      uint64  `gorm:"primaryKey;autoIncrement:false" json:"asset_id"`
	SkillType    string  `gorm:"type:varchar(32);not null;default:'prompt'" json:"skill_type"`
	InputSchema  JSONRaw `gorm:"type:json" json:"input_schema,omitempty"`
	OutputSchema JSONRaw `gorm:"type:json" json:"output_schema,omitempty"`
	Content      string  `gorm:"type:mediumtext" json:"content"`
	RuntimeEnv   string  `gorm:"type:varchar(64);not null;default:''" json:"runtime_env"`
	Dependencies JSONRaw `gorm:"type:json" json:"dependencies,omitempty"`
}

func (AIAssetSkill) TableName() string {
	return "ai_asset_skill"
}

type AIAssetAgent struct {
	AssetID       uint64      `gorm:"primaryKey;autoIncrement:false" json:"asset_id"`
	ModelProvider string      `gorm:"type:varchar(64);not null;default:''" json:"model_provider"`
	ModelName     string      `gorm:"type:varchar(64);not null;default:''" json:"model_name"`
	SystemPrompt  string      `gorm:"type:text" json:"system_prompt"`
	Temperature   float64     `gorm:"type:decimal(3,2);not null;default:0.70" json:"temperature"`
	MaxTokens     int         `gorm:"type:int;not null;default:2048" json:"max_tokens"`
	BoundMCPIDs   Uint64Slice `gorm:"column:bound_mcp_ids;type:json" json:"bound_mcp_ids"`
	BoundSkillIDs Uint64Slice `gorm:"column:bound_skill_ids;type:json" json:"bound_skill_ids"`
	BoundKBIDs    Uint64Slice `gorm:"column:bound_kb_ids;type:json" json:"bound_kb_ids"`
}

func (AIAssetAgent) TableName() string {
	return "ai_asset_agent"
}

type KBKnowledgeBase struct {
	AssetID        uint64 `gorm:"primaryKey;autoIncrement:false" json:"asset_id"`
	ChunkSize      int    `gorm:"type:int;not null;default:500" json:"chunk_size"`
	ChunkOverlap   int    `gorm:"type:int;not null;default:50" json:"chunk_overlap"`
	EmbeddingModel string `gorm:"type:varchar(64);not null;default:''" json:"embedding_model"`
	DocCount       int    `gorm:"type:int;not null;default:0" json:"doc_count"`
	TotalChunks    int    `gorm:"type:int;not null;default:0" json:"total_chunks"`
}

func (KBKnowledgeBase) TableName() string {
	return "kb_knowledge_base"
}

// AssetListItem includes domain_name and owner_name for list views.
type AssetListItem struct {
	AIAsset
	DomainName string `json:"domain_name"`
	OwnerName  string `json:"owner_name"`
}

// AssetDetail includes extension config alongside base asset fields.
type AssetDetail struct {
	AIAsset
	DomainName  string           `json:"domain_name"`
	OwnerName   string           `json:"owner_name"`
	MCPConfig   *AIAssetMCP      `json:"mcp_config,omitempty"`
	SkillConfig *AIAssetSkill    `json:"skill_config,omitempty"`
	AgentConfig *AIAssetAgent    `json:"agent_config,omitempty"`
	KBConfig    *KBKnowledgeBase `json:"kb_config,omitempty"`
}

type MCPConfigInput struct {
	TransportType string  `json:"transport_type"`
	EndpointURL   string  `json:"endpoint_url"`
	AuthType      string  `json:"auth_type"`
	AuthConfigEnc string  `json:"auth_config_enc"`
	ToolsSchema   JSONRaw `json:"tools_schema"`
}

type SkillConfigInput struct {
	SkillType    string  `json:"skill_type"`
	InputSchema  JSONRaw `json:"input_schema"`
	OutputSchema JSONRaw `json:"output_schema"`
	Content      string  `json:"content"`
	RuntimeEnv   string  `json:"runtime_env"`
	Dependencies JSONRaw `json:"dependencies"`
}

type AgentConfigInput struct {
	ModelProvider string      `json:"model_provider"`
	ModelName     string      `json:"model_name"`
	SystemPrompt  string      `json:"system_prompt"`
	Temperature   *float64    `json:"temperature"`
	MaxTokens     *int        `json:"max_tokens"`
	BoundMCPIDs   Uint64Slice `json:"bound_mcp_ids"`
	BoundSkillIDs Uint64Slice `json:"bound_skill_ids"`
	BoundKBIDs    Uint64Slice `json:"bound_kb_ids"`
}

type KBConfigInput struct {
	ChunkSize      *int   `json:"chunk_size"`
	ChunkOverlap   *int   `json:"chunk_overlap"`
	EmbeddingModel string `json:"embedding_model"`
}

// CreateAssetRequest represents POST /api/v1/assets payload.
type CreateAssetRequest struct {
	AssetCode   string            `json:"asset_code" validate:"required,min=2,max=64"`
	AssetType   string            `json:"asset_type" validate:"required,oneof=mcp skill agent kb"`
	Name        string            `json:"name" validate:"required,min=1,max=128"`
	Summary     string            `json:"summary" validate:"max=512"`
	Tags        StringSlice       `json:"tags"`
	DomainID    uint64            `json:"domain_id" validate:"required,gt=0"`
	Visibility  string            `json:"visibility" validate:"omitempty,oneof=private domain public"`
	MCPConfig   *MCPConfigInput   `json:"mcp_config"`
	SkillConfig *SkillConfigInput `json:"skill_config"`
	AgentConfig *AgentConfigInput `json:"agent_config"`
	KBConfig    *KBConfigInput    `json:"kb_config"`
}

// UpdateAssetRequest represents PUT /api/v1/assets/:id payload.
type UpdateAssetRequest struct {
	Name        string            `json:"name" validate:"omitempty,min=1,max=128"`
	Summary     *string           `json:"summary" validate:"omitempty,max=512"`
	Tags        *StringSlice      `json:"tags"`
	Visibility  string            `json:"visibility" validate:"omitempty,oneof=private domain public"`
	MCPConfig   *MCPConfigInput   `json:"mcp_config"`
	SkillConfig *SkillConfigInput `json:"skill_config"`
	AgentConfig *AgentConfigInput `json:"agent_config"`
	KBConfig    *KBConfigInput    `json:"kb_config"`
}

// PublishAssetRequest represents optional POST /api/v1/assets/:id/publish payload.
type PublishAssetRequest struct {
	Version string `json:"version" validate:"omitempty,max=32"`
}

// AssetQuery defines filter and pagination parameters for listing assets.
type AssetQuery struct {
	AssetType  string
	DomainID   uint64
	Status     string
	Visibility string
	Keyword    string
	Scope      string
	OwnerID    uint64
	Page       int
	PageSize   int
}
