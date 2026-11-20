package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/agentpkg"
	"ascp-backend/internal/pkg/mcpregistry"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/pkg/skillpkg"
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
	PublishAsset(userID uint64, isSuperAdmin bool, id uint64, version, changelog string) (*model.AssetDetail, error)
	OfflineAsset(userID uint64, isSuperAdmin bool, id uint64) (*model.AssetDetail, error)
	ListVersions(userID uint64, isSuperAdmin bool, id uint64) ([]model.AIAssetVersion, error)
	RollbackAsset(userID uint64, isSuperAdmin bool, id, versionID uint64) (*model.AssetDetail, error)
	CheckMCPHealth(userID uint64, isSuperAdmin bool, id uint64) (*model.AIAssetMCP, error)
	SyncMCPTools(userID uint64, isSuperAdmin bool, id uint64) (*model.AIAssetMCP, error)
	ParseSkillFile(filename string, data []byte) (*model.SkillParseResult, error)
	ImportSkillFromURL(rawURL string) (*model.SkillImportResult, error)
	UploadSkillPackage(userID uint64, isSuperAdmin bool, id uint64, filename string, data []byte) (*model.AssetDetail, error)
	DownloadSkillPackage(userID uint64, isSuperAdmin bool, id uint64, format string) (filename, contentType string, data []byte, err error)
	SearchMCPRegistry(ctx context.Context, query string, limit int) ([]mcpregistry.MCPImportItem, error)
	ParseMCPConfigJSON(raw string) ([]mcpregistry.MCPImportItem, error)
	ParseAgentFile(raw, filename string) (*agentpkg.AgentImportResult, error)
	ImportAgentFromURL(ctx context.Context, rawURL string) (*agentpkg.AgentImportResult, error)
	DownloadAgentPackage(userID uint64, isSuperAdmin bool, id uint64, format string) (filename, contentType string, data []byte, err error)
	SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository)
}

type assetService struct {
	assetRepo  repository.AssetRepository
	domainRepo repository.DomainRepository
	govRepo    repository.GovernanceRepository
	orgRepo    repository.OrgRepository
	auditRepo  repository.AuditLogRepository
	userRepo   repository.UserRepository
	uploadDir  string
	httpClient *http.Client
}

func (s *assetService) SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository) {
	s.auditRepo = auditRepo
	s.userRepo = userRepo
}

