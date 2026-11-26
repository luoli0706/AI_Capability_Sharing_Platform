package handler

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type stubKBSecurityService struct {
	service.KBService
	rawFileName string
	rawMimeType string
	rawIsSVG    bool
	rawData     []byte
}

func (s *stubKBSecurityService) CreateFolder(userID uint64, isSuperAdmin bool, kbID uint64, folderPath string) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) CreateMarkdownFile(userID uint64, isSuperAdmin bool, kbID uint64, req model.CreateKBFileRequest) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) UploadDocuments(userID uint64, isSuperAdmin bool, kbID uint64, folderPath, relativePath string, files []*multipart.FileHeader) ([]model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) ListDocuments(userID uint64, isSuperAdmin bool, kbID, refDomainID uint64) ([]model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) ListDocumentsLazy(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, refDomainID uint64) ([]model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) ListDocumentsLazyPaged(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, offset, limit int, extGroup, fields string, refDomainID uint64) (*model.KBDocumentListResult, error) {
	return nil, nil
}
func (s *stubKBSecurityService) GetFolderETag(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, offset, limit int, extGroup, fields string, refDomainID uint64) (string, error) {
	return "", nil
}
func (s *stubKBSecurityService) GetDocumentMeta(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentListItem, error) {
	return nil, nil
}
func (s *stubKBSecurityService) GetDocumentContent(userID uint64, isSuperAdmin bool, kbID, docID uint64, obf bool, refDomainID uint64) (map[string]interface{}, error) {
	return nil, nil
}
func (s *stubKBSecurityService) GetDocumentRaw(userID uint64, isSuperAdmin bool, kbID, docID uint64, obf bool, refDomainID uint64) (string, string, bool, []byte, error) {
	return s.rawFileName, s.rawMimeType, s.rawIsSVG, s.rawData, nil
}
func (s *stubKBSecurityService) GetBilibiliPreview(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBBilibiliPreview, error) {
	return nil, nil
}
func (s *stubKBSecurityService) ResolveExternalVideoURL(ctx context.Context, rawURL string) (string, error) {
	return "", nil
}
func (s *stubKBSecurityService) UpdateMarkdownContent(userID uint64, isSuperAdmin bool, kbID, docID uint64, req model.UpdateKBFileRequest) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) ListChunks(userID uint64, isSuperAdmin bool, kbID, docID uint64, page, pageSize int, refDomainID uint64) ([]model.KBChunk, int64, error) {
	return nil, 0, nil
}
func (s *stubKBSecurityService) DeleteDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) error {
	return nil
}
func (s *stubKBSecurityService) RestoreDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) Search(userID uint64, isSuperAdmin bool, kbID uint64, req model.KBSearchRequest, refDomainID uint64) ([]model.KBSearchHit, error) {
	return nil, nil
}
func (s *stubKBSecurityService) GetDownloadPreview(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string, refDomainID uint64) (*model.KBDownloadPreview, error) {
	return nil, nil
}
func (s *stubKBSecurityService) DownloadKB(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string, refDomainID uint64) (string, string, []byte, error) {
	return "", "", nil, nil
}
func (s *stubKBSecurityService) MoveDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, targetFolderPath string) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) CopyDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, targetFolderPath, newName string) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) RenameDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, newName string) (*model.KBDocument, error) {
	return nil, nil
}
func (s *stubKBSecurityService) SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository) {
}
func (s *stubKBSecurityService) SetCrossRefRepo(crossRefRepo repository.CrossRefRepository) {}
func (s *stubKBSecurityService) EncryptExistingPlainFiles(ctx context.Context) (int, error) {
	return 0, nil
}

func TestGetDocumentRaw_SecurityHeadersAndDisposition(t *testing.T) {
	cases := []struct {
		name           string
		fileName       string
		mimeType       string
		isSVG          bool
		wantDispPrefix string
	}{
		{"svg forces attachment", "evil.svg", "image/svg+xml", true, "attachment;"},
		{"html forces attachment", "xss.html", "text/html", false, "attachment;"},
		{"htm forces attachment", "xss.htm", "text/html", false, "attachment;"},
		{"xhtml forces attachment", "xss.xhtml", "application/xhtml+xml", false, "attachment;"},
		{"xml forces attachment", "data.xml", "application/xml", false, "attachment;"},
		{"vmd forces attachment", "motion.vmd", "application/octet-stream", false, "attachment;"},
		{"js forces attachment", "evil.js", "application/octet-stream", false, "attachment;"},
		{"exe forces attachment", "bin.exe", "application/octet-stream", false, "attachment;"},
		{"png stays inline", "logo.png", "image/png", false, "inline;"},
		{"pdf stays inline", "doc.pdf", "application/pdf", false, "inline;"},
		{"spoofed png with html mime forces attachment", "fake.png", "text/html", false, "attachment;"},
	}

	e := echo.New()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewKBHandler(&stubKBSecurityService{
				rawFileName: tc.fileName,
				rawMimeType: tc.mimeType,
				rawIsSVG:    tc.isSVG,
				rawData:     []byte("<svg onload=alert(1)>"),
			})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/kb/1/documents/2/raw", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id", "docId")
			c.SetParamValues("1", "2")

			if err := h.GetDocumentRaw(c); err != nil {
				t.Fatalf("GetDocumentRaw failed: %v", err)
			}
			if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := rec.Header().Get("Content-Security-Policy"); got != "sandbox; default-src 'none'" {
				t.Fatalf("Content-Security-Policy = %q, want sandbox; default-src 'none'", got)
			}
			disp := rec.Header().Get(echo.HeaderContentDisposition)
			if !strings.HasPrefix(disp, tc.wantDispPrefix) {
				t.Fatalf("Content-Disposition = %q, want prefix %q", disp, tc.wantDispPrefix)
			}
		})
	}
}

func TestSetContentDisposition_EscapesSpecialChars(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	setContentDisposition(c, "attachment", "bad\";name=\"evil\r\n.txt")
	disp := rec.Header().Get(echo.HeaderContentDisposition)
	if strings.ContainsAny(disp, "\r\n") {
		t.Fatalf("Content-Disposition contains newline: %q", disp)
	}
	if !strings.HasPrefix(disp, "attachment;") {
		t.Fatalf("unexpected Content-Disposition: %q", disp)
	}
}

func TestUploadDocument_MaxFilesLimit(t *testing.T) {
	e := echo.New()
	h := NewKBHandler(&stubKBSecurityService{})

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for i := 0; i < maxKBUploadFiles+1; i++ {
		fw, err := mw.CreateFormFile("files", fmt.Sprintf("f%d.txt", i))
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		_, _ = fw.Write([]byte("ok"))
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/kb/1/documents", &buf)
	req.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	if err := h.UploadDocument(c); err != nil {
		t.Fatalf("UploadDocument returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 BadRequest when exceeding %d files, got %d (%s)", maxKBUploadFiles, rec.Code, rec.Body.String())
	}
}
