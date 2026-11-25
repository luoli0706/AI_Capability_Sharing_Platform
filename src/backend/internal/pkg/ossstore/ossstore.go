package ossstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// ObjectStore abstracts object storage operations for tiered KB storage.
type ObjectStore interface {
	PutFile(ctx context.Context, objectKey, localPath string) error
	GetToFile(ctx context.Context, objectKey, localPath string) error
	DeleteObject(ctx context.Context, objectKey string) error
}

// AliyunOSSStore implements ObjectStore backed by Aliyun OSS.
type AliyunOSSStore struct {
	bucket *oss.Bucket
}

// NormalizeEndpoint strips a leading "<bucket>." prefix if present and ensures an http/https scheme.
// This prevents the Aliyun OSS SDK from constructing a double-bucket host such as "<bucket>.<bucket>.oss-...".
func NormalizeEndpoint(endpoint, bucketName string) string {
	ep := strings.TrimSpace(endpoint)
	ep = strings.TrimRight(ep, "/")
	if ep == "" {
		return ""
	}

	scheme := "https://"
	if strings.HasPrefix(strings.ToLower(ep), "http://") {
		scheme = "http://"
		ep = ep[len("http://"):]
	} else if strings.HasPrefix(strings.ToLower(ep), "https://") {
		ep = ep[len("https://"):]
	}

	bucket := strings.TrimSpace(bucketName)
	if bucket != "" {
		prefix := bucket + "."
		if len(ep) > len(prefix) && strings.EqualFold(ep[:len(prefix)], prefix) {
			ep = ep[len(prefix):]
		}
	}

	return scheme + ep
}

// NewAliyunOSSStore initializes an AliyunOSSStore with endpoint normalization.
func NewAliyunOSSStore(endpoint, accessKeyID, accessKeySecret, bucketName string) (*AliyunOSSStore, error) {
	if strings.TrimSpace(accessKeyID) == "" || strings.TrimSpace(accessKeySecret) == "" {
		return nil, fmt.Errorf("oss access key id and secret are required")
	}
	if strings.TrimSpace(bucketName) == "" {
		return nil, fmt.Errorf("oss bucket name is required")
	}
	normEndpoint := NormalizeEndpoint(endpoint, bucketName)
	if normEndpoint == "" {
		return nil, fmt.Errorf("oss endpoint is required")
	}

	client, err := oss.New(normEndpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("create oss client: %w", err)
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, fmt.Errorf("open oss bucket %q: %w", bucketName, err)
	}
	return &AliyunOSSStore{bucket: bucket}, nil
}

// PutFile uploads a local file to OSS under objectKey.
func (s *AliyunOSSStore) PutFile(ctx context.Context, objectKey, localPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.bucket.PutObjectFromFile(objectKey, localPath)
}

// GetToFile downloads objectKey to a temporary file and atomically renames it to localPath.
func (s *AliyunOSSStore) GetToFile(ctx context.Context, objectKey, localPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("generate temp suffix: %w", err)
	}
	tmpPath := localPath + ".tmp." + hex.EncodeToString(randBytes)

	if err := s.bucket.GetObjectToFile(objectKey, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, localPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("atomic rename temp file: %w", err)
	}
	return nil
}

// DeleteObject removes objectKey from OSS.
func (s *AliyunOSSStore) DeleteObject(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.bucket.DeleteObject(objectKey)
}

// BuildObjectKey computes a slash-separated OSS object key relative to uploadDir.
// If filePath is outside uploadDir or relative computation fails, it falls back to a sanitized base/path.
func BuildObjectKey(uploadDir, filePath string) string {
	cleanFile := filepath.Clean(filePath)
	if strings.TrimSpace(uploadDir) != "" {
		cleanBase := filepath.Clean(uploadDir)
		if rel, err := filepath.Rel(cleanBase, cleanFile); err == nil &&
			rel != "" && rel != "." && rel != ".." &&
			!strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
			!filepath.IsAbs(rel) {
			return strings.TrimLeft(filepath.ToSlash(rel), "/")
		}
	}

	base := filepath.Base(cleanFile)
	if base != "" && base != "." && base != ".." && base != string(filepath.Separator) {
		return base
	}
	return strings.TrimLeft(filepath.ToSlash(cleanFile), "/")
}