func NewAssetService(assetRepo repository.AssetRepository, domainRepo repository.DomainRepository, govRepo repository.GovernanceRepository, orgRepo repository.OrgRepository, uploadDir string) AssetService {
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return &assetService{
		assetRepo:  assetRepo,
		domainRepo: domainRepo,
		govRepo:    govRepo,
		orgRepo:    orgRepo,
		uploadDir:  uploadDir,
		httpClient: &http.Client{Timeout: 5 * time.Second},
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

	accessibleDomainIDs, err := getAccessibleDomainIDs(s.domainRepo, s.orgRepo, userID)
	if err != nil {
		return nil, 0, err
	}
	adminDomainIDs, err := s.domainRepo.GetUserAdminDomainIDs(userID)
	if err != nil {
		return nil, 0, err
	}

	var permittedAssetIDs, subscribedAssetIDs []uint64
	if s.govRepo != nil {
		permittedAssetIDs, _ = s.govRepo.GetUserPermittedAssetIDs(userID)
		subscribedAssetIDs, _ = s.govRepo.GetUserSubscribedAssetIDs(userID)
	}

	items, total, err := s.assetRepo.ListAssets(q, userID, isSuperAdmin, accessibleDomainIDs, adminDomainIDs, permittedAssetIDs)
	if err != nil {
		return nil, 0, err
	}

	accessibleSet := toUint64Set(accessibleDomainIDs)
	permSet := toUint64Set(permittedAssetIDs)
	subSet := toUint64Set(subscribedAssetIDs)

	for i := range items {
		items[i].IsSubscribed = subSet[items[i].ID]
		items[i].HasPermission = isSuperAdmin ||
			items[i].OwnerID == userID ||
			items[i].Visibility == model.VisibilityPublic ||
			(items[i].Visibility == model.VisibilityDomain && accessibleSet[items[i].DomainID]) ||
			permSet[items[i].ID]
	}

	return items, total, nil
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

	var activePerm *model.AssetPermission
	if s.govRepo != nil {
		activePerm, _ = s.govRepo.GetActiveUserPermission(id, userID)
	}
	if activePerm != nil {
		detail.HasPermission = true
		detail.IsSubscribed = activePerm.Source == model.PermSourceSubscribe
	}

	if isSuperAdmin || detail.OwnerID == userID {
		detail.HasPermission = true
		return detail, nil
	}

	role, err := s.domainRepo.GetMemberRole(detail.DomainID, userID)
	if err != nil {
		return nil, err
	}
	isDomainAdmin := role == model.DomainRoleOwner || role == model.DomainRoleAdmin
	if isDomainAdmin {
		detail.HasPermission = true
		return detail, nil
	}

	if detail.Status != model.AssetStatusPublished {
		return nil, NewForbidden("no permission to view this asset")
	}

	if detail.Visibility == model.VisibilityPublic {
		detail.HasPermission = true
		return detail, nil
	}
	if detail.Visibility == model.VisibilityDomain {
		ok, err := hasDomainAccess(s.domainRepo, s.orgRepo, detail.DomainID, userID)
		if err != nil {
			return nil, err
		}
		if ok {
			detail.HasPermission = true
			return detail, nil
		}
	}

	if activePerm != nil {
		return detail, nil
	}

	if detail.Visibility == model.VisibilityPrivate {
		return nil, NewForbidden("no permission to view private asset")
	}
	return nil, NewForbidden("no permission to view domain asset")
}

func (s *assetService) CreateAsset(userID uint64, isSuperAdmin bool, req model.CreateAssetRequest) (*model.AssetDetail, error) {
	if _, err := s.domainRepo.FindByID(req.DomainID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("domain not found")
		}
		return nil, err
	}

	if !isSuperAdmin {
		ok, err := hasDomainAccess(s.domainRepo, s.orgRepo, req.DomainID, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, NewForbidden("you must have access to the domain to create assets")
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

	detail, err := s.assetRepo.GetDetailByID(asset.ID)
	if err == nil {
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionAssetCreate, detail.Name, "创建资产 "+detail.AssetCode)
	}
	return detail, err
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

	detail, err := s.assetRepo.GetDetailByID(id)
	if err == nil {
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionAssetUpdate, detail.Name, "更新资产配置")
	}
	return detail, err
}

func (s *assetService) DeleteAsset(userID uint64, isSuperAdmin bool, id uint64) error {
	asset, err := s.ensureManagePermission(userID, isSuperAdmin, id)
	if err != nil {
		return err
	}
	if err := s.assetRepo.DeleteAsset(id); err != nil {
		return err
	}
	recordAudit(s.auditRepo, s.userRepo, userID, asset.DomainID, "", asset.ID, asset.AssetCode, asset.Name, asset.AssetType, model.AuditActionAssetDelete, asset.Name, "删除资产 "+asset.AssetCode)
	return nil
}

func (s *assetService) PublishAsset(userID uint64, isSuperAdmin bool, id uint64, version, changelog string) (*model.AssetDetail, error) {
	asset, err := s.ensureManagePermission(userID, isSuperAdmin, id)
	if err != nil {
		return nil, err
	}
	if asset.Status == model.AssetStatusPending {
		return nil, NewConflict("资产已在发布审批中，请勿重复提交")
	}

	domain, err := s.domainRepo.FindByID(asset.DomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("domain not found")
		}
		return nil, err
	}

	detail, err := s.assetRepo.GetDetailByID(id)
	if err != nil {
		return nil, err
	}

	nextVersion := strings.TrimSpace(version)
	if nextVersion == "" {
		nextVersion = bumpVersion(detail.CurrentVersion)
	}
	if changelog == "" {
		changelog = "申请发布资产上线"
	}

	detail.Status = model.AssetStatusPublished
	detail.CurrentVersion = nextVersion

	snapshotBytes, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}

	requestNo := fmt.Sprintf("APR-%s-%04d", time.Now().Format("20060102150405"), time.Now().Nanosecond()%10000)
	if err := s.assetRepo.SubmitPublishApproval(
		id,
		asset.DomainID,
		nextVersion,
		changelog,
		model.JSONRaw(snapshotBytes),
		userID,
		domain.OwnerID,
		requestNo,
	); err != nil {
		return nil, err
	}
	updated, err := s.assetRepo.GetDetailByID(id)
	if err == nil {
		recordAudit(s.auditRepo, s.userRepo, userID, updated.DomainID, updated.DomainName, updated.ID, updated.AssetCode, updated.Name, updated.AssetType, model.AuditActionAssetPublishSubmit, nextVersion, "提交发布审批: "+changelog)
	}
	return updated, err
}

