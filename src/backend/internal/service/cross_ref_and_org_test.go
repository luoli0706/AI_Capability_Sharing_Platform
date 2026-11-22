package service

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type mockCrossRefRepo struct {
	nextID  uint64
	refs    map[uint64]*model.DomainCrossRef
	govRepo *mockP1GovRepo
}

func newMockCrossRefRepo(govRepo *mockP1GovRepo) *mockCrossRefRepo {
	return &mockCrossRefRepo{
		nextID:  1,
		refs:    make(map[uint64]*model.DomainCrossRef),
		govRepo: govRepo,
	}
}

func (m *mockCrossRefRepo) Create(ref *model.DomainCrossRef) error {
	ref.ID = m.nextID
	m.nextID++
	ref.CreatedAt = time.Now()
	cp := *ref
	m.refs[ref.ID] = &cp
	return nil
}

func (m *mockCrossRefRepo) UpdateApprovalRequestID(id, approvalRequestID uint64) error {
	if r, ok := m.refs[id]; ok {
		r.ApprovalRequestID = approvalRequestID
	}
	return nil
}

func (m *mockCrossRefRepo) FindByID(id uint64) (*model.DomainCrossRef, error) {
	if r, ok := m.refs[id]; ok {
		cp := *r
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockCrossRefRepo) ExistsPendingOrActive(targetDomainID uint64, refType string, sourceAssetID, sourceDocID uint64) (bool, error) {
	for _, r := range m.refs {
		if r.TargetDomainID == targetDomainID &&
			r.RefType == refType &&
			r.SourceAssetID == sourceAssetID &&
			r.SourceDocID == sourceDocID &&
			(r.Status == model.CrossRefStatusPending || r.Status == model.CrossRefStatusActive) {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockCrossRefRepo) ListByTargetDomain(targetDomainID uint64, includePending bool, pendingCreatorID uint64, allPending bool) ([]model.CrossRefView, error) {
	var out []model.CrossRefView
	for _, r := range m.refs {
		if r.TargetDomainID != targetDomainID {
			continue
		}
		if r.Status == model.CrossRefStatusActive {
			// always visible
		} else if includePending && r.Status == model.CrossRefStatusPending && (allPending || r.CreatedBy == pendingCreatorID) {
			// visible
		} else {
			continue
		}
		out = append(out, model.CrossRefView{
			ID:                r.ID,
			TargetDomainID:    r.TargetDomainID,
			SourceDomainID:    r.SourceDomainID,
			RefType:           r.RefType,
			SourceAssetID:     r.SourceAssetID,
			SourceDocID:       r.SourceDocID,
			Title:             r.Title,
			Description:       r.Description,
			CreatedBy:         r.CreatedBy,
			Status:            r.Status,
			ApprovalRequestID: r.ApprovalRequestID,
			CreatedAt:         r.CreatedAt,
		})
	}
	return out, nil
}

func (m *mockCrossRefRepo) DeleteByID(id uint64) error {
	r, ok := m.refs[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if r.ApprovalRequestID > 0 && m.govRepo != nil {
		if req, exists := m.govRepo.reqs[r.ApprovalRequestID]; exists && req.Status == model.ApprovalStatusPending {
			req.Status = model.ApprovalStatusCancelled
		}
	}
	delete(m.refs, id)
	return nil
}

func (m *mockCrossRefRepo) DeleteByAssetID(assetID uint64) error {
	for id, r := range m.refs {
		if r.SourceAssetID == assetID {
			delete(m.refs, id)
		}
	}
	return nil
}

func (m *mockCrossRefRepo) DeleteByKBDocIDs(docIDs []uint64) error {
	set := toUint64Set(docIDs)
	for id, r := range m.refs {
		if r.RefType == model.CrossRefTypeKBFile && set[r.SourceDocID] {
			delete(m.refs, id)
		}
	}
	return nil
}

func (m *mockCrossRefRepo) DeleteByDomainID(domainID uint64) error {
	for id, r := range m.refs {
		if r.SourceDomainID == domainID || r.TargetDomainID == domainID {
			delete(m.refs, id)
		}
	}
	return nil
}

func (m *mockCrossRefRepo) ListSourceDomains(isSuperAdmin bool, accessibleDomainIDs []uint64, excludeDomainID uint64) ([]model.BizDomain, error) {
	return nil, nil
}

func (m *mockCrossRefRepo) ListPublishedAssetsByDomain(domainID uint64) ([]model.AIAsset, error) {
	return nil, nil
}

func (m *mockCrossRefRepo) ListKBFilesByAssetIDs(kbAssetIDs []uint64) ([]model.KBDocument, error) {
	return nil, nil
}

func (m *mockCrossRefRepo) HasActiveAssetRef(targetDomainID, sourceAssetID uint64) (bool, error) {
	for _, r := range m.refs {
		if r.TargetDomainID == targetDomainID &&
			r.Status == model.CrossRefStatusActive &&
			r.RefType == model.CrossRefTypeAsset &&
			r.SourceAssetID == sourceAssetID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockCrossRefRepo) HasActiveDocOrAssetRef(targetDomainID, sourceAssetID, sourceDocID uint64) (bool, error) {
	for _, r := range m.refs {
		if r.TargetDomainID != targetDomainID || r.Status != model.CrossRefStatusActive {
			continue
		}
		if r.RefType == model.CrossRefTypeAsset && r.SourceAssetID == sourceAssetID {
			return true, nil
		}
		if r.RefType == model.CrossRefTypeKBFile && r.SourceAssetID == sourceAssetID && r.SourceDocID == sourceDocID {
			return true, nil
		}
	}
	return false, nil
}

func TestOrgVisibilityPublicPrivateAndJoinReject(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "owner", Status: 1},
			2: {ID: 2, Username: "member", Status: 1},
			3: {ID: 3, Username: "outsider", Status: 1},
		},
	}
	orgRepo := newMockOrgRepo()
	domainRepo := newMockSquareDomainRepo()
	govRepo := newMockP1GovRepo()
	govRepo.orgRepo = orgRepo

	orgSvc := NewOrgService(orgRepo, userRepo)
	govSvc := NewGovernanceService(govRepo, &mockP1AssetRepo{}, domainRepo, orgRepo)

	pubOrg, err := orgSvc.CreateOrg(1, model.CreateOrgRequest{
		OrgCode: "pub-org",
		OrgName: "Public Org",
	})
	if err != nil {
		t.Fatalf("CreateOrg public failed: %v", err)
	}
	if pubOrg.Visibility != model.OrgVisibilityPublic {
		t.Fatalf("expected default visibility 'public', got %q", pubOrg.Visibility)
	}

	privOrg, err := orgSvc.CreateOrg(1, model.CreateOrgRequest{
		OrgCode:    "priv-org",
		OrgName:    "Private Org",
		Visibility: model.OrgVisibilityPrivate,
	})
	if err != nil {
		t.Fatalf("CreateOrg private failed: %v", err)
	}
	if privOrg.Visibility != model.OrgVisibilityPrivate {
		t.Fatalf("expected visibility 'private', got %q", privOrg.Visibility)
	}
	_ = orgSvc.AddMember(1, privOrg.ID, model.AddOrgMemberRequest{UserID: 2, OrgRole: model.OrgRoleMember})

	// 1. ListOrgs(square): outsider (3) only sees public org; member (2) sees both; super_admin sees both.
	list3, total3, err := orgSvc.ListOrgs(3, model.OrgQuery{Tab: "square", Page: 1, PageSize: 10})
	if err != nil || total3 != 1 || len(list3) != 1 || list3[0].ID != pubOrg.ID {
		t.Fatalf("outsider should only see 1 public org, got total=%d list=%+v err=%v", total3, list3, err)
	}
	list2, total2, err := orgSvc.ListOrgs(2, model.OrgQuery{Tab: "square", Page: 1, PageSize: 10})
	if err != nil || total2 != 2 || len(list2) != 2 {
		t.Fatalf("member should see both orgs, got total=%d err=%v", total2, err)
	}
	listSA, totalSA, err := orgSvc.ListOrgs(3, model.OrgQuery{Tab: "square", IsSuperAdmin: true, Page: 1, PageSize: 10})
	if err != nil || totalSA != 2 || len(listSA) != 2 {
		t.Fatalf("super_admin should see all 2 orgs, got total=%d err=%v", totalSA, err)
	}

	// 2. GetOrgDetail / ListMembers on private org: outsider -> 403; member & super_admin -> 200.
	var appErr *AppError
	if _, err := orgSvc.GetOrgDetail(3, false, privOrg.ID); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider GetOrgDetail on private org, got %v", err)
	}
	if _, err := orgSvc.ListMembers(3, false, privOrg.ID); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 for outsider ListMembers on private org, got %v", err)
	}
	if _, err := orgSvc.GetOrgDetail(2, false, privOrg.ID); err != nil {
		t.Fatalf("expected member GetOrgDetail to succeed, got %v", err)
	}
	if _, err := orgSvc.GetOrgDetail(3, true, privOrg.ID); err != nil {
		t.Fatalf("expected super_admin GetOrgDetail to succeed, got %v", err)
	}

	// 3. RequestJoinOrg on private org -> 403 "私密组织仅支持管理员邀请加入".
	_, err = govSvc.RequestJoinOrg(3, privOrg.ID, "let me in")
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden || !strings.Contains(appErr.Message, "私密组织仅支持管理员邀请加入") {
		t.Fatalf("expected 403 '私密组织仅支持管理员邀请加入', got %v", err)
	}

	// 4. UpdateOrg visibility from private -> public allows RequestJoinOrg.
	updated, err := orgSvc.UpdateOrg(1, privOrg.ID, model.UpdateOrgRequest{Visibility: model.OrgVisibilityPublic})
	if err != nil || updated.Visibility != model.OrgVisibilityPublic {
		t.Fatalf("UpdateOrg visibility failed: updated=%+v err=%v", updated, err)
	}
	if _, err := govSvc.RequestJoinOrg(3, privOrg.ID, "now public"); err != nil {
		t.Fatalf("expected RequestJoinOrg to succeed after making org public, got %v", err)
	}
}

