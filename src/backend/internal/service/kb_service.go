package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/chunker"
	"ascp-backend/internal/pkg/kbcodec"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

const maxKBFileSize = 10 << 20 // 10MB

var allowedKBExts = map[string]bool{
	".txt":      true,
	".md":       true,
	".markdown": true,
	".bili":     true,
	".video":    true,
}

var textKBExts = map[string]bool{
	".md":       true,
	".markdown": true,
	".txt":      true,
	".bili":     true,
	".video":    true,
	".json":     true,
	".yaml":     true,
	".yml":      true,
	".csv":      true,
	".xml":      true,
	".html":     true,
	".css":      true,
	".js":       true,
	".ts":       true,
	".go":       true,
	".py":       true,
	".sql":      true,
	".sh":       true,
	".log":      true,
	".ini":      true,
	".conf":     true,
}

var kbMIMETypes = map[string]string{
	".png":      "image/png",
	".jpg":      "image/jpeg",
	".jpeg":     "image/jpeg",
	".gif":      "image/gif",
	".webp":     "image/webp",
	".svg":      "image/svg+xml",
	".bmp":      "image/bmp",
	".ico":      "image/x-icon",
	".avif":     "image/avif",
	".md":       "text/markdown; charset=utf-8",
	".markdown": "text/markdown; charset=utf-8",
	".txt":      "text/plain; charset=utf-8",
	".bili":     "text/plain; charset=utf-8",
	".video":    "text/plain; charset=utf-8",
	".json":     "application/json; charset=utf-8",
	".pdf":      "application/pdf",
}

type KBService interface {
	CreateFolder(userID uint64, isSuperAdmin bool, kbID uint64, folderPath string) (*model.KBDocument, error)
	CreateMarkdownFile(userID uint64, isSuperAdmin bool, kbID uint64, req model.CreateKBFileRequest) (*model.KBDocument, error)
	UploadDocuments(userID uint64, isSuperAdmin bool, kbID uint64, folderPath, relativePath string, files []*multipart.FileHeader) ([]model.KBDocument, error)
	ListDocuments(userID uint64, isSuperAdmin bool, kbID, refDomainID uint64) ([]model.KBDocument, error)
	ListDocumentsLazy(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, refDomainID uint64) ([]model.KBDocument, error)
	ListDocumentsLazyPaged(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, offset, limit int, extGroup, fields string, refDomainID uint64) (*model.KBDocumentListResult, error)
	GetFolderETag(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, offset, limit int, extGroup, fields string, refDomainID uint64) (string, error)
	GetDocumentMeta(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentListItem, error)
	GetDocumentContent(userID uint64, isSuperAdmin bool, kbID, docID uint64, obf bool, refDomainID uint64) (map[string]interface{}, error)
	GetDocumentRaw(userID uint64, isSuperAdmin bool, kbID, docID uint64, obf bool, refDomainID uint64) (fileName, mimeType string, isSVG bool, data []byte, err error)
	GetBilibiliPreview(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBBilibiliPreview, error)
	UpdateMarkdownContent(userID uint64, isSuperAdmin bool, kbID, docID uint64, req model.UpdateKBFileRequest) (*model.KBDocument, error)
	ListChunks(userID uint64, isSuperAdmin bool, kbID, docID uint64, page, pageSize int, refDomainID uint64) ([]model.KBChunk, int64, error)
	DeleteDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) error
	RestoreDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) (*model.KBDocument, error)
	Search(userID uint64, isSuperAdmin bool, kbID uint64, req model.KBSearchRequest, refDomainID uint64) ([]model.KBSearchHit, error)
	GetDownloadPreview(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string, refDomainID uint64) (*model.KBDownloadPreview, error)
	DownloadKB(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string, refDomainID uint64) (filename, contentType string, data []byte, err error)
	MoveDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, targetFolderPath string) (*model.KBDocument, error)
	CopyDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, targetFolderPath, newName string) (*model.KBDocument, error)
	RenameDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, newName string) (*model.KBDocument, error)
	EncryptExistingPlainFiles(ctx context.Context) (int, error)
	SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository)
	SetCrossRefRepo(crossRefRepo repository.CrossRefRepository)
	SetInteractionRepo(interactionRepo repository.KBInteractionRepository)
}

type kbService struct {
	kbRepo          repository.KBRepository
	assetRepo       repository.AssetRepository
	domainRepo      repository.DomainRepository
	govRepo         repository.GovernanceRepository
	orgRepo         repository.OrgRepository
	crossRefRepo    repository.CrossRefRepository
	interactionRepo repository.KBInteractionRepository
	auditRepo       repository.AuditLogRepository
	userRepo        repository.UserRepository
	uploadDir       string
	storageSecret   string
	httpClient      *http.Client
}

func (s *kbService) SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository) {
	s.auditRepo = auditRepo
	s.userRepo = userRepo
}

func (s *kbService) SetCrossRefRepo(crossRefRepo repository.CrossRefRepository) {
	s.crossRefRepo = crossRefRepo
}

func (s *kbService) SetInteractionRepo(interactionRepo repository.KBInteractionRepository) {
	s.interactionRepo = interactionRepo
}

