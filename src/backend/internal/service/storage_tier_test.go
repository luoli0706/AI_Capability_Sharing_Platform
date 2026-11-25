package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/kbcodec"

	"go.uber.org/zap"
)

type mockObjectStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	putCnt  atomic.Int32
	getCnt  atomic.Int32
	delCnt  atomic.Int32
	getWait time.Duration
}

func newMockObjectStore() *mockObjectStore {
	return &mockObjectStore{
		objects: make(map[string][]byte),
	}
}

func (m *mockObjectStore) PutFile(ctx context.Context, objectKey, localPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.objects[objectKey] = append([]byte(nil), data...)
	m.mu.Unlock()
	m.putCnt.Add(1)
	return nil
}

func (m *mockObjectStore) GetToFile(ctx context.Context, objectKey, localPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.getWait > 0 {
		time.Sleep(m.getWait)
	}
	m.mu.Lock()
	data, ok := m.objects[objectKey]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("object %q not found", objectKey)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	tmpPath := localPath + ".tmp.mock"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, localPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	m.getCnt.Add(1)
	return nil
}

func (m *mockObjectStore) DeleteObject(ctx context.Context, objectKey string) error {
	m.mu.Lock()
	delete(m.objects, objectKey)
	m.mu.Unlock()
	m.delCnt.Add(1)
	return nil
}

func (m *mockObjectStore) Has(objectKey string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[objectKey]
	return ok
}

func (m *mockObjectStore) GetBytes(objectKey string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[objectKey]
	return b, ok
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

func setupTierTestKBService(t *testing.T) (*kbService, *mockFileKBRepo, *mockObjectStore, *StorageTierService, string) {
	t.Helper()
	uploadDir := t.TempDir()
	kbRepo := newMockFileKBRepo()
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			5: {
				ID:         5,
				AssetCode:  "kb-tiered",
				AssetType:  model.AssetTypeKB,
				Name:       "Tiered KB",
				DomainID:   1,
				OwnerID:    1,
				Visibility: model.VisibilityPublic,
				Status:     model.AssetStatusPublished,
			},
		},
	}
	domainRepo := &mockP1DomainRepo{members: map[uint64]map[uint64]string{}}
	svc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, uploadDir, "test-storage-secret-32-bytes!!")
	kbSvc := svc.(*kbService)
	ossStore := newMockObjectStore()
	tierSvc := NewStorageTierService(kbRepo, ossStore, uploadDir, 48, 8192, zap.NewNop())
	kbSvc.SetStorageTierService(tierSvc)
	return kbSvc, kbRepo, ossStore, tierSvc, uploadDir
}

func TestStorageTier_AsyncDualWriteAndEditResync(t *testing.T) {
	kbSvc, kbRepo, ossStore, tierSvc, _ := setupTierTestKBService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tierSvc.Start(ctx)

	// 1. Create file -> async dual-write transitions state to synced & inherits relative path as oss_key
	doc, err := kbSvc.CreateMarkdownFile(1, true, 5, model.CreateKBFileRequest{
		FolderPath: "guides",
		FileName:   "hello.md",
		Content:    "initial tiered content",
	})
	if err != nil {
		t.Fatalf("CreateMarkdownFile failed: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		d, err := kbRepo.FindDocumentByID(5, doc.ID)
		return err == nil && d.StorageTier == model.StorageTierSynced
	}, "document synced after create")

	syncedDoc, err := kbRepo.FindDocumentByID(5, doc.ID)
	if err != nil {
		t.Fatalf("FindDocumentByID failed: %v", err)
	}
	if !filepath.IsAbs(syncedDoc.FilePath) && syncedDoc.OSSKey == "" {
		t.Fatalf("expected non-empty oss_key")
	}
	expectedPrefix := "5/guides/"
	if len(syncedDoc.OSSKey) <= len(expectedPrefix) || syncedDoc.OSSKey[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected oss_key to inherit relative path starting with %q, got %q", expectedPrefix, syncedDoc.OSSKey)
	}
	if !ossStore.Has(syncedDoc.OSSKey) {
		t.Fatalf("expected OSS object %q to exist", syncedDoc.OSSKey)
	}
	firstPutCount := ossStore.putCnt.Load()

	// 2. Update content -> triggers re-sync and updates OSS object ciphertext
	_, err = kbSvc.UpdateMarkdownContent(1, true, 5, doc.ID, model.UpdateKBFileRequest{
		Content: "updated tiered content v2",
	})
	if err != nil {
		t.Fatalf("UpdateMarkdownContent failed: %v", err)
	}

	waitUntil(t, 2*time.Second, func() bool {
		d, err := kbRepo.FindDocumentByID(5, doc.ID)
		return err == nil && d.StorageTier == model.StorageTierSynced && ossStore.putCnt.Load() > firstPutCount
	}, "document re-synced after update")

	ossBytes, ok := ossStore.GetBytes(syncedDoc.OSSKey)
	if !ok {
		t.Fatalf("expected OSS object after update")
	}
	plain, err := kbcodec.DecryptAtRest(ossBytes, "test-storage-secret-32-bytes!!")
	if err != nil {
		t.Fatalf("DecryptAtRest failed: %v", err)
	}
	if string(plain) != "updated tiered content v2" {
		t.Fatalf("expected updated content in OSS, got %q", string(plain))
	}
}

