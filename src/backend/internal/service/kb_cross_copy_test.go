package service

import (
	"errors"
	"path"
	"sort"
	"strings"
	"testing"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type crossCopyKBRepo struct {
	*mockFileKBRepo
}

func newCrossCopyKBRepo() *crossCopyKBRepo {
	return &crossCopyKBRepo{mockFileKBRepo: newMockFileKBRepo()}
}

func (m *crossCopyKBRepo) EnsureFolderHierarchy(kbAssetID, uploaderID uint64, folderPath string) (*model.KBDocument, error) {
	if folderPath == "" {
		return nil, nil
	}
	segments := strings.Split(folderPath, "/")
	var lastFolder *model.KBDocument
	cur := ""
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if cur == "" {
			cur = seg
		} else {
			cur = cur + "/" + seg
		}
		var existing *model.KBDocument
		for _, d := range m.docs {
			if d.KBAssetID == kbAssetID && !d.DeletedAt.Valid && d.IsFolder && d.FolderPath == cur {
				existing = d
				break
			}
		}
		if existing != nil {
			lastFolder = existing
			continue
		}
		folder := &model.KBDocument{
			KBAssetID:   kbAssetID,
			FolderPath:  cur,
			IsFolder:    true,
			FileName:    path.Base(cur),
			ParseStatus: model.ParseStatusCompleted,
			UploaderID:  uploaderID,
		}
		if err := m.CreateDocument(folder); err != nil {
			return nil, err
		}
		lastFolder = m.docs[folder.ID]
	}
	return lastFolder, nil
}

type crossCopyAssetRepo struct {
	repository.AssetRepository
	assets map[uint64]*model.AssetDetail
}

func (m *crossCopyAssetRepo) GetDetailByID(id uint64) (*model.AssetDetail, error) {
	a, ok := m.assets[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *crossCopyAssetRepo) ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs, permittedAssetIDs []uint64) ([]model.AssetListItem, int64, error) {
	adminSet := make(map[uint64]bool, len(adminDomainIDs))
	for _, id := range adminDomainIDs {
		adminSet[id] = true
	}

	var out []model.AssetListItem
	for _, a := range m.assets {
		if q.AssetType != "" && a.AssetType != q.AssetType {
			continue
		}
		if q.Scope == "workspace" && !isSuperAdmin {
			if a.OwnerID != userID && !adminSet[a.DomainID] {
				continue
			}
		}
		out = append(out, model.AssetListItem{
			AIAsset:    a.AIAsset,
			DomainName: a.DomainName,
			OwnerName:  a.OwnerName,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID > out[j].ID
	})
	total := int64(len(out))
	if q.Page > 0 && q.PageSize > 0 {
		start := (q.Page - 1) * q.PageSize
		if start >= len(out) {
			return []model.AssetListItem{}, total, nil
		}
		end := start + q.PageSize
		if end > len(out) {
			end = len(out)
		}
		out = out[start:end]
	}
	return out, total, nil
}

type crossCopyDomainRepo struct {
	repository.DomainRepository
	roles map[uint64]map[uint64]string // domainID -> userID -> role
}

func (m *crossCopyDomainRepo) GetMemberRole(domainID, userID uint64) (string, error) {
	if users, ok := m.roles[domainID]; ok {
		return users[userID], nil
	}
	return "", nil
}

func (m *crossCopyDomainRepo) GetUserAdminDomainIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	for domainID, users := range m.roles {
		role := users[userID]
		if role == model.DomainRoleOwner || role == model.DomainRoleAdmin {
			ids = append(ids, domainID)
		}
	}
	return ids, nil
}