func TestCrossRefLifecycleApprovalAndCascadeCleanup(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "target_owner", Status: 1},
			2: {ID: 2, Username: "target_member", Status: 1},
			3: {ID: 3, Username: "source_owner", Status: 1},
		},
	}
	domainRepo := newMockSquareDomainRepo()
	_ = domainRepo.CreateWithOwner(&model.BizDomain{ID: 10, DomainCode: "target-dom", DomainName: "Target Domain", Visibility: model.DomainVisibilityPublic, OwnerID: 1})
	targetDom, _ := domainRepo.FindByCode("target-dom")
	_ = domainRepo.UpsertMember(&model.DomainMember{DomainID: targetDom.ID, UserID: 2, DomainRole: model.DomainRoleMember})

	_ = domainRepo.CreateWithOwner(&model.BizDomain{ID: 20, DomainCode: "source-dom", DomainName: "Source Domain", Visibility: model.DomainVisibilityPublic, OwnerID: 3})
	sourceDom, _ := domainRepo.FindByCode("source-dom")

	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			100: {
				ID: 100, DomainID: sourceDom.ID, OwnerID: 3, AssetCode: "AST-100",
				Name: "Public MCP", AssetType: model.AssetTypeMCP,
				Visibility: model.VisibilityPublic, Status: model.AssetStatusPublished,
			},
			101: {
				ID: 101, DomainID: sourceDom.ID, OwnerID: 3, AssetCode: "KB-101",
				Name: "Public KB", AssetType: model.AssetTypeKB,
				Visibility: model.VisibilityPublic, Status: model.AssetStatusPublished,
			},
		},
	}
	kbRepo := newMockFileKBRepo()
	doc1 := &model.KBDocument{ID: 201, KBAssetID: 101, FolderPath: "docs", IsFolder: false, FileName: "guide.md", FileExt: ".md"}
	doc2 := &model.KBDocument{ID: 202, KBAssetID: 101, FolderPath: "docs/sub", IsFolder: false, FileName: "nested.md", FileExt: ".md"}
	folderDoc := &model.KBDocument{ID: 200, KBAssetID: 101, FolderPath: "docs", IsFolder: true, FileName: "docs"}
	_ = kbRepo.CreateDocument(folderDoc)
	_ = kbRepo.CreateDocument(doc1)
	_ = kbRepo.CreateDocument(doc2)

	govRepo := newMockP1GovRepo()
	crossRefRepo := newMockCrossRefRepo(govRepo)
	auditRepo := &mockAuditRepo{}

	crossRefSvc := NewCrossRefService(crossRefRepo, domainRepo, nil, assetRepo, kbRepo, govRepo)
	crossRefSvc.SetAuditRepos(auditRepo, userRepo)

	govSvc := NewGovernanceService(govRepo, assetRepo, domainRepo, nil)
	govSvc.SetAuditRepos(auditRepo, userRepo)
	govSvc.SetCrossRefRepo(crossRefRepo)

	assetSvc := NewAssetService(assetRepo, domainRepo, govRepo, nil, "")
	assetSvc.SetCrossRefRepo(crossRefRepo)

	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, govRepo, nil, t.TempDir(), "secret")
	kbSvc.SetCrossRefRepo(crossRefRepo)

	domainSvc := NewDomainService(domainRepo, userRepo, nil)
	domainSvc.SetCrossRefRepo(crossRefRepo)

	// 1. Admin (user 1, target owner) creates cross-ref to asset 100 -> immediately active, no approval needed.
	adminRef, err := crossRefSvc.CreateCrossRef(1, false, targetDom.ID, model.CreateCrossRefRequest{
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeAsset,
		SourceAssetID:  100,
		Description:    "reuse MCP",
	})
	if err != nil {
		t.Fatalf("admin CreateCrossRef failed: %v", err)
	}
	if adminRef.Status != model.CrossRefStatusActive || adminRef.ApprovalRequestID != 0 {
		t.Fatalf("expected active cross-ref without approval, got status=%s reqID=%d", adminRef.Status, adminRef.ApprovalRequestID)
	}

	// Duplicate active cross-ref -> 409 Conflict.
	var appErr *AppError
	if _, err := crossRefSvc.CreateCrossRef(2, false, targetDom.ID, model.CreateCrossRefRequest{
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeAsset,
		SourceAssetID:  100,
	}); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate cross-ref, got %v", err)
	}

	// 2. Normal member (user 2) creates cross-ref to KB file doc1 -> pending + approval request created.
	memberRef, err := crossRefSvc.CreateCrossRef(2, false, targetDom.ID, model.CreateCrossRefRequest{
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeKBFile,
		SourceAssetID:  101,
		SourceDocID:    doc1.ID,
		Description:    "need guide.md",
	})
	if err != nil {
		t.Fatalf("member CreateCrossRef failed: %v", err)
	}
	if memberRef.Status != model.CrossRefStatusPending || memberRef.ApprovalRequestID == 0 {
		t.Fatalf("expected pending cross-ref with approval request, got status=%s reqID=%d", memberRef.Status, memberRef.ApprovalRequestID)
	}

	// Default ListCrossRefs only shows active (1 item); include_pending=1 shows both for member.
	activeList, err := crossRefSvc.ListCrossRefs(2, false, targetDom.ID, false)
	if err != nil || len(activeList) != 1 {
		t.Fatalf("expected 1 active cross-ref, got %d err=%v", len(activeList), err)
	}
	allList, err := crossRefSvc.ListCrossRefs(2, false, targetDom.ID, true)
	if err != nil || len(allList) != 2 {
		t.Fatalf("expected 2 cross-refs with include_pending=1, got %d err=%v", len(allList), err)
	}

	// Non-admin (user 2) cannot approve -> 403.
	if _, err := govSvc.HandleApproval(2, false, memberRef.ApprovalRequestID, model.HandleApprovalRequest{Action: "approve"}); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 when non-admin handles cross-ref approval, got %v", err)
	}

	// Target owner (user 1) approves -> approval approved, crossRef status updated (mock govRepo updates req; let's also verify audit log).
	approvedReq, err := govSvc.HandleApproval(1, false, memberRef.ApprovalRequestID, model.HandleApprovalRequest{Action: "approve", Comment: "approved"})
	if err != nil || approvedReq.Status != model.ApprovalStatusApproved {
		t.Fatalf("HandleApproval failed: req=%+v err=%v", approvedReq, err)
	}

	// 3. Delete cross-ref does NOT delete source asset 100.
	if err := crossRefSvc.DeleteCrossRef(1, false, targetDom.ID, adminRef.ID); err != nil {
		t.Fatalf("DeleteCrossRef failed: %v", err)
	}
	if _, err := assetRepo.FindByID(100); err != nil {
		t.Fatalf("source asset 100 must remain intact after DeleteCrossRef, got %v", err)
	}

	// 4. Cascade cleanup:
	// 4a. DeleteAsset(100) cleans up cross-refs pointing to asset 100.
	refToDeleteByAsset, err := crossRefSvc.CreateCrossRef(1, false, targetDom.ID, model.CreateCrossRefRequest{
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeAsset,
		SourceAssetID:  100,
	})
	if err != nil {
		t.Fatalf("re-create cross-ref failed: %v", err)
	}
	if err := assetSvc.DeleteAsset(3, false, 100); err != nil {
		t.Fatalf("DeleteAsset failed: %v", err)
	}
	if _, err := crossRefRepo.FindByID(refToDeleteByAsset.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected cross-ref %d to be cascade-deleted when asset 100 deleted, got %v", refToDeleteByAsset.ID, err)
	}

	// 4b. DeleteDocument on folder cleans up cross-refs pointing to doc1 & doc2.
	refDoc2, err := crossRefSvc.CreateCrossRef(1, false, targetDom.ID, model.CreateCrossRefRequest{
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeKBFile,
		SourceAssetID:  101,
		SourceDocID:    doc2.ID,
	})
	if err != nil {
		t.Fatalf("create cross-ref to doc2 failed: %v", err)
	}
	if err := kbSvc.DeleteDocument(3, false, 101, folderDoc.ID); err != nil {
		t.Fatalf("DeleteDocument folder failed: %v", err)
	}
	if _, err := crossRefRepo.FindByID(memberRef.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected doc1 cross-ref to be cascade-deleted, got %v", err)
	}
	if _, err := crossRefRepo.FindByID(refDoc2.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected doc2 cross-ref to be cascade-deleted, got %v", err)
	}

	// 4c. DeleteDomain cleans up any remaining cross-refs referencing sourceDom or targetDom.
	_, err = crossRefSvc.CreateCrossRef(1, false, targetDom.ID, model.CreateCrossRefRequest{
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeAsset,
		SourceAssetID:  101,
	})
	if err != nil {
		t.Fatalf("create cross-ref to 101 failed: %v", err)
	}
	if err := domainSvc.DeleteDomain(1, targetDom.ID); err != nil {
		t.Fatalf("DeleteDomain failed: %v", err)
	}
	if len(crossRefRepo.refs) != 0 {
		t.Fatalf("expected all cross-refs for deleted domain to be cleaned up, remaining=%d", len(crossRefRepo.refs))
	}
}