func TestStorageTier_TTLEvictionAndSingleflightOriginPull(t *testing.T) {
	kbSvc, kbRepo, ossStore, tierSvc, _ := setupTierTestKBService(t)
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	tierSvc.nowFunc = func() time.Time { return now }

	doc, err := kbSvc.CreateMarkdownFile(1, true, 5, model.CreateKBFileRequest{
		FileName: "cold.md",
		Content:  "cold encrypted payload",
	})
	if err != nil {
		t.Fatalf("CreateMarkdownFile failed: %v", err)
	}

	// Run one sync pass directly (without background timer)
	tierSvc.RunSyncAndEvictOnce(context.Background())

	d, err := kbRepo.FindDocumentByID(5, doc.ID)
	if err != nil {
		t.Fatalf("FindDocumentByID failed: %v", err)
	}
	if d.StorageTier != model.StorageTierSynced {
		t.Fatalf("expected synced, got %q", d.StorageTier)
	}

	// Advance clock by 49 hours (> 48h retention) -> eviction should delete local file & mark oss_only
	now = now.Add(49 * time.Hour)
	tierSvc.RunSyncAndEvictOnce(context.Background())

	d, err = kbRepo.FindDocumentByID(5, doc.ID)
	if err != nil {
		t.Fatalf("FindDocumentByID failed: %v", err)
	}
	if d.StorageTier != model.StorageTierOSSOnly {
		t.Fatalf("expected oss_only after 49h idle, got %q", d.StorageTier)
	}
	if _, statErr := os.Stat(d.FilePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected local file %q to be deleted, statErr=%v", d.FilePath, statErr)
	}
	if !ossStore.Has(d.OSSKey) {
		t.Fatalf("expected OSS backup %q to remain", d.OSSKey)
	}

	// Concurrent reads of evicted file -> singleflight must coalesce GetToFile to exactly 1 call
	ossStore.getWait = 80 * time.Millisecond
	const concurrency = 8
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)
	contents := make(chan string, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := kbSvc.GetDocumentContent(1, true, 5, doc.ID, false, 0)
			if err != nil {
				errs <- err
				return
			}
			c, _ := res["content"].(string)
			contents <- c
		}()
	}
	wg.Wait()
	close(errs)
	close(contents)

	for err := range errs {
		t.Fatalf("concurrent GetDocumentContent failed: %v", err)
	}
	for c := range contents {
		if c != "cold encrypted payload" {
			t.Fatalf("unexpected decrypted content: %q", c)
		}
	}
	if got := ossStore.getCnt.Load(); got != 1 {
		t.Fatalf("expected singleflight to coalesce GetToFile to 1 call, got %d", got)
	}

	d, err = kbRepo.FindDocumentByID(5, doc.ID)
	if err != nil {
		t.Fatalf("FindDocumentByID failed: %v", err)
	}
	if d.StorageTier != model.StorageTierSynced {
		t.Fatalf("expected storage_tier restored to synced after origin pull, got %q", d.StorageTier)
	}
}

func TestStorageTier_WatermarkLRUEviction(t *testing.T) {
	kbSvc, kbRepo, _, tierSvc, _ := setupTierTestKBService(t)
	// Set MaxLocalMB = 1 MB (target 85% = 891,289 bytes)
	tierSvc.maxLocalMB = 1

	baseTime := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	tierSvc.nowFunc = func() time.Time { return baseTime }

	// Create 3 files ~500KB each (~1.5MB total > 1MB limit)
	payload := make([]byte, 500*1024)
	for i := range payload {
		payload[i] = 'a'
	}

	var docs []*model.KBDocument
	for i := 0; i < 3; i++ {
		tierSvc.nowFunc = func() time.Time { return baseTime.Add(time.Duration(i) * time.Minute) }
		d, err := kbSvc.CreateMarkdownFile(1, true, 5, model.CreateKBFileRequest{
			FileName: fmt.Sprintf("lru_%d.md", i),
			Content:  string(payload),
		})
		if err != nil {
			t.Fatalf("CreateMarkdownFile %d failed: %v", i, err)
		}
		docs = append(docs, d)
	}

	// Sync all 3, then run eviction pass at baseTime+10m (within 48h TTL, so only LRU watermark triggers!)
	tierSvc.nowFunc = func() time.Time { return baseTime.Add(10 * time.Minute) }
	tierSvc.RunSyncAndEvictOnce(context.Background())

	d0, _ := kbRepo.FindDocumentByID(5, docs[0].ID)
	d1, _ := kbRepo.FindDocumentByID(5, docs[1].ID)
	d2, _ := kbRepo.FindDocumentByID(5, docs[2].ID)

	// Oldest (d0, and d1 since 1MB -> 85% is ~870KB, 1 file is ~500KB <= 870KB) should be evicted before d2
	if d0.StorageTier != model.StorageTierOSSOnly {
		t.Fatalf("expected oldest doc d0 to be LRU-evicted to oss_only, got %q", d0.StorageTier)
	}
	if d2.StorageTier != model.StorageTierSynced {
		t.Fatalf("expected newest doc d2 to remain synced, got %q", d2.StorageTier)
	}
	if _, err := os.Stat(d0.FilePath); !os.IsNotExist(err) {
		t.Fatalf("expected d0 local file removed")
	}
	if _, err := os.Stat(d2.FilePath); err != nil {
		t.Fatalf("expected d2 local file kept: %v", err)
	}
	_ = d1
}