func setupCrossCopyTestEnv(t *testing.T) (*kbService, *crossCopyKBRepo) {
	t.Helper()
	kbRepo := newCrossCopyKBRepo()
	assetRepo := &crossCopyAssetRepo{
		assets: map[uint64]*model.AssetDetail{
			1: {
				AIAsset: model.AIAsset{
					ID:         1,
					AssetCode:  "KB-001",
					Name:       "Public Source KB",
					AssetType:  model.AssetTypeKB,
					DomainID:   10,
					OwnerID:    100,
					Status:     model.AssetStatusPublished,
					Visibility: model.VisibilityPublic,
				},
				DomainName: "Alpha Domain",
			},
			2: {
				AIAsset: model.AIAsset{
					ID:         2,
					AssetCode:  "KB-002",
					Name:       "User B Personal KB",
					AssetType:  model.AssetTypeKB,
					DomainID:   20,
					OwnerID:    200,
					Status:     model.AssetStatusDraft,
					Visibility: model.VisibilityDomain,
				},
				DomainName: "Beta Domain",
			},
			3: {
				AIAsset: model.AIAsset{
					ID:         3,
					AssetCode:  "KB-003",
					Name:       "Alpha Private KB",
					AssetType:  model.AssetTypeKB,
					DomainID:   10,
					OwnerID:    100,
					Status:     model.AssetStatusDraft,
					Visibility: model.VisibilityDomain,
				},
				DomainName: "Alpha Domain",
			},
			4: {
				AIAsset: model.AIAsset{
					ID:         4,
					AssetCode:  "KB-004",
					Name:       "Gamma Admin KB",
					AssetType:  model.AssetTypeKB,
					DomainID:   30,
					OwnerID:    100,
					Status:     model.AssetStatusPublished,
					Visibility: model.VisibilityDomain,
				},
				DomainName: "Gamma Domain",
			},
		},
	}
	domainRepo := &crossCopyDomainRepo{
		roles: map[uint64]map[uint64]string{
			10: {100: model.DomainRoleOwner, 200: model.DomainRoleMember},
			20: {200: model.DomainRoleOwner},
			30: {100: model.DomainRoleOwner, 200: model.DomainRoleAdmin},
		},
	}

	svc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, t.TempDir(), "test-storage-secret").(*kbService)
	return svc, kbRepo
}

func seedFolderTreeInKB1(t *testing.T, svc *kbService, kbRepo *crossCopyKBRepo) *model.KBDocument {
	t.Helper()
	rootFolder, err := svc.CreateFolder(100, false, 1, "docs")
	if err != nil {
		t.Fatalf("CreateFolder(docs) failed: %v", err)
	}
	if _, err := svc.CreateFolder(100, false, 1, "docs/empty"); err != nil {
		t.Fatalf("CreateFolder(docs/empty) failed: %v", err)
	}
	if _, err := svc.CreateFolder(100, false, 1, "docs/sub"); err != nil {
		t.Fatalf("CreateFolder(docs/sub) failed: %v", err)
	}
	if _, err := svc.CreateMarkdownFile(100, false, 1, model.CreateKBFileRequest{
		FolderPath: "docs",
		FileName:   "readme.md",
		Content:    "# Root Readme",
	}); err != nil {
		t.Fatalf("CreateMarkdownFile(docs/readme.md) failed: %v", err)
	}
	if _, err := svc.CreateMarkdownFile(100, false, 1, model.CreateKBFileRequest{
		FolderPath: "docs/sub",
		FileName:   "guide.md",
		Content:    "# Nested Guide",
	}); err != nil {
		t.Fatalf("CreateMarkdownFile(docs/sub/guide.md) failed: %v", err)
	}
	return rootFolder
}

func TestKBService_CopyFolderRecursive_SameKB(t *testing.T) {
	svc, kbRepo := setupCrossCopyTestEnv(t)
	rootFolder := seedFolderTreeInKB1(t, svc, kbRepo)

	copiedRoot, err := svc.CopyDocumentWithOptions(100, false, 1, rootFolder.ID, model.CopyKBDocumentRequest{
		TargetFolderPath: "archive",
	})
	if err != nil {
		t.Fatalf("CopyDocumentWithOptions same KB failed: %v", err)
	}
	if copiedRoot.FolderPath != "archive/docs" {
		t.Fatalf("copiedRoot.FolderPath = %q, want %q", copiedRoot.FolderPath, "archive/docs")
	}

	docs, err := kbRepo.ListDocuments(1)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}

	hasFolder := func(p string) bool {
		for _, d := range docs {
			if d.IsFolder && d.FolderPath == p {
				return true
			}
		}
		return false
	}
	findFile := func(folder, name string) *model.KBDocument {
		for i := range docs {
			if !docs[i].IsFolder && docs[i].FolderPath == folder && docs[i].FileName == name {
				return &docs[i]
			}
		}
		return nil
	}

	for _, expectedFolder := range []string{"archive", "archive/docs", "archive/docs/empty", "archive/docs/sub"} {
		if !hasFolder(expectedFolder) {
			t.Fatalf("expected folder %q to exist in KB 1", expectedFolder)
		}
	}
	rootFile := findFile("archive/docs", "readme.md")
	if rootFile == nil {
		t.Fatal("expected archive/docs/readme.md to exist")
	}
	nestedFile := findFile("archive/docs/sub", "guide.md")
	if nestedFile == nil {
		t.Fatal("expected archive/docs/sub/guide.md to exist")
	}

	data, err := svc.readAndDecryptFile(nestedFile.FilePath)
	if err != nil {
		t.Fatalf("readAndDecryptFile failed: %v", err)
	}
	if string(data) != "# Nested Guide" {
		t.Fatalf("nested file content = %q, want %q", string(data), "# Nested Guide")
	}
}