func TestCrossRefKBReadOnlyBrowsing(t *testing.T) {
	userRepo := &mockP1UserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "target_owner", Status: 1},
			2: {ID: 2, Username: "target_member", Status: 1},
			3: {ID: 3, Username: "source_owner", Status: 1},
			4: {ID: 4, Username: "outsider", Status: 1},
		},
	}
	orgRepo := newMockOrgRepo()
	domainRepo := newMockSquareDomainRepo()
	_ = domainRepo.CreateWithOwner(&model.BizDomain{ID: 10, DomainCode: "target_kb_dom", DomainName: "Target", Visibility: model.DomainVisibilityPrivate, OwnerID: 1})
	targetDom, _ := domainRepo.FindByCode("target_kb_dom")
	_ = domainRepo.UpsertMember(&model.DomainMember{DomainID: targetDom.ID, UserID: 2, DomainRole: model.DomainRoleMember})
	_ = domainRepo.CreateWithOwner(&model.BizDomain{ID: 20, DomainCode: "source_kb_dom", DomainName: "Source", Visibility: model.DomainVisibilityPrivate, OwnerID: 3})
	sourceDom, _ := domainRepo.FindByCode("source_kb_dom")

	// Source domain KB (domain visibility, user 2 has NO direct access to sourceDom).
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			200: {
				ID: 200, AssetCode: "KB-200", Name: "Whole Ref KB", AssetType: model.AssetTypeKB,
				DomainID: sourceDom.ID, OwnerID: 3, Visibility: model.VisibilityDomain, Status: model.AssetStatusPublished,
			},
			201: {
				ID: 201, AssetCode: "KB-201", Name: "Single File KB", AssetType: model.AssetTypeKB,
				DomainID: sourceDom.ID, OwnerID: 3, Visibility: model.VisibilityDomain, Status: model.AssetStatusPublished,
			},
		},
	}
	govRepo := newMockP1GovRepo()
	kbRepo := newMockFileKBRepo()
	crossRefRepo := newMockCrossRefRepo(govRepo)
	interRepo := newMockKBInteractionRepo()

	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, govRepo, orgRepo, t.TempDir(), "secret")
	kbSvc.SetCrossRefRepo(crossRefRepo)
	kbSvc.SetInteractionRepo(interRepo)

	interSvc := NewKBInteractionService(interRepo, kbRepo, assetRepo, domainRepo, govRepo, orgRepo, userRepo)
	interSvc.SetCrossRefRepo(crossRefRepo)

	docWhole, err := kbSvc.CreateMarkdownFile(3, false, 200, model.CreateKBFileRequest{FileName: "whole.md", Content: "# Whole KB"})
	if err != nil {
		t.Fatalf("create docWhole failed: %v", err)
	}
	docAllowed, err := kbSvc.CreateMarkdownFile(3, false, 201, model.CreateKBFileRequest{FileName: "allowed.md", Content: "# Allowed File"})
	if err != nil {
		t.Fatalf("create docAllowed failed: %v", err)
	}
	docSecret, err := kbSvc.CreateMarkdownFile(3, false, 201, model.CreateKBFileRequest{FileName: "secret.md", Content: "# Secret File"})
	if err != nil {
		t.Fatalf("create docSecret failed: %v", err)
	}

	// Cross-refs:
	// 1. Active whole-asset ref to KB 200.
	if err := crossRefRepo.Create(&model.DomainCrossRef{
		TargetDomainID: targetDom.ID,
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeAsset,
		SourceAssetID:  200,
		Title:          "Whole Ref KB",
		CreatedBy:      1,
		Status:         model.CrossRefStatusActive,
	}); err != nil {
		t.Fatalf("create active asset ref failed: %v", err)
	}
	// 2. Active single-file ref to docAllowed in KB 201.
	if err := crossRefRepo.Create(&model.DomainCrossRef{
		TargetDomainID: targetDom.ID,
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeKBFile,
		SourceAssetID:  201,
		SourceDocID:    docAllowed.ID,
		Title:          "allowed.md",
		CreatedBy:      1,
		Status:         model.CrossRefStatusActive,
	}); err != nil {
		t.Fatalf("create active kb_file ref failed: %v", err)
	}
	// 3. Pending ref to docSecret in KB 201 (must NOT grant access).
	if err := crossRefRepo.Create(&model.DomainCrossRef{
		TargetDomainID: targetDom.ID,
		SourceDomainID: sourceDom.ID,
		RefType:        model.CrossRefTypeKBFile,
		SourceAssetID:  201,
		SourceDocID:    docSecret.ID,
		Title:          "secret.md",
		CreatedBy:      2,
		Status:         model.CrossRefStatusPending,
	}); err != nil {
		t.Fatalf("create pending kb_file ref failed: %v", err)
	}

	// Helper to assert 403 Forbidden.
	assertForbidden := func(err error, msg string) {
		t.Helper()
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
			t.Fatalf("%s: expected 403 Forbidden, got %v", msg, err)
		}
	}

	// Case A: Whole KB asset cross-ref (KB 200).
	// Without refDomainID -> 403.
	_, err = kbSvc.ListDocuments(2, false, 200, 0)
	assertForbidden(err, "whole KB ListDocuments without refDomainID")

	// With refDomainID -> target member (user 2) can list, read content, search, and read interactions.
	docs, err := kbSvc.ListDocuments(2, false, 200, targetDom.ID)
	if err != nil || len(docs) != 1 {
		t.Fatalf("expected 1 doc in whole KB ListDocuments with refDomainID, got %d err=%v", len(docs), err)
	}
	contentRes, err := kbSvc.GetDocumentContent(2, false, 200, docWhole.ID, false, targetDom.ID)
	if err != nil || contentRes["content"] != "# Whole KB" {
		t.Fatalf("expected whole KB content, got %+v err=%v", contentRes, err)
	}
	if _, err := kbSvc.Search(2, false, 200, model.KBSearchRequest{Query: "Whole"}, targetDom.ID); err != nil {
		t.Fatalf("expected whole KB Search to succeed with refDomainID, got %v", err)
	}
	if _, err := interSvc.GetInteractions(2, false, 200, docWhole.ID, targetDom.ID); err != nil {
		t.Fatalf("expected whole KB GetInteractions to succeed with refDomainID, got %v", err)
	}
	// Outsider (user 4) is NOT in targetDom -> 403 even with refDomainID.
	_, err = kbSvc.ListDocuments(4, false, 200, targetDom.ID)
	assertForbidden(err, "outsider ListDocuments with refDomainID")

	// Case B: Single KB file cross-ref (docAllowed in KB 201).
	// Target member (user 2) can read docAllowed content, raw, chunks, download single file, and interactions.
	allowedContent, err := kbSvc.GetDocumentContent(2, false, 201, docAllowed.ID, false, targetDom.ID)
	if err != nil || allowedContent["content"] != "# Allowed File" {
		t.Fatalf("expected allowed file content, got %+v err=%v", allowedContent, err)
	}
	if _, _, _, _, err := kbSvc.GetDocumentRaw(2, false, 201, docAllowed.ID, false, targetDom.ID); err != nil {
		t.Fatalf("expected GetDocumentRaw on allowed file to succeed, got %v", err)
	}
	if _, err := kbSvc.GetDownloadPreview(2, false, 201, "file", docAllowed.ID, "", targetDom.ID); err != nil {
		t.Fatalf("expected single-file GetDownloadPreview to succeed, got %v", err)
	}
	if _, err := interSvc.GetInteractions(2, false, 201, docAllowed.ID, targetDom.ID); err != nil {
		t.Fatalf("expected GetInteractions on allowed file to succeed, got %v", err)
	}

	// Target member CANNOT list source KB 201, search KB 201, download scope=all/folder, or read unreferenced/pending docSecret.
	_, err = kbSvc.ListDocuments(2, false, 201, targetDom.ID)
	assertForbidden(err, "kb_file ref must not allow ListDocuments")
	_, err = kbSvc.ListDocumentsLazy(2, false, 201, "", targetDom.ID)
	assertForbidden(err, "kb_file ref must not allow ListDocumentsLazy")
	_, err = kbSvc.Search(2, false, 201, model.KBSearchRequest{Query: "Secret"}, targetDom.ID)
	assertForbidden(err, "kb_file ref must not allow Search")
	_, err = kbSvc.GetDownloadPreview(2, false, 201, "all", docAllowed.ID, "", targetDom.ID)
	assertForbidden(err, "kb_file ref must not allow DownloadPreview scope=all")
	_, err = kbSvc.GetDownloadPreview(2, false, 201, "folder", docAllowed.ID, "", targetDom.ID)
	assertForbidden(err, "kb_file ref must not allow DownloadPreview scope=folder")
	_, _, _, err = kbSvc.DownloadKB(2, false, 201, "all", docAllowed.ID, "", targetDom.ID)
	assertForbidden(err, "kb_file ref must not allow DownloadKB scope=all")
	_, err = kbSvc.GetDocumentContent(2, false, 201, docSecret.ID, false, targetDom.ID)
	assertForbidden(err, "pending kb_file ref must not allow GetDocumentContent on docSecret")
	_, err = interSvc.GetInteractions(2, false, 201, docSecret.ID, targetDom.ID)
	assertForbidden(err, "pending kb_file ref must not allow GetInteractions on docSecret")

	// Case C: Write operations remain strictly 403 even with active cross-ref (both whole KB and kb_file).
	_, err = kbSvc.CreateFolder(2, false, 200, "hacked")
	assertForbidden(err, "CreateFolder on cross-ref KB must be 403")
	_, err = kbSvc.UpdateMarkdownContent(2, false, 200, docWhole.ID, model.UpdateKBFileRequest{Content: "mutated"})
	assertForbidden(err, "UpdateMarkdownContent on cross-ref KB must be 403")
	err = kbSvc.DeleteDocument(2, false, 200, docWhole.ID)
	assertForbidden(err, "DeleteDocument on cross-ref KB must be 403")
	err = kbSvc.DeleteDocument(2, false, 201, docAllowed.ID)
	assertForbidden(err, "DeleteDocument on cross-ref kb_file must be 403")
}
