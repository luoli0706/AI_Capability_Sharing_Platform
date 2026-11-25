package repository

import (
	"path"
	"sort"
	"strings"
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type KBRepository interface {
	CreateDocument(doc *model.KBDocument) error
	EnsureFolderHierarchy(kbAssetID, uploaderID uint64, folderPath string) (*model.KBDocument, error)
	ListDocuments(kbAssetID uint64) ([]model.KBDocument, error)
	ListDocumentsByParent(kbAssetID uint64, parentFolder string) ([]model.KBDocument, error)
	ListDocumentsByParentPaged(kbAssetID uint64, parentFolder string, offset, limit int, extGroup string) ([]model.KBDocument, int64, error)
	GetFolderSummary(kbAssetID uint64, parentFolder string, extGroup string) (int64, time.Time, error)
	FindDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error)
	FindDeletedDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error)
	ListMatchingFiles(kbAssetID uint64, scope string, docID uint64, folderPath string) ([]model.KBDocument, error)
	FileExistsInFolder(kbAssetID uint64, folderPath, fileName string, excludeDocID uint64) (bool, error)
	FolderExists(kbAssetID uint64, folderPath string, excludeDocID uint64) (bool, error)
	RenameFile(kbAssetID, docID uint64, newName, newExt string) error
	RenameFolder(kbAssetID, folderDocID uint64, oldPath, newName, newPath string) error
	MoveDocument(kbAssetID, docID uint64, targetFolder string) error
	ReplaceDocumentChunks(kbAssetID uint64, doc *model.KBDocument, chunks []model.KBChunk) error
	CompleteDocumentParsing(kbAssetID, docID uint64, chunks []model.KBChunk) error
	FailDocumentParsing(docID uint64, errMsg string) error
	RecalculateKBCounts(kbAssetID uint64) error
	ListChunks(kbAssetID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error)
	DeleteDocument(kbAssetID, docID uint64) error
	DeleteFolderRecursive(kbAssetID uint64, folderPath string) ([]string, error)
	RestoreDocument(kbAssetID, docID uint64) error
	RestoreFolderRecursive(kbAssetID, folderDocID uint64, folderPath string, deletedAt time.Time) error
	SearchChunks(kbAssetID uint64, query string, topK int) ([]model.KBSearchHit, error)
	FindDocumentByFilePath(filePath string) (*model.KBDocument, error)
	UpdateStorageSyncState(docID uint64, tier, ossKey string, syncedAt, lastAccessedAt *time.Time) error
	TouchLastAccessed(docID uint64, accessedAt time.Time) error
	ListUnsyncedLocalDocs(limit int) ([]model.KBDocument, error)
	ListEvictableSyncedDocs(cutoff time.Time, limit int) ([]model.KBDocument, error)
	ListLRUSyncedDocs(limit int) ([]model.KBDocument, error)
	TryMarkOSSOnly(docID uint64, cutoff *time.Time) (bool, error)
}

type kbRepository struct {
	db *gorm.DB
}

func NewKBRepository(db *gorm.DB) KBRepository {
	return &kbRepository{db: db}
}

func (r *kbRepository) CreateDocument(doc *model.KBDocument) error {
	return r.db.Create(doc).Error
}

func (r *kbRepository) EnsureFolderHierarchy(kbAssetID, uploaderID uint64, folderPath string) (*model.KBDocument, error) {
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
		var existing model.KBDocument
		err := r.db.Where("kb_asset_id = ? AND is_folder = ? AND folder_path = ?", kbAssetID, true, cur).First(&existing).Error
		if err == nil {
			lastFolder = &existing
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		folder := model.KBDocument{
			KBAssetID:   kbAssetID,
			FolderPath:  cur,
			IsFolder:    true,
			FileName:    path.Base(cur),
			FileExt:     "",
			FileSize:    0,
			FilePath:    "",
			ParseStatus: model.ParseStatusCompleted,
			UploaderID:  uploaderID,
		}
		if err := r.db.Create(&folder).Error; err != nil {
			return nil, err
		}
		lastFolder = &folder
	}
	return lastFolder, nil
}

