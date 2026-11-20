package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// --- Minimal in-memory mocks for P1 unit tests ---

type mockP1UserRepo struct {
	users map[uint64]*model.SysUser
}

func (m *mockP1UserRepo) CreateWithRole(user *model.SysUser, roleCode string) error { return nil }
func (m *mockP1UserRepo) FindByID(id uint64) (*model.SysUser, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1UserRepo) FindByUsername(username string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1UserRepo) FindByEmail(email string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1UserRepo) GetUserRoles(userID uint64) ([]string, error) {
	return []string{model.RoleDeveloper}, nil
}
func (m *mockP1UserRepo) UpdateProfile(userID uint64, nickname, email, avatar string) error {
	return nil
}
func (m *mockP1UserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	return nil
}
func (m *mockP1UserRepo) IncrementTokenVersion(userID uint64) error          { return nil }
func (m *mockP1UserRepo) UpdateLastLogin(userID uint64, t time.Time) error { return nil }

type mockP1DomainRepo struct {
	domains map[uint64]*model.BizDomain
	members map[uint64]map[uint64]string
}

func (m *mockP1DomainRepo) ListDomains(userID uint64, q model.DomainQuery, userOrgIDs []uint64) ([]model.DomainListItem, error) {
	return nil, nil
}
func (m *mockP1DomainRepo) FindByID(id uint64) (*model.BizDomain, error) {
	if d, ok := m.domains[id]; ok {
		return d, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1DomainRepo) FindByCode(code string) (*model.BizDomain, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1DomainRepo) FindByUID(uid string) (*model.BizDomain, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1DomainRepo) CreateWithOwner(domain *model.BizDomain) error { return nil }
func (m *mockP1DomainRepo) UpdateDomain(domain *model.BizDomain) error    { return nil }
func (m *mockP1DomainRepo) ListMembers(domainID uint64) ([]model.DomainMemberItem, error) {
	return nil, nil
}
func (m *mockP1DomainRepo) GetMemberRole(domainID, userID uint64) (string, error) {
	if m.members[domainID] != nil {
		return m.members[domainID][userID], nil
	}
	return "", nil
}
func (m *mockP1DomainRepo) UpsertMember(member *model.DomainMember) error     { return nil }
func (m *mockP1DomainRepo) RemoveMember(domainID, userID uint64) error        { return nil }
func (m *mockP1DomainRepo) GetUserDomainIDs(userID uint64) ([]uint64, error)  { return []uint64{}, nil }
func (m *mockP1DomainRepo) GetUserAdminDomainIDs(userID uint64) ([]uint64, error) {
	return []uint64{}, nil
}
func (m *mockP1DomainRepo) GetAccessibleDomainIDs(userID uint64, userOrgIDs []uint64) ([]uint64, error) {
	return []uint64{}, nil
}

type mockP1AssetRepo struct {
	assets map[uint64]*model.AIAsset
}

func (m *mockP1AssetRepo) ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs, permittedAssetIDs []uint64) ([]model.AssetListItem, int64, error) {
	return nil, 0, nil
}
func (m *mockP1AssetRepo) ListUserAssets(q model.AssetQuery) ([]model.AssetListItem, int64, error) {
	return nil, 0, nil
}
func (m *mockP1AssetRepo) FindByID(id uint64) (*model.AIAsset, error) {
	if a, ok := m.assets[id]; ok {
		return a, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1AssetRepo) FindByCode(code string) (*model.AIAsset, error) {
	for _, a := range m.assets {
		if a.AssetCode == code {
			return a, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1AssetRepo) GetDetailByID(id uint64) (*model.AssetDetail, error) {
	a, ok := m.assets[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.AssetDetail{AIAsset: *a}, nil
}
func (m *mockP1AssetRepo) GetDetailByCode(code string) (*model.AssetDetail, error) {
	for _, a := range m.assets {
		if a.AssetCode == code {
			return &model.AssetDetail{
				AIAsset:     *a,
				SkillConfig: &model.AIAssetSkill{AssetID: a.ID, SkillType: "prompt", Content: "hello"},
			}, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1AssetRepo) CreateAsset(asset *model.AIAsset, ext interface{}) error { return nil }
func (m *mockP1AssetRepo) UpdateAsset(asset *model.AIAsset, ext interface{}) error { return nil }
func (m *mockP1AssetRepo) DeleteAsset(id uint64) error                             { return nil }
func (m *mockP1AssetRepo) UpdateStatus(id uint64, status string, version string) error {
	return nil
}
func (m *mockP1AssetRepo) PublishWithSnapshot(assetID uint64, version, changelog string, snapshot model.JSONRaw, publisherID uint64) error {
	return nil
}
func (m *mockP1AssetRepo) ListVersions(assetID uint64) ([]model.AIAssetVersion, error) {
	return nil, nil
}
func (m *mockP1AssetRepo) FindVersionByID(assetID, versionID uint64) (*model.AIAssetVersion, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1AssetRepo) RestoreFromSnapshot(asset *model.AIAsset, ext interface{}, versionID uint64, version string) error {
	return nil
}
func (m *mockP1AssetRepo) UpdateMCPHealth(assetID uint64, healthStatus string, checkedAt time.Time) error {
	return nil
}
func (m *mockP1AssetRepo) UpdateMCPTools(assetID uint64, toolsSchema model.JSONRaw) error {
	return nil
}
func (m *mockP1AssetRepo) UpdateSkillPackage(assetID uint64, packagePath string, packageFiles model.StringSlice, content string) error {
	return nil
}
func (m *mockP1AssetRepo) IncrementInvokeCount(assetID uint64) error {
	if a, ok := m.assets[assetID]; ok {
		a.InvokeCount++
	}
	return nil
}

type mockP1GovRepo struct {
	nextReqID uint64
	nextRecID uint64
	reqs      map[uint64]*model.ApprovalRequest
	recs      map[uint64][]model.ApprovalRecord
	perms     []*model.AssetPermission
}

func newMockP1GovRepo() *mockP1GovRepo {
	return &mockP1GovRepo{
		nextReqID: 1,
		nextRecID: 1,
		reqs:      make(map[uint64]*model.ApprovalRequest),
		recs:      make(map[uint64][]model.ApprovalRecord),
	}
}

func (m *mockP1GovRepo) GetActiveUserPermission(assetID, userID uint64) (*model.AssetPermission, error) {
	for _, p := range m.perms {
		if p.AssetID == assetID && p.GranteeType == model.GranteeTypeUser && p.GranteeID == userID && p.Status == model.PermStatusActive {
			if p.ExpiresAt == nil || p.ExpiresAt.After(time.Now()) {
				return p, nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1GovRepo) GetUserPermittedAssetIDs(userID uint64) ([]uint64, error)  { return nil, nil }
func (m *mockP1GovRepo) GetUserSubscribedAssetIDs(userID uint64) ([]uint64, error) { return nil, nil }
func (m *mockP1GovRepo) UpsertPermission(perm *model.AssetPermission) error {
	m.perms = append(m.perms, perm)
	return nil
}
func (m *mockP1GovRepo) CreateApprovalWithRecords(req *model.ApprovalRequest, records []model.ApprovalRecord) error {
	req.ID = m.nextReqID
	m.nextReqID++
	cp := *req
	m.reqs[req.ID] = &cp
	for i := range records {
		records[i].ID = m.nextRecID
		m.nextRecID++
		records[i].RequestID = req.ID
	}
	m.recs[req.ID] = records
	return nil
}
func (m *mockP1GovRepo) FindApprovalByID(id uint64) (*model.ApprovalRequest, error) {
	if r, ok := m.reqs[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1GovRepo) ListApprovalRecords(requestID uint64) ([]model.ApprovalRecordView, error) {
	var out []model.ApprovalRecordView
	for _, rec := range m.recs[requestID] {
		out = append(out, model.ApprovalRecordView{ApprovalRecord: rec})
	}
	return out, nil
}
func (m *mockP1GovRepo) ListMyRequests(applicantID uint64) ([]model.ApprovalRequestView, error) {
	return nil, nil
}
func (m *mockP1GovRepo) ListPendingRequests(approverID uint64, isSuperAdmin bool) ([]model.ApprovalRequestView, error) {
	return nil, nil
}
func (m *mockP1GovRepo) ListHandledRequests(approverID uint64) ([]model.ApprovalRequestView, error) {
	return nil, nil
}
func (m *mockP1GovRepo) HandleApprovalStep(req *model.ApprovalRequest, recordID uint64, action, comment string, grantPerm *model.AssetPermission) error {
	stored := m.reqs[req.ID]
	for i := range m.recs[req.ID] {
		if m.recs[req.ID][i].ID == recordID {
			m.recs[req.ID][i].Action = action
			m.recs[req.ID][i].Comment = comment
		}
	}
	if action == model.ApprovalActionReject {
		stored.Status = model.ApprovalStatusRejected
	} else if stored.CurrentStep < stored.TotalSteps {
		stored.CurrentStep++
	} else {
		stored.Status = model.ApprovalStatusApproved
	}
	if grantPerm != nil {
		m.perms = append(m.perms, grantPerm)
	}
	return nil
}

type mockP1KeyRepo struct {
	nextID uint64
	keys   map[uint64]*model.UserAPIKey
}

func newMockP1KeyRepo() *mockP1KeyRepo {
	return &mockP1KeyRepo{nextID: 1, keys: make(map[uint64]*model.UserAPIKey)}
}

func (m *mockP1KeyRepo) ListByUser(userID uint64) ([]model.UserAPIKey, error) {
	var out []model.UserAPIKey
	for _, k := range m.keys {
		if k.UserID == userID {
			out = append(out, *k)
		}
	}
	return out, nil
}
func (m *mockP1KeyRepo) Create(key *model.UserAPIKey) error {
	key.ID = m.nextID
	m.nextID++
	cp := *key
	m.keys[key.ID] = &cp
	return nil
}
func (m *mockP1KeyRepo) FindActiveByHash(keyHash string) (*model.UserAPIKey, error) {
	for _, k := range m.keys {
		if k.KeyHash == keyHash && k.Status == 1 {
			return k, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockP1KeyRepo) Revoke(id, userID uint64) error {
	if k, ok := m.keys[id]; ok && k.UserID == userID {
		k.Status = 0
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *mockP1KeyRepo) TouchLastUsed(id uint64, t time.Time) error { return nil }

type mockP1LogRepo struct {
	logs []*model.AssetInvokeLog
}

func (m *mockP1LogRepo) Create(log *model.AssetInvokeLog) error {
	m.logs = append(m.logs, log)
	return nil
}
func (m *mockP1LogRepo) ListByCaller(callerUserID uint64, assetType string, isSuccess *int8, page, pageSize int) ([]model.InvokeLogItem, int64, error) {
	return nil, 0, nil
}

// --- P1 Unit Tests ---

func TestMultiStepApprovalAndInvokeGuard(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			10: {ID: 10, Username: "applicant", Status: 1},
			20: {ID: 20, Username: "asset_owner", Status: 1},
			30: {ID: 30, Username: "domain_owner", Status: 1},
		},
	}
	domainRepo := &mockP1DomainRepo{
		domains: map[uint64]*model.BizDomain{
			1: {ID: 1, DomainCode: "finance", DomainName: "Finance", OwnerID: 30},
		},
		members: map[uint64]map[uint64]string{},
	}
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			100: {
				ID:         100,
				AssetCode:  "skill-private",
				AssetType:  model.AssetTypeSkill,
				Name:       "Private Skill",
				DomainID:   1,
				OwnerID:    20,
				Visibility: model.VisibilityPrivate,
				Status:     model.AssetStatusPublished,
			},
		},
	}
	govRepo := newMockP1GovRepo()
	logRepo := &mockP1LogRepo{}
	keyRepo := newMockP1KeyRepo()

	govSvc := NewGovernanceService(govRepo, assetRepo, domainRepo, nil)
	keySvc := NewAPIKeyService(keyRepo)
	invokeSvc := NewInvokeService(assetRepo, domainRepo, govRepo, repository.KBRepository(nil), logRepo, nil)

	// 1. Create API key for applicant (user 10)
	plainKey, keyInfo, err := keySvc.CreateKey(10, model.CreateAPIKeyRequest{KeyName: "ci-key", ExpiresDays: 7})
	if err != nil {
		t.Fatalf("CreateKey failed: %v", err)
	}
	if keyInfo.KeyPrefix == "" || plainKey == "" {
		t.Fatal("expected non-empty api key")
	}

	// Setup Echo with UnifiedAuthMiddleware + AssetGuardMiddleware
	e := echo.New()
	authMW := middleware.UnifiedAuthMiddleware("secret", userRepo, keyRepo)
	guardMW := middleware.AssetGuardMiddleware(invokeSvc)
	handler := authMW(guardMW(func(c echo.Context) error {
		res, err := invokeSvc.InvokeAsset(
			middleware.GetUserID(c),
			middleware.HasRole(c, model.RoleSuperAdmin),
			c.Param("assetCode"),
			model.AssetTypeSkill,
			middleware.GetAuthMode(c),
			c.RealIP(),
			"trace-1",
			map[string]interface{}{"x": 1},
		)
		if err != nil {
			return HandleError(c, err)
		}
		return c.JSON(http.StatusOK, res)
	}))

	// 2. Before approval, invoke with API key -> 403 and logged as failure
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoke/skill/skill-private", nil)
	req.Header.Set("X-API-Key", plainKey)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("assetCode")
	c.SetParamValues("skill-private")
	_ = handler(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 before approval, got %d", rec.Code)
	}
	if len(logRepo.logs) != 1 || logRepo.logs[0].IsSuccess != 0 || logRepo.logs[0].AuthMode != model.AuthModeAPIKey {
		t.Fatalf("expected failed api_key invoke log, got %+v", logRepo.logs)
	}

	// 3. Create approval request -> Step 1 approve -> Step 2 approve
	apr, err := govSvc.CreateApproval(10, model.CreateApprovalRequest{
		AssetID:      100,
		DurationDays: 30,
		Reason:       "need access",
	})
	if err != nil {
		t.Fatalf("CreateApproval failed: %v", err)
	}

	// Wrong approver on step 1 -> forbidden
	if _, err := govSvc.HandleApproval(30, false, apr.ID, model.HandleApprovalRequest{Action: "approve"}); err == nil {
		t.Fatal("expected error when non-step1-approver tries to approve step 1")
	}

	// Step 1 by asset owner (20)
	aprAfter1, err := govSvc.HandleApproval(20, false, apr.ID, model.HandleApprovalRequest{Action: "approve", Comment: "ok1"})
	if err != nil {
		t.Fatalf("step 1 approve failed: %v", err)
	}
	if aprAfter1.CurrentStep != 2 || aprAfter1.Status != model.ApprovalStatusPending {
		t.Fatalf("expected step 2 pending, got %+v", aprAfter1)
	}

	// Step 2 by domain owner (30)
	aprAfter2, err := govSvc.HandleApproval(30, false, apr.ID, model.HandleApprovalRequest{Action: "approve", Comment: "ok2"})
	if err != nil {
		t.Fatalf("step 2 approve failed: %v", err)
	}
	if aprAfter2.Status != model.ApprovalStatusApproved {
		t.Fatalf("expected approved, got %+v", aprAfter2)
	}

	// 4. After approval, invoke with API key -> 200 and invoke_count incremented
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/invoke/skill/skill-private", nil)
	req2.Header.Set("X-API-Key", plainKey)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("assetCode")
	c2.SetParamValues("skill-private")
	if err := handler(c2); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 after approval, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if assetRepo.assets[100].InvokeCount != 1 {
		t.Fatalf("expected invoke_count=1, got %d", assetRepo.assets[100].InvokeCount)
	}

	// 5. Revoke API key -> 401
	if err := keySvc.RevokeKey(10, keyInfo.ID); err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/invoke/skill/skill-private", nil)
	req3.Header.Set("X-API-Key", plainKey)
	rec3 := httptest.NewRecorder()
	c3 := e.NewContext(req3, rec3)
	c3.SetParamNames("assetCode")
	c3.SetParamValues("skill-private")
	_ = handler(c3)
	if rec3.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after key revocation, got %d", rec3.Code)
	}
}
