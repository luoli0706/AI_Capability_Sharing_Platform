package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/pkg/validator"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// --- In-memory mock repositories ---

type memUserRepo struct {
	nextID uint64
	users  map[uint64]*model.SysUser
	roles  map[uint64][]string
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{
		nextID: 1,
		users:  make(map[uint64]*model.SysUser),
		roles:  make(map[uint64][]string),
	}
}

func (m *memUserRepo) CreateWithRole(user *model.SysUser, roleCode string) error {
	user.ID = m.nextID
	m.nextID++
	user.CreatedAt = time.Now()
	cp := *user
	m.users[user.ID] = &cp
	m.roles[user.ID] = []string{roleCode}
	return nil
}
func (m *memUserRepo) FindByID(id uint64) (*model.SysUser, error) {
	if u, ok := m.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memUserRepo) FindByUsername(username string) (*model.SysUser, error) {
	for _, u := range m.users {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memUserRepo) FindByEmail(email string) (*model.SysUser, error) {
	for _, u := range m.users {
		if u.Email != nil && *u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memUserRepo) GetUserRoles(userID uint64) ([]string, error) {
	return m.roles[userID], nil
}
func (m *memUserRepo) UpdateProfile(userID uint64, nickname string, email *string, avatar, bio string, favoritesPublic bool) error {
	if u, ok := m.users[userID]; ok {
		u.Nickname = nickname
		u.Email = email
		u.Avatar = avatar
		u.Bio = bio
		u.FavoritesPublic = favoritesPublic
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *memUserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	if u, ok := m.users[userID]; ok {
		u.PasswordHash = passwordHash
		u.TokenVersion++
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *memUserRepo) IncrementTokenVersion(userID uint64) error {
	if u, ok := m.users[userID]; ok {
		u.TokenVersion++
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *memUserRepo) UpdateLastLogin(userID uint64, t time.Time) error { return nil }
func (m *memUserRepo) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	return []model.UserBrief{}, nil
}
func (m *memUserRepo) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	res := make(map[uint64]model.UserBrief)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			res[id] = model.ToUserBrief(u)
		}
	}
	return res, nil
}

type memDomainRepo struct {
	nextID  uint64
	domains map[uint64]*model.BizDomain
	members map[uint64]map[uint64]string
}

func newMemDomainRepo() *memDomainRepo {
	return &memDomainRepo{
		nextID:  1,
		domains: make(map[uint64]*model.BizDomain),
		members: make(map[uint64]map[uint64]string),
	}
}

func (m *memDomainRepo) ListDomains(userID uint64, q model.DomainQuery, userOrgIDs, adminOrgIDs []uint64) ([]model.DomainListItem, int64, error) {
	var list []model.DomainListItem
	for _, d := range m.domains {
		list = append(list, model.DomainListItem{
			ID:          d.ID,
			DomainUID:   d.DomainUID,
			DomainCode:  d.DomainCode,
			DomainName:  d.DomainName,
			Description: d.Description,
			Visibility:  d.Visibility,
			OrgID:       d.OrgID,
			OrgName:     d.OrgName,
			OwnerID:     d.OwnerID,
			Status:      d.Status,
			MyRole:      m.members[d.ID][userID],
			CanManage:   d.OwnerID == userID,
			MemberCount: int64(len(m.members[d.ID])),
			CreatedAt:   d.CreatedAt,
		})
	}
	return list, int64(len(list)), nil
}
func (m *memDomainRepo) FindByID(id uint64) (*model.BizDomain, error) {
	if d, ok := m.domains[id]; ok {
		return d, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memDomainRepo) FindByCode(code string) (*model.BizDomain, error) {
	for _, d := range m.domains {
		if d.DomainCode == code {
			return d, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memDomainRepo) FindByUID(uid string) (*model.BizDomain, error) {
	for _, d := range m.domains {
		if d.DomainUID == uid {
			return d, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memDomainRepo) CreateWithOwner(domain *model.BizDomain) error {
	domain.ID = m.nextID
	m.nextID++
	domain.CreatedAt = time.Now()
	m.domains[domain.ID] = domain
	m.members[domain.ID] = map[uint64]string{domain.OwnerID: model.DomainRoleOwner}
	return nil
}
func (m *memDomainRepo) UpdateDomain(domain *model.BizDomain) error {
	m.domains[domain.ID] = domain
	return nil
}
func (m *memDomainRepo) DeleteDomain(id uint64) error {
	delete(m.domains, id)
	return nil
}
func (m *memDomainRepo) ListMembers(domainID uint64) ([]model.DomainMemberItem, error) {
	var list []model.DomainMemberItem
	for uid, role := range m.members[domainID] {
		list = append(list, model.DomainMemberItem{UserID: uid, DomainRole: role})
	}
	return list, nil
}
func (m *memDomainRepo) GetMemberRole(domainID, userID uint64) (string, error) {
	if roles, ok := m.members[domainID]; ok {
		return roles[userID], nil
	}
	return "", nil
}
func (m *memDomainRepo) UpsertMember(member *model.DomainMember) error {
	if _, ok := m.members[member.DomainID]; !ok {
		m.members[member.DomainID] = make(map[uint64]string)
	}
	m.members[member.DomainID][member.UserID] = member.DomainRole
	return nil
}
func (m *memDomainRepo) RemoveMember(domainID, userID uint64) error {
	if roles, ok := m.members[domainID]; ok {
		delete(roles, userID)
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *memDomainRepo) GetUserDomainIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	for did, roles := range m.members {
		if _, ok := roles[userID]; ok {
			ids = append(ids, did)
		}
	}
	return ids, nil
}
func (m *memDomainRepo) GetUserAdminDomainIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	for did, roles := range m.members {
		if r := roles[userID]; r == model.DomainRoleOwner || r == model.DomainRoleAdmin {
			ids = append(ids, did)
		}
	}
	return ids, nil
}
func (m *memDomainRepo) GetAccessibleDomainIDs(userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]uint64, error) {
	return m.GetUserDomainIDs(userID)
}

type memAssetRepo struct {
	nextID uint64
	assets map[uint64]*model.AIAsset
	mcp    map[uint64]*model.AIAssetMCP
}

func newMemAssetRepo() *memAssetRepo {
	return &memAssetRepo{
		nextID: 1,
		assets: make(map[uint64]*model.AIAsset),
		mcp:    make(map[uint64]*model.AIAssetMCP),
	}
}

func (m *memAssetRepo) ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs, permittedAssetIDs []uint64) ([]model.AssetListItem, int64, error) {
	var items []model.AssetListItem
	for _, a := range m.assets {
		items = append(items, model.AssetListItem{AIAsset: *a})
	}
	return items, int64(len(items)), nil
}
func (m *memAssetRepo) ListUserAssets(q model.AssetQuery) ([]model.AssetListItem, int64, error) {
	var items []model.AssetListItem
	for _, a := range m.assets {
		if a.OwnerID == q.OwnerID {
			items = append(items, model.AssetListItem{AIAsset: *a})
		}
	}
	return items, int64(len(items)), nil
}
func (m *memAssetRepo) FindByID(id uint64) (*model.AIAsset, error) {
	if a, ok := m.assets[id]; ok {
		return a, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memAssetRepo) FindByCode(code string) (*model.AIAsset, error) {
	for _, a := range m.assets {
		if a.AssetCode == code {
			return a, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memAssetRepo) GetDetailByID(id uint64) (*model.AssetDetail, error) {
	a, ok := m.assets[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.AssetDetail{AIAsset: *a, MCPConfig: m.mcp[id]}, nil
}
func (m *memAssetRepo) CreateAsset(asset *model.AIAsset, ext interface{}) error {
	asset.ID = m.nextID
	m.nextID++
	m.assets[asset.ID] = asset
	if mcp, ok := ext.(*model.AIAssetMCP); ok {
		mcp.AssetID = asset.ID
		m.mcp[asset.ID] = mcp
	}
	return nil
}
func (m *memAssetRepo) UpdateAsset(asset *model.AIAsset, ext interface{}) error {
	m.assets[asset.ID] = asset
	return nil
}
func (m *memAssetRepo) DeleteAsset(id uint64) error {
	delete(m.assets, id)
	return nil
}
func (m *memAssetRepo) UpdateStatus(id uint64, status string, version string) error {
	a, ok := m.assets[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	a.Status = status
	if version != "" {
		a.CurrentVersion = version
	}
	return nil
}

func (m *memAssetRepo) GetDetailByCode(code string) (*model.AssetDetail, error) {
	for _, a := range m.assets {
		if a.AssetCode == code {
			return &model.AssetDetail{AIAsset: *a, MCPConfig: m.mcp[a.ID]}, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *memAssetRepo) PublishWithSnapshot(assetID uint64, version, changelog string, snapshot model.JSONRaw, publisherID uint64) error {
	return m.UpdateStatus(assetID, model.AssetStatusPublished, version)
}
func (m *memAssetRepo) SubmitPublishApproval(assetID, domainID uint64, version, changelog string, snapshot model.JSONRaw, applicantID, approverID uint64, requestNo string) error {
	return m.UpdateStatus(assetID, model.AssetStatusPending, "")
}
func (m *memAssetRepo) ListVersions(assetID uint64) ([]model.AIAssetVersion, error) {
	return []model.AIAssetVersion{}, nil
}
func (m *memAssetRepo) FindVersionByID(assetID, versionID uint64) (*model.AIAssetVersion, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *memAssetRepo) RestoreFromSnapshot(asset *model.AIAsset, ext interface{}, versionID uint64, version string) error {
	return nil
}
func (m *memAssetRepo) UpdateMCPHealth(assetID uint64, healthStatus string, checkedAt time.Time) error {
	return nil
}
func (m *memAssetRepo) UpdateMCPTools(assetID uint64, toolsSchema model.JSONRaw) error {
	return nil
}
func (m *memAssetRepo) UpdateSkillPackage(assetID uint64, packagePath string, packageFiles model.StringSlice, content string) error {
	return nil
}
func (m *memAssetRepo) IncrementInvokeCount(assetID uint64) error {
	if a, ok := m.assets[assetID]; ok {
		a.InvokeCount++
	}
	return nil
}

func setupTestEcho() *echo.Echo {
	userRepo := newMemUserRepo()
	domainRepo := newMemDomainRepo()
	assetRepo := newMemAssetRepo()
	jwtCfg := config.JWTConfig{Secret: "handler-test-secret", ExpireHours: 24}

	authSvc := service.NewAuthService(userRepo, jwtCfg)
	domainSvc := service.NewDomainService(domainRepo, userRepo, nil)
	assetSvc := service.NewAssetService(assetRepo, domainRepo, nil, nil, "")

	authH := NewAuthHandler(authSvc)
	domainH := NewDomainHandler(domainSvc)
	assetH := NewAssetHandler(assetSvc)

	e := echo.New()
	e.Validator = validator.New()

	v1 := e.Group("/api/v1")
	v1.POST("/auth/register", authH.Register)
	v1.POST("/auth/login", authH.Login)

	protected := v1.Group("", middleware.AuthMiddleware(jwtCfg.Secret, userRepo))
	protected.POST("/auth/logout", authH.Logout)
	protected.GET("/user/profile", authH.GetProfile)
	protected.PUT("/user/profile", authH.UpdateProfile)
	protected.POST("/domains", domainH.CreateDomain)
	protected.POST("/domains/:id/members", domainH.AddMember)
	protected.POST("/assets", assetH.CreateAsset)
	protected.POST("/assets/:id/publish", assetH.PublishAsset)
	protected.POST("/assets/:id/offline", assetH.OfflineAsset)

	return e
}

func doJSON(e *echo.Echo, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func assertStatus(t *testing.T, got, want int, body string) {
	t.Helper()
	if got != want {
		t.Fatalf("expected HTTP %d, got %d: %s", want, got, body)
	}
}

func TestP0EndToEndFlow(t *testing.T) {
	e := setupTestEcho()

	// 1. Register user
	rec := doJSON(e, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"username": "alice",
		"password": "password123",
		"nickname": "Alice",
		"email":    "alice@example.com",
	})
	assertStatus(t, rec.Code, http.StatusOK, rec.Body.String())

	// Duplicate username -> 409
	recDup := doJSON(e, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"username": "alice",
		"password": "password123",
		"email":    "alice2@example.com",
	})
	assertStatus(t, recDup.Code, http.StatusConflict, recDup.Body.String())

	// 2. Login
	recLogin := doJSON(e, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "alice",
		"password": "password123",
	})
	assertStatus(t, recLogin.Code, http.StatusOK, recLogin.Body.String())

	var loginRes struct {
		Code int                 `json:"code"`
		Data model.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(recLogin.Body.Bytes(), &loginRes); err != nil {
		t.Fatalf("unmarshal login response failed: %v", err)
	}
	if loginRes.Code != response.CodeSuccess || loginRes.Data.Token == "" {
		t.Fatalf("unexpected login response: %+v", loginRes)
	}
	token := loginRes.Data.Token

	// 3. Create domain
	recDom := doJSON(e, http.MethodPost, "/api/v1/domains", token, map[string]string{
		"domain_code": "rd-core",
		"domain_name": "R&D Core",
		"description": "Core engineering domain",
	})
	assertStatus(t, recDom.Code, http.StatusOK, recDom.Body.String())

	// 4. Create MCP asset (missing endpoint -> 400)
	recBadAsset := doJSON(e, http.MethodPost, "/api/v1/assets", token, map[string]interface{}{
		"asset_code": "mcp-bad",
		"asset_type": "mcp",
		"name":       "Bad MCP",
		"domain_id":  1,
	})
	assertStatus(t, recBadAsset.Code, http.StatusBadRequest, recBadAsset.Body.String())

	// Create valid MCP asset -> 200
	recAsset := doJSON(e, http.MethodPost, "/api/v1/assets", token, map[string]interface{}{
		"asset_code": "mcp-git",
		"asset_type": "mcp",
		"name":       "Git MCP",
		"domain_id":  1,
		"mcp_config": map[string]string{
			"transport_type": "sse",
			"endpoint_url":   "https://mcp.example.com/sse",
		},
	})
	assertStatus(t, recAsset.Code, http.StatusOK, recAsset.Body.String())

	// 5. Publish & Offline asset
	recPub := doJSON(e, http.MethodPost, "/api/v1/assets/1/publish", token, map[string]string{"version": "v1.0.0"})
	assertStatus(t, recPub.Code, http.StatusOK, recPub.Body.String())

	recOff := doJSON(e, http.MethodPost, "/api/v1/assets/1/offline", token, nil)
	assertStatus(t, recOff.Code, http.StatusOK, recOff.Body.String())

	// 6. Logout invalidates token
	recLogout := doJSON(e, http.MethodPost, "/api/v1/auth/logout", token, nil)
	assertStatus(t, recLogout.Code, http.StatusOK, recLogout.Body.String())

	recProf := doJSON(e, http.MethodGet, "/api/v1/user/profile", token, nil)
	assertStatus(t, recProf.Code, http.StatusUnauthorized, recProf.Body.String())
}

func TestOptionalEmailAndBioAvatarProfile(t *testing.T) {
	e := setupTestEcho()

	// Register two users without email (should not collide on NULL email)
	rec1 := doJSON(e, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"username": "noemail1",
		"password": "password123",
	})
	assertStatus(t, rec1.Code, http.StatusOK, rec1.Body.String())

	rec2 := doJSON(e, http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"username": "noemail2",
		"password": "password123",
	})
	assertStatus(t, rec2.Code, http.StatusOK, rec2.Body.String())

	recLogin := doJSON(e, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "noemail1",
		"password": "password123",
	})
	assertStatus(t, recLogin.Code, http.StatusOK, recLogin.Body.String())
	var loginRes struct {
		Data model.LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(recLogin.Body.Bytes(), &loginRes); err != nil {
		t.Fatalf("unmarshal login failed: %v", err)
	}
	token := loginRes.Data.Token

	// Update profile with custom data-URL avatar and bio
	avatarData := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	recUpd := doJSON(e, http.MethodPut, "/api/v1/user/profile", token, map[string]interface{}{
		"nickname": "NoEmail One",
		"avatar":   avatarData,
		"bio":      "Building AI capabilities",
	})
	assertStatus(t, recUpd.Code, http.StatusOK, recUpd.Body.String())

	var profRes struct {
		Data model.UserInfo `json:"data"`
	}
	if err := json.Unmarshal(recUpd.Body.Bytes(), &profRes); err != nil {
		t.Fatalf("unmarshal profile failed: %v", err)
	}
	if profRes.Data.Email != "" || profRes.Data.Bio != "Building AI capabilities" || profRes.Data.Avatar != avatarData {
		t.Fatalf("unexpected updated profile: %+v", profRes.Data)
	}
}