func (r *kbRepository) ListDocuments(kbAssetID uint64) ([]model.KBDocument, error) {
	var docs []model.KBDocument
	if err := r.db.Where("kb_asset_id = ?", kbAssetID).
		Order("is_folder DESC, folder_path ASC, file_name ASC").
		Find(&docs).Error; err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, nil
}

func escapeLikePattern(s string) string {
	r := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")
	return r.Replace(s)
}

func (r *kbRepository) buildParentFolderQuery(kbAssetID uint64, parentFolder, extGroup string) *gorm.DB {
	tx := r.db.Model(&model.KBDocument{}).Where("kb_asset_id = ?", kbAssetID)
	if parentFolder == "" {
		tx = tx.Where("((is_folder = ? AND folder_path <> '' AND folder_path NOT LIKE ?) OR (is_folder = ? AND folder_path = ''))",
			true, "%/%", false)
	} else {
		escaped := escapeLikePattern(parentFolder)
		tx = tx.Where("((is_folder = ? AND folder_path LIKE ? ESCAPE '!' AND folder_path NOT LIKE ? ESCAPE '!') OR (is_folder = ? AND folder_path = ?))",
			true, escaped+"/%", escaped+"/%/%", false, parentFolder)
	}
	switch strings.ToLower(strings.TrimSpace(extGroup)) {
	case "image":
		tx = tx.Where("is_folder = ? AND LOWER(file_ext) IN ?", false, []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico", ".avif"})
	case "txt":
		tx = tx.Where("is_folder = ? AND LOWER(file_ext) = ?", false, ".txt")
	}
	return tx
}

// ListDocumentsByParent returns direct child folders and direct child files under parentFolder.
func (r *kbRepository) ListDocumentsByParent(kbAssetID uint64, parentFolder string) ([]model.KBDocument, error) {
	var docs []model.KBDocument
	if err := r.buildParentFolderQuery(kbAssetID, parentFolder, "").
		Order("is_folder DESC, file_name ASC").
		Find(&docs).Error; err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, nil
}

// ListDocumentsByParentPaged returns a paginated, slim-column projection of direct children under parentFolder.
func (r *kbRepository) ListDocumentsByParentPaged(kbAssetID uint64, parentFolder string, offset, limit int, extGroup string) ([]model.KBDocument, int64, error) {
	base := r.buildParentFolderQuery(kbAssetID, parentFolder, extGroup)
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var docs []model.KBDocument
	q := base.Session(&gorm.Session{}).Select("id, kb_asset_id, folder_path, is_folder, file_name, file_ext, file_size, parse_status, uploader_id, updated_at").
		Order("is_folder DESC, file_name ASC")
	if offset > 0 {
		q = q.Offset(offset)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&docs).Error; err != nil {
		return nil, 0, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, total, nil
}

// GetFolderSummary returns matching row count and latest updated_at timestamp for ETag calculation.
func (r *kbRepository) GetFolderSummary(kbAssetID uint64, parentFolder string, extGroup string) (int64, time.Time, error) {
	var row struct {
		Total      int64      `gorm:"column:total"`
		MaxUpdated *time.Time `gorm:"column:max_updated"`
	}
	if err := r.buildParentFolderQuery(kbAssetID, parentFolder, extGroup).
		Select("COUNT(*) AS total, MAX(updated_at) AS max_updated").
		Scan(&row).Error; err != nil {
		return 0, time.Time{}, err
	}
	if row.MaxUpdated != nil {
		return row.Total, *row.MaxUpdated, nil
	}
	return row.Total, time.Time{}, nil
}

func (r *kbRepository) FindDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error) {
	var doc model.KBDocument
	if err := r.db.Where("id = ? AND kb_asset_id = ?", docID, kbAssetID).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *kbRepository) ListMatchingFiles(kbAssetID uint64, scope string, docID uint64, folderPath string) ([]model.KBDocument, error) {
	tx := r.db.Where("kb_asset_id = ? AND is_folder = ?", kbAssetID, false)
	switch scope {
	case "file":
		tx = tx.Where("id = ?", docID)
	case "folder":
		if folderPath != "" {
			escaped := escapeLikePattern(folderPath)
			tx = tx.Where("(folder_path = ? OR folder_path LIKE ? ESCAPE '!')", folderPath, escaped+"/%")
		}
	}
	var docs []model.KBDocument
	if err := tx.Order("folder_path ASC, file_name ASC").Find(&docs).Error; err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, nil
}

func (r *kbRepository) FileExistsInFolder(kbAssetID uint64, folderPath, fileName string, excludeDocID uint64) (bool, error) {
	tx := r.db.Model(&model.KBDocument{}).
		Where("kb_asset_id = ? AND is_folder = ? AND folder_path = ? AND file_name = ?", kbAssetID, false, folderPath, fileName)
	if excludeDocID > 0 {
		tx = tx.Where("id <> ?", excludeDocID)
	}
	var count int64
	if err := tx.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *kbRepository) FolderExists(kbAssetID uint64, folderPath string, excludeDocID uint64) (bool, error) {
	tx := r.db.Model(&model.KBDocument{}).
		Where("kb_asset_id = ? AND is_folder = ? AND folder_path = ?", kbAssetID, true, folderPath)
	if excludeDocID > 0 {
		tx = tx.Where("id <> ?", excludeDocID)
	}
	var count int64
	if err := tx.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *kbRepository) RenameFile(kbAssetID, docID uint64, newName, newExt string) error {
	return r.db.Model(&model.KBDocument{}).
		Where("id = ? AND kb_asset_id = ? AND is_folder = ?", docID, kbAssetID, false).
		Updates(map[string]interface{}{
			"file_name": newName,
			"file_ext":  newExt,
		}).Error
}

func (r *kbRepository) RenameFolder(kbAssetID, folderDocID uint64, oldPath, newName, newPath string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.KBDocument{}).
			Where("id = ? AND kb_asset_id = ? AND is_folder = ?", folderDocID, kbAssetID, true).
			Updates(map[string]interface{}{
				"file_name":   newName,
				"folder_path": newPath,
			}).Error; err != nil {
			return err
		}
		var descendants []model.KBDocument
		escapedOldPath := escapeLikePattern(oldPath)
		if err := tx.Where("kb_asset_id = ? AND id <> ? AND (folder_path = ? OR folder_path LIKE ? ESCAPE '!')",
			kbAssetID, folderDocID, oldPath, escapedOldPath+"/%").Find(&descendants).Error; err != nil {
			return err
		}
		oldPrefix := oldPath + "/"
		newPrefix := newPath + "/"
		for _, d := range descendants {
			var updatedPath string
			switch {
			case d.FolderPath == oldPath:
				updatedPath = newPath
			case strings.HasPrefix(d.FolderPath, oldPrefix):
				updatedPath = newPrefix + strings.TrimPrefix(d.FolderPath, oldPrefix)
			default:
				continue
			}
			if err := tx.Model(&model.KBDocument{}).
				Where("id = ? AND kb_asset_id = ?", d.ID, kbAssetID).
				Update("folder_path", updatedPath).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *kbRepository) MoveDocument(kbAssetID, docID uint64, targetFolder string) error {
	return r.db.Model(&model.KBDocument{}).
		Where("id = ? AND kb_asset_id = ? AND is_folder = ?", docID, kbAssetID, false).
		Update("folder_path", targetFolder).Error
}

func (r *kbRepository) ReplaceDocumentChunks(kbAssetID uint64, doc *model.KBDocument, chunks []model.KBChunk) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.KBDocument{}).Where("id = ? AND kb_asset_id = ?", doc.ID, kbAssetID).Updates(map[string]interface{}{
			"file_name":    doc.FileName,
			"file_ext":     doc.FileExt,
			"file_size":    doc.FileSize,
			"file_path":    doc.FilePath,
			"file_hash":    doc.FileHash,
			"parse_status": model.ParseStatusCompleted,
			"chunk_count":  len(chunks),
			"error_msg":    "",
		}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("kb_asset_id = ? AND doc_id = ?", kbAssetID, doc.ID).Delete(&model.KBChunk{}).Error; err != nil {
			return err
		}
		if len(chunks) > 0 {
			if err := tx.CreateInBatches(&chunks, 100).Error; err != nil {
				return err
			}
		}
		return recalculateKBCounts(tx, kbAssetID)
	})
}

