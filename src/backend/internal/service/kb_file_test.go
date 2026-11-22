package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/kbcodec"

	"gorm.io/gorm"
)

type mockFileKBRepo struct {
	nextID uint64
	docs   map[uint64]*model.KBDocument
	chunks map[uint64][]model.KBChunk
}

func newMockFileKBRepo() *mockFileKBRepo {
	return &mockFileKBRepo{
		nextID: 1,
		docs:   make(map[uint64]*model.KBDocument),
		chunks: make(map[uint64][]model.KBChunk),
	}
}

func (m *mockFileKBRepo) CreateDocument(doc *model.KBDocument) error {
	doc.ID = m.nextID
	m.nextID++
	cp := *doc
	m.docs[doc.ID] = &cp
	return nil
}
func (m *mockFileKBRepo) EnsureFolderHierarchy(kbAssetID, uploaderID uint64, folderPath string) (*model.KBDocument, error) {
	return &model.KBDocument{ID: 99, KBAssetID: kbAssetID, FolderPath: folderPath, IsFolder: true}, nil
}
func (m *mockFileKBRepo) ListDocuments(kbAssetID uint64) ([]model.KBDocument, error) {
	var out []model.KBDocument
	for _, d := range m.docs {
		if d.KBAssetID == kbAssetID {
			out = append(out, *d)
		}
	}
	return out, nil
}
func (m *mockFileKBRepo) ListDocumentsByParent(kbAssetID uint64, parentFolder string) ([]model.KBDocument, error) {
	var out []model.KBDocument
	for _, d := range m.docs {
		if d.KBAssetID != kbAssetID {
			continue
		}
		if d.IsFolder {
			if parentFolder == "" {
				if d.FolderPath != "" && !strings.Contains(d.FolderPath, "/") {
					out = append(out, *d)
				}
			} else {
				prefix := parentFolder + "/"
				if strings.HasPrefix(d.FolderPath, prefix) && !strings.Contains(strings.TrimPrefix(d.FolderPath, prefix), "/") {
					out = append(out, *d)
				}
			}
		} else if d.FolderPath == parentFolder {
			out = append(out, *d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsFolder != out[j].IsFolder {
			return out[i].IsFolder
		}
		return out[i].FileName < out[j].FileName
	})
	return out, nil
}
func (m *mockFileKBRepo) FindDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error) {
	if d, ok := m.docs[docID]; ok && d.KBAssetID == kbAssetID {
		return d, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockFileKBRepo) ListMatchingFiles(kbAssetID uint64, scope string, docID uint64, folderPath string) ([]model.KBDocument, error) {
	var out []model.KBDocument
	for _, d := range m.docs {
		if d.KBAssetID != kbAssetID || d.IsFolder {
			continue
		}
		switch scope {
		case "file":
			if d.ID == docID {
				out = append(out, *d)
			}
		case "folder":
			if folderPath == "" || d.FolderPath == folderPath || strings.HasPrefix(d.FolderPath, folderPath+"/") {
				out = append(out, *d)
			}
		case "all":
			out = append(out, *d)
		}
	}
	return out, nil
}
func (m *mockFileKBRepo) FileExistsInFolder(kbAssetID uint64, folderPath, fileName string, excludeDocID uint64) (bool, error) {
	for _, d := range m.docs {
		if d.KBAssetID == kbAssetID && !d.IsFolder && d.FolderPath == folderPath && d.FileName == fileName && d.ID != excludeDocID {
			return true, nil
		}
	}
	return false, nil
}
func (m *mockFileKBRepo) MoveDocument(kbAssetID, docID uint64, targetFolder string) error {
	if d, ok := m.docs[docID]; ok && d.KBAssetID == kbAssetID {
		d.FolderPath = targetFolder
		return nil
	}
	return gorm.ErrRecordNotFound
}
func (m *mockFileKBRepo) ReplaceDocumentChunks(kbAssetID uint64, doc *model.KBDocument, chunks []model.KBChunk) error {
	cp := *doc
	m.docs[doc.ID] = &cp
	if m.chunks == nil {
		m.chunks = make(map[uint64][]model.KBChunk)
	}
	m.chunks[doc.ID] = chunks
	return nil
}
func (m *mockFileKBRepo) CompleteDocumentParsing(kbAssetID, docID uint64, chunks []model.KBChunk) error {
	if m.chunks == nil {
		m.chunks = make(map[uint64][]model.KBChunk)
	}
	m.chunks[docID] = chunks
	return nil
}
func (m *mockFileKBRepo) FailDocumentParsing(docID uint64, errMsg string) error { return nil }
func (m *mockFileKBRepo) ListChunks(kbAssetID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error) {
	return nil, 0, nil
}
func (m *mockFileKBRepo) DeleteDocument(kbAssetID, docID uint64) error {
	delete(m.docs, docID)
	return nil
}
func (m *mockFileKBRepo) DeleteFolderRecursive(kbAssetID uint64, folderPath string) ([]string, error) {
	return nil, nil
}
func (m *mockFileKBRepo) SearchChunks(kbAssetID uint64, query string, topK int) ([]model.KBSearchHit, error) {
	return nil, nil
}

func TestNormalizeFolderPath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"/guides/api/", "guides/api", false},
		{`\guides\\nested\`, "guides/nested", false},
		{"guides/../secret", "", true},
		{"./guides", "", true},
	}

	for _, tc := range cases {
		got, err := NormalizeFolderPath(tc.in)
		if (err != nil) != tc.wantErr {
			t.Fatalf("NormalizeFolderPath(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
		}
		if !tc.wantErr && got != tc.want {
			t.Fatalf("NormalizeFolderPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestKBDownloadPreviewAndZip(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "intro.md")
	f2 := filepath.Join(tmpDir, "auth.md")
	f3 := filepath.Join(tmpDir, "readme.md")
	_ = os.WriteFile(f1, []byte("# Intro\nHello"), 0644)           // 13 bytes
	_ = os.WriteFile(f2, []byte("# Auth\nJWT token"), 0644)        // 16 bytes
	_ = os.WriteFile(f3, []byte("# Root Readme"), 0644)            // 13 bytes

	kbRepo := newMockFileKBRepo()
	_ = kbRepo.CreateDocument(&model.KBDocument{KBAssetID: 1, FolderPath: "guides", FileName: "intro.md", FileSize: 13, FilePath: f1})
	_ = kbRepo.CreateDocument(&model.KBDocument{KBAssetID: 1, FolderPath: "guides/api", FileName: "auth.md", FileSize: 16, FilePath: f2})
	_ = kbRepo.CreateDocument(&model.KBDocument{KBAssetID: 1, FolderPath: "", FileName: "readme.md", FileSize: 13, FilePath: f3})

	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			1: {ID: 1, AssetCode: "kb-docs", AssetType: model.AssetTypeKB, DomainID: 1, OwnerID: 10, Visibility: model.VisibilityPublic, Status: model.AssetStatusPublished},
		},
	}
	domainRepo := &mockP1DomainRepo{members: map[uint64]map[uint64]string{}}
	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, tmpDir, "test-storage-secret")

	// 1. Recursive folder preview ("guides" includes "guides/intro.md" + "guides/api/auth.md")
	prevFolder, err := kbSvc.GetDownloadPreview(10, false, 1, "folder", 0, "/guides/", 0)
	if err != nil {
		t.Fatalf("GetDownloadPreview folder failed: %v", err)
	}
	if prevFolder.FileCount != 2 || prevFolder.TotalBytes != 29 {
		t.Fatalf("expected 2 files / 29 bytes, got %+v", prevFolder)
	}

	// 2. All files preview
	prevAll, err := kbSvc.GetDownloadPreview(10, false, 1, "all", 0, "", 0)
	if err != nil {
		t.Fatalf("GetDownloadPreview all failed: %v", err)
	}
	if prevAll.FileCount != 3 || prevAll.TotalBytes != 42 {
		t.Fatalf("expected 3 files / 42 bytes, got %+v", prevAll)
	}

	// 3. Single file preview
	prevFile, err := kbSvc.GetDownloadPreview(10, false, 1, "file", 2, "", 0)
	if err != nil {
		t.Fatalf("GetDownloadPreview file failed: %v", err)
	}
	if prevFile.FileCount != 1 || prevFile.TotalBytes != 16 || prevFile.Target != "guides/api/auth.md" {
		t.Fatalf("unexpected single file preview: %+v", prevFile)
	}

	// 4. Download folder as zip and inspect entries
	fn, ct, zipBytes, err := kbSvc.DownloadKB(10, false, 1, "folder", 0, "guides", 0)
	if err != nil {
		t.Fatalf("DownloadKB folder failed: %v", err)
	}
	if fn != "kb-docs.zip" || ct != "application/zip" {
		t.Fatalf("unexpected filename/contentType: %s %s", fn, ct)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("invalid zip output: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("expected 2 files in zip, got %d", len(zr.File))
	}
}

type mockAuditRepo struct {
	logs []model.AssetAuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *model.AssetAuditLog) error {
	cp := *log
	cp.ID = uint64(len(m.logs) + 1)
	m.logs = append(m.logs, cp)
	return nil
}

func (m *mockAuditRepo) ListManagedLogs(ctx context.Context, userID uint64, managedDomainIDs, managedOrgIDs []uint64, q model.AuditLogQuery) ([]model.AssetAuditLog, int64, error) {
	domSet := toUint64Set(managedDomainIDs)
	orgSet := toUint64Set(managedOrgIDs)
	var out []model.AssetAuditLog
	for _, l := range m.logs {
		if l.OperatorID == userID || domSet[l.DomainID] || (l.AssetType == "org" && orgSet[l.AssetID]) {
			out = append(out, l)
		}
	}
	return out, int64(len(out)), nil
}

func TestKBMoveCopyAndAuditLogs(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "intro.md")
	_ = os.WriteFile(srcFile, []byte("# Intro\nHello world"), 0644)

	kbRepo := newMockFileKBRepo()
	_ = kbRepo.CreateDocument(&model.KBDocument{
		KBAssetID: 1, FolderPath: "guides", FileName: "intro.md", FileExt: ".md", FileSize: 19, FilePath: srcFile,
	})
	_ = kbRepo.CreateDocument(&model.KBDocument{
		KBAssetID: 1, FolderPath: "archive", FileName: "intro.md", FileExt: ".md", FileSize: 19, FilePath: srcFile,
	})

	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			1: {ID: 1, AssetCode: "kb-docs", Name: "Docs KB", AssetType: model.AssetTypeKB, DomainID: 5, OwnerID: 10},
		},
	}
	domainRepo := &mockP1DomainRepo{
		members: map[uint64]map[uint64]string{
			5: {10: model.DomainRoleMember, 30: model.DomainRoleAdmin},
		},
	}
	auditRepo := &mockAuditRepo{}

	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, tmpDir, "test-storage-secret")
	kbSvc.SetAuditRepos(auditRepo, nil)
	auditSvc := NewAuditService(auditRepo, domainRepo, nil)

	// 1. Move to folder with conflict -> error
	if _, err := kbSvc.MoveDocument(10, false, 1, 1, "archive"); err == nil {
		t.Fatal("expected conflict error when moving to folder with same filename")
	}

	// 2. Move to root ("") -> succeeds and logs kb.file_move
	moved, err := kbSvc.MoveDocument(10, false, 1, 1, "")
	if err != nil {
		t.Fatalf("MoveDocument to root failed: %v", err)
	}
	if moved.FolderPath != "" {
		t.Fatalf("expected root folder_path '', got %q", moved.FolderPath)
	}

	// 3. Copy to "archive" -> auto-deduplicates to "intro-copy.md" and logs kb.file_copy
	copied, err := kbSvc.CopyDocument(10, false, 1, 1, "archive", "")
	if err != nil {
		t.Fatalf("CopyDocument failed: %v", err)
	}
	if copied.FileName != "intro-copy.md" || copied.FolderPath != "archive" {
		t.Fatalf("expected archive/intro-copy.md, got %s/%s", copied.FolderPath, copied.FileName)
	}
	if _, err := os.Stat(copied.FilePath); err != nil {
		t.Fatalf("expected copied file on disk, got err: %v", err)
	}

	// 4. Verify audit logs & scoping
	if len(auditRepo.logs) != 2 || auditRepo.logs[0].Action != model.AuditActionKBFileMove || auditRepo.logs[1].Action != model.AuditActionKBFileCopy {
		t.Fatalf("unexpected audit logs: %+v", auditRepo.logs)
	}
	adminLogs, total, err := auditSvc.ListManagedLogs(context.Background(), 30, model.AuditLogQuery{})
	if err != nil || total != 2 || len(adminLogs) != 2 {
		t.Fatalf("expected domain admin (30) to see 2 logs, got total=%d len=%d err=%v", total, len(adminLogs), err)
	}
	strangerLogs, totalStranger, err := auditSvc.ListManagedLogs(context.Background(), 99, model.AuditLogQuery{})
	if err != nil || totalStranger != 0 || len(strangerLogs) != 0 {
		t.Fatalf("expected stranger (99) to see 0 logs, got total=%d", totalStranger)
	}
}

func TestKBCreateAndUpdateMarkdownFile(t *testing.T) {
	tmpDir := t.TempDir()
	kbRepo := newMockFileKBRepo()
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			1: {ID: 1, AssetCode: "kb-md", Name: "Markdown KB", AssetType: model.AssetTypeKB, DomainID: 5, OwnerID: 10},
		},
	}
	domainRepo := &mockP1DomainRepo{
		members: map[uint64]map[uint64]string{
			5: {10: model.DomainRoleOwner},
		},
	}
	auditRepo := &mockAuditRepo{}

	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, tmpDir, "test-storage-secret")
	kbSvc.SetAuditRepos(auditRepo, nil)

	// 1. CreateMarkdownFile without extension -> auto-appends .md
	doc1, err := kbSvc.CreateMarkdownFile(10, false, 1, model.CreateKBFileRequest{
		FolderPath: "/guides/",
		FileName:   "quickstart",
		Content:    "# Quickstart\nStep 1",
	})
	if err != nil {
		t.Fatalf("CreateMarkdownFile failed: %v", err)
	}
	if doc1.FileName != "quickstart.md" || doc1.FolderPath != "guides" || doc1.ChunkCount != 1 {
		t.Fatalf("unexpected created doc: %+v", doc1)
	}

	// 2. Duplicate CreateMarkdownFile -> 409 Conflict
	_, err = kbSvc.CreateMarkdownFile(10, false, 1, model.CreateKBFileRequest{
		FolderPath: "guides",
		FileName:   "quickstart.md",
		Content:    "dup",
	})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate file, got %v", err)
	}

	// Create a second file to test rename conflict
	_, err = kbSvc.CreateMarkdownFile(10, false, 1, model.CreateKBFileRequest{
		FolderPath: "guides",
		FileName:   "existing.md",
		Content:    "existing",
	})
	if err != nil {
		t.Fatalf("CreateMarkdownFile second failed: %v", err)
	}

	// 3. Rename conflict on UpdateMarkdownContent -> 409 Conflict
	_, err = kbSvc.UpdateMarkdownContent(10, false, 1, doc1.ID, model.UpdateKBFileRequest{
		FileName: "existing.md",
		Content:  "updated",
	})
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on rename collision, got %v", err)
	}

	// 4. Valid UpdateMarkdownContent with rename + new multi-line content
	longContent := strings.Repeat("ASCP knowledge base markdown content. ", 30)
	updated, err := kbSvc.UpdateMarkdownContent(10, false, 1, doc1.ID, model.UpdateKBFileRequest{
		FileName: "getting-started",
		Content:  longContent,
	})
	if err != nil {
		t.Fatalf("UpdateMarkdownContent failed: %v", err)
	}
	if updated.FileName != "getting-started.md" || updated.ChunkCount < 2 {
		t.Fatalf("expected renamed file with >=2 re-indexed chunks, got %+v", updated)
	}
	diskBytes, err := os.ReadFile(updated.FilePath)
	if err != nil {
		t.Fatalf("os.ReadFile failed: %v", err)
	}
	if !kbcodec.IsEncryptedAtRest(diskBytes) {
		t.Fatalf("expected updated disk file to be encrypted with %s", kbcodec.MagicHeader)
	}
	decrypted, err := kbcodec.DecryptAtRest(diskBytes, "test-storage-secret")
	if err != nil || string(decrypted) != longContent {
		t.Fatalf("expected decrypted updated content to match, err=%v", err)
	}

	// 5. Verify audit logs include kb.file_create and kb.file_update
	if len(auditRepo.logs) != 3 ||
		auditRepo.logs[0].Action != model.AuditActionKBFileCreate ||
		auditRepo.logs[2].Action != model.AuditActionKBFileUpdate {
		t.Fatalf("unexpected audit logs: %+v", auditRepo.logs)
	}
}

func makeTestFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Write part failed: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("Close multipart writer failed: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("ParseMultipartForm failed: %v", err)
	}
	return req.MultipartForm.File[fieldName][0]
}

func TestKBAnyFileUploadSizeLimitAndRawPreview(t *testing.T) {
	tmpDir := t.TempDir()
	kbRepo := newMockFileKBRepo()
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			1: {ID: 1, AssetCode: "kb-media", Name: "Media KB", AssetType: model.AssetTypeKB, DomainID: 5, OwnerID: 10, Visibility: model.VisibilityPublic, Status: model.AssetStatusPublished},
		},
	}
	domainRepo := &mockP1DomainRepo{
		members: map[uint64]map[uint64]string{
			5: {10: model.DomainRoleOwner},
		},
	}
	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, tmpDir, "test-storage-secret")

	// 1. Upload PNG image (no chunks created)
	pngBytes := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	pngFH := makeTestFileHeader(t, "file", "diagram.png", pngBytes)
	docs, err := kbSvc.UploadDocuments(10, false, 1, "assets", "", []*multipart.FileHeader{pngFH})
	if err != nil {
		t.Fatalf("UploadDocuments .png failed: %v", err)
	}
	if len(docs) != 1 || docs[0].ChunkCount != 0 || docs[0].ParseStatus != model.ParseStatusCompleted {
		t.Fatalf("expected 1 completed binary doc with chunk_count=0, got %+v", docs)
	}

	// 2. GetDocumentContent returns is_binary=true and empty content
	contentRes, err := kbSvc.GetDocumentContent(10, false, 1, docs[0].ID, false, 0)
	if err != nil {
		t.Fatalf("GetDocumentContent failed: %v", err)
	}
	if isBin, _ := contentRes["is_binary"].(bool); !isBin || contentRes["content"] != "" {
		t.Fatalf("expected is_binary=true & empty content, got %+v", contentRes)
	}

	// 3. GetDocumentRaw returns exact PNG bytes & image/png
	fn, mt, isSVG, raw, err := kbSvc.GetDocumentRaw(10, false, 1, docs[0].ID, false, 0)
	if err != nil {
		t.Fatalf("GetDocumentRaw failed: %v", err)
	}
	if fn != "diagram.png" || mt != "image/png" || isSVG || !bytes.Equal(raw, pngBytes) {
		t.Fatalf("unexpected raw result: fn=%s mt=%s isSVG=%v len=%d", fn, mt, isSVG, len(raw))
	}

	// 4. Reject > 10MB file with 400 Bad Request
	oversized := make([]byte, maxKBFileSize+1)
	bigFH := makeTestFileHeader(t, "file", "huge.bin", oversized)
	_, err = kbSvc.UploadDocuments(10, false, 1, "", "", []*multipart.FileHeader{bigFH})
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for >10MB file, got %v", err)
	}

	// 5. Upload a .md file and verify DownloadPreview & DownloadKB include both .md and .png
	mdBytes := []byte("# Notes\nSee diagram")
	mdFH := makeTestFileHeader(t, "file", "notes.md", mdBytes)
	if _, err := kbSvc.UploadDocuments(10, false, 1, "", "", []*multipart.FileHeader{mdFH}); err != nil {
		t.Fatalf("UploadDocuments .md failed: %v", err)
	}

	prev, err := kbSvc.GetDownloadPreview(10, false, 1, "all", 0, "", 0)
	if err != nil {
		t.Fatalf("GetDownloadPreview failed: %v", err)
	}
	wantTotal := int64(len(pngBytes) + len(mdBytes))
	if prev.FileCount != 2 || prev.TotalBytes != wantTotal {
		t.Fatalf("expected 2 files / %d bytes, got %+v", wantTotal, prev)
	}

	zipName, zipCT, zipData, err := kbSvc.DownloadKB(10, false, 1, "all", 0, "", 0)
	if err != nil || zipName != "kb-media.zip" || zipCT != "application/zip" {
		t.Fatalf("DownloadKB failed: name=%s ct=%s err=%v", zipName, zipCT, err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil || len(zr.File) != 2 {
		t.Fatalf("expected 2 files in zip, err=%v", err)
	}
}

func TestKBAtRestEncryptionAndTransportObfuscation(t *testing.T) {
	tmpDir := t.TempDir()
	kbRepo := newMockFileKBRepo()
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			1: {ID: 1, AssetCode: "kb-enc", Name: "Encrypted KB", AssetType: model.AssetTypeKB, DomainID: 5, OwnerID: 10, Visibility: model.VisibilityPublic, Status: model.AssetStatusPublished},
		},
	}
	domainRepo := &mockP1DomainRepo{members: map[uint64]map[uint64]string{}}
	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, tmpDir, "test-storage-secret")

	plainText := "# Secret Architecture\nSensitive plaintext that must NEVER appear on disk."
	doc, err := kbSvc.CreateMarkdownFile(10, false, 1, model.CreateKBFileRequest{
		FileName: "arch.md",
		Content:  plainText,
	})
	if err != nil {
		t.Fatalf("CreateMarkdownFile failed: %v", err)
	}

	// 1. Disk file starts with ASCPENC1 and does NOT contain plaintext
	diskBytes, err := os.ReadFile(doc.FilePath)
	if err != nil {
		t.Fatalf("os.ReadFile failed: %v", err)
	}
	if !kbcodec.IsEncryptedAtRest(diskBytes) {
		t.Fatalf("expected disk file to start with %s", kbcodec.MagicHeader)
	}
	if bytes.Contains(diskBytes, []byte("Sensitive plaintext")) {
		t.Fatal("disk file must not contain plaintext")
	}

	// 2. GetDocumentContent (obf=false) returns exact plaintext
	plainRes, err := kbSvc.GetDocumentContent(10, false, 1, doc.ID, false, 0)
	if err != nil {
		t.Fatalf("GetDocumentContent(obf=false) failed: %v", err)
	}
	if plainRes["content"] != plainText {
		t.Fatalf("unexpected content: %v", plainRes["content"])
	}

	// 3. GetDocumentContent (obf=true) returns obfuscated=true, payload, and empty content
	obfRes, err := kbSvc.GetDocumentContent(10, false, 1, doc.ID, true, 0)
	if err != nil {
		t.Fatalf("GetDocumentContent(obf=true) failed: %v", err)
	}
	if obfRes["obfuscated"] != true || obfRes["content"] != "" {
		t.Fatalf("unexpected obfuscated response metadata: %+v", obfRes)
	}
	payloadB64, _ := obfRes["payload"].(string)
	obfBytes, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	deobfBytes, err := kbcodec.DeobfuscateTransport(obfBytes)
	if err != nil {
		t.Fatalf("DeobfuscateTransport failed: %v", err)
	}
	if string(deobfBytes) != plainText {
		t.Fatalf("deobfuscated content mismatch: got %q want %q", string(deobfBytes), plainText)
	}

	// 4. GetDocumentRaw (obf=false & obf=true)
	_, mtPlain, _, rawPlain, err := kbSvc.GetDocumentRaw(10, false, 1, doc.ID, false, 0)
	if err != nil || !strings.HasPrefix(mtPlain, "text/markdown") || string(rawPlain) != plainText {
		t.Fatalf("GetDocumentRaw(obf=false) mismatch: mt=%s err=%v", mtPlain, err)
	}
	_, mtObf, _, rawObf, err := kbSvc.GetDocumentRaw(10, false, 1, doc.ID, true, 0)
	if err != nil || mtObf != "application/octet-stream" {
		t.Fatalf("GetDocumentRaw(obf=true) mismatch: mt=%s err=%v", mtObf, err)
	}
	rawDeobf, err := kbcodec.DeobfuscateTransport(rawObf)
	if err != nil || string(rawDeobf) != plainText {
		t.Fatalf("GetDocumentRaw deobfuscation mismatch: err=%v", err)
	}

	// 5. DownloadKB returns exact plaintext (single file)
	_, _, dlPlain, err := kbSvc.DownloadKB(10, false, 1, "file", doc.ID, "", 0)
	if err != nil || string(dlPlain) != plainText {
		t.Fatalf("DownloadKB file mismatch: err=%v", err)
	}

	// 6. EncryptExistingPlainFiles encrypts legacy plain KB files and skips non-KB dirs
	legacyPath := filepath.Join(tmpDir, "1", "legacy.md")
	if err := os.WriteFile(legacyPath, []byte("legacy plain text"), 0644); err != nil {
		t.Fatalf("write legacy file failed: %v", err)
	}
	skillDir := filepath.Join(tmpDir, "skills", "99")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill dir failed: %v", err)
	}
	skillPkgPath := filepath.Join(skillDir, "pkg.zip")
	if err := os.WriteFile(skillPkgPath, []byte("PK-skill-zip"), 0644); err != nil {
		t.Fatalf("write skill file failed: %v", err)
	}

	count, err := kbSvc.EncryptExistingPlainFiles(context.Background())
	if err != nil {
		t.Fatalf("EncryptExistingPlainFiles failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 legacy file encrypted, got %d", count)
	}

	legacyDisk, _ := os.ReadFile(legacyPath)
	if !kbcodec.IsEncryptedAtRest(legacyDisk) {
		t.Fatal("expected legacy KB file to be encrypted at rest")
	}
	legacyPlain, err := kbcodec.DecryptAtRest(legacyDisk, "test-storage-secret")
	if err != nil || string(legacyPlain) != "legacy plain text" {
		t.Fatalf("decrypted legacy mismatch: err=%v", err)
	}

	skillDisk, _ := os.ReadFile(skillPkgPath)
	if string(skillDisk) != "PK-skill-zip" {
		t.Fatal("expected non-KB skills file to remain untouched")
	}
}

func TestListDocumentsLazy(t *testing.T) {
	kbRepo := newMockFileKBRepo()
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			1: {ID: 1, AssetCode: "kb-lazy", Name: "Lazy KB", AssetType: model.AssetTypeKB, DomainID: 5, OwnerID: 10, Visibility: model.VisibilityDomain, Status: model.AssetStatusPublished},
		},
	}
	domainRepo := &mockP1DomainRepo{
		members: map[uint64]map[uint64]string{
			5: {
				10: model.DomainRoleOwner,
				20: model.DomainRoleMember,
			},
		},
	}
	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, t.TempDir(), "test-storage-secret")

	// Seed hierarchy:
	// - guides (folder)
	//   - intro.md (file)
	//   - guides/api (folder)
	//     - auth.md (file)
	// - notes (folder)
	// - readme.md (root file)
	seed := []model.KBDocument{
		{KBAssetID: 1, FolderPath: "guides", IsFolder: true, FileName: "guides", UploaderID: 10},
		{KBAssetID: 1, FolderPath: "guides/api", IsFolder: true, FileName: "api", UploaderID: 10},
		{KBAssetID: 1, FolderPath: "notes", IsFolder: true, FileName: "notes", UploaderID: 10},
		{KBAssetID: 1, FolderPath: "", IsFolder: false, FileName: "readme.md", UploaderID: 10},
		{KBAssetID: 1, FolderPath: "guides", IsFolder: false, FileName: "intro.md", UploaderID: 10},
		{KBAssetID: 1, FolderPath: "guides/api", IsFolder: false, FileName: "auth.md", UploaderID: 10},
	}
	for i := range seed {
		if err := kbRepo.CreateDocument(&seed[i]); err != nil {
			t.Fatalf("seed CreateDocument failed: %v", err)
		}
	}

	// 1. Root "" as domain member (userID 20): returns top-level folders ("guides", "notes") + root file ("readme.md"), NOT nested grandchild files
	rootDocs, err := kbSvc.ListDocumentsLazy(20, false, 1, "", 0)
	if err != nil {
		t.Fatalf("ListDocumentsLazy root failed: %v", err)
	}
	if len(rootDocs) != 3 {
		t.Fatalf("expected 3 root items, got %d: %+v", len(rootDocs), rootDocs)
	}
	if !rootDocs[0].IsFolder || rootDocs[0].FolderPath != "guides" ||
		!rootDocs[1].IsFolder || rootDocs[1].FolderPath != "notes" ||
		rootDocs[2].IsFolder || rootDocs[2].FileName != "readme.md" {
		t.Fatalf("unexpected root lazy items: %+v", rootDocs)
	}

	// 2. Subfolder "guides" (and unnormalized "/guides/"): returns "guides/api" folder + "guides/intro.md", NOT "guides/api/auth.md"
	guideDocs, err := kbSvc.ListDocumentsLazy(20, false, 1, "/guides/", 0)
	if err != nil {
		t.Fatalf("ListDocumentsLazy guides failed: %v", err)
	}
	if len(guideDocs) != 2 {
		t.Fatalf("expected 2 items under guides, got %d: %+v", len(guideDocs), guideDocs)
	}
	if !guideDocs[0].IsFolder || guideDocs[0].FolderPath != "guides/api" ||
		guideDocs[1].IsFolder || guideDocs[1].FileName != "intro.md" {
		t.Fatalf("unexpected guides lazy items: %+v", guideDocs)
	}

	// 3. Invalid traversal path is rejected
	if _, err := kbSvc.ListDocumentsLazy(20, false, 1, "../secret", 0); err == nil {
		t.Fatal("expected error for invalid folder_path traversal")
	}

	// 4. Non-member cannot access domain-visibility KB
	if _, err := kbSvc.ListDocumentsLazy(99, false, 1, "", 0); err == nil {
		t.Fatal("expected forbidden error for non-member on domain KB")
	}
}
