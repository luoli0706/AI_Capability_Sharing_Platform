package service

import (
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

func (m *mockOrgRepo) ListOrgs(userID uint64) ([]model.OrgListItem, error) { return nil, nil }
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

func (m *mockSquareDomainRepo) ListDomains(userID uint64, q model.DomainQuery, userOrgIDs []uint64) ([]model.DomainListItem, error) {
	uid := strings.ToUpper(strings.TrimSpace(q.DomainUID))
	kw := strings.TrimSpace(q.Keyword)
	if uid == "" && strings.HasPrefix(strings.ToUpper(kw), "DOM-") {
		uid = strings.ToUpper(kw)
	}
	var out []model.DomainListItem
	for _, d := range m.domains {
		if q.Tab == "square" {
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
	return out, nil
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
func (m *mockSquareDomainRepo) GetAccessibleDomainIDs(userID uint64, userOrgIDs []uint64) ([]uint64, error) {
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
	sqFuzzy, err := domainSvc.ListDomains(2, model.DomainQuery{Tab: "square", Keyword: "Secret"})
	if err != nil {
		t.Fatalf("ListDomains square fuzzy failed: %v", err)
	}
	if len(sqFuzzy) != 0 {
		t.Fatalf("expected private domain to be hidden from square fuzzy search, got %d", len(sqFuzzy))
	}
	sqExact, err := domainSvc.ListDomains(2, model.DomainQuery{Tab: "square", Keyword: personalDom.DomainUID})
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
