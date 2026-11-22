package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type mockOrgRepo struct {
	nextID  uint64
	orgs    map[uint64]*model.BizOrganization
	members map[uint64]map[uint64]string
}

func newMockOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{
		nextID:  1,
		orgs:    make(map[uint64]*model.BizOrganization),
		members: make(map[uint64]map[uint64]string),
	}
}

func (m *mockOrgRepo) ListOrgs(userID uint64, q model.OrgQuery) ([]model.OrgListItem, int64, error) {
	kw := strings.ToLower(strings.TrimSpace(q.Keyword))
	var out []model.OrgListItem
	for _, o := range m.orgs {
		myRole := m.members[o.ID][userID]
		vis := o.Visibility
		if vis == "" {
			vis = model.OrgVisibilityPublic
		}
		if q.Tab != "square" {
			if myRole == "" {
				continue
			}
		} else if !q.IsSuperAdmin && vis == model.OrgVisibilityPrivate && myRole == "" {
			continue
		}
		if kw != "" && !strings.Contains(strings.ToLower(o.OrgName), kw) && !strings.Contains(strings.ToLower(o.OrgCode), kw) {
			continue
		}
		out = append(out, model.OrgListItem{
			ID:         o.ID,
			OrgCode:    o.OrgCode,
			OrgName:    o.OrgName,
			Visibility: vis,
			OwnerID:    o.OwnerID,
			MyRole:     myRole,
			CreatedAt:  o.CreatedAt,
		})
	}
	total := int64(len(out))
	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 12
	}
	start := (page - 1) * pageSize
	if start >= len(out) {
		return []model.OrgListItem{}, total, nil
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}
func (m *mockOrgRepo) FindByID(id uint64) (*model.BizOrganization, error) {
	if o, ok := m.orgs[id]; ok {
		return o, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockOrgRepo) FindByCode(code string) (*model.BizOrganization, error) {
	for _, o := range m.orgs {
		if o.OrgCode == code {
			return o, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockOrgRepo) CreateWithOwner(org *model.BizOrganization) error {
	org.ID = m.nextID
	m.nextID++
	org.CreatedAt = time.Now()
	m.orgs[org.ID] = org
	m.members[org.ID] = map[uint64]string{org.OwnerID: model.OrgRoleOwner}
	return nil
}
func (m *mockOrgRepo) UpdateOrg(org *model.BizOrganization) error {
	m.orgs[org.ID] = org
	return nil
}
func (m *mockOrgRepo) DeleteOrg(id uint64) error {
	delete(m.orgs, id)
	return nil
}
func (m *mockOrgRepo) ListMembers(orgID uint64) ([]model.OrgMemberItem, error) { return nil, nil }
func (m *mockOrgRepo) GetMemberRole(orgID, userID uint64) (string, error) {
	if m.members[orgID] != nil {
		return m.members[orgID][userID], nil
	}
	return "", nil
}
func (m *mockOrgRepo) UpsertMember(member *model.OrganizationMember) error {
	if m.members[member.OrgID] == nil {
		m.members[member.OrgID] = make(map[uint64]string)
	}
	m.members[member.OrgID][member.UserID] = member.OrgRole
	return nil
}
func (m *mockOrgRepo) RemoveMember(orgID, userID uint64) error {
	if m.members[orgID] != nil {
		delete(m.members[orgID], userID)
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *mockOrgRepo) GetUserOrgIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	for oid, roles := range m.members {
		if roles[userID] != "" {
			ids = append(ids, oid)
		}
	}
	return ids, nil
}
func (m *mockOrgRepo) GetUserAdminOrgIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	for oid, roles := range m.members {
		if r := roles[userID]; r == model.OrgRoleOwner || r == model.OrgRoleAdmin {
			ids = append(ids, oid)
		}
	}
	return ids, nil
}
func (m *mockOrgRepo) ListPublicDomainsByOrg(orgID, userID uint64) ([]model.DomainListItem, error) {
	return nil, nil
}

type mockSquareDomainRepo struct {
	nextID  uint64
	domains map[uint64]*model.BizDomain
	members map[uint64]map[uint64]string
}

func newMockSquareDomainRepo() *mockSquareDomainRepo {
	return &mockSquareDomainRepo{
		nextID:  1,
		domains: make(map[uint64]*model.BizDomain),
		members: make(map[uint64]map[uint64]string),
	}
}

func (m *mockSquareDomainRepo) ListDomains(userID uint64, q model.DomainQuery, userOrgIDs, adminOrgIDs []uint64) ([]model.DomainListItem, int64, error) {
	uid := strings.ToUpper(strings.TrimSpace(q.DomainUID))
	kw := strings.TrimSpace(q.Keyword)
	if uid == "" && strings.HasPrefix(strings.ToUpper(kw), "DOM-") {
		uid = strings.ToUpper(kw)
	}
	orgSet := toUint64Set(userOrgIDs)
	adminOrgSet := toUint64Set(adminOrgIDs)

	var out []model.DomainListItem
	for _, d := range m.domains {
		switch q.Tab {
		case "square":
			if uid != "" {
				if d.DomainUID != uid {
					continue
				}
			} else {
				if d.Visibility != model.DomainVisibilityPublic {
					continue
				}
				if kw != "" && !strings.Contains(strings.ToLower(d.DomainName), strings.ToLower(kw)) && !strings.Contains(strings.ToLower(d.DomainCode), strings.ToLower(kw)) {
					continue
				}
			}
		case "mine":
			isMember := m.members[d.ID] != nil && m.members[d.ID][userID] != ""
			isOrgPub := d.Visibility == model.DomainVisibilityPublic && d.OrgID > 0 && orgSet[d.OrgID]
			isOrgPrivAdmin := d.Visibility == model.DomainVisibilityOrgPrivate && d.OrgID > 0 && adminOrgSet[d.OrgID]
			if !isMember && !isOrgPub && !isOrgPrivAdmin {
				continue
			}
		}
		out = append(out, model.DomainListItem{
			ID:         d.ID,
			DomainUID:  d.DomainUID,
			DomainCode: d.DomainCode,
			DomainName: d.DomainName,
			Visibility: d.Visibility,
			OrgID:      d.OrgID,
			OrgName:    d.OrgName,
			OwnerID:    d.OwnerID,
		})
	}
	total := int64(len(out))
	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 12
	}
	start := (page - 1) * pageSize
	if start >= len(out) {
		return []model.DomainListItem{}, total, nil
	}
	end := start + pageSize
	if end > len(out) {
		end = len(out)
	}
	return out[start:end], total, nil
}
func (m *mockSquareDomainRepo) FindByID(id uint64) (*model.BizDomain, error) {
	if d, ok := m.domains[id]; ok {
		return d, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockSquareDomainRepo) FindByCode(code string) (*model.BizDomain, error) {
	for _, d := range m.domains {
		if d.DomainCode == code {
			return d, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockSquareDomainRepo) FindByUID(uid string) (*model.BizDomain, error) {
	for _, d := range m.domains {
		if d.DomainUID == uid {
			return d, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockSquareDomainRepo) CreateWithOwner(domain *model.BizDomain) error {
	domain.ID = m.nextID
	m.nextID++
	m.domains[domain.ID] = domain
	m.members[domain.ID] = map[uint64]string{domain.OwnerID: model.DomainRoleOwner}
	return nil
}
func (m *mockSquareDomainRepo) UpdateDomain(domain *model.BizDomain) error {
	m.domains[domain.ID] = domain
	return nil
}
func (m *mockSquareDomainRepo) DeleteDomain(id uint64) error {
	delete(m.domains, id)
	return nil
}
func (m *mockSquareDomainRepo) ListMembers(domainID uint64) ([]model.DomainMemberItem, error) {
	return nil, nil
}
func (m *mockSquareDomainRepo) GetMemberRole(domainID, userID uint64) (string, error) {
	if m.members[domainID] != nil {
		return m.members[domainID][userID], nil
	}
	return "", nil
}
func (m *mockSquareDomainRepo) UpsertMember(member *model.DomainMember) error {
	if m.members[member.DomainID] == nil {
		m.members[member.DomainID] = make(map[uint64]string)
	}
	m.members[member.DomainID][member.UserID] = member.DomainRole
	return nil
}
func (m *mockSquareDomainRepo) RemoveMember(domainID, userID uint64) error {
	if m.members[domainID] != nil {
		delete(m.members[domainID], userID)
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *mockSquareDomainRepo) GetUserDomainIDs(userID uint64) ([]uint64, error) { return nil, nil }
func (m *mockSquareDomainRepo) GetUserAdminDomainIDs(userID uint64) ([]uint64, error) {
	return nil, nil
}
func (m *mockSquareDomainRepo) GetAccessibleDomainIDs(userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]uint64, error) {
	return nil, nil
}

func TestOrgDomainVisibilityAndStrictPermissions(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "creator", Status: 1},
			2: {ID: 2, Username: "org_member", Status: 1},
			3: {ID: 3, Username: "super_admin_user", Status: 1},
			4: {ID: 4, Username: "domain_admin", Status: 1},
		},
	}
	orgRepo := newMockOrgRepo()
	domainRepo := newMockSquareDomainRepo()

	orgSvc := NewOrgService(orgRepo, userRepo)
	domainSvc := NewDomainService(domainRepo, userRepo, orgRepo)

	// 1. Personal domain defaults to org_name="个人" and has DOM-XXXXXXXX UID
	personalDom, err := domainSvc.CreateDomain(1, model.CreateDomainRequest{
		DomainCode: "personal-secret",
		DomainName: "Secret Lab",
		Visibility: model.DomainVisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("CreateDomain personal failed: %v", err)
	}
	if personalDom.OrgName != "个人" || personalDom.OrgID != 0 {
		t.Fatalf("expected personal domain org_name='个人', got %q (org_id=%d)", personalDom.OrgName, personalDom.OrgID)
	}
	if !strings.HasPrefix(personalDom.DomainUID, "DOM-") || len(personalDom.DomainUID) != 12 {
		t.Fatalf("unexpected domain_uid: %q", personalDom.DomainUID)
	}

	// 2. Private domain hidden from square fuzzy search, only found by exact domain_uid
	sqFuzzy, _, err := domainSvc.ListDomains(2, model.DomainQuery{Tab: "square", Keyword: "Secret"})
	if err != nil {
		t.Fatalf("ListDomains square fuzzy failed: %v", err)
	}
	if len(sqFuzzy) != 0 {
		t.Fatalf("expected private domain to be hidden from square fuzzy search, got %d", len(sqFuzzy))
	}
	sqExact, _, err := domainSvc.ListDomains(2, model.DomainQuery{Tab: "square", Keyword: personalDom.DomainUID})
	if err != nil {
		t.Fatalf("ListDomains square exact UID failed: %v", err)
	}
	if len(sqExact) != 1 || sqExact[0].ID != personalDom.ID {
		t.Fatalf("expected private domain to be found by exact domain_uid, got %+v", sqExact)
	}

	// 3. Create organization and add user 2 as org member
	org, err := orgSvc.CreateOrg(1, model.CreateOrgRequest{
		OrgCode: "ai-lab",
		OrgName: "AI Research Lab",
	})
	if err != nil {
		t.Fatalf("CreateOrg failed: %v", err)
	}
	if err := orgSvc.AddMember(1, org.ID, model.AddOrgMemberRequest{UserID: 2, OrgRole: model.OrgRoleMember}); err != nil {
		t.Fatalf("org AddMember failed: %v", err)
	}

	// 4. Create org public domain & org private domain
	orgPubDom, err := domainSvc.CreateDomain(1, model.CreateDomainRequest{
		DomainCode: "org-pub",
		DomainName: "Org Public Domain",
		Visibility: model.DomainVisibilityPublic,
		OrgID:      org.ID,
	})
	if err != nil {
		t.Fatalf("CreateDomain org public failed: %v", err)
	}
	if orgPubDom.OrgName != "AI Research Lab" {
		t.Fatalf("expected org_name 'AI Research Lab', got %q", orgPubDom.OrgName)
	}

	orgPrivDom, err := domainSvc.CreateDomain(1, model.CreateDomainRequest{
		DomainCode: "org-priv",
		DomainName: "Org Private Domain",
		Visibility: model.DomainVisibilityPrivate,
		OrgID:      org.ID,
	})
	if err != nil {
		t.Fatalf("CreateDomain org private failed: %v", err)
	}

	// 5. Strict CanManageDomain: org_member (2) and super_admin_user (3) are rejected with 403
	for _, uid := range []uint64{2, 3} {
		err := domainSvc.AddMember(uid, orgPubDom.ID, model.AddDomainMemberRequest{UserID: 4, DomainRole: model.DomainRoleMember})
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden for non-domain-manager user %d, got %v", uid, err)
		}
	}

	// Creator (1) assigns user 4 as domain_admin -> user 4 can add a regular member
	if err := domainSvc.AddMember(1, orgPubDom.ID, model.AddDomainMemberRequest{UserID: 4, DomainRole: model.DomainRoleAdmin}); err != nil {
		t.Fatalf("creator AddMember admin failed: %v", err)
	}
	if err := domainSvc.AddMember(4, orgPubDom.ID, model.AddDomainMemberRequest{UserID: 3, DomainRole: model.DomainRoleMember}); err != nil {
		t.Fatalf("domain_admin AddMember member failed: %v", err)
	}
	// domain_admin (4) cannot remove creator (1)
	if err := domainSvc.RemoveMember(4, orgPubDom.ID, 1); err == nil {
		t.Fatal("expected error when domain_admin tries to remove creator")
	}

	// 6. HasDomainAccess: org_member (2) has access to org's PUBLIC domain assets, NOT org's PRIVATE domain assets
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			10: {
				ID:         10,
				AssetCode:  "skill-org-pub",
				AssetType:  model.AssetTypeSkill,
				DomainID:   orgPubDom.ID,
				OwnerID:    1,
				Visibility: model.VisibilityDomain,
				Status:     model.AssetStatusPublished,
			},
			11: {
				ID:         11,
				AssetCode:  "skill-org-priv",
				AssetType:  model.AssetTypeSkill,
				DomainID:   orgPrivDom.ID,
				OwnerID:    1,
				Visibility: model.VisibilityDomain,
				Status:     model.AssetStatusPublished,
			},
		},
	}
	govRepo := newMockP1GovRepo()
	invokeSvc := NewInvokeService(assetRepo, domainRepo, govRepo, repository.KBRepository(nil), &mockP1LogRepo{}, orgRepo)

	if _, err := invokeSvc.CheckAccess(2, false, "skill-org-pub"); err != nil {
		t.Fatalf("expected org member to access org public domain asset, got %v", err)
	}
	if _, err := invokeSvc.CheckAccess(2, false, "skill-org-priv"); err == nil {
		t.Fatal("expected org member NOT to have automatic access to org private domain asset")
	}
}

func TestOrgPrivateDomainAndOrgLifecycle(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "org_owner", Status: 1},
			2: {ID: 2, Username: "org_admin", Status: 1},
			3: {ID: 3, Username: "org_member_invited", Status: 1},
			4: {ID: 4, Username: "org_member_other", Status: 1},
		},
	}
	orgRepo := newMockOrgRepo()
	domainRepo := newMockSquareDomainRepo()
	auditRepo := &mockAuditRepo{}

	orgSvc := NewOrgService(orgRepo, userRepo)
	orgSvc.SetAuditRepos(auditRepo, userRepo)
	domainSvc := NewDomainService(domainRepo, userRepo, orgRepo)
	domainSvc.SetAuditRepos(auditRepo, userRepo)
	auditSvc := NewAuditService(auditRepo, domainRepo, orgRepo)

	org, err := orgSvc.CreateOrg(1, model.CreateOrgRequest{OrgCode: "core-org", OrgName: "Core Org"})
	if err != nil {
		t.Fatalf("CreateOrg failed: %v", err)
	}
	_ = orgSvc.AddMember(1, org.ID, model.AddOrgMemberRequest{UserID: 2, OrgRole: model.OrgRoleAdmin})
	_ = orgSvc.AddMember(1, org.ID, model.AddOrgMemberRequest{UserID: 3, OrgRole: model.OrgRoleMember})
	_ = orgSvc.AddMember(1, org.ID, model.AddOrgMemberRequest{UserID: 4, OrgRole: model.OrgRoleMember})

	// 1. Non-admin org member (3) cannot create org_private domain -> 403
	_, err = domainSvc.CreateDomain(3, model.CreateDomainRequest{
		DomainCode: "bad-org-priv",
		DomainName: "Bad Org Priv",
		Visibility: model.DomainVisibilityOrgPrivate,
		OrgID:      org.ID,
	})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin creating org_private, got %v", err)
	}

	// 2. Org admin (2) creates org_private domain -> 200
	orgPriv, err := domainSvc.CreateDomain(2, model.CreateDomainRequest{
		DomainCode: "vip-group",
		DomainName: "VIP Group",
		Visibility: model.DomainVisibilityOrgPrivate,
		OrgID:      org.ID,
	})
	if err != nil {
		t.Fatalf("org admin CreateDomain org_private failed: %v", err)
	}

	// 3. Invite user 3 into orgPriv; user 4 (other org member) is NOT invited
	if err := domainSvc.AddMember(2, orgPriv.ID, model.AddDomainMemberRequest{UserID: 3, DomainRole: model.DomainRoleAdmin}); err != nil {
		t.Fatalf("org admin AddMember failed: %v", err)
	}

	// User 4 cannot see org_private in mine or square
	mine4, _, _ := domainSvc.ListDomains(4, model.DomainQuery{Tab: "mine"})
	if len(mine4) != 0 {
		t.Fatalf("expected other org member (4) to see 0 domains in mine, got %+v", mine4)
	}
	sq4, _, _ := domainSvc.ListDomains(4, model.DomainQuery{Tab: "square", Keyword: "VIP"})
	if len(sq4) != 0 {
		t.Fatalf("expected org_private hidden from square search, got %+v", sq4)
	}

	// User 3 (invited) sees it in mine, but CANNOT manage or invite others (even if domain_role='admin')
	mine3, _, _ := domainSvc.ListDomains(3, model.DomainQuery{Tab: "mine"})
	if len(mine3) != 1 || mine3[0].ID != orgPriv.ID {
		t.Fatalf("expected invited member (3) to see org_private in mine, got %+v", mine3)
	}
	err = domainSvc.AddMember(3, orgPriv.ID, model.AddDomainMemberRequest{UserID: 4, DomainRole: model.DomainRoleMember})
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 when invited member tries to manage org_private, got %v", err)
	}

	// 4. Domain pagination
	_, _ = domainSvc.CreateDomain(1, model.CreateDomainRequest{DomainCode: "d2", DomainName: "Domain 2", Visibility: model.DomainVisibilityPublic})
	_, _ = domainSvc.CreateDomain(1, model.CreateDomainRequest{DomainCode: "d3", DomainName: "Domain 3", Visibility: model.DomainVisibilityPublic})
	page1, total, err := domainSvc.ListDomains(1, model.DomainQuery{Tab: "mine", Page: 1, PageSize: 2})
	if err != nil || total != 3 || len(page1) != 2 {
		t.Fatalf("expected total=3 len=2 on page 1, got total=%d len=%d err=%v", total, len(page1), err)
	}

	// 5. PUT/DELETE orgs & DELETE domains + audit logs
	if _, err := orgSvc.UpdateOrg(2, org.ID, model.UpdateOrgRequest{OrgName: "Core Org Updated"}); err != nil {
		t.Fatalf("org admin UpdateOrg failed: %v", err)
	}
	// Org admin (2) cannot delete org -> 403; org owner (1) can
	if err := orgSvc.DeleteOrg(2, org.ID); err == nil {
		t.Fatal("expected 403 when org admin tries to delete org")
	}
	if err := domainSvc.DeleteDomain(1, orgPriv.ID); err != nil {
		t.Fatalf("org owner DeleteDomain org_private failed: %v", err)
	}
	if err := orgSvc.DeleteOrg(1, org.ID); err != nil {
		t.Fatalf("org owner DeleteOrg failed: %v", err)
	}

	logs, logTotal, err := auditSvc.ListManagedLogs(context.Background(), 1, model.AuditLogQuery{})
	if err != nil || logTotal == 0 || len(logs) == 0 {
		t.Fatalf("expected org/domain audit logs for owner, got total=%d err=%v", logTotal, err)
	}

	// 6. Org keyword search & pagination
	_, _ = orgSvc.CreateOrg(1, model.CreateOrgRequest{OrgCode: "alpha-ai", OrgName: "Alpha AI"})
	_, _ = orgSvc.CreateOrg(1, model.CreateOrgRequest{OrgCode: "beta-ops", OrgName: "Beta Ops"})
	filteredOrgs, filteredTotal, err := orgSvc.ListOrgs(1, model.OrgQuery{Keyword: "alpha", Page: 1, PageSize: 10})
	if err != nil || filteredTotal != 1 || len(filteredOrgs) != 1 || filteredOrgs[0].OrgCode != "alpha-ai" {
		t.Fatalf("expected 1 filtered org 'alpha-ai', got total=%d list=%+v err=%v", filteredTotal, filteredOrgs, err)
	}
}

