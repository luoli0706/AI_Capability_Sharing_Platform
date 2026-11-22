package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
)

func TestSevenOService_ListReleases(t *testing.T) {
	dir := t.TempDir()

	winContent := []byte("fake-windows-gui-binary")
	linuxContent := []byte("fake-linux-engine-binary")
	if err := os.WriteFile(filepath.Join(dir, "seventhcv-7o-windows-amd64.exe"), winContent, 0o644); err != nil {
		t.Fatalf("write win file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "seventhcv-7o-linux-amd64"), linuxContent, 0o755); err != nil {
		t.Fatalf("write linux file: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	svc := NewSevenOService("", dir)
	res, err := svc.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases error: %v", err)
	}

	list, ok := res["list"].([]SevenORelease)
	if !ok {
		t.Fatalf("expected []SevenORelease, got %T", res["list"])
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(list))
	}

	winSum := sha256.Sum256(winContent)
	linuxSum := sha256.Sum256(linuxContent)

	byName := map[string]SevenORelease{}
	for _, item := range list {
		byName[item.FileName] = item
	}

	winRel, ok := byName["seventhcv-7o-windows-amd64.exe"]
	if !ok {
		t.Fatalf("missing windows release")
	}
	if winRel.Platform != "Windows x64 (GUI 客户端分发)" || winRel.Kind != "gui" {
		t.Fatalf("unexpected windows classification: %+v", winRel)
	}
	if winRel.Size != int64(len(winContent)) || winRel.SHA256 != hex.EncodeToString(winSum[:]) {
		t.Fatalf("unexpected windows size/sha256: %+v", winRel)
	}

	linuxRel, ok := byName["seventhcv-7o-linux-amd64"]
	if !ok {
		t.Fatalf("missing linux release")
	}
	if linuxRel.Platform != "Linux x64 (服务端在线执行引擎)" || linuxRel.Kind != "cli" {
		t.Fatalf("unexpected linux classification: %+v", linuxRel)
	}
	if linuxRel.Size != int64(len(linuxContent)) || linuxRel.SHA256 != hex.EncodeToString(linuxSum[:]) {
		t.Fatalf("unexpected linux size/sha256: %+v", linuxRel)
	}

	// Non-existent directory should return empty list without error.
	emptySvc := NewSevenOService("", filepath.Join(dir, "missing"))
	emptyRes, err := emptySvc.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases missing dir error: %v", err)
	}
	emptyList := emptyRes["list"].([]SevenORelease)
	if len(emptyList) != 0 {
		t.Fatalf("expected empty list for missing dir, got %d", len(emptyList))
	}
}

func TestSevenOService_GetReleaseFilePath_TraversalRejection(t *testing.T) {
	dir := t.TempDir()
	validFile := filepath.Join(dir, "seventhcv-7o-linux-amd64")
	if err := os.WriteFile(validFile, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write valid file: %v", err)
	}

	svc := NewSevenOService("", dir)

	invalidInputs := []string{
		"",
		".",
		"..",
		"../secret.txt",
		"..\\secret.txt",
		"sub/file.exe",
		"sub\\file.exe",
		"foo/../bar",
	}
	for _, in := range invalidInputs {
		if _, err := svc.GetReleaseFilePath(in); err == nil {
			t.Fatalf("expected error for invalid filename %q, got nil", in)
		}
	}

	if _, err := svc.GetReleaseFilePath("nonexistent.exe"); err == nil {
		t.Fatalf("expected error for missing release file, got nil")
	}

	got, err := svc.GetReleaseFilePath("seventhcv-7o-linux-amd64")
	if err != nil {
		t.Fatalf("expected valid path, got error: %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(validFile) {
		t.Fatalf("expected %q, got %q", validFile, got)
	}
}

func TestPackDirToZip(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested", "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "root.txt"), []byte("hello root"), 0o644); err != nil {
		t.Fatalf("write root.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "sub", "deep.txt"), []byte("hello deep"), 0o644); err != nil {
		t.Fatalf("write deep.txt: %v", err)
	}

	zipBytes, err := packDirToZip(dir)
	if err != nil {
		t.Fatalf("packDirToZip error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader error: %v", err)
	}

	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry %s: %v", f.Name, err)
		}
		got[f.Name] = string(data)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 entries in zip, got %d: %+v", len(got), got)
	}
	if got["root.txt"] != "hello root" {
		t.Fatalf("unexpected root.txt content: %q", got["root.txt"])
	}
	if got["nested/sub/deep.txt"] != "hello deep" {
		t.Fatalf("unexpected nested/sub/deep.txt content: %q", got["nested/sub/deep.txt"])
	}
}

