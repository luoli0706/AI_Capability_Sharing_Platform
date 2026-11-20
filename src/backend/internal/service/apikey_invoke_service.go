package service

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type APIKeyService interface {
	ListKeys(userID uint64) ([]model.UserAPIKey, error)
	CreateKey(userID uint64, req model.CreateAPIKeyRequest) (string, *model.UserAPIKey, error)
	RevokeKey(userID, keyID uint64) error
}

type apiKeyService struct {
	keyRepo repository.APIKeyRepository
}

func NewAPIKeyService(keyRepo repository.APIKeyRepository) APIKeyService {
	return &apiKeyService{keyRepo: keyRepo}
}

func (s *apiKeyService) ListKeys(userID uint64) ([]model.UserAPIKey, error) {
	return s.keyRepo.ListByUser(userID)
}

func (s *apiKeyService) CreateKey(userID uint64, req model.CreateAPIKeyRequest) (string, *model.UserAPIKey, error) {
	rawBytes := make([]byte, 16)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", nil, err
	}
	plainKey := "ascp_" + hex.EncodeToString(rawBytes)
	prefix := plainKey[:12]
	hash := HashAPIKey(plainKey)

	var expiresAt *time.Time
	if req.ExpiresDays > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresDays) * 24 * time.Hour)
		expiresAt = &t
	}

	key := &model.UserAPIKey{
		UserID:    userID,
		KeyName:   req.KeyName,
		KeyPrefix: prefix,
		KeyHash:   hash,
		ExpiresAt: expiresAt,
		Status:    1,
	}
	if err := s.keyRepo.Create(key); err != nil {
		return "", nil, err
	}
	return plainKey, key, nil
}

func (s *apiKeyService) RevokeKey(userID, keyID uint64) error {
	err := s.keyRepo.Revoke(keyID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NewNotFound("api key not found")
	}
	return err
}

func HashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

type InvokeService interface {
	CheckAccess(userID uint64, isSuperAdmin bool, assetCode string) (*model.AssetDetail, error)
	LogDenied(userID uint64, assetCode, authMode, clientIP, traceID string, err error)
	InvokeAsset(userID uint64, isSuperAdmin bool, assetCode, expectedType, authMode, clientIP, traceID string, payload map[string]interface{}) (interface{}, error)
	ListInvokeLogs(userID uint64, assetType string, isSuccess *int8, page, pageSize int) ([]model.InvokeLogItem, int64, error)
}

type invokeService struct {
	assetRepo  repository.AssetRepository
	domainRepo repository.DomainRepository
	govRepo    repository.GovernanceRepository
	kbRepo     repository.KBRepository
	logRepo    repository.InvokeLogRepository
	orgRepo    repository.OrgRepository
	httpClient *http.Client
}

func NewInvokeService(
	assetRepo repository.AssetRepository,
	domainRepo repository.DomainRepository,
	govRepo repository.GovernanceRepository,
	kbRepo repository.KBRepository,
	logRepo repository.InvokeLogRepository,
	orgRepo repository.OrgRepository,
) InvokeService {
	return &invokeService{
		assetRepo:  assetRepo,
		domainRepo: domainRepo,
		govRepo:    govRepo,
		kbRepo:     kbRepo,
		logRepo:    logRepo,
		orgRepo:    orgRepo,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *invokeService) CheckAccess(userID uint64, isSuperAdmin bool, assetCode string) (*model.AssetDetail, error) {
	detail, err := s.assetRepo.GetDetailByCode(assetCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("asset not found")
		}
		return nil, err
	}

	if detail.Status != model.AssetStatusPublished {
		return nil, NewForbidden("asset is not published")
	}

	if isSuperAdmin || detail.OwnerID == userID || detail.Visibility == model.VisibilityPublic {
		return detail, nil
	}

	if detail.Visibility == model.VisibilityDomain {
		ok, err := hasDomainAccess(s.domainRepo, s.orgRepo, detail.DomainID, userID)
		if err != nil {
			return nil, err
		}
		if ok {
			return detail, nil
		}
	}

	if s.govRepo != nil {
		if perm, err := s.govRepo.GetActiveUserPermission(detail.ID, userID); err == nil && perm != nil {
			return detail, nil
		}
	}

	return nil, NewForbidden("no permission to invoke this asset")
}

func (s *invokeService) LogDenied(userID uint64, assetCode, authMode, clientIP, traceID string, err error) {
	if traceID == "" {
		traceID = fmt.Sprintf("tr-%d", time.Now().UnixNano())
	}
	statusCode := http.StatusForbidden
	var appErr *AppError
	if errors.As(err, &appErr) {
		statusCode = appErr.HTTPStatus
	}
	var assetID uint64
	var aType, aVer string
	if found, fErr := s.assetRepo.FindByCode(assetCode); fErr == nil {
		assetID = found.ID
		aType = found.AssetType
		aVer = found.CurrentVersion
	}
	s.recordLog(traceID, assetID, aType, aVer, userID, authMode, statusCode, 0, time.Now(), err.Error(), clientIP)
}