func TestOrgJoinApprovalAndOrgPrivateMemberCheck(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "owner", Status: 1},
			2: {ID: 2, Username: "admin", Status: 1},
			3: {ID: 3, Username: "outsider", Status: 1},
		},
	}
	orgRepo := newMockOrgRepo()
	domainRepo := newMockSquareDomainRepo()
	govRepo := newMockP1GovRepo()
	govRepo.orgRepo = orgRepo
	auditRepo := &mockAuditRepo{}

	orgSvc := NewOrgService(orgRepo, userRepo)
	domainSvc := NewDomainService(domainRepo, userRepo, orgRepo)
	govSvc := NewGovernanceService(govRepo, &mockP1AssetRepo{}, domainRepo, orgRepo)
	govSvc.SetAuditRepos(auditRepo, userRepo)

	org, err := orgSvc.CreateOrg(1, model.CreateOrgRequest{OrgCode: "join-org", OrgName: "Joinable Org"})
	if err != nil {
		t.Fatalf("CreateOrg failed: %v", err)
	}
	_ = orgSvc.AddMember(1, org.ID, model.AddOrgMemberRequest{UserID: 2, OrgRole: model.OrgRoleAdmin})

	orgPriv, err := domainSvc.CreateDomain(1, model.CreateDomainRequest{
		DomainCode: "strict-priv",
		DomainName: "Strict Priv",
		Visibility: model.DomainVisibilityOrgPrivate,
		OrgID:      org.ID,
	})
	if err != nil {
		t.Fatalf("CreateDomain org_private failed: %v", err)
	}

	// 1. Adding non-org member (3) to org_private domain -> 400 Bad Request
	err = domainSvc.AddMember(1, orgPriv.ID, model.AddDomainMemberRequest{UserID: 3, DomainRole: model.DomainRoleMember})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request when adding non-org member to org_private domain, got %v", err)
	}

	// 2. Outsider (3) requests to join org -> pending; duplicate request -> 409 Conflict
	req, err := govSvc.RequestJoinOrg(3, org.ID, "Please let me in")
	if err != nil {
		t.Fatalf("RequestJoinOrg failed: %v", err)
	}
	if _, err := govSvc.RequestJoinOrg(3, org.ID, "dup"); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate join request, got %v", err)
	}

	// 3. Org admin (2) sees it in pending & approves -> user 3 becomes org member + audit log recorded
	pending, err := govSvc.ListPendingRequests(2, false)
	if err != nil || len(pending) != 1 || pending[0].ID != req.ID || pending[0].TargetType != model.ApprovalTargetOrgJoin {
		t.Fatalf("expected org admin to see 1 org_join pending request, got %+v, err=%v", pending, err)
	}

	approved, err := govSvc.HandleApproval(2, false, req.ID, model.HandleApprovalRequest{Action: "approve", Comment: "Welcome"})
	if err != nil {
		t.Fatalf("HandleApproval org_join failed: %v", err)
	}
	if approved.Status != model.ApprovalStatusApproved {
		t.Fatalf("expected approved status, got %s", approved.Status)
	}

	role, _ := orgRepo.GetMemberRole(org.ID, 3)
	if role != model.OrgRoleMember {
		t.Fatalf("expected user 3 to be org member, got %q", role)
	}
	if len(auditRepo.logs) != 1 || auditRepo.logs[0].Action != model.AuditActionOrgMemberAdd {
		t.Fatalf("expected org.member_add audit log, got %+v", auditRepo.logs)
	}

	// 4. Now user 3 CAN be added to org_private domain
	if err := domainSvc.AddMember(1, orgPriv.ID, model.AddDomainMemberRequest{UserID: 3, DomainRole: model.DomainRoleMember}); err != nil {
		t.Fatalf("expected org member (3) to be added to org_private domain, got %v", err)
	}
}