func (r *kbRepository) CompleteDocumentParsing(kbAssetID, docID uint64, chunks []model.KBChunk) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if len(chunks) > 0 {
			if err := tx.CreateInBatches(&chunks, 100).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.KBDocument{}).Where("id = ?", docID).Updates(map[string]interface{}{
			"parse_status": model.ParseStatusCompleted,
			"chunk_count":  len(chunks),
			"error_msg":    "",
		}).Error; err != nil {
			return err
		}
		return recalculateKBCounts(tx, kbAssetID)
	})
}

func (r *kbRepository) FailDocumentParsing(docID uint64, errMsg string) error {
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	return r.db.Model(&model.KBDocument{}).Where("id = ?", docID).Updates(map[string]interface{}{
		"parse_status": model.ParseStatusFailed,
		"error_msg":    errMsg,
	}).Error
}

func (r *kbRepository) RecalculateKBCounts(kbAssetID uint64) error {
	return recalculateKBCounts(r.db, kbAssetID)
}

func (r *kbRepository) ListChunks(kbAssetID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error) {
	tx := r.db.Model(&model.KBChunk{}).Where("kb_asset_id = ? AND doc_id = ?", kbAssetID, docID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var chunks []model.KBChunk
	offset := (page - 1) * pageSize
	if err := tx.Order("chunk_index ASC").Offset(offset).Limit(pageSize).Find(&chunks).Error; err != nil {
		return nil, 0, err
	}
	if chunks == nil {
		chunks = []model.KBChunk{}
	}
	return chunks, total, nil
}

func (r *kbRepository) DeleteDocument(kbAssetID, docID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("kb_asset_id = ? AND doc_id = ?", kbAssetID, docID).Delete(&model.KBChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ? AND kb_asset_id = ?", docID, kbAssetID).Delete(&model.KBDocument{}).Error; err != nil {
			return err
		}
		return recalculateKBCounts(tx, kbAssetID)
	})
}