func TestSanitizeSevenORelPath(t *testing.T) {
	valid, err := sanitizeSevenORelPath(`folder\sub/file.txt`)
	if err != nil || valid != "folder/sub/file.txt" {
		t.Fatalf("expected folder/sub/file.txt, got %q (err=%v)", valid, err)
	}

	for _, bad := range []string{"", ".", "..", "../a.txt", "a/../b.txt", "a\\..\\b.txt"} {
		if _, err := sanitizeSevenORelPath(bad); err == nil {
			t.Fatalf("expected error for relPath %q", bad)
		}
	}
}

func makeSevenOTestFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="`+fieldName+`"; filename="`+filename+`"`)
	h.Set("Content-Type", "application/octet-stream")
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatalf("CreatePart: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("part.Write: %v", err)
	}
	_ = mw.Close()

	mr := multipart.NewReader(&buf, mw.Boundary())
	form, err := mr.ReadForm(10 << 20)
	if err != nil {
		t.Fatalf("ReadForm: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File[fieldName][0]
}

func TestSevenOService_ConcurrencyAndSizeLimits(t *testing.T) {
	svc := NewSevenOService("", t.TempDir())
	svc.SetLimits(2, 1024)

	// 1. Concurrency limit: occupy 2 slots, 3rd call must return 429 TooManyRequests.
	if err := svc.acquireSlot(); err != nil {
		t.Fatalf("slot 1 failed: %v", err)
	}
	if err := svc.acquireSlot(); err != nil {
		t.Fatalf("slot 2 failed: %v", err)
	}

	fhSmall := makeSevenOTestFileHeader(t, "file", "small.txt", []byte("hello"))
	_, _, err := svc.EncryptFiles(context.Background(), []*multipart.FileHeader{fhSmall}, nil, "7thcv", 3)
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("expected 429 TooManyRequests when semaphore full, got %v", err)
	}

	_, err = svc.DecryptZip(context.Background(), bytes.NewReader([]byte("zipdata")), "7thcv")
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("expected 429 TooManyRequests on DecryptZip when semaphore full, got %v", err)
	}

	_, err = svc.InspectZip(context.Background(), bytes.NewReader([]byte("zipdata")))
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusTooManyRequests {
		t.Fatalf("expected 429 TooManyRequests on InspectZip when semaphore full, got %v", err)
	}

	svc.releaseSlot()
	svc.releaseSlot()

	// 2. Size limit: exceed 1024 bytes on EncryptFiles, DecryptZip, InspectZip -> 413 PayloadTooLarge.
	oversized := bytes.Repeat([]byte("A"), 2048)
	fhBig := makeSevenOTestFileHeader(t, "file", "big.bin", oversized)
	_, _, err = svc.EncryptFiles(context.Background(), []*multipart.FileHeader{fhBig}, nil, "7thcv", 3)
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 PayloadTooLarge on EncryptFiles, got %v", err)
	}

	_, err = svc.DecryptZip(context.Background(), bytes.NewReader(oversized), "7thcv")
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 PayloadTooLarge on DecryptZip, got %v", err)
	}

	_, err = svc.InspectZip(context.Background(), bytes.NewReader(oversized))
	if !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 PayloadTooLarge on InspectZip, got %v", err)
	}
}
