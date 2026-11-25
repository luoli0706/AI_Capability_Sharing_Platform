package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/ossstore"
	"ascp-backend/internal/repository"

	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

const (
	defaultRetentionHours = 48
	defaultMaxLocalMB     = int64(8192)
	syncQueueCapacity     = 1024
	touchThrottleInterval = time.Hour
	scanBatchLimit        = 200
	startupScanDelay      = 10 * time.Second
	periodicScanInterval  = 15 * time.Minute
)

type syncJob struct {
	docID    uint64
	filePath string
}

// StorageTierService manages ECS <-> OSS tiered storage:
// async dual-write, TTL/LRU watermark eviction, and singleflight OSS origin-pull.
type StorageTierService struct {
	kbRepo         repository.KBRepository
	ossStore       ossstore.ObjectStore
	uploadDir      string
	retentionHours int
	maxLocalMB     int64
	logger         *zap.Logger

	syncQueue  chan syncJob
	pendingMu  sync.Mutex
	pendingSet map[uint64]struct{}

	sfGroup singleflight.Group

	touchMu     sync.Mutex
	lastTouched map[string]time.Time

	nowFunc func() time.Time
}

// NewStorageTierService creates a StorageTierService with sensible defaults.
func NewStorageTierService(
	kbRepo repository.KBRepository,
	ossStore ossstore.ObjectStore,
	uploadDir string,
	retentionHours int,
	maxLocalMB int64,
	logger *zap.Logger,
) *StorageTierService {
	if retentionHours <= 0 {
		retentionHours = defaultRetentionHours
	}
	if maxLocalMB <= 0 {
		maxLocalMB = defaultMaxLocalMB
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StorageTierService{
		kbRepo:         kbRepo,
		ossStore:       ossStore,
		uploadDir:      uploadDir,
		retentionHours: retentionHours,
		maxLocalMB:     maxLocalMB,
		logger:         logger,
		syncQueue:      make(chan syncJob, syncQueueCapacity),
		pendingSet:     make(map[uint64]struct{}),
		lastTouched:    make(map[string]time.Time),
		nowFunc:        time.Now,
	}
}

// MarkLocalAndEnqueueSync marks a document as local and enqueues it for async upload to OSS.
func (s *StorageTierService) MarkLocalAndEnqueueSync(docID uint64, filePath string) {
	if s == nil || docID == 0 || filePath == "" {
		return
	}
	now := s.nowFunc()
	ossKey := ossstore.BuildObjectKey(s.uploadDir, filePath)
	if err := s.kbRepo.UpdateStorageSyncState(docID, model.StorageTierLocal, ossKey, nil, &now); err != nil {
		s.logger.Warn("failed to mark document local state", zap.Uint64("doc_id", docID), zap.Error(err))
	}

	s.touchMu.Lock()
	s.lastTouched[filePath] = now
	s.touchMu.Unlock()

	s.pendingMu.Lock()
	if _, exists := s.pendingSet[docID]; exists {
		s.pendingMu.Unlock()
		return
	}
	select {
	case s.syncQueue <- syncJob{docID: docID, filePath: filePath}:
		s.pendingSet[docID] = struct{}{}
	default:
		// Queue full; periodic RunSyncAndEvictOnce will pick up local docs.
	}
	s.pendingMu.Unlock()
}

// EnsureLocalFile guarantees filePath exists on local disk, pulling from OSS via singleflight if evicted.
func (s *StorageTierService) EnsureLocalFile(ctx context.Context, filePath string) error {
	if s == nil || filePath == "" {
		return nil
	}

	if _, err := os.Stat(filePath); err == nil {
		s.touchAccessThrottled(filePath)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if s.ossStore == nil {
		return os.ErrNotExist
	}

	_, err, _ := s.sfGroup.Do(filePath, func() (interface{}, error) {
		if _, statErr := os.Stat(filePath); statErr == nil {
			s.touchAccessThrottled(filePath)
			return nil, nil
		}

		doc, findErr := s.kbRepo.FindDocumentByFilePath(filePath)
		ossKey := ossstore.BuildObjectKey(s.uploadDir, filePath)
		if findErr == nil && doc != nil && doc.OSSKey != "" {
			ossKey = doc.OSSKey
		}

		if err := s.ossStore.GetToFile(ctx, ossKey, filePath); err != nil {
			return nil, err
		}

		now := s.nowFunc()
		if findErr == nil && doc != nil {
			if err := s.kbRepo.UpdateStorageSyncState(doc.ID, model.StorageTierSynced, ossKey, doc.OSSSyncedAt, &now); err != nil {
				s.logger.Warn("failed to update synced state after OSS pull", zap.Uint64("doc_id", doc.ID), zap.Error(err))
			}
		}

		s.touchMu.Lock()
		s.lastTouched[filePath] = now
		s.touchMu.Unlock()

		return nil, nil
	})
	return err
}

func (s *StorageTierService) touchAccessThrottled(filePath string) {
	now := s.nowFunc()
	s.touchMu.Lock()
	prev, ok := s.lastTouched[filePath]
	if ok && now.Sub(prev) < touchThrottleInterval {
		s.touchMu.Unlock()
		return
	}
	s.lastTouched[filePath] = now
	s.touchMu.Unlock()

	doc, err := s.kbRepo.FindDocumentByFilePath(filePath)
	if err != nil || doc == nil {
		return
	}
	if err := s.kbRepo.TouchLastAccessed(doc.ID, now); err != nil {
		s.logger.Warn("failed to touch last_accessed_at", zap.Uint64("doc_id", doc.ID), zap.Error(err))
	}
}

func (s *StorageTierService) syncDoc(ctx context.Context, doc *model.KBDocument) error {
	if doc == nil || doc.IsFolder || doc.FilePath == "" || s.ossStore == nil {
		return nil
	}
	if _, err := os.Stat(doc.FilePath); err != nil {
		return err
	}
	ossKey := doc.OSSKey
	if ossKey == "" {
		ossKey = ossstore.BuildObjectKey(s.uploadDir, doc.FilePath)
	}
	if err := s.ossStore.PutFile(ctx, ossKey, doc.FilePath); err != nil {
		return err
	}
	now := s.nowFunc()
	lastAccessed := doc.LastAccessedAt
	if lastAccessed == nil {
		t := now
		lastAccessed = &t
	}
	return s.kbRepo.UpdateStorageSyncState(doc.ID, model.StorageTierSynced, ossKey, &now, lastAccessed)
}

func (s *StorageTierService) evictDoc(doc model.KBDocument, cutoff *time.Time) bool {
	ok, err := s.kbRepo.TryMarkOSSOnly(doc.ID, cutoff)
	if err != nil || !ok {
		return false
	}
	if err := os.Remove(doc.FilePath); err != nil && !os.IsNotExist(err) {
		_ = s.kbRepo.UpdateStorageSyncState(doc.ID, model.StorageTierSynced, doc.OSSKey, doc.OSSSyncedAt, doc.LastAccessedAt)
		s.logger.Warn("failed to remove local evicted file, rolled back state",
			zap.Uint64("doc_id", doc.ID),
			zap.String("file_path", doc.FilePath),
			zap.Error(err),
		)
		return false
	}
	return true
}

// RunSyncAndEvictOnce runs one full pass of:
// 1. Syncing unsynced local documents to OSS
// 2. Evicting synced documents idle longer than retentionHours
// 3. LRU eviction if uploadDir exceeds maxLocalMB down to 85% watermark
func (s *StorageTierService) RunSyncAndEvictOnce(ctx context.Context) {
	if s == nil || s.ossStore == nil {
		return
	}

	// 1. Sync unsynced local docs
	unsynced, err := s.kbRepo.ListUnsyncedLocalDocs(scanBatchLimit)
	if err != nil {
		s.logger.Warn("failed to list unsynced local docs", zap.Error(err))
	} else {
		for i := range unsynced {
			if ctx.Err() != nil {
				return
			}
			if err := s.syncDoc(ctx, &unsynced[i]); err != nil {
				s.logger.Warn("failed to sync doc to OSS",
					zap.Uint64("doc_id", unsynced[i].ID),
					zap.Error(err),
				)
			}
		}
	}

	// 2. TTL eviction for synced docs past retentionHours
	cutoff := s.nowFunc().Add(-time.Duration(s.retentionHours) * time.Hour)
	evictable, err := s.kbRepo.ListEvictableSyncedDocs(cutoff, scanBatchLimit)
	if err != nil {
		s.logger.Warn("failed to list evictable synced docs", zap.Error(err))
	} else {
		for _, doc := range evictable {
			if ctx.Err() != nil {
				return
			}
			s.evictDoc(doc, &cutoff)
		}
	}

	// 3. Disk watermark LRU eviction (> maxLocalMB -> target 85%)
	totalBytes, err := dirSizeBytes(s.uploadDir)
	if err != nil {
		s.logger.Warn("failed to compute uploadDir size", zap.Error(err))
		return
	}
	maxBytes := s.maxLocalMB * 1024 * 1024
	if totalBytes <= maxBytes {
		return
	}
	targetBytes := maxBytes * 85 / 100

	lruDocs, err := s.kbRepo.ListLRUSyncedDocs(scanBatchLimit)
	if err != nil {
		s.logger.Warn("failed to list LRU synced docs", zap.Error(err))
		return
	}
	for _, doc := range lruDocs {
		if ctx.Err() != nil || totalBytes <= targetBytes {
			break
		}
		fileSize := int64(doc.FileSize)
		if info, statErr := os.Stat(doc.FilePath); statErr == nil {
			fileSize = info.Size()
		}
		if s.evictDoc(doc, nil) {
			totalBytes -= fileSize
			if totalBytes < 0 {
				totalBytes = 0
			}
		}
	}
}

// Start launches the async sync worker and periodic scan/eviction loop.
func (s *StorageTierService) Start(ctx context.Context) {
	if s == nil {
		return
	}

	// Async dual-write worker
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-s.syncQueue:
				s.pendingMu.Lock()
				delete(s.pendingSet, job.docID)
				s.pendingMu.Unlock()

				doc, err := s.kbRepo.FindDocumentByFilePath(job.filePath)
				if err != nil || doc == nil {
					continue
				}
				if err := s.syncDoc(ctx, doc); err != nil {
					s.logger.Warn("async OSS sync failed",
						zap.Uint64("doc_id", job.docID),
						zap.Error(err),
					)
				}
			}
		}
	}()

	// Periodic scan & eviction worker
	go func() {
		delayTimer := time.NewTimer(startupScanDelay)
		select {
		case <-ctx.Done():
			delayTimer.Stop()
			return
		case <-delayTimer.C:
			s.RunSyncAndEvictOnce(ctx)
		}

		ticker := time.NewTicker(periodicScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunSyncAndEvictOnce(ctx)
			}
		}
	}()
}

func dirSizeBytes(root string) (int64, error) {
	if root == "" {
		return 0, nil
	}
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		total += info.Size()
		return nil
	})
	if os.IsNotExist(err) {
		return 0, nil
	}
	return total, err
}