func (r *kbRepository) DeleteFolderRecursive(kbAssetID uint64, folderPath string) ([]string, error) {
	var filePaths []string
	prefix := folderPath + "/"
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var allDocs []model.KBDocument
		if err := tx.Where("kb_asset_id = ?", kbAssetID).Find(&allDocs).Error; err != nil {
			return err
		}
		docIDs := make([]uint64, 0)
		for _, d := range allDocs {
			if d.FolderPath == folderPath || strings.HasPrefix(d.FolderPath, prefix) {
				docIDs = append(docIDs, d.ID)
				if !d.IsFolder && d.FilePath != "" {
					filePaths = append(filePaths, d.FilePath)
				}
			}
		}
		if len(docIDs) == 0 {
			return nil
		}
		if err := tx.Where("kb_asset_id = ? AND doc_id IN ?", kbAssetID, docIDs).Delete(&model.KBChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_asset_id = ? AND id IN ?", kbAssetID, docIDs).Delete(&model.KBDocument{}).Error; err != nil {
			return err
		}
		return recalculateKBCounts(tx, kbAssetID)
	})
	return filePaths, err
}

func (r *kbRepository) FindDeletedDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error) {
	var doc model.KBDocument
	if err := r.db.Unscoped().Where("id = ? AND kb_asset_id = ? AND deleted_at IS NOT NULL", docID, kbAssetID).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *kbRepository) RestoreDocument(kbAssetID, docID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Model(&model.KBDocument{}).
			Where("id = ? AND kb_asset_id = ? AND deleted_at IS NOT NULL", docID, kbAssetID).
			Update("deleted_at", nil).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Model(&model.KBChunk{}).
			Where("kb_asset_id = ? AND doc_id = ? AND deleted_at IS NOT NULL", kbAssetID, docID).
			Update("deleted_at", nil).Error; err != nil {
			return err
		}
		return recalculateKBCounts(tx, kbAssetID)
	})
}

