package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strings"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/chunker"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

var allowedKBExts = map[string]bool{
	".txt":      true,
	".md":       true,
	".markdown": true,
}

type KBService interface {
	CreateFolder(userID uint64, isSuperAdmin bool, kbID uint64, folderPath string) (*model.KBDocument, error)
	UploadDocuments(userID uint64, isSuperAdmin bool, kbID uint64, folderPath, relativePath string, files []*multipart.FileHeader) ([]model.KBDocument, error)
	ListDocuments(userID uint64, isSuperAdmin bool, kbID uint64) ([]model.KBDocument, error)
	GetDocumentContent(userID uint64, isSuperAdmin bool, kbID, docID uint64) (map[string]interface{}, error)
	ListChunks(userID uint64, isSuperAdmin bool, kbID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error)
	DeleteDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) error
	Search(userID uint64, isSuperAdmin bool, kbID uint64, req model.KBSearchRequest) ([]model.KBSearchHit, error)
	GetDownloadPreview(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string) (*model.KBDownloadPreview, error)
	DownloadKB(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string) (filename, contentType string, data []byte, err error)
}

type kbService struct {
	kbRepo     repository.KBRepository
	assetRepo  repository.AssetRepository
	domainRepo repository.DomainRepository
	govRepo    repository.GovernanceRepository
	orgRepo    repository.OrgRepository
	uploadDir  string
}

func NewKBService(
	kbRepo repository.KBRepository,
	assetRepo repository.AssetRepository,
	domainRepo repository.DomainRepository,
	govRepo repository.GovernanceRepository,
	orgRepo repository.OrgRepository,
	uploadDir string,
) KBService {
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return &kbService{
		kbRepo:     kbRepo,
		assetRepo:  assetRepo,
		domainRepo: domainRepo,
		govRepo:    govRepo,
		orgRepo:    orgRepo,
		uploadDir:  uploadDir,
	}
}

// NormalizeFolderPath converts backslashes to '/', trims leading/trailing '/',
// collapses repeated slashes, and rejects '.' or '..' traversal segments.
func NormalizeFolderPath(raw string) (string, error) {
	replaced := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	trimmed := strings.Trim(replaced, "/")
	if trimmed == "" {
		return "", nil
	}

	parts := strings.Split(trimmed, "/")
	cleanParts := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "." || p == ".." {
			return "", NewBadRequest("invalid folder_path: '.' and '..' segments are not allowed")
		}
		cleanParts = append(cleanParts, p)
	}
	return strings.Join(cleanParts, "/"), nil
}

func (s *kbService) CreateFolder(userID uint64, isSuperAdmin bool, kbID uint64, folderPath string) (*model.KBDocument, error) {
	if _, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID); err != nil {
		return nil, err
	}
	norm, err := NormalizeFolderPath(folderPath)
	if err != nil {
		return nil, err
	}
	if norm == "" {
		return nil, NewBadRequest("folder_path cannot be empty")
	}
	return s.kbRepo.EnsureFolderHierarchy(kbID, userID, norm)
}

func (s *kbService) UploadDocuments(
	userID uint64,
	isSuperAdmin bool,
	kbID uint64,
	folderPath, relativePath string,
	files []*multipart.FileHeader,
) ([]model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, NewBadRequest("at least one file is required")
	}

	baseFolder, err := NormalizeFolderPath(folderPath)
	if err != nil {
		return nil, err
	}

	chunkSize := 500
	chunkOverlap := 50
	if detail.KBConfig != nil {
		if detail.KBConfig.ChunkSize > 0 {
			chunkSize = detail.KBConfig.ChunkSize
		}
		if detail.KBConfig.ChunkOverlap >= 0 {
			chunkOverlap = detail.KBConfig.ChunkOverlap
		}
	}

	var created []model.KBDocument
	for idx, fh := range files {
		rel := ""
		if idx == 0 && relativePath != "" {
			rel = relativePath
		}
		docFolder, safeName, err := resolveUploadTarget(baseFolder, rel, fh.Filename)
		if err != nil {
			return nil, err
		}

		ext := strings.ToLower(filepath.Ext(safeName))
		if !allowedKBExts[ext] {
			return nil, NewBadRequest("unsupported file type; allowed: .md, .markdown, .txt")
		}

		src, err := fh.Open()
		if err != nil {
			return nil, err
		}
		contentBytes, readErr := io.ReadAll(io.LimitReader(src, 20<<20))
		_ = src.Close()
		if readErr != nil {
			return nil, readErr
		}

		if docFolder != "" {
			if _, err := s.kbRepo.EnsureFolderHierarchy(kbID, userID, docFolder); err != nil {
				return nil, err
			}
		}

		sum := sha256.Sum256(contentBytes)
		fileHash := hex.EncodeToString(sum[:])

		diskDir := filepath.Join(s.uploadDir, fmt.Sprintf("%d", kbID))
		if docFolder != "" {
			diskDir = filepath.Join(diskDir, filepath.FromSlash(docFolder))
		}
		if err := os.MkdirAll(diskDir, 0755); err != nil {
			return nil, err
		}
		dstPath := filepath.Join(diskDir, fmt.Sprintf("%s_%s", fileHash[:12], safeName))
		if err := os.WriteFile(dstPath, contentBytes, 0644); err != nil {
			return nil, err
		}

		doc := &model.KBDocument{
			KBAssetID:   kbID,
			FolderPath:  docFolder,
			IsFolder:    false,
			FileName:    safeName,
			FileExt:     ext,
			FileSize:    uint64(len(contentBytes)),
			FilePath:    dstPath,
			FileHash:    fileHash,
			ParseStatus: model.ParseStatusCompleted,
			UploaderID:  userID,
		}
		if err := s.kbRepo.CreateDocument(doc); err != nil {
			return nil, err
		}

		pieces := chunker.SplitText(string(contentBytes), chunkSize, chunkOverlap)
		chunks := make([]model.KBChunk, 0, len(pieces))
		for _, p := range pieces {
			chunks = append(chunks, model.KBChunk{
				KBAssetID:  kbID,
				DocID:      doc.ID,
				ChunkIndex: p.Index,
				Content:    p.Content,
				TokenCount: p.TokenCount,
			})
		}
		if err := s.kbRepo.CompleteDocumentParsing(kbID, doc.ID, chunks); err != nil {
			_ = s.kbRepo.FailDocumentParsing(doc.ID, err.Error())
		} else {
			doc.ChunkCount = len(chunks)
		}

		created = append(created, *doc)
	}

	return created, nil
}