func NewKBService(
	kbRepo repository.KBRepository,
	assetRepo repository.AssetRepository,
	domainRepo repository.DomainRepository,
	govRepo repository.GovernanceRepository,
	orgRepo repository.OrgRepository,
	uploadDir string,
	storageSecret string,
) KBService {
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return &kbService{
		kbRepo:        kbRepo,
		assetRepo:     assetRepo,
		domainRepo:    domainRepo,
		govRepo:       govRepo,
		orgRepo:       orgRepo,
		uploadDir:     uploadDir,
		storageSecret: storageSecret,
		httpClient:    &http.Client{Timeout: 8 * time.Second, CheckRedirect: bilibiliCheckRedirect},
	}
}

func (s *kbService) encryptAndWriteFile(filePath string, plain []byte) error {
	enc, err := kbcodec.EncryptAtRest(plain, s.storageSecret)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, enc, 0644)
}

func (s *kbService) readAndDecryptFile(filePath string) ([]byte, error) {
	diskBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return kbcodec.DecryptAtRest(diskBytes, s.storageSecret)
}

// EncryptExistingPlainFiles walks KB asset directories under uploadDir and encrypts
// any unencrypted files in-place at rest.
func (s *kbService) EncryptExistingPlainFiles(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(s.uploadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	encryptedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// KB files are stored under numeric <kbID> directories; skip non-KB dirs like "skills".
		if _, err := strconv.ParseUint(entry.Name(), 10, 64); err != nil {
			continue
		}
		kbDir := filepath.Join(s.uploadDir, entry.Name())
		err := filepath.WalkDir(kbDir, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if kbcodec.IsEncryptedAtRest(raw) {
				return nil
			}
			if err := s.encryptAndWriteFile(p, raw); err != nil {
				return err
			}
			encryptedCount++
			return nil
		})
		if err != nil {
			return encryptedCount, err
		}
	}
	return encryptedCount, nil
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
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return nil, err
	}
	norm, err := NormalizeFolderPath(folderPath)
	if err != nil {
		return nil, err
	}
	if norm == "" {
		return nil, NewBadRequest("folder_path cannot be empty")
	}
	folder, err := s.kbRepo.EnsureFolderHierarchy(kbID, userID, norm)
	if err == nil {
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFolderCreate, norm, "新建目录 "+norm)
	}
	return folder, err
}

func sanitizeMarkdownFileName(raw string) (string, string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return "", "", NewBadRequest("invalid file_name")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		name += ".md"
		ext = ".md"
	}
	if !allowedKBExts[ext] {
		return "", "", NewBadRequest("unsupported file type; allowed: .md, .markdown, .txt")
	}
	return name, ext, nil
}

func kbChunkParams(detail *model.AssetDetail) (int, int) {
	chunkSize, chunkOverlap := 500, 50
	if detail.KBConfig != nil {
		if detail.KBConfig.ChunkSize > 0 {
			chunkSize = detail.KBConfig.ChunkSize
		}
		if detail.KBConfig.ChunkOverlap >= 0 {
			chunkOverlap = detail.KBConfig.ChunkOverlap
		}
	}
	return chunkSize, chunkOverlap
}

func (s *kbService) CreateMarkdownFile(userID uint64, isSuperAdmin bool, kbID uint64, req model.CreateKBFileRequest) (*model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return nil, err
	}
	normFolder, err := NormalizeFolderPath(req.FolderPath)
	if err != nil {
		return nil, err
	}
	fileName, ext, err := sanitizeMarkdownFileName(req.FileName)
	if err != nil {
		return nil, err
	}

	exists, err := s.kbRepo.FileExistsInFolder(kbID, normFolder, fileName, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, NewConflict("a file with the same name already exists in this folder")
	}

	chunkSize, chunkOverlap := kbChunkParams(detail)
	doc, err := s.persistKBFileAndChunks(detail, userID, normFolder, fileName, ext, []byte(req.Content), chunkSize, chunkOverlap)
	if err != nil {
		return nil, err
	}
	recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileCreate, relativeDocPath(*doc), "新建文档 "+relativeDocPath(*doc))
	return doc, nil
}

func (s *kbService) UpdateMarkdownContent(userID uint64, isSuperAdmin bool, kbID, docID uint64, req model.UpdateKBFileRequest) (*model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
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
		return nil, NewBadRequest("cannot edit content of a folder")
	}

	if strings.TrimSpace(req.FileName) != "" {
		newName, newExt, err := sanitizeMarkdownFileName(req.FileName)
		if err != nil {
			return nil, err
		}
		if newName != doc.FileName {
			exists, err := s.kbRepo.FileExistsInFolder(kbID, doc.FolderPath, newName, doc.ID)
			if err != nil {
				return nil, err
			}
			if exists {
				return nil, NewConflict("a file with the same name already exists in this folder")
			}
			doc.FileName = newName
			doc.FileExt = newExt
		}
	}

	contentBytes := []byte(req.Content)
	sum := sha256.Sum256(contentBytes)
	fileHash := hex.EncodeToString(sum[:])

	if doc.FilePath == "" {
		diskDir := filepath.Join(s.uploadDir, fmt.Sprintf("%d", kbID))
		if doc.FolderPath != "" {
			diskDir = filepath.Join(diskDir, filepath.FromSlash(doc.FolderPath))
		}
		if err := os.MkdirAll(diskDir, 0755); err != nil {
			return nil, err
		}
		doc.FilePath = filepath.Join(diskDir, fmt.Sprintf("%s_%s", fileHash[:12], doc.FileName))
	} else {
		if err := os.MkdirAll(filepath.Dir(doc.FilePath), 0755); err != nil {
			return nil, err
		}
	}
	if err := s.encryptAndWriteFile(doc.FilePath, contentBytes); err != nil {
		return nil, err
	}

	doc.FileSize = uint64(len(contentBytes))
	doc.FileHash = fileHash
	doc.ParseStatus = model.ParseStatusCompleted

	chunkSize, chunkOverlap := kbChunkParams(detail)
	pieces := chunker.SplitText(req.Content, chunkSize, chunkOverlap)
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
	doc.ChunkCount = len(chunks)

	if err := s.kbRepo.ReplaceDocumentChunks(kbID, doc, chunks); err != nil {
		return nil, err
	}

	recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileUpdate, relativeDocPath(*doc), "编辑文档 "+relativeDocPath(*doc))
	return doc, nil
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
		if fh.Size > maxKBFileSize {
			return nil, NewBadRequest("单文件大小不能超过 10MB")
		}

		src, err := fh.Open()
		if err != nil {
			return nil, err
		}
		contentBytes, readErr := io.ReadAll(io.LimitReader(src, maxKBFileSize+1))
		_ = src.Close()
		if readErr != nil {
			return nil, readErr
		}
		if len(contentBytes) > maxKBFileSize {
			return nil, NewBadRequest("单文件大小不能超过 10MB")
		}

		doc, err := s.persistKBFileAndChunks(detail, userID, docFolder, safeName, ext, contentBytes, chunkSize, chunkOverlap)
		if err != nil {
			return nil, err
		}
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileUpload, relativeDocPath(*doc), "上传文档 "+relativeDocPath(*doc))
		created = append(created, *doc)
	}

	return created, nil
}