func (r *kbRepository) RestoreFolderRecursive(kbAssetID, folderDocID uint64, folderPath string, deletedAt time.Time) error {
	prefix := folderPath + "/"
	winStart := deletedAt.Add(-2 * time.Second)
	winEnd := deletedAt.Add(2 * time.Second)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var candidates []model.KBDocument
		if err := tx.Unscoped().
			Where("kb_asset_id = ? AND deleted_at IS NOT NULL", kbAssetID).
			Find(&candidates).Error; err != nil {
			return err
		}
		restoredDocIDs := make([]uint64, 0)
		for _, d := range candidates {
			if d.ID == folderDocID {
				restoredDocIDs = append(restoredDocIDs, d.ID)
				continue
			}
			if (d.FolderPath == folderPath || strings.HasPrefix(d.FolderPath, prefix)) &&
				!d.DeletedAt.Time.Before(winStart) && !d.DeletedAt.Time.After(winEnd) {
				restoredDocIDs = append(restoredDocIDs, d.ID)
			}
		}
		if len(restoredDocIDs) > 0 {
			if err := tx.Unscoped().Model(&model.KBDocument{}).
				Where("kb_asset_id = ? AND id IN ?", kbAssetID, restoredDocIDs).
				Update("deleted_at", nil).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Model(&model.KBChunk{}).
				Where("kb_asset_id = ? AND doc_id IN ? AND deleted_at IS NOT NULL", kbAssetID, restoredDocIDs).
				Update("deleted_at", nil).Error; err != nil {
				return err
			}
		}
		return recalculateKBCounts(tx, kbAssetID)
	})
}

func recalculateKBCounts(tx *gorm.DB, kbAssetID uint64) error {
	var docCount int64
	if err := tx.Model(&model.KBDocument{}).Where("kb_asset_id = ? AND is_folder = ?", kbAssetID, false).Count(&docCount).Error; err != nil {
		return err
	}
	var totalChunks int64
	if err := tx.Model(&model.KBChunk{}).Where("kb_asset_id = ?", kbAssetID).Count(&totalChunks).Error; err != nil {
		return err
	}
	return tx.Model(&model.KBKnowledgeBase{}).Where("asset_id = ?", kbAssetID).Updates(map[string]interface{}{
		"doc_count":    int(docCount),
		"total_chunks": int(totalChunks),
	}).Error
}

func (r *kbRepository) SearchChunks(kbAssetID uint64, query string, topK int) ([]model.KBSearchHit, error) {
	if topK <= 0 {
		topK = 5
	}

	type chunkRow struct {
		ID         uint64
		DocID      uint64
		FileName   string
		ChunkIndex int
		Content    string
	}
	var rows []chunkRow
	err := r.db.Table("kb_chunk").
		Select("kb_chunk.id, kb_chunk.doc_id, COALESCE(kb_document.file_name, '') AS file_name, kb_chunk.chunk_index, kb_chunk.content").
		Joins("LEFT JOIN kb_document ON kb_document.id = kb_chunk.doc_id AND kb_document.deleted_at IS NULL").
		Where("kb_chunk.kb_asset_id = ? AND kb_chunk.deleted_at IS NULL", kbAssetID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	qLower := strings.ToLower(strings.TrimSpace(query))
	terms := strings.Fields(qLower)
	if len(terms) == 0 {
		return []model.KBSearchHit{}, nil
	}

	var hits []model.KBSearchHit
	for _, row := range rows {
		cLower := strings.ToLower(row.Content)
		score := 0.0
		if strings.Contains(cLower, qLower) {
			score += 2.0
		}
		for _, term := range terms {
			if cnt := strings.Count(cLower, term); cnt > 0 {
				score += float64(cnt)
			}
		}
		if score > 0 {
			hits = append(hits, model.KBSearchHit{
				ChunkID:    row.ID,
				DocID:      row.DocID,
				FileName:   row.FileName,
				ChunkIndex: row.ChunkIndex,
				Content:    row.Content,
				Score:      score,
			})
		}
	}

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ChunkID < hits[j].ChunkID
		}
		return hits[i].Score > hits[j].Score
	})

	if len(hits) > topK {
		hits = hits[:topK]
	}
	if hits == nil {
		hits = []model.KBSearchHit{}
	}
	return hits, nil
}

