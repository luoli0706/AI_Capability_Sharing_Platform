package service

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type mockFileKBRepo struct {
	nextID uint64
	docs   map[uint64]*model.KBDocument
}

func newMockFileKBRepo() *mockFileKBRepo {
	return &mockFileKBRepo{
		nextID: 1,
		docs:   make(map[uint64]*model.KBDocument),
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
func (m *mockFileKBRepo) CompleteDocumentParsing(kbAssetID, docID uint64, chunks []model.KBChunk) error {
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
	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, tmpDir)

	// 1. Recursive folder preview ("guides" includes "guides/intro.md" + "guides/api/auth.md")
	prevFolder, err := kbSvc.GetDownloadPreview(10, false, 1, "folder", 0, "/guides/")
	if err != nil {
		t.Fatalf("GetDownloadPreview folder failed: %v", err)
	}
	if prevFolder.FileCount != 2 || prevFolder.TotalBytes != 29 {
		t.Fatalf("expected 2 files / 29 bytes, got %+v", prevFolder)
	}

	// 2. All files preview
	prevAll, err := kbSvc.GetDownloadPreview(10, false, 1, "all", 0, "")
	if err != nil {
		t.Fatalf("GetDownloadPreview all failed: %v", err)
	}
	if prevAll.FileCount != 3 || prevAll.TotalBytes != 42 {
		t.Fatalf("expected 3 files / 42 bytes, got %+v", prevAll)
	}

	// 3. Single file preview
	prevFile, err := kbSvc.GetDownloadPreview(10, false, 1, "file", 2, "")
	if err != nil {
		t.Fatalf("GetDownloadPreview file failed: %v", err)
	}
	if prevFile.FileCount != 1 || prevFile.TotalBytes != 16 || prevFile.Target != "guides/api/auth.md" {
		t.Fatalf("unexpected single file preview: %+v", prevFile)
	}

	// 4. Download folder as zip and inspect entries
	fn, ct, zipBytes, err := kbSvc.DownloadKB(10, false, 1, "folder", 0, "guides")
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