func (s *kbService) persistKBFileAndChunks(
	detail *model.AssetDetail,
	userID uint64,
	docFolder, safeName, ext string,
	contentBytes []byte,
	chunkSize, chunkOverlap int,
) (*model.KBDocument, error) {
	if docFolder != "" {
		if _, err := s.kbRepo.EnsureFolderHierarchy(detail.ID, userID, docFolder); err != nil {
			return nil, err
		}
	}

	sum := sha256.Sum256(contentBytes)
	fileHash := hex.EncodeToString(sum[:])

	diskDir := filepath.Join(s.uploadDir, fmt.Sprintf("%d", detail.ID))
	if docFolder != "" {
		diskDir = filepath.Join(diskDir, filepath.FromSlash(docFolder))
	}
	if err := os.MkdirAll(diskDir, 0755); err != nil {
		return nil, err
	}
	dstPath := filepath.Join(diskDir, fmt.Sprintf("%s_%s", fileHash[:12], safeName))
	if err := s.encryptAndWriteFile(dstPath, contentBytes); err != nil {
		return nil, err
	}

	doc := &model.KBDocument{
		KBAssetID:   detail.ID,
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

	var chunks []model.KBChunk
	if isTextKBFile(ext, contentBytes) {
		pieces := chunker.SplitText(string(contentBytes), chunkSize, chunkOverlap)
		chunks = make([]model.KBChunk, 0, len(pieces))
		for _, p := range pieces {
			chunks = append(chunks, model.KBChunk{
				KBAssetID:  detail.ID,
				DocID:      doc.ID,
				ChunkIndex: p.Index,
				Content:    p.Content,
				TokenCount: p.TokenCount,
			})
		}
	}
	if err := s.kbRepo.CompleteDocumentParsing(detail.ID, doc.ID, chunks); err != nil {
		_ = s.kbRepo.FailDocumentParsing(doc.ID, err.Error())
	} else {
		doc.ChunkCount = len(chunks)
	}
	return doc, nil
}

func isTextKBFile(ext string, data []byte) bool {
	if !textKBExts[strings.ToLower(ext)] {
		return false
	}
	return !bytes.Contains(data, []byte{0})
}

func detectKBMIMEType(ext string, data []byte) string {
	lowerExt := strings.ToLower(ext)
	if mt, ok := kbMIMETypes[lowerExt]; ok {
		return mt
	}
	if mt := mime.TypeByExtension(lowerExt); mt != "" {
		return mt
	}
	if len(data) > 0 {
		return http.DetectContentType(data)
	}
	return "application/octet-stream"
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

func (s *kbService) ListDocuments(userID uint64, isSuperAdmin bool, kbID, refDomainID uint64) ([]model.KBDocument, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, 0); err != nil {
		return nil, err
	}
	docs, err := s.kbRepo.ListDocuments(kbID)
	if err != nil {
		return nil, err
	}
	s.enrichDocumentUploaders(docs)
	return docs, nil
}

// ListDocumentsLazy returns only direct child folders and files under parentFolder.
func (s *kbService) ListDocumentsLazy(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, refDomainID uint64) ([]model.KBDocument, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, 0); err != nil {
		return nil, err
	}
	normParent, err := NormalizeFolderPath(parentFolder)
	if err != nil {
		return nil, err
	}
	docs, err := s.kbRepo.ListDocumentsByParent(kbID, normParent)
	if err != nil {
		return nil, err
	}
	s.enrichDocumentUploaders(docs)
	return docs, nil
}

func normalizeLazyPageParams(offset, limit int, extGroup, fields string) (int, int, string, string) {
	if offset < 0 {
		offset = 0
	}
	normExtGroup := strings.ToLower(strings.TrimSpace(extGroup))
	normFields := strings.ToLower(strings.TrimSpace(fields))
	if normFields == "minimal" && normExtGroup != "" {
		if limit <= 0 || limit > 1000 {
			limit = 1000
		}
	} else {
		if limit <= 0 {
			limit = 50
		} else if limit > 200 {
			limit = 200
		}
	}
	return offset, limit, normExtGroup, normFields
}