func (s *assetService) OfflineAsset(userID uint64, isSuperAdmin bool, id uint64) (*model.AssetDetail, error) {
	if _, err := s.ensureManagePermission(userID, isSuperAdmin, id); err != nil {
		return nil, err
	}
	if err := s.assetRepo.UpdateStatus(id, model.AssetStatusDeprecated, ""); err != nil {
		return nil, err
	}
	updated, err := s.assetRepo.GetDetailByID(id)
	if err == nil {
		recordAudit(s.auditRepo, s.userRepo, userID, updated.DomainID, updated.DomainName, updated.ID, updated.AssetCode, updated.Name, updated.AssetType, model.AuditActionAssetOffline, updated.Name, "下线资产")
	}
	return updated, err
}

func (s *assetService) ListVersions(userID uint64, isSuperAdmin bool, id uint64) ([]model.AIAssetVersion, error) {
	if _, err := s.GetAssetDetail(userID, isSuperAdmin, id); err != nil {
		return nil, err
	}
	return s.assetRepo.ListVersions(id)
}

func (s *assetService) RollbackAsset(userID uint64, isSuperAdmin bool, id, versionID uint64) (*model.AssetDetail, error) {
	asset, err := s.ensureManagePermission(userID, isSuperAdmin, id)
	if err != nil {
		return nil, err
	}

	ver, err := s.assetRepo.FindVersionByID(id, versionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("version not found")
		}
		return nil, err
	}

	var snap model.AssetDetail
	if err := json.Unmarshal(ver.SnapshotJSON, &snap); err != nil {
		return nil, NewBadRequest("corrupted version snapshot")
	}

	asset.Name = snap.Name
	asset.Summary = snap.Summary
	asset.Tags = snap.Tags
	if snap.Visibility != "" {
		asset.Visibility = snap.Visibility
	}

	var ext interface{}
	switch asset.AssetType {
	case model.AssetTypeMCP:
		if snap.MCPConfig != nil {
			ext = snap.MCPConfig
		}
	case model.AssetTypeSkill:
		if snap.SkillConfig != nil {
			ext = snap.SkillConfig
		}
	case model.AssetTypeAgent:
		if snap.AgentConfig != nil {
			ext = snap.AgentConfig
		}
	case model.AssetTypeKB:
		if snap.KBConfig != nil {
			ext = snap.KBConfig
		}
	}

	if err := s.assetRepo.RestoreFromSnapshot(asset, ext, ver.ID, ver.Version); err != nil {
		return nil, err
	}
	updated, err := s.assetRepo.GetDetailByID(id)
	if err == nil {
		recordAudit(s.auditRepo, s.userRepo, userID, updated.DomainID, updated.DomainName, updated.ID, updated.AssetCode, updated.Name, updated.AssetType, model.AuditActionAssetRollback, ver.Version, "回滚至版本 "+ver.Version)
	}
	return updated, err
}

func (s *assetService) CheckMCPHealth(userID uint64, isSuperAdmin bool, id uint64) (*model.AIAssetMCP, error) {
	if _, err := s.ensureManagePermission(userID, isSuperAdmin, id); err != nil {
		return nil, err
	}
	detail, err := s.assetRepo.GetDetailByID(id)
	if err != nil {
		return nil, err
	}
	if detail.AssetType != model.AssetTypeMCP || detail.MCPConfig == nil {
		return nil, NewBadRequest("asset is not an MCP server")
	}

	status := "unhealthy"
	req, err := http.NewRequest(http.MethodGet, detail.MCPConfig.EndpointURL, nil)
	if err == nil {
		resp, err := s.httpClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				status = "healthy"
			}
		}
	}

	now := time.Now()
	if err := s.assetRepo.UpdateMCPHealth(id, status, now); err != nil {
		return nil, err
	}
	detail.MCPConfig.HealthStatus = status
	detail.MCPConfig.LastCheckedAt = &now
	return detail.MCPConfig, nil
}