func resolveUploadTarget(baseFolder, relativePath, origFilename string) (folder string, fileName string, err error) {
	if relativePath != "" {
		normRel, err := NormalizeFolderPath(relativePath)
		if err != nil {
			return "", "", err
		}
		if normRel != "" {
			dir := path.Dir(normRel)
			base := path.Base(normRel)
			if dir == "." {
				dir = ""
			}
			if baseFolder != "" && dir != "" {
				return baseFolder + "/" + dir, base, nil
			}
			if dir != "" {
				return dir, base, nil
			}
			return baseFolder, base, nil
		}
	}

	replaced := strings.ReplaceAll(origFilename, "\\", "/")
	return baseFolder, path.Base(replaced), nil
}

func (s *kbService) ListDocuments(userID uint64, isSuperAdmin bool, kbID uint64) ([]model.KBDocument, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID); err != nil {
		return nil, err
	}
	return s.kbRepo.ListDocuments(kbID)
}

func (s *kbService) GetDocumentContent(userID uint64, isSuperAdmin bool, kbID, docID uint64) (map[string]interface{}, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID); err != nil {
		return nil, err
	}
	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("document not found")
		}
		return nil, err
	}
	if doc.IsFolder {
		return nil, NewBadRequest("cannot preview content of a folder")
	}

	raw, err := os.ReadFile(doc.FilePath)
	if err != nil {
		return nil, NewNotFound("document file not found on disk")
	}
	return map[string]interface{}{
		"id":          doc.ID,
		"file_name":   doc.FileName,
		"folder_path": doc.FolderPath,
		"file_size":   doc.FileSize,
		"content":     string(raw),
	}, nil
}

func (s *kbService) ListChunks(userID uint64, isSuperAdmin bool, kbID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID); err != nil {
		return nil, 0, err
	}
	if _, err := s.kbRepo.FindDocumentByID(kbID, docID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, NewNotFound("document not found")
		}
		return nil, 0, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.kbRepo.ListChunks(kbID, docID, page, pageSize)
}

func (s *kbService) DeleteDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) error {
	if _, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID); err != nil {
		return err
	}
	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("document not found")
		}
		return err
	}

	if doc.IsFolder {
		paths, err := s.kbRepo.DeleteFolderRecursive(kbID, doc.FolderPath)
		if err != nil {
			return err
		}
		for _, p := range paths {
			_ = os.Remove(p)
		}
		return nil
	}

	if err := s.kbRepo.DeleteDocument(kbID, docID); err != nil {
		return err
	}
	if doc.FilePath != "" {
		_ = os.Remove(doc.FilePath)
	}
	return nil
}

func (s *kbService) Search(userID uint64, isSuperAdmin bool, kbID uint64, req model.KBSearchRequest) ([]model.KBSearchHit, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID); err != nil {
		return nil, err
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 50 {
		topK = 50
	}
	return s.kbRepo.SearchChunks(kbID, req.Query, topK)
}

func (s *kbService) GetDownloadPreview(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string) (*model.KBDownloadPreview, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID); err != nil {
		return nil, err
	}

	scope, normFolder, docs, err := s.collectDownloadFiles(kbID, scope, docID, folderPath)
	if err != nil {
		return nil, err
	}

	target := normFolder
	if scope == "file" && len(docs) > 0 {
		target = relativeDocPath(docs[0])
	} else if scope == "all" {
		target = ""
	}

	previewFiles := make([]model.KBPreviewFile, 0, len(docs))
	var totalBytes int64
	for _, d := range docs {
		previewFiles = append(previewFiles, model.KBPreviewFile{
			Path: relativeDocPath(d),
			Size: int64(d.FileSize),
		})
		totalBytes += int64(d.FileSize)
	}

	return &model.KBDownloadPreview{
		Scope:      scope,
		Target:     target,
		FileCount:  len(previewFiles),
		TotalBytes: totalBytes,
		Files:      previewFiles,
	}, nil
}

