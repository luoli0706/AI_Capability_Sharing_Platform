package repository

import (
	"path"
	"sort"
	"strings"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type KBRepository interface {
	CreateDocument(doc *model.KBDocument) error
	EnsureFolderHierarchy(kbAssetID, uploaderID uint64, folderPath string) (*model.KBDocument, error)
	ListDocuments(kbAssetID uint64) ([]model.KBDocument, error)
	FindDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error)
	ListMatchingFiles(kbAssetID uint64, scope string, docID uint64, folderPath string) ([]model.KBDocument, error)
	CompleteDocumentParsing(kbAssetID, docID uint64, chunks []model.KBChunk) error
	FailDocumentParsing(docID uint64, errMsg string) error
	ListChunks(kbAssetID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error)
	DeleteDocument(kbAssetID, docID uint64) error
	DeleteFolderRecursive(kbAssetID uint64, folderPath string) ([]string, error)
	SearchChunks(kbAssetID uint64, query string, topK int) ([]model.KBSearchHit, error)
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
			tx = tx.Where("(folder_path = ? OR folder_path LIKE ?)", folderPath, folderPath+"/%")
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
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var docs []model.KBDocument
		if err := tx.Where("kb_asset_id = ? AND (folder_path = ? OR folder_path LIKE ?)",
			kbAssetID, folderPath, folderPath+"/%").Find(&docs).Error; err != nil {
			return err
		}
		if len(docs) == 0 {
			return nil
		}
		docIDs := make([]uint64, 0, len(docs))
		for _, d := range docs {
			docIDs = append(docIDs, d.ID)
			if !d.IsFolder && d.FilePath != "" {
				filePaths = append(filePaths, d.FilePath)
			}
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
		Where("kb_chunk.kb_asset_id = ?", kbAssetID).
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
