package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	sevenODefaultPassword      = "7thcv"
	sevenODefaultLevel         = 3
	sevenOExecTimeout          = 60 * time.Second
	sevenODefaultMaxConcurrent = 2
	sevenODefaultMaxBytes      = int64(64 << 20) // 64MB
	sevenOLinuxVirtMemKB       = 262144          // 256MB
)

// SevenORelease describes a downloadable 7o client/engine binary in the release directory.
type SevenORelease struct {
	FileName  string `json:"file_name"`
	Platform  string `json:"platform"`
	Kind      string `json:"kind"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	UpdatedAt string `json:"updated_at"`
}

// SevenOService provides release listing/downloading and online 7o encryption, decryption, and inspection.
type SevenOService struct {
	binPath         string
	releaseDir      string
	maxRequestBytes int64
	sem             chan struct{}
}

// NewSevenOService creates a SevenOService with configured binary path and release directory.
func NewSevenOService(binPath, releaseDir string) *SevenOService {
	if strings.TrimSpace(binPath) == "" {
		binPath = "./bin/seventhcv-7o"
	}
	if strings.TrimSpace(releaseDir) == "" {
		releaseDir = "./releases"
	}
	return &SevenOService{
		binPath:         binPath,
		releaseDir:      releaseDir,
		maxRequestBytes: sevenODefaultMaxBytes,
		sem:             make(chan struct{}, sevenODefaultMaxConcurrent),
	}
}

// SetLimits overrides the default concurrency semaphore size and max request bytes.
func (s *SevenOService) SetLimits(maxConcurrent int, maxRequestBytes int64) {
	if maxConcurrent > 0 {
		s.sem = make(chan struct{}, maxConcurrent)
	}
	if maxRequestBytes > 0 {
		s.maxRequestBytes = maxRequestBytes
	}
}

// MaxRequestBytes returns the configured maximum request payload size in bytes.
func (s *SevenOService) MaxRequestBytes() int64 {
	if s.maxRequestBytes <= 0 {
		return sevenODefaultMaxBytes
	}
	return s.maxRequestBytes
}

func (s *SevenOService) acquireSlot() error {
	if s.sem == nil {
		return nil
	}
	select {
	case s.sem <- struct{}{}:
		return nil
	default:
		return NewTooManyRequests("在线加解密任务繁忙，请稍后重试或下载桌面客户端处理")
	}
}

func (s *SevenOService) releaseSlot() {
	if s.sem == nil {
		return
	}
	<-s.sem
}

func (s *SevenOService) payloadTooLargeError() error {
	mb := s.MaxRequestBytes() / (1024 * 1024)
	if mb <= 0 {
		return NewPayloadTooLarge("在线处理文件大小超过限制，请下载桌面客户端处理")
	}
	return NewPayloadTooLarge(fmt.Sprintf("在线处理文件总大小超过 %dMB 限制，请下载桌面客户端处理", mb))
}

// ListReleases scans ReleaseDir and returns metadata for each release binary.
func (s *SevenOService) ListReleases() (map[string]interface{}, error) {
	entries, err := os.ReadDir(s.releaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]interface{}{"list": []SevenORelease{}}, nil
		}
		return nil, err
	}

	releases := make([]SevenORelease, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fullPath := filepath.Join(s.releaseDir, entry.Name())
		sumHex, err := computeFileSHA256(fullPath)
		if err != nil {
			return nil, err
		}
		platform, kind := classifyReleaseBinary(entry.Name())
		releases = append(releases, SevenORelease{
			FileName:  entry.Name(),
			Platform:  platform,
			Kind:      kind,
			Size:      info.Size(),
			SHA256:    sumHex,
			UpdatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	sort.Slice(releases, func(i, j int) bool {
		return releases[i].FileName < releases[j].FileName
	})

	return map[string]interface{}{"list": releases}, nil
}

// GetReleaseFilePath validates filename against path traversal and verifies the file exists in ReleaseDir.
func (s *SevenOService) GetReleaseFilePath(filename string) (string, error) {
	if filename == "" || filename == "." || strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") || filepath.Base(filename) != filename {
		return "", NewBadRequest("invalid release filename")
	}

	fullPath := filepath.Join(s.releaseDir, filename)
	absRelease, err := filepath.Abs(s.releaseDir)
	if err != nil {
		return "", err
	}
	absFile, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absFile, absRelease+string(filepath.Separator)) {
		return "", NewBadRequest("invalid release filename")
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return "", NewNotFound("release file not found")
	}

	return fullPath, nil
}

// EncryptFiles packages uploaded files and runs the 7o CLI encrypt command, returning the encrypted zip bytes.
func (s *SevenOService) EncryptFiles(ctx context.Context, files []*multipart.FileHeader, relPaths []string, password string, level int) ([]byte, string, error) {
	if len(files) == 0 {
		return nil, "", NewBadRequest("at least one file is required")
	}
	if strings.TrimSpace(password) == "" {
		password = sevenODefaultPassword
	}
	if level == 0 {
		level = sevenODefaultLevel
	}
	if level != 1 && level != 3 && level != 9 {
		return nil, "", NewBadRequest("level must be 1, 3, or 9")
	}

	maxBytes := s.MaxRequestBytes()
	var declaredTotal int64
	for _, fh := range files {
		if fh.Size > 0 {
			declaredTotal += fh.Size
			if declaredTotal > maxBytes {
				return nil, "", s.payloadTooLargeError()
			}
		}
	}

	if err := s.acquireSlot(); err != nil {
		return nil, "", err
	}
	defer s.releaseSlot()

	tmpDir, err := os.MkdirTemp("", "ascp-7o-enc-*")
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	inputRoot := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputRoot, 0o755); err != nil {
		return nil, "", err
	}

	var firstCleanRel string
	var writtenTotal int64
	for i, fh := range files {
		rawRel := fh.Filename
		if i < len(relPaths) && strings.TrimSpace(relPaths[i]) != "" {
			rawRel = relPaths[i]
		}
		cleanRel, err := sanitizeSevenORelPath(rawRel)
		if err != nil {
			return nil, "", err
		}
		if i == 0 {
			firstCleanRel = cleanRel
		}

		dstPath := filepath.Join(inputRoot, filepath.FromSlash(cleanRel))
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return nil, "", err
		}
		written, err := saveMultipartFileLimited(fh, dstPath, maxBytes-writtenTotal)
		if err != nil {
			if errors.Is(err, errPayloadTooLarge) {
				return nil, "", s.payloadTooLargeError()
			}
			return nil, "", err
		}
		writtenTotal += written
	}

	inputPath := inputRoot
	outName := "encrypted-7o.zip"
	if len(files) == 1 && !strings.Contains(firstCleanRel, "/") {
		inputPath = filepath.Join(inputRoot, firstCleanRel)
		outName = firstCleanRel + ".7o.zip"
	}

	outZip := filepath.Join(tmpDir, "output.zip")
	_, stderr, err := s.run7oCommand(ctx, "encrypt", inputPath, outZip, "--password", password, "--level", strconv.Itoa(level))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, "", NewBadRequest("7o 加密执行超时")
		}
		return nil, "", fmt.Errorf("7o encrypt failed: %w: %s", err, stderr)
	}

	zipBytes, err := os.ReadFile(outZip)
	if err != nil {
		return nil, "", err
	}
	return zipBytes, outName, nil
}

// DecryptZip runs the 7o CLI decrypt command on the uploaded zip and repacks extracted files into a standard zip.
func (s *SevenOService) DecryptZip(ctx context.Context, zipReader io.Reader, password string) ([]byte, error) {
	if zipReader == nil {
		return nil, NewBadRequest("file is required")
	}
	if strings.TrimSpace(password) == "" {
		password = sevenODefaultPassword
	}

	if err := s.acquireSlot(); err != nil {
		return nil, err
	}
	defer s.releaseSlot()

	tmpDir, err := os.MkdirTemp("", "ascp-7o-dec-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	inZip := filepath.Join(tmpDir, "input.zip")
	if _, err := saveReaderToFileLimited(zipReader, inZip, s.MaxRequestBytes()); err != nil {
		if errors.Is(err, errPayloadTooLarge) {
			return nil, s.payloadTooLargeError()
		}
		return nil, err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return nil, err
	}

	_, stderr, err := s.run7oCommand(ctx, "decrypt", inZip, extractDir, "--password", password)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, NewBadRequest("7o 解密执行超时")
		}
		if isWrongKeyError(stderr) {
			return nil, NewBadRequest("密钥错误，无法解密该 7o 压缩包")
		}
		return nil, fmt.Errorf("7o decrypt failed: %w: %s", err, stderr)
	}

	return packDirToZip(extractDir)
}

// InspectZip runs the 7o CLI inspect command on the uploaded zip and parses its JSON output.
func (s *SevenOService) InspectZip(ctx context.Context, zipReader io.Reader) (map[string]interface{}, error) {
	if zipReader == nil {
		return nil, NewBadRequest("file is required")
	}

	if err := s.acquireSlot(); err != nil {
		return nil, err
	}
	defer s.releaseSlot()

	tmpDir, err := os.MkdirTemp("", "ascp-7o-insp-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	inZip := filepath.Join(tmpDir, "input.zip")
	if _, err := saveReaderToFileLimited(zipReader, inZip, s.MaxRequestBytes()); err != nil {
		if errors.Is(err, errPayloadTooLarge) {
			return nil, s.payloadTooLargeError()
		}
		return nil, err
	}

	stdout, stderr, err := s.run7oCommand(ctx, "inspect", inZip)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, NewBadRequest("7o 解析执行超时")
		}
		return nil, fmt.Errorf("7o inspect failed: %w: %s", err, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("invalid inspect output: %w", err)
	}
	return result, nil
}

func (s *SevenOService) run7oCommand(ctx context.Context, args ...string) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	execCtx, cancel := context.WithTimeout(ctx, sevenOExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "linux" {
		shellArgs := append([]string{"-c", fmt.Sprintf(`ulimit -v %d && exec "$@"`, sevenOLinuxVirtMemKB), "_", s.binPath}, args...)
		cmd = exec.CommandContext(execCtx, "sh", shellArgs...)
	} else {
		cmd = exec.CommandContext(execCtx, s.binPath, args...)
	}
	cmd.Env = append(os.Environ(), "RAYON_NUM_THREADS=1", "OMP_NUM_THREADS=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if execCtx.Err() == context.DeadlineExceeded {
		return stdout.Bytes(), strings.TrimSpace(stderr.String()), context.DeadlineExceeded
	}
	return stdout.Bytes(), strings.TrimSpace(stderr.String()), err
}

func classifyReleaseBinary(filename string) (platform string, kind string) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".exe") {
		return "Windows x64 (GUI 客户端分发)", "gui"
	}
	if strings.Contains(lower, "linux") || !strings.Contains(filename, ".") {
		return "Linux x64 (服务端在线执行引擎)", "cli"
	}
	return "其他平台二进制", "other"
}

func computeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sanitizeSevenORelPath(raw string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), "\\", "/")
	normalized = strings.TrimLeft(normalized, "/")
	if normalized == "" {
		return "", NewBadRequest("invalid file relative path")
	}

	for _, seg := range strings.Split(normalized, "/") {
		if seg == ".." {
			return "", NewBadRequest("invalid file relative path: '..' is not allowed")
		}
	}

	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", NewBadRequest("invalid file relative path")
	}
	return cleaned, nil
}

var errPayloadTooLarge = errors.New("payload too large")

func saveMultipartFileLimited(fh *multipart.FileHeader, dstPath string, remainingBytes int64) (int64, error) {
	src, err := fh.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = src.Close() }()

	return saveReaderToFileLimited(src, dstPath, remainingBytes)
}

func saveReaderToFileLimited(r io.Reader, dstPath string, maxBytes int64) (int64, error) {
	if maxBytes < 0 {
		return 0, errPayloadTooLarge
	}
	out, err := os.Create(dstPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = out.Close() }()

	n, err := io.Copy(out, io.LimitReader(r, maxBytes+1))
	if err != nil {
		return n, err
	}
	if n > maxBytes {
		return n, errPayloadTooLarge
	}
	return n, nil
}

func isWrongKeyError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(stderr, "WrongKey") ||
		strings.Contains(stderr, "密钥") ||
		strings.Contains(lower, "key")
}

// packDirToZip walks dir and writes all regular files into an in-memory standard .zip archive.
func packDirToZip(dir string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(rel)

		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = zipName
		header.Method = zip.Deflate

		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