func (s *assetService) SyncMCPTools(userID uint64, isSuperAdmin bool, id uint64) (*model.AIAssetMCP, error) {
	if _, err := s.ensureManagePermission(userID, isSuperAdmin, id); err != nil {
		return nil, err
	}
	detail, err := s.assetRepo.GetDetailByID(id)
	if err != nil {
		return nil, err
	}
	if detail.AssetType != model.AssetTypeMCP || detail.MCPConfig == nil {
		return nil, NewBadRequest("asset is not an MCP server")
	}

	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	req, err := http.NewRequest(http.MethodPost, detail.MCPConfig.EndpointURL, bytes.NewReader(payload))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.httpClient.Do(req)
		if err == nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()
			var rpcResp struct {
				Result struct {
					Tools json.RawMessage `json:"tools"`
				} `json:"result"`
			}
			if json.Unmarshal(body, &rpcResp) == nil && len(rpcResp.Result.Tools) > 0 {
				tools := model.JSONRaw(rpcResp.Result.Tools)
				if err := s.assetRepo.UpdateMCPTools(id, tools); err != nil {
					return nil, err
				}
				detail.MCPConfig.ToolsSchema = tools
			}
		}
	}

	return detail.MCPConfig, nil
}

func (s *assetService) ParseSkillFile(filename string, data []byte) (*model.SkillParseResult, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		name, desc, body := skillpkg.ParseSkillMarkdown(string(data))
		return &model.SkillParseResult{
			Name:        name,
			Description: desc,
			Content:     body,
			Files:       []string{filepath.Base(filename)},
		}, nil
	case ".zip":
		name, desc, body, files, err := skillpkg.ParseSkillZip(data)
		if err != nil {
			return nil, NewBadRequest(err.Error())
		}
		return &model.SkillParseResult{
			Name:        name,
			Description: desc,
			Content:     body,
			Files:       files,
		}, nil
	default:
		return nil, NewBadRequest("unsupported skill file; allowed: .md, .markdown, .zip")
	}
}

func (s *assetService) ImportSkillFromURL(rawURL string) (*model.SkillImportResult, error) {
	candidates, err := skillpkg.ResolveSkillsShURL(rawURL)
	if err != nil {
		return nil, NewBadRequest(err.Error())
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for _, candidate := range candidates {
		req, err := http.NewRequest(http.MethodGet, candidate, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusOK {
			name, desc, body := skillpkg.ParseSkillMarkdown(string(bodyBytes))
			return &model.SkillImportResult{
				Name:        name,
				Description: desc,
				Content:     body,
				SourceURL:   rawURL,
			}, nil
		}
		lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, candidate)
	}

	msg := "failed to fetch skill from URL"
	if lastErr != nil {
		msg = fmt.Sprintf("failed to fetch skill from URL: %v", lastErr)
	}
	return nil, &AppError{
		HTTPStatus: http.StatusBadGateway,
		Code:       response.CodeInternal,
		Message:    msg,
	}
}

func (s *assetService) UploadSkillPackage(userID uint64, isSuperAdmin bool, id uint64, filename string, data []byte) (*model.AssetDetail, error) {
	asset, err := s.ensureManagePermission(userID, isSuperAdmin, id)
	if err != nil {
		return nil, err
	}
	if asset.AssetType != model.AssetTypeSkill {
		return nil, NewBadRequest("asset is not a skill")
	}

	parsed, err := s.ParseSkillFile(filename, data)
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(s.uploadDir, "skills", fmt.Sprintf("%d", id))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	safeName := filepath.Base(filename)
	dstPath := filepath.Join(dir, safeName)
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return nil, err
	}

	if err := s.assetRepo.UpdateSkillPackage(id, dstPath, model.StringSlice(parsed.Files), parsed.Content); err != nil {
		return nil, err
	}
	return s.assetRepo.GetDetailByID(id)
}

func (s *assetService) DownloadSkillPackage(userID uint64, isSuperAdmin bool, id uint64, format string) (string, string, []byte, error) {
	detail, err := s.GetAssetDetail(userID, isSuperAdmin, id)
	if err != nil {
		return "", "", nil, err
	}
	if detail.AssetType != model.AssetTypeSkill {
		return "", "", nil, NewBadRequest("asset is not a skill")
	}

	body := ""
	pkgPath := ""
	if detail.SkillConfig != nil {
		body = detail.SkillConfig.Content
		pkgPath = detail.SkillConfig.PackagePath
	}

	if strings.EqualFold(format, "md") {
		md := skillpkg.BuildSkillMarkdown(detail.Name, detail.Summary, body)
		return detail.AssetCode + "-SKILL.md", "text/markdown; charset=utf-8", []byte(md), nil
	}

	if pkgPath != "" && strings.HasSuffix(strings.ToLower(pkgPath), ".zip") {
		if diskBytes, readErr := os.ReadFile(pkgPath); readErr == nil {
			return detail.AssetCode + ".zip", "application/zip", diskBytes, nil
		}
	}

	zipBytes, err := skillpkg.BuildSkillZip(detail.AssetCode, detail.Name, detail.Summary, body)
	if err != nil {
		return "", "", nil, err
	}
	return detail.AssetCode + ".zip", "application/zip", zipBytes, nil
}