func (s *kbService) DownloadKB(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string) (string, string, []byte, error) {
	detail, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return "", "", nil, err
	}

	scope, _, docs, err := s.collectDownloadFiles(kbID, scope, docID, folderPath)
	if err != nil {
		return "", "", nil, err
	}
	if len(docs) == 0 {
		return "", "", nil, NewNotFound("no files matched for download")
	}

	if scope == "file" {
		d := docs[0]
		raw, err := os.ReadFile(d.FilePath)
		if err != nil {
			return "", "", nil, NewNotFound("file not found on disk")
		}
		return d.FileName, "text/markdown; charset=utf-8", raw, nil
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, d := range docs {
		raw, err := os.ReadFile(d.FilePath)
		if err != nil {
			continue
		}
		w, err := zw.Create(relativeDocPath(d))
		if err != nil {
			_ = zw.Close()
			return "", "", nil, err
		}
		if _, err := w.Write(raw); err != nil {
			_ = zw.Close()
			return "", "", nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return "", "", nil, err
	}

	return detail.AssetCode + ".zip", "application/zip", buf.Bytes(), nil
}

func (s *kbService) collectDownloadFiles(kbID uint64, scope string, docID uint64, folderPath string) (string, string, []model.KBDocument, error) {
	if scope == "" {
		scope = "all"
	}
	switch scope {
	case "file":
		if docID == 0 {
			return "", "", nil, NewBadRequest("doc_id is required when scope=file")
		}
		docs, err := s.kbRepo.ListMatchingFiles(kbID, "file", docID, "")
		if err != nil {
			return "", "", nil, err
		}
		if len(docs) == 0 {
			return "", "", nil, NewNotFound("document not found")
		}
		return scope, "", docs, nil
	case "folder":
		norm, err := NormalizeFolderPath(folderPath)
		if err != nil {
			return "", "", nil, err
		}
		docs, err := s.kbRepo.ListMatchingFiles(kbID, "folder", 0, norm)
		return scope, norm, docs, err
	case "all":
		docs, err := s.kbRepo.ListMatchingFiles(kbID, "all", 0, "")
		return scope, "", docs, err
	default:
		return "", "", nil, NewBadRequest("invalid scope; allowed: file, folder, all")
	}
}

func relativeDocPath(d model.KBDocument) string {
	if d.FolderPath != "" {
		return d.FolderPath + "/" + d.FileName
	}
	return d.FileName
}

func (s *kbService) ensureKBEditPermission(userID uint64, isSuperAdmin bool, kbID uint64) (*model.AssetDetail, error) {
	detail, err := s.assetRepo.GetDetailByID(kbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("knowledge base not found")
		}
		return nil, err
	}
	if detail.AssetType != model.AssetTypeKB {
		return nil, NewBadRequest("asset is not a knowledge base")
	}
	if isSuperAdmin || detail.OwnerID == userID {
		return detail, nil
	}
	role, err := s.domainRepo.GetMemberRole(detail.DomainID, userID)
	if err != nil {
		return nil, err
	}
	if role != model.DomainRoleOwner && role != model.DomainRoleAdmin {
		return nil, NewForbidden("only owner or domain admin can manage knowledge base documents")
	}
	return detail, nil
}

func (s *kbService) ensureKBViewPermission(userID uint64, isSuperAdmin bool, kbID uint64) (*model.AssetDetail, error) {
	detail, err := s.assetRepo.GetDetailByID(kbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("knowledge base not found")
		}
		return nil, err
	}
	if detail.AssetType != model.AssetTypeKB {
		return nil, NewBadRequest("asset is not a knowledge base")
	}
	if isSuperAdmin || detail.OwnerID == userID {
		return detail, nil
	}
	role, err := s.domainRepo.GetMemberRole(detail.DomainID, userID)
	if err != nil {
		return nil, err
	}
	if role == model.DomainRoleOwner || role == model.DomainRoleAdmin {
		return detail, nil
	}
	if detail.Status == model.AssetStatusPublished {
		if detail.Visibility == model.VisibilityPublic {
			return detail, nil
		}
		if detail.Visibility == model.VisibilityDomain {
			if ok, _ := hasDomainAccess(s.domainRepo, s.orgRepo, detail.DomainID, userID); ok {
				return detail, nil
			}
		}
		if s.govRepo != nil {
			if perm, _ := s.govRepo.GetActiveUserPermission(kbID, userID); perm != nil {
				return detail, nil
			}
		}
	}
	return nil, NewForbidden("no permission to access this knowledge base")
}