func (r *kbRepository) FindDocumentByFilePath(filePath string) (*model.KBDocument, error) {
	var doc model.KBDocument
	if err := r.db.Where("is_folder = ? AND file_path = ?", false, filePath).First(&doc).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (r *kbRepository) UpdateStorageSyncState(docID uint64, tier, ossKey string, syncedAt, lastAccessedAt *time.Time) error {
	updates := map[string]interface{}{
		"storage_tier": tier,
		"oss_key":      ossKey,
	}
	if syncedAt != nil {
		updates["oss_synced_at"] = syncedAt
	}
	if lastAccessedAt != nil {
		updates["last_accessed_at"] = lastAccessedAt
	}
	return r.db.Model(&model.KBDocument{}).Where("id = ?", docID).Updates(updates).Error
}

func (r *kbRepository) TouchLastAccessed(docID uint64, accessedAt time.Time) error {
	return r.db.Model(&model.KBDocument{}).Where("id = ?", docID).Update("last_accessed_at", accessedAt).Error
}

func (r *kbRepository) ListUnsyncedLocalDocs(limit int) ([]model.KBDocument, error) {
	tx := r.db.Where("is_folder = ? AND file_path <> '' AND (storage_tier = ? OR storage_tier = '')", false, model.StorageTierLocal).
		Order("id ASC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	var docs []model.KBDocument
	if err := tx.Find(&docs).Error; err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, nil
}

func (r *kbRepository) ListEvictableSyncedDocs(cutoff time.Time, limit int) ([]model.KBDocument, error) {
	tx := r.db.Where(
		"is_folder = ? AND file_path <> '' AND storage_tier = ? AND ((last_accessed_at IS NOT NULL AND last_accessed_at < ?) OR (last_accessed_at IS NULL AND updated_at < ?))",
		false, model.StorageTierSynced, cutoff, cutoff,
	).Order("COALESCE(last_accessed_at, updated_at) ASC, id ASC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	var docs []model.KBDocument
	if err := tx.Find(&docs).Error; err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, nil
}

func (r *kbRepository) ListLRUSyncedDocs(limit int) ([]model.KBDocument, error) {
	tx := r.db.Where("is_folder = ? AND file_path <> '' AND storage_tier = ?", false, model.StorageTierSynced).
		Order("COALESCE(last_accessed_at, updated_at) ASC, id ASC")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	var docs []model.KBDocument
	if err := tx.Find(&docs).Error; err != nil {
		return nil, err
	}
	if docs == nil {
		docs = []model.KBDocument{}
	}
	return docs, nil
}

func (r *kbRepository) TryMarkOSSOnly(docID uint64, cutoff *time.Time) (bool, error) {
	tx := r.db.Model(&model.KBDocument{}).
		Where("id = ? AND storage_tier = ?", docID, model.StorageTierSynced)
	if cutoff != nil {
		tx = tx.Where("((last_accessed_at IS NOT NULL AND last_accessed_at < ?) OR (last_accessed_at IS NULL AND updated_at < ?))", *cutoff, *cutoff)
	}
	res := tx.Update("storage_tier", model.StorageTierOSSOnly)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
