package service

import (
	"errors"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type AssetService interface {
	ListAssets(userID uint64, isSuperAdmin bool, q model.AssetQuery) ([]model.AssetListItem, int64, error)
	ListUserAssets(userID uint64, q model.AssetQuery) ([]model.AssetListItem, int64, error)
	GetAssetDetail(userID uint64, isSuperAdmin bool, id uint64) (*model.AssetDetail, error)
	CreateAsset(userID uint64, isSuperAdmin bool, req model.CreateAssetRequest) (*model.AssetDetail, error)
	UpdateAsset(userID uint64, isSuperAdmin bool, id uint64, req model.UpdateAssetRequest) (*model.AssetDetail, error)
	DeleteAsset(userID uint64, isSuperAdmin bool, id uint64) error
	PublishAsset(userID uint64, isSuperAdmin bool, id uint64, version string) (*model.AssetDetail, error)
	OfflineAsset(userID uint64, isSuperAdmin bool, id uint64) (*model.AssetDetail, error)
}

type assetService struct {
	assetRepo  repository.AssetRepository
	domainRepo repository.DomainRepository
}

func NewAssetService(assetRepo repository.AssetRepository, domainRepo repository.DomainRepository) AssetService {
	return &assetService{
		assetRepo:  assetRepo,
		domainRepo: domainRepo,
	}
}

func normalizePagination(q *model.AssetQuery) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
}

func (s *assetService) ListAssets(userID uint64, isSuperAdmin bool, q model.AssetQuery) ([]model.AssetListItem, int64, error) {
	normalizePagination(&q)

	memberDomainIDs, err := s.domainRepo.GetUserDomainIDs(userID)
	if err != nil {
		return nil, 0, err
	}
	adminDomainIDs, err := s.domainRepo.GetUserAdminDomainIDs(userID)
	if err != nil {
		return nil, 0, err
	}

	return s.assetRepo.ListAssets(q, userID, isSuperAdmin, memberDomainIDs, adminDomainIDs)
}

func (s *assetService) ListUserAssets(userID uint64, q model.AssetQuery) ([]model.AssetListItem, int64, error) {
	normalizePagination(&q)
	q.OwnerID = userID
	return s.assetRepo.ListUserAssets(q)
}

func (s *assetService) GetAssetDetail(userID uint64, isSuperAdmin bool, id uint64) (*model.AssetDetail, error) {
	detail, err := s.assetRepo.GetDetailByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("asset not found")
		}
		return nil, err
	}

	if !isSuperAdmin && detail.OwnerID != userID {
		role, err := s.domainRepo.GetMemberRole(detail.DomainID, userID)
		if err != nil {
			return nil, err
		}
		isDomainAdmin := role == model.DomainRoleOwner || role == model.DomainRoleAdmin
		if !isDomainAdmin {
			if detail.Status != model.AssetStatusPublished {
				return nil, NewForbidden("no permission to view this asset")
			}
			if detail.Visibility == model.VisibilityPrivate {
				return nil, NewForbidden("no permission to view private asset")
			}
			if detail.Visibility == model.VisibilityDomain && role == "" {
				return nil, NewForbidden("no permission to view domain asset")
			}
		}
	}

	return detail, nil
}

func (s *assetService) CreateAsset(userID uint64, isSuperAdmin bool, req model.CreateAssetRequest) (*model.AssetDetail, error) {
	if _, err := s.domainRepo.FindByID(req.DomainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("domain not found")
		}
		return nil, err
	}

	if !isSuperAdmin {
		role, err := s.domainRepo.GetMemberRole(req.DomainID, userID)
		if err != nil {
			return nil, err
		}
		if role == "" {
			return nil, NewForbidden("you must be a member of the domain to create assets")
		}
	}

	if _, err := s.assetRepo.FindByCode(req.AssetCode); err == nil {
		return nil, NewConflict("asset code already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = model.VisibilityDomain
	}

	asset := &model.AIAsset{
		AssetCode:      req.AssetCode,
		AssetType:      req.AssetType,
		Name:           req.Name,
		Summary:        req.Summary,
		Tags:           req.Tags,
		DomainID:       req.DomainID,
		OwnerID:        userID,
		Visibility:     visibility,
		CurrentVersion: "v0.1.0",
		Status:         model.AssetStatusDraft,
	}

	ext, err := buildExtensionForCreate(req)
	if err != nil {
		return nil, err
	}

	if err := s.assetRepo.CreateAsset(asset, ext); err != nil {
		return nil, err
	}

	return s.assetRepo.GetDetailByID(asset.ID)
}

func (s *assetService) UpdateAsset(userID uint64, isSuperAdmin bool, id uint64, req model.UpdateAssetRequest) (*model.AssetDetail, error) {
	asset, err := s.ensureManagePermission(userID, isSuperAdmin, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		asset.Name = req.Name
	}
	if req.Summary != nil {
		asset.Summary = *req.Summary
	}
	if req.Tags != nil {
		asset.Tags = *req.Tags
	}
	if req.Visibility != "" {
		asset.Visibility = req.Visibility
	}

	ext := buildExtensionForUpdate(asset.AssetType, id, req)
	if err := s.assetRepo.UpdateAsset(asset, ext); err != nil {
		return nil, err
	}

	return s.assetRepo.GetDetailByID(id)
}

func (s *assetService) DeleteAsset(userID uint64, isSuperAdmin bool, id uint64) error {
	if _, err := s.ensureManagePermission(userID, isSuperAdmin, id); err != nil {
		return err
	}
	return s.assetRepo.DeleteAsset(id)
}