func toKBDocumentListItem(d model.KBDocument) model.KBDocumentListItem {
	return model.KBDocumentListItem{
		ID:          d.ID,
		KBAssetID:   d.KBAssetID,
		FolderPath:  d.FolderPath,
		IsFolder:    d.IsFolder,
		FileName:    d.FileName,
		FileExt:     d.FileExt,
		FileSize:    d.FileSize,
		ParseStatus: d.ParseStatus,
		UploaderID:  d.UploaderID,
		UpdatedAt:   d.UpdatedAt,
	}
}

func (s *kbService) buildUserSummaryMap(docs []model.KBDocument) map[uint64]model.KBUserSummary {
	users := make(map[uint64]model.KBUserSummary)
	if s.userRepo == nil || len(docs) == 0 {
		return users
	}
	seen := make(map[uint64]struct{})
	ids := make([]uint64, 0, len(docs))
	for _, d := range docs {
		if d.UploaderID == 0 {
			continue
		}
		if _, ok := seen[d.UploaderID]; !ok {
			seen[d.UploaderID] = struct{}{}
			ids = append(ids, d.UploaderID)
		}
	}
	if len(ids) == 0 {
		return users
	}
	briefs, err := s.userRepo.FindBriefsByIDs(ids)
	if err != nil {
		return users
	}
	for id, brief := range briefs {
		users[id] = model.KBUserSummary{
			ID:       brief.ID,
			Username: brief.Username,
			Nickname: brief.Nickname,
		}
	}
	return users
}

// ListDocumentsLazyPaged returns paginated slim document projections with deduplicated user summaries.
func (s *kbService) ListDocumentsLazyPaged(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, offset, limit int, extGroup, fields string, refDomainID uint64) (*model.KBDocumentListResult, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, 0); err != nil {
		return nil, err
	}
	normParent, err := NormalizeFolderPath(parentFolder)
	if err != nil {
		return nil, err
	}
	normOffset, normLimit, normExtGroup, normFields := normalizeLazyPageParams(offset, limit, extGroup, fields)
	docs, total, err := s.kbRepo.ListDocumentsByParentPaged(kbID, normParent, normOffset, normLimit, normExtGroup)
	if err != nil {
		return nil, err
	}

	nextOffset := normOffset + len(docs)
	hasMore := int64(nextOffset) < total

	if normFields == "minimal" {
		items := make([]model.KBDocumentMinimalItem, 0, len(docs))
		for _, d := range docs {
			items = append(items, model.KBDocumentMinimalItem{
				ID:         d.ID,
				KBAssetID:  d.KBAssetID,
				FolderPath: d.FolderPath,
				IsFolder:   d.IsFolder,
				FileName:   d.FileName,
				FileExt:    d.FileExt,
			})
		}
		return &model.KBDocumentListResult{
			List:       items,
			Users:      make(map[uint64]model.KBUserSummary),
			Total:      total,
			HasMore:    hasMore,
			NextOffset: nextOffset,
		}, nil
	}

	items := make([]model.KBDocumentListItem, 0, len(docs))
	for _, d := range docs {
		items = append(items, toKBDocumentListItem(d))
	}
	return &model.KBDocumentListResult{
		List:       items,
		Users:      s.buildUserSummaryMap(docs),
		Total:      total,
		HasMore:    hasMore,
		NextOffset: nextOffset,
	}, nil
}

// GetFolderETag computes a strong ETag for a paginated/filtered folder listing.
func (s *kbService) GetFolderETag(userID uint64, isSuperAdmin bool, kbID uint64, parentFolder string, offset, limit int, extGroup, fields string, refDomainID uint64) (string, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, 0); err != nil {
		return "", err
	}
	normParent, err := NormalizeFolderPath(parentFolder)
	if err != nil {
		return "", err
	}
	normOffset, normLimit, normExtGroup, normFields := normalizeLazyPageParams(offset, limit, extGroup, fields)
	total, maxUpdated, err := s.kbRepo.GetFolderSummary(kbID, normParent, normExtGroup)
	if err != nil {
		return "", err
	}
	raw := fmt.Sprintf("kb=%d|parent=%s|ext=%s|fields=%s|offset=%d|limit=%d|total=%d|max=%d",
		kbID, normParent, normExtGroup, normFields, normOffset, normLimit, total, maxUpdated.UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf(`"%x"`, sum[:16]), nil
}

// GetDocumentMeta returns slim metadata for a single document node.
func (s *kbService) GetDocumentMeta(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBDocumentListItem, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, docID); err != nil {
		return nil, err
	}
	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("document not found")
		}
		return nil, err
	}
	item := toKBDocumentListItem(*doc)
	return &item, nil
}

func (s *kbService) enrichDocumentUploaders(docs []model.KBDocument) {
	if s.userRepo == nil || len(docs) == 0 {
		return
	}
	seen := make(map[uint64]struct{})
	ids := make([]uint64, 0, len(docs))
	for _, d := range docs {
		if d.UploaderID == 0 {
			continue
		}
		if _, ok := seen[d.UploaderID]; !ok {
			seen[d.UploaderID] = struct{}{}
			ids = append(ids, d.UploaderID)
		}
	}
	if len(ids) == 0 {
		return
	}
	briefs, err := s.userRepo.FindBriefsByIDs(ids)
	if err != nil {
		return
	}
	for i := range docs {
		if brief, ok := briefs[docs[i].UploaderID]; ok {
			b := brief
			docs[i].Uploader = &b
		}
	}
}