func TestKBService_CopyFolder_RejectSelfOrSubfolderInSameKB(t *testing.T) {
	svc, kbRepo := setupCrossCopyTestEnv(t)
	rootFolder := seedFolderTreeInKB1(t, svc, kbRepo)

	for _, target := range []string{"docs", "docs/sub", "docs/empty"} {
		_, err := svc.CopyDocumentWithOptions(100, false, 1, rootFolder.ID, model.CopyKBDocumentRequest{
			TargetFolderPath: target,
		})
		if err == nil {
			t.Fatalf("expected error copying folder into %q, got nil", target)
		}
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.HTTPStatus != 400 {
			t.Fatalf("expected 400 BadRequest for target %q, got %v", target, err)
		}
	}
}

func TestKBService_CrossDomainCopy_PublicSourceToEditableTarget(t *testing.T) {
	svc, kbRepo := setupCrossCopyTestEnv(t)
	rootFolder := seedFolderTreeInKB1(t, svc, kbRepo)

	// User 200 is only a member in Domain 10 (KB 1 is Published+Public), and Owner of KB 2 in Domain 20.
	copiedFolder, err := svc.CopyDocumentWithOptions(200, false, 1, rootFolder.ID, model.CopyKBDocumentRequest{
		TargetKBID:       2,
		TargetFolderPath: "imported",
		NewName:          "alpha-docs",
	})
	if err != nil {
		t.Fatalf("cross-domain folder copy failed: %v", err)
	}
	if copiedFolder.KBAssetID != 2 || copiedFolder.FolderPath != "imported/alpha-docs" {
		t.Fatalf("unexpected copiedFolder: %+v", copiedFolder)
	}

	kb2Docs, err := kbRepo.ListDocuments(2)
	if err != nil {
		t.Fatalf("ListDocuments(2) failed: %v", err)
	}
	var foundEmpty, foundNestedFile bool
	for _, d := range kb2Docs {
		if d.IsFolder && d.FolderPath == "imported/alpha-docs/empty" {
			foundEmpty = true
		}
		if !d.IsFolder && d.FolderPath == "imported/alpha-docs/sub" && d.FileName == "guide.md" {
			foundNestedFile = true
			raw, err := svc.readAndDecryptFile(d.FilePath)
			if err != nil || string(raw) != "# Nested Guide" {
				t.Fatalf("unexpected copied file content: %q, err=%v", string(raw), err)
			}
		}
	}
	if !foundEmpty || !foundNestedFile {
		t.Fatalf("expected empty subfolder and nested file in KB 2, foundEmpty=%v foundNestedFile=%v", foundEmpty, foundNestedFile)
	}
}

func TestKBService_CrossCopy_PermissionDeniedOnTargetKB(t *testing.T) {
	svc, kbRepo := setupCrossCopyTestEnv(t)
	rootFolder := seedFolderTreeInKB1(t, svc, kbRepo)

	// User 200 lacks edit permission on KB 3 (in Domain 10, owned by 100, user 200 is only DomainRoleMember).
	_, err := svc.CopyDocumentWithOptions(200, false, 1, rootFolder.ID, model.CopyKBDocumentRequest{
		TargetKBID:       3,
		TargetFolderPath: "stolen",
	})
	if err == nil {
		t.Fatal("expected permission denied error when copying to non-editable target KB")
	}
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != 403 {
		t.Fatalf("expected 403 Forbidden, got %v", err)
	}
}

func TestKBService_ListEditableKBTargets(t *testing.T) {
	svc, _ := setupCrossCopyTestEnv(t)

	// User 200 owns KB 2 (Beta Domain) and is DomainRoleAdmin of Domain 30 (KB 4, Gamma Domain).
	targets, err := svc.ListEditableKBTargets(200, false)
	if err != nil {
		t.Fatalf("ListEditableKBTargets(200) failed: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 editable targets for user 200, got %d: %+v", len(targets), targets)
	}
	if targets[0].KBID != 2 || targets[1].KBID != 4 {
		t.Fatalf("unexpected targets order/IDs for user 200: %+v", targets)
	}

	// SuperAdmin sees all 4 KBs sorted by DomainName, KBName, KBID.
	allTargets, err := svc.ListEditableKBTargets(999, true)
	if err != nil {
		t.Fatalf("ListEditableKBTargets(superAdmin) failed: %v", err)
	}
	if len(allTargets) != 4 {
		t.Fatalf("expected 4 editable targets for superAdmin, got %d", len(allTargets))
	}
}