func (s *invokeService) InvokeAsset(
	userID uint64,
	isSuperAdmin bool,
	assetCode, expectedType, authMode, clientIP, traceID string,
	payload map[string]interface{},
) (interface{}, error) {
	start := time.Now()
	if traceID == "" {
		traceID = fmt.Sprintf("tr-%d", start.UnixNano())
	}

	detail, err := s.CheckAccess(userID, isSuperAdmin, assetCode)
	if err != nil {
		var appErr *AppError
		statusCode := http.StatusForbidden
		if errors.As(err, &appErr) {
			statusCode = appErr.HTTPStatus
		}
		var assetID uint64
		var aType, aVer string
		if found, fErr := s.assetRepo.FindByCode(assetCode); fErr == nil {
			assetID = found.ID
			aType = found.AssetType
			aVer = found.CurrentVersion
		}
		s.recordLog(traceID, assetID, aType, aVer, userID, authMode, statusCode, 0, start, err.Error(), clientIP)
		return nil, err
	}

	if expectedType != "" && detail.AssetType != expectedType {
		err = NewBadRequest(fmt.Sprintf("asset %s is type %s, expected %s", assetCode, detail.AssetType, expectedType))
		s.recordLog(traceID, detail.ID, detail.AssetType, detail.CurrentVersion, userID, authMode, http.StatusBadRequest, 0, start, err.Error(), clientIP)
		return nil, err
	}

	res, err := s.executeAsset(detail, payload)
	if err != nil {
		statusCode := http.StatusInternalServerError
		var appErr *AppError
		if errors.As(err, &appErr) {
			statusCode = appErr.HTTPStatus
		}
		s.recordLog(traceID, detail.ID, detail.AssetType, detail.CurrentVersion, userID, authMode, statusCode, 0, start, err.Error(), clientIP)
		return nil, err
	}

	_ = s.assetRepo.IncrementInvokeCount(detail.ID)
	s.recordLog(traceID, detail.ID, detail.AssetType, detail.CurrentVersion, userID, authMode, http.StatusOK, 1, start, "", clientIP)
	return res, nil
}

func (s *invokeService) executeAsset(detail *model.AssetDetail, payload map[string]interface{}) (interface{}, error) {
	switch detail.AssetType {
	case model.AssetTypeKB:
		query, _ := payload["query"].(string)
		if query == "" {
			return nil, NewBadRequest("query is required for kb invocation")
		}
		topK := 5
		if v, ok := payload["top_k"].(float64); ok && v > 0 {
			topK = int(v)
		}
		hits, err := s.kbRepo.SearchChunks(detail.ID, query, topK)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"asset_code": detail.AssetCode,
			"asset_type": detail.AssetType,
			"query":      query,
			"hits":       hits,
		}, nil

	case model.AssetTypeMCP:
		endpoint := ""
		if detail.MCPConfig != nil {
			endpoint = detail.MCPConfig.EndpointURL
		}
		if endpoint != "" {
			bodyBytes, _ := json.Marshal(payload)
			if req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes)); err == nil {
				req.Header.Set("Content-Type", "application/json")
				if resp, err := s.httpClient.Do(req); err == nil {
					raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
					_ = resp.Body.Close()
					var upstream interface{}
					if json.Unmarshal(raw, &upstream) == nil && resp.StatusCode < 500 {
						return map[string]interface{}{
							"asset_code": detail.AssetCode,
							"asset_type": detail.AssetType,
							"proxied":    true,
							"result":     upstream,
						}, nil
					}
				}
			}
		}
		return map[string]interface{}{
			"asset_code": detail.AssetCode,
			"asset_type": detail.AssetType,
			"proxied":    false,
			"endpoint":   endpoint,
			"echo":       payload,
		}, nil

	case model.AssetTypeSkill:
		skillType := "prompt"
		content := ""
		if detail.SkillConfig != nil {
			skillType = detail.SkillConfig.SkillType
			content = detail.SkillConfig.Content
		}
		return map[string]interface{}{
			"asset_code": detail.AssetCode,
			"asset_type": detail.AssetType,
			"skill_type": skillType,
			"content":    content,
			"input":      payload,
		}, nil

	case model.AssetTypeAgent:
		modelName := ""
		prompt := ""
		if detail.AgentConfig != nil {
			modelName = detail.AgentConfig.ModelName
			prompt = detail.AgentConfig.SystemPrompt
		}
		inputMsg, _ := payload["message"].(string)
		return map[string]interface{}{
			"asset_code":    detail.AssetCode,
			"asset_type":    detail.AssetType,
			"model_name":    modelName,
			"system_prompt": prompt,
			"reply":         fmt.Sprintf("[Agent %s] received: %s", detail.Name, inputMsg),
		}, nil

	default:
		return nil, NewBadRequest("unsupported asset_type")
	}
}

func (s *invokeService) recordLog(
	traceID string,
	assetID uint64,
	assetType, assetVersion string,
	callerID uint64,
	authMode string,
	statusCode int,
	isSuccess int8,
	start time.Time,
	errMsg, clientIP string,
) {
	if s.logRepo == nil {
		return
	}
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	_ = s.logRepo.Create(&model.AssetInvokeLog{
		TraceID:      traceID,
		AssetID:      assetID,
		AssetType:    assetType,
		AssetVersion: assetVersion,
		CallerUserID: callerID,
		AuthMode:     authMode,
		StatusCode:   statusCode,
		IsSuccess:    isSuccess,
		LatencyMS:    uint(time.Since(start).Milliseconds()),
		ErrorMessage: errMsg,
		ClientIP:     clientIP,
	})
}

func (s *invokeService) ListInvokeLogs(userID uint64, assetType string, isSuccess *int8, page, pageSize int) ([]model.InvokeLogItem, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.logRepo.ListByCaller(userID, assetType, isSuccess, page, pageSize)
}