func (s *assetService) SearchMCPRegistry(ctx context.Context, query string, limit int) ([]mcpregistry.MCPImportItem, error) {
	items, err := mcpregistry.SearchServers(ctx, query, limit)
	if err != nil {
		return nil, &AppError{
			HTTPStatus: http.StatusBadGateway,
			Code:       response.CodeInternal,
			Message:    fmt.Sprintf("failed to query MCP registry: %v", err),
		}
	}
	return items, nil
}

func (s *assetService) ParseMCPConfigJSON(raw string) ([]mcpregistry.MCPImportItem, error) {
	items, err := mcpregistry.ParseMCPConfigJSON(raw)
	if err != nil {
		return nil, NewBadRequest(err.Error())
	}
	return items, nil
}

func (s *assetService) ParseAgentFile(raw, filename string) (*agentpkg.AgentImportResult, error) {
	res, err := agentpkg.ParseAgentContent(raw, filename)
	if err != nil {
		return nil, NewBadRequest(err.Error())
	}
	return &res, nil
}

func (s *assetService) ImportAgentFromURL(ctx context.Context, rawURL string) (*agentpkg.AgentImportResult, error) {
	res, err := agentpkg.ImportAgentFromURL(ctx, rawURL)
	if err != nil {
		return nil, &AppError{
			HTTPStatus: http.StatusBadGateway,
			Code:       response.CodeInternal,
			Message:    fmt.Sprintf("failed to import agent from URL: %v", err),
		}
	}
	return &res, nil
}

func (s *assetService) DownloadAgentPackage(userID uint64, isSuperAdmin bool, id uint64, format string) (string, string, []byte, error) {
	detail, err := s.GetAssetDetail(userID, isSuperAdmin, id)
	if err != nil {
		return "", "", nil, err
	}
	if detail.AssetType != model.AssetTypeAgent {
		return "", "", nil, NewBadRequest("asset is not an agent")
	}

	var provider, modelName, prompt string
	temp := 0.70
	maxTokens := 2048
	if detail.AgentConfig != nil {
		provider = detail.AgentConfig.ModelProvider
		modelName = detail.AgentConfig.ModelName
		prompt = detail.AgentConfig.SystemPrompt
		temp = detail.AgentConfig.Temperature
		maxTokens = detail.AgentConfig.MaxTokens
	}

	if strings.EqualFold(format, "json") {
		payload := map[string]interface{}{
			"slug":           detail.AssetCode,
			"name":           detail.Name,
			"description":    detail.Summary,
			"model_provider": provider,
			"model_name":     modelName,
			"system_prompt":  prompt,
			"temperature":    temp,
			"max_tokens":     maxTokens,
			"tags":           detail.Tags,
		}
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", "", nil, err
		}
		return detail.AssetCode + ".agent.json", "application/json; charset=utf-8", b, nil
	}

	md := agentpkg.BuildAgentMarkdown(detail.AssetCode, detail.Name, detail.Summary, provider, modelName, prompt, temp, maxTokens)
	return detail.AssetCode + ".agent.md", "text/markdown; charset=utf-8", []byte(md), nil
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

func bumpVersion(current string) string {
	v := strings.TrimPrefix(strings.TrimSpace(current), "v")
	if v == "" || strings.HasPrefix(v, "0.") {
		return "v1.0.0"
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return "v1.0.0"
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "v1.0.0"
	}
	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch+1)
}

func toUint64Set(ids []uint64) map[uint64]bool {
	set := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
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
	var endpoint, authEnc, sourceURL, pkgCmd string
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
		sourceURL = in.SourceURL
		pkgCmd = in.PackageCommand
	}
	return &model.AIAssetMCP{
		AssetID:        assetID,
		TransportType:  transport,
		EndpointURL:    endpoint,
		AuthType:       authType,
		AuthConfigEnc:  authEnc,
		ToolsSchema:    tools,
		HealthStatus:   "unknown",
		SourceURL:      sourceURL,
		PackageCommand: pkgCmd,
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
		m.SourceURL = in.SourceURL
		m.PackagePath = in.PackagePath
		m.PackageFiles = in.PackageFiles
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
		m.SourceURL = in.SourceURL
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