func (s *kbService) GetDocumentContent(userID uint64, isSuperAdmin bool, kbID, docID uint64, obf bool, refDomainID uint64) (map[string]interface{}, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, docID); err != nil {
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

	raw, err := s.readAndDecryptFile(doc.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewNotFound("document file not found on disk")
		}
		return nil, err
	}
	if !isTextKBFile(doc.FileExt, raw) {
		return map[string]interface{}{
			"id":          doc.ID,
			"file_name":   doc.FileName,
			"folder_path": doc.FolderPath,
			"file_size":   doc.FileSize,
			"is_binary":   true,
			"content":     "",
		}, nil
	}
	if obf {
		obfBytes, err := kbcodec.ObfuscateTransport(raw)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"id":          doc.ID,
			"file_name":   doc.FileName,
			"folder_path": doc.FolderPath,
			"file_size":   doc.FileSize,
			"is_binary":   false,
			"obfuscated":  true,
			"payload":     base64.StdEncoding.EncodeToString(obfBytes),
			"content":     "",
		}, nil
	}
	return map[string]interface{}{
		"id":          doc.ID,
		"file_name":   doc.FileName,
		"folder_path": doc.FolderPath,
		"file_size":   doc.FileSize,
		"is_binary":   false,
		"content":     string(raw),
	}, nil
}

func (s *kbService) GetDocumentRaw(userID uint64, isSuperAdmin bool, kbID, docID uint64, obf bool, refDomainID uint64) (string, string, bool, []byte, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, docID); err != nil {
		return "", "", false, nil, err
	}
	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", false, nil, NewNotFound("document not found")
		}
		return "", "", false, nil, err
	}
	if doc.IsFolder {
		return "", "", false, nil, NewBadRequest("cannot stream raw content of a folder")
	}

	raw, err := s.readAndDecryptFile(doc.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", false, nil, NewNotFound("document file not found on disk")
		}
		return "", "", false, nil, err
	}

	ext := doc.FileExt
	if ext == "" {
		ext = filepath.Ext(doc.FileName)
	}
	mimeType := detectKBMIMEType(ext, raw)
	isSVG := strings.EqualFold(ext, ".svg")
	if obf {
		obfData, err := kbcodec.ObfuscateTransport(raw)
		if err != nil {
			return "", "", false, nil, err
		}
		return doc.FileName, "application/octet-stream", isSVG, obfData, nil
	}
	return doc.FileName, mimeType, isSVG, raw, nil
}