func (s *assetService) PublishAsset(userID uint64, isSuperAdmin bool, id uint64, version string) (*model.AssetDetail, error) {
	if _, err := s.ensureManagePermission(userID, isSuperAdmin, id); err != nil {
		return nil, err
	}
	if err := s.assetRepo.UpdateStatus(id, model.AssetStatusPublished, version); err != nil {
		return nil, err
	}
	return s.assetRepo.GetDetailByID(id)
}

func (s *assetService) OfflineAsset(userID uint64, isSuperAdmin bool, id uint64) (*model.AssetDetail, error) {
	if _, err := s.ensureManagePermission(userID, isSuperAdmin, id); err != nil {
		return nil, err
	}
	if err := s.assetRepo.UpdateStatus(id, model.AssetStatusDeprecated, ""); err != nil {
		return nil, err
	}
	return s.assetRepo.GetDetailByID(id)
}

func (s *assetService) ensureManagePermission(userID uint64, isSuperAdmin bool, assetID uint64) (*model.AIAsset, error) {
	asset, err := s.assetRepo.FindByID(assetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("asset not found")
		}
		return nil, err
	}

	if isSuperAdmin || asset.OwnerID == userID {
		return asset, nil
	}

	role, err := s.domainRepo.GetMemberRole(asset.DomainID, userID)
	if err != nil {
		return nil, err
	}
	if role != model.DomainRoleOwner && role != model.DomainRoleAdmin {
		return nil, NewForbidden("only asset owner, domain admin, or super_admin can modify this asset")
	}
	return asset, nil
}

func buildExtensionForCreate(req model.CreateAssetRequest) (interface{}, error) {
	switch req.AssetType {
	case model.AssetTypeMCP:
		if req.MCPConfig == nil || req.MCPConfig.EndpointURL == "" {
			return nil, NewBadRequest("mcp_config.endpoint_url is required for mcp asset")
		}
		return toMCPModel(0, req.MCPConfig), nil
	case model.AssetTypeSkill:
		return toSkillModel(0, req.SkillConfig), nil
	case model.AssetTypeAgent:
		return toAgentModel(0, req.AgentConfig), nil
	case model.AssetTypeKB:
		return toKBModel(0, req.KBConfig), nil
	default:
		return nil, NewBadRequest("unsupported asset_type")
	}
}

func buildExtensionForUpdate(assetType string, assetID uint64, req model.UpdateAssetRequest) interface{} {
	switch assetType {
	case model.AssetTypeMCP:
		if req.MCPConfig != nil {
			return toMCPModel(assetID, req.MCPConfig)
		}
	case model.AssetTypeSkill:
		if req.SkillConfig != nil {
			return toSkillModel(assetID, req.SkillConfig)
		}
	case model.AssetTypeAgent:
		if req.AgentConfig != nil {
			return toAgentModel(assetID, req.AgentConfig)
		}
	case model.AssetTypeKB:
		if req.KBConfig != nil {
			return toKBModel(assetID, req.KBConfig)
		}
	}
	return nil
}

func toMCPModel(assetID uint64, in *model.MCPConfigInput) *model.AIAssetMCP {
	transport := "sse"
	authType := "none"
	var endpoint, authEnc string
	var tools model.JSONRaw
	if in != nil {
		if in.TransportType != "" {
			transport = in.TransportType
		}
		if in.AuthType != "" {
			authType = in.AuthType
		}
		endpoint = in.EndpointURL
		authEnc = in.AuthConfigEnc
		tools = in.ToolsSchema
	}
	return &model.AIAssetMCP{
		AssetID:       assetID,
		TransportType: transport,
		EndpointURL:   endpoint,
		AuthType:      authType,
		AuthConfigEnc: authEnc,
		ToolsSchema:   tools,
		HealthStatus:  "unknown",
	}
}

func toSkillModel(assetID uint64, in *model.SkillConfigInput) *model.AIAssetSkill {
	m := &model.AIAssetSkill{
		AssetID:   assetID,
		SkillType: "prompt",
	}
	if in != nil {
		if in.SkillType != "" {
			m.SkillType = in.SkillType
		}
		m.InputSchema = in.InputSchema
		m.OutputSchema = in.OutputSchema
		m.Content = in.Content
		m.RuntimeEnv = in.RuntimeEnv
		m.Dependencies = in.Dependencies
	}
	return m
}

func toAgentModel(assetID uint64, in *model.AgentConfigInput) *model.AIAssetAgent {
	m := &model.AIAssetAgent{
		AssetID:     assetID,
		Temperature: 0.70,
		MaxTokens:   2048,
	}
	if in != nil {
		m.ModelProvider = in.ModelProvider
		m.ModelName = in.ModelName
		m.SystemPrompt = in.SystemPrompt
		if in.Temperature != nil {
			m.Temperature = *in.Temperature
		}
		if in.MaxTokens != nil {
			m.MaxTokens = *in.MaxTokens
		}
		m.BoundMCPIDs = in.BoundMCPIDs
		m.BoundSkillIDs = in.BoundSkillIDs
		m.BoundKBIDs = in.BoundKBIDs
	}
	return m
}

func toKBModel(assetID uint64, in *model.KBConfigInput) *model.KBKnowledgeBase {
	m := &model.KBKnowledgeBase{
		AssetID:      assetID,
		ChunkSize:    500,
		ChunkOverlap: 50,
	}
	if in != nil {
		if in.ChunkSize != nil && *in.ChunkSize > 0 {
			m.ChunkSize = *in.ChunkSize
		}
		if in.ChunkOverlap != nil && *in.ChunkOverlap >= 0 {
			m.ChunkOverlap = *in.ChunkOverlap
		}
		m.EmbeddingModel = in.EmbeddingModel
	}
	return m
}