var (
	biliExactBVIDRe   = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)
	biliSearchBVIDRe  = regexp.MustCompile(`BV[0-9A-Za-z]{10}`)
	biliHTMLTitleRe   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	biliMetaTagRe     = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	biliMetaPropRe    = regexp.MustCompile(`(?is)(?:property|name)\s*=\s*["']([^"']+)["']`)
	biliMetaContentRe = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']*)["']`)
)

func (s *kbService) GetBilibiliPreview(userID uint64, isSuperAdmin bool, kbID, docID, refDomainID uint64) (*model.KBBilibiliPreview, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, docID); err != nil {
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
		return nil, NewBadRequest("cannot preview bilibili link of a folder")
	}

	ext := strings.ToLower(doc.FileExt)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(doc.FileName))
	}
	baseName := strings.TrimSuffix(doc.FileName, filepath.Ext(doc.FileName))
	bvid := ""
	if biliExactBVIDRe.MatchString(baseName) {
		bvid = baseName
	} else if ext == ".bili" && doc.FilePath != "" {
		if raw, readErr := s.readAndDecryptFile(doc.FilePath); readErr == nil {
			bvid = biliSearchBVIDRe.FindString(string(raw))
		}
	}
	if bvid == "" {
		return nil, NewBadRequest("no valid Bilibili BV ID found")
	}

	watchURL := "https://www.bilibili.com/video/" + bvid
	baseClient := s.httpClient
	if baseClient == nil {
		baseClient = &http.Client{Timeout: 8 * time.Second}
	}
	clientCopy := *baseClient
	if clientCopy.CheckRedirect == nil {
		clientCopy.CheckRedirect = bilibiliCheckRedirect
	}
	client := &clientCopy
	req, err := http.NewRequest(http.MethodGet, watchURL, nil)
	if err != nil {
		return &model.KBBilibiliPreview{
			Available: false,
			BVID:      bvid,
			WatchURL:  watchURL,
			Message:   err.Error(),
		}, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com")

	resp, err := client.Do(req)
	if err != nil {
		return &model.KBBilibiliPreview{
			Available: false,
			BVID:      bvid,
			WatchURL:  watchURL,
			Message:   err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	bodyStr := string(bodyBytes)

	embedURL := "https://player.bilibili.com/player.html?bvid=" + bvid + "&page=1&as_wide=1&high_quality=1&danmaku=0"

	if resp.StatusCode == http.StatusPreconditionFailed || resp.StatusCode == http.StatusForbidden {
		playerReq, pErr := http.NewRequest(http.MethodGet, embedURL, nil)
		if pErr == nil {
			playerReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
			playerReq.Header.Set("Referer", "https://www.bilibili.com")
			if pResp, pDoErr := client.Do(playerReq); pDoErr == nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(pResp.Body, 64*1024))
				_ = pResp.Body.Close()
				if pResp.StatusCode == http.StatusOK {
					return &model.KBBilibiliPreview{
						Available:  true,
						StatusCode: http.StatusOK,
						BVID:       bvid,
						Title:      bvid,
						EmbedURL:   embedURL,
						WatchURL:   watchURL,
					}, nil
				}
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return &model.KBBilibiliPreview{
			Available:  false,
			StatusCode: resp.StatusCode,
			BVID:       bvid,
			WatchURL:   watchURL,
			Message:    fmt.Sprintf("bilibili returned status %d", resp.StatusCode),
		}, nil
	}

	if strings.Contains(bodyStr, "啊叻？视频不见了") || strings.Contains(bodyStr, "视频去哪了呢") {
		return &model.KBBilibiliPreview{
			Available:  false,
			StatusCode: http.StatusNotFound,
			BVID:       bvid,
			WatchURL:   watchURL,
			Message:    "video not found or deleted",
		}, nil
	}

	title, cover := parseBilibiliHTMLMeta(bodyStr)
	return &model.KBBilibiliPreview{
		Available:  true,
		StatusCode: http.StatusOK,
		BVID:       bvid,
		Title:      title,
		Cover:      cover,
		EmbedURL:   embedURL,
		WatchURL:   watchURL,
	}, nil
}

func isAllowedBilibiliHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return false
	}
	return h == "bilibili.com" || strings.HasSuffix(h, ".bilibili.com") ||
		h == "b23.tv" || strings.HasSuffix(h, ".b23.tv")
}

func bilibiliCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("stopped after 5 redirects")
	}
	if req.URL == nil || (req.URL.Scheme != "https" && req.URL.Scheme != "http") {
		return errors.New("redirect to non-http scheme blocked")
	}
	if !isAllowedBilibiliHost(req.URL.Hostname()) {
		return fmt.Errorf("redirect to disallowed host %q blocked", req.URL.Hostname())
	}
	return nil
}

func parseBilibiliHTMLMeta(htmlStr string) (title, cover string) {
	for _, tag := range biliMetaTagRe.FindAllString(htmlStr, -1) {
		propMatch := biliMetaPropRe.FindStringSubmatch(tag)
		contentMatch := biliMetaContentRe.FindStringSubmatch(tag)
		if len(propMatch) < 2 || len(contentMatch) < 2 {
			continue
		}
		prop := strings.ToLower(strings.TrimSpace(propMatch[1]))
		content := strings.TrimSpace(html.UnescapeString(contentMatch[1]))
		switch prop {
		case "og:title":
			if title == "" {
				title = content
			}
		case "og:image":
			if cover == "" {
				candidate := content
				if strings.HasPrefix(candidate, "//") {
					candidate = "https:" + candidate
				}
				lower := strings.ToLower(candidate)
				if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
					cover = candidate
				}
			}
		}
	}
	if title == "" {
		if m := biliHTMLTitleRe.FindStringSubmatch(htmlStr); len(m) >= 2 {
			title = strings.TrimSpace(html.UnescapeString(m[1]))
		}
	}
	title = strings.TrimSuffix(title, "_哔哩哔哩_bilibili")
	title = strings.TrimSpace(title)
	return title, cover
}

func (s *kbService) ListChunks(userID uint64, isSuperAdmin bool, kbID, docID uint64, page, pageSize int, refDomainID uint64) ([]model.KBChunk, int64, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, docID); err != nil {
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
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
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
		var folderDocIDs []uint64
		if s.crossRefRepo != nil {
			if allDocs, listErr := s.kbRepo.ListDocuments(kbID); listErr == nil {
				for _, d := range allDocs {
					if d.FolderPath == doc.FolderPath || strings.HasPrefix(d.FolderPath, doc.FolderPath+"/") {
						folderDocIDs = append(folderDocIDs, d.ID)
					}
				}
			}
		}
		paths, err := s.kbRepo.DeleteFolderRecursive(kbID, doc.FolderPath)
		if err != nil {
			return err
		}
		_ = paths
		if s.crossRefRepo != nil && len(folderDocIDs) > 0 {
			_ = s.crossRefRepo.DeleteByKBDocIDs(folderDocIDs)
		}
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileDelete, doc.FolderPath, "删除目录 "+doc.FolderPath)
		return nil
	}

	if err := s.kbRepo.DeleteDocument(kbID, docID); err != nil {
		return err
	}
	if s.crossRefRepo != nil {
		_ = s.crossRefRepo.DeleteByKBDocIDs([]uint64{docID})
	}
	recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileDelete, relativeDocPath(*doc), "删除文档 "+relativeDocPath(*doc))
	return nil
}

func (s *kbService) RestoreDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64) (*model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return nil, err
	}
	doc, err := s.kbRepo.FindDeletedDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("deleted document not found")
		}
		return nil, err
	}

	if doc.IsFolder {
		folderPath := strings.Trim(doc.FolderPath, "/")
		if folderPath == "" {
			folderPath = doc.FileName
		}
		exists, err := s.kbRepo.FolderExists(kbID, folderPath, doc.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, NewConflict("a folder with the same name already exists")
		}
		if idx := strings.LastIndex(folderPath, "/"); idx >= 0 {
			parentPath := folderPath[:idx]
			if parentPath != "" {
				if _, err := s.kbRepo.EnsureFolderHierarchy(kbID, userID, parentPath); err != nil {
					return nil, err
				}
			}
		}
		if err := s.kbRepo.RestoreFolderRecursive(kbID, doc.ID, folderPath, doc.DeletedAt.Time); err != nil {
			return nil, err
		}
		doc.DeletedAt = gorm.DeletedAt{}
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileRestore, folderPath, "恢复目录 "+folderPath)
		return doc, nil
	}

	exists, err := s.kbRepo.FileExistsInFolder(kbID, doc.FolderPath, doc.FileName, doc.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, NewConflict("a file with the same name already exists in this folder")
	}
	if doc.FolderPath != "" {
		if _, err := s.kbRepo.EnsureFolderHierarchy(kbID, userID, doc.FolderPath); err != nil {
			return nil, err
		}
	}
	if err := s.kbRepo.RestoreDocument(kbID, doc.ID); err != nil {
		return nil, err
	}
	doc.DeletedAt = gorm.DeletedAt{}
	recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileRestore, relativeDocPath(*doc), "恢复文档 "+relativeDocPath(*doc))
	return doc, nil
}

func (s *kbService) MoveDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, targetFolderPath string) (*model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return nil, err
	}
	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("document not found")
		}
		return nil, err
	}
	targetFolder, err := NormalizeFolderPath(targetFolderPath)
	if err != nil {
		return nil, err
	}

	if doc.IsFolder {
		oldPath := strings.Trim(doc.FolderPath, "/")
		if oldPath == "" {
			oldPath = doc.FileName
		}
		if targetFolder == oldPath || strings.HasPrefix(targetFolder, oldPath+"/") {
			return nil, NewBadRequest("cannot move folder into itself or its subfolder")
		}
		newPath := doc.FileName
		if targetFolder != "" {
			newPath = targetFolder + "/" + doc.FileName
		}
		if newPath == oldPath {
			return doc, nil
		}
		if targetFolder != "" {
			if _, err := s.kbRepo.EnsureFolderHierarchy(kbID, userID, targetFolder); err != nil {
				return nil, err
			}
		}
		exists, err := s.kbRepo.FolderExists(kbID, newPath, doc.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, NewConflict("a folder with the same name already exists in the target folder")
		}
		if err := s.kbRepo.RenameFolder(kbID, doc.ID, oldPath, doc.FileName, newPath); err != nil {
			return nil, err
		}
		doc.FolderPath = newPath
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileMove, newPath, fmt.Sprintf("移动目录 %s -> %s", oldPath, newPath))
		return doc, nil
	}

	if doc.FolderPath != targetFolder {
		if targetFolder != "" {
			if _, err := s.kbRepo.EnsureFolderHierarchy(kbID, userID, targetFolder); err != nil {
				return nil, err
			}
		}
		exists, err := s.kbRepo.FileExistsInFolder(kbID, targetFolder, doc.FileName, doc.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, NewConflict("a file with the same name already exists in the target folder")
		}
		if err := s.kbRepo.MoveDocument(kbID, docID, targetFolder); err != nil {
			return nil, err
		}
		oldPath := relativeDocPath(*doc)
		doc.FolderPath = targetFolder
		newPath := relativeDocPath(*doc)
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileMove, newPath, fmt.Sprintf("移动 %s -> %s", oldPath, newPath))
	}

	return doc, nil
}

func validateRenameTarget(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || name == "." || name == ".." {
		return "", NewBadRequest("invalid new_name")
	}
	if strings.ContainsAny(name, "/\\") {
		return "", NewBadRequest("new_name cannot contain path separators")
	}
	if len(name) > 255 {
		return "", NewBadRequest("new_name exceeds 255 characters")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", NewBadRequest("new_name cannot contain control characters")
		}
	}
	return name, nil
}

func (s *kbService) RenameDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, newName string) (*model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
		return nil, err
	}
	trimmed, err := validateRenameTarget(newName)
	if err != nil {
		return nil, err
	}
	doc, err := s.kbRepo.FindDocumentByID(kbID, docID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("document not found")
		}
		return nil, err
	}

	if !doc.IsFolder {
		if trimmed == doc.FileName {
			return doc, nil
		}
		exists, err := s.kbRepo.FileExistsInFolder(kbID, doc.FolderPath, trimmed, doc.ID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, NewConflict("a file with the same name already exists in this folder")
		}
		newExt := strings.ToLower(filepath.Ext(trimmed))
		if err := s.kbRepo.RenameFile(kbID, doc.ID, trimmed, newExt); err != nil {
			return nil, err
		}
		oldRelPath := relativeDocPath(*doc)
		doc.FileName = trimmed
		doc.FileExt = newExt
		newRelPath := relativeDocPath(*doc)
		recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileRename, newRelPath, fmt.Sprintf("重命名 %s -> %s", oldRelPath, newRelPath))
		return doc, nil
	}

	oldPath := strings.Trim(doc.FolderPath, "/")
	if oldPath == "" {
		oldPath = doc.FileName
	}
	parentPath := ""
	if idx := strings.LastIndex(oldPath, "/"); idx >= 0 {
		parentPath = oldPath[:idx]
	}
	newPath := trimmed
	if parentPath != "" {
		newPath = parentPath + "/" + trimmed
	}
	if newPath == oldPath {
		return doc, nil
	}
	exists, err := s.kbRepo.FolderExists(kbID, newPath, doc.ID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, NewConflict("a folder with the same name already exists")
	}
	if err := s.kbRepo.RenameFolder(kbID, doc.ID, oldPath, trimmed, newPath); err != nil {
		return nil, err
	}
	doc.FileName = trimmed
	doc.FolderPath = newPath
	recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileRename, newPath, fmt.Sprintf("重命名目录 %s -> %s", oldPath, newPath))
	return doc, nil
}

func (s *kbService) CopyDocument(userID uint64, isSuperAdmin bool, kbID, docID uint64, targetFolderPath, newName string) (*model.KBDocument, error) {
	detail, err := s.ensureKBEditPermission(userID, isSuperAdmin, kbID)
	if err != nil {
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
		return nil, NewBadRequest("only files can be copied")
	}

	targetFolder, err := NormalizeFolderPath(targetFolderPath)
	if err != nil {
		return nil, err
	}

	contentBytes, err := s.readAndDecryptFile(doc.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewNotFound("source file not found on disk")
		}
		return nil, err
	}

	candidateName := doc.FileName
	if strings.TrimSpace(newName) != "" {
		candidateName = path.Base(strings.ReplaceAll(strings.TrimSpace(newName), "\\", "/"))
	}
	finalName, err := s.deduplicateFileName(kbID, targetFolder, candidateName)
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

	copied, err := s.persistKBFileAndChunks(detail, userID, targetFolder, finalName, doc.FileExt, contentBytes, chunkSize, chunkOverlap)
	if err != nil {
		return nil, err
	}
	recordAudit(s.auditRepo, s.userRepo, userID, detail.DomainID, detail.DomainName, detail.ID, detail.AssetCode, detail.Name, detail.AssetType, model.AuditActionKBFileCopy, relativeDocPath(*copied), fmt.Sprintf("复制 %s -> %s", relativeDocPath(*doc), relativeDocPath(*copied)))
	return copied, nil
}

func (s *kbService) deduplicateFileName(kbID uint64, targetFolder, fileName string) (string, error) {
	exists, err := s.kbRepo.FileExistsInFolder(kbID, targetFolder, fileName, 0)
	if err != nil {
		return "", err
	}
	if !exists {
		return fileName, nil
	}

	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	for i := 1; i <= 100; i++ {
		var candidate string
		if i == 1 {
			candidate = base + "-copy" + ext
		} else {
			candidate = fmt.Sprintf("%s-copy-%d%s", base, i, ext)
		}
		exists, err := s.kbRepo.FileExistsInFolder(kbID, targetFolder, candidate, 0)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return fmt.Sprintf("%s-copy-%d%s", base, os.Getpid(), ext), nil
}

func (s *kbService) Search(userID uint64, isSuperAdmin bool, kbID uint64, req model.KBSearchRequest, refDomainID uint64) ([]model.KBSearchHit, error) {
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, 0); err != nil {
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

func (s *kbService) GetDownloadPreview(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string, refDomainID uint64) (*model.KBDownloadPreview, error) {
	permDocID := uint64(0)
	if scope == "file" && docID > 0 {
		permDocID = docID
	}
	if _, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, permDocID); err != nil {
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

func (s *kbService) DownloadKB(userID uint64, isSuperAdmin bool, kbID uint64, scope string, docID uint64, folderPath string, refDomainID uint64) (string, string, []byte, error) {
	permDocID := uint64(0)
	if scope == "file" && docID > 0 {
		permDocID = docID
	}
	detail, err := s.ensureKBViewPermission(userID, isSuperAdmin, kbID, refDomainID, permDocID)
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
		raw, err := s.readAndDecryptFile(d.FilePath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", "", nil, NewNotFound("file not found on disk")
			}
			return "", "", nil, err
		}
		return d.FileName, "text/markdown; charset=utf-8", raw, nil
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, d := range docs {
		raw, err := s.readAndDecryptFile(d.FilePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			_ = zw.Close()
			return "", "", nil, err
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

func (s *kbService) ensureKBViewPermission(userID uint64, isSuperAdmin bool, kbID, refDomainID, docID uint64) (*model.AssetDetail, error) {
	return ensureKBViewAccessWithCrossRef(
		s.assetRepo, s.domainRepo, s.orgRepo, s.govRepo, s.crossRefRepo,
		userID, isSuperAdmin, kbID, refDomainID, docID,
	)
}

func ensureKBViewAccessWithCrossRef(
	assetRepo repository.AssetRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	govRepo repository.GovernanceRepository,
	crossRefRepo repository.CrossRefRepository,
	userID uint64,
	isSuperAdmin bool,
	kbID, refDomainID, docID uint64,
) (*model.AssetDetail, error) {
	detail, err := assetRepo.GetDetailByID(kbID)
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
	role, err := domainRepo.GetMemberRole(detail.DomainID, userID)
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
			if ok, _ := hasDomainAccess(domainRepo, orgRepo, detail.DomainID, userID); ok {
				return detail, nil
			}
		}
		if govRepo != nil {
			if perm, _ := govRepo.GetActiveUserPermission(kbID, userID); perm != nil {
				return detail, nil
			}
		}
	}
	if refDomainID > 0 && crossRefRepo != nil {
		hasTargetAccess, err := hasDomainAccess(domainRepo, orgRepo, refDomainID, userID)
		if err != nil {
			return nil, err
		}
		if hasTargetAccess {
			var hasRef bool
			if docID == 0 {
				hasRef, err = crossRefRepo.HasActiveAssetRef(refDomainID, kbID)
			} else {
				hasRef, err = crossRefRepo.HasActiveDocOrAssetRef(refDomainID, kbID, docID)
			}
			if err != nil {
				return nil, err
			}
			if hasRef {
				return detail, nil
			}
		}
	}
	return nil, NewForbidden("no permission to access this knowledge base")
}
