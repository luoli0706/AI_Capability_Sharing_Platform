package ossstore

import (
	"path/filepath"
	"testing"
)

func TestNormalizeEndpoint(t *testing.T) {
	cases := []struct {
		endpoint string
		bucket   string
		want     string
	}{
		{
			endpoint: "oss-cn-shanghai-internal.aliyuncs.com",
			bucket:   "ascp-7thcv-sh",
			want:     "https://oss-cn-shanghai-internal.aliyuncs.com",
		},
		{
			endpoint: "ascp-7thcv-sh.oss-cn-shanghai-internal.aliyuncs.com",
			bucket:   "ascp-7thcv-sh",
			want:     "https://oss-cn-shanghai-internal.aliyuncs.com",
		},
		{
			endpoint: "https://ascp-7thcv-sh.oss-cn-shanghai-internal.aliyuncs.com/",
			bucket:   "ascp-7thcv-sh",
			want:     "https://oss-cn-shanghai-internal.aliyuncs.com",
		},
		{
			endpoint: "http://ascp-7thcv-sh.oss-cn-shanghai.aliyuncs.com/",
			bucket:   "ascp-7thcv-sh",
			want:     "http://oss-cn-shanghai.aliyuncs.com",
		},
		{
			endpoint: "ASCP-7THCV-SH.oss-cn-shanghai-internal.aliyuncs.com",
			bucket:   "ascp-7thcv-sh",
			want:     "https://oss-cn-shanghai-internal.aliyuncs.com",
		},
		{
			endpoint: "https://oss-cn-beijing.aliyuncs.com",
			bucket:   "other-bucket",
			want:     "https://oss-cn-beijing.aliyuncs.com",
		},
		{
			endpoint: "",
			bucket:   "ascp-7thcv-sh",
			want:     "",
		},
	}

	for _, tc := range cases {
		got := NormalizeEndpoint(tc.endpoint, tc.bucket)
		if got != tc.want {
			t.Errorf("NormalizeEndpoint(%q, %q) = %q, want %q", tc.endpoint, tc.bucket, got, tc.want)
		}
	}
}

func TestBuildObjectKey(t *testing.T) {
	uploadDir := filepath.Join("data", "uploads")
	cases := []struct {
		uploadDir string
		filePath  string
		want      string
	}{
		{
			uploadDir: uploadDir,
			filePath:  filepath.Join(uploadDir, "5", "abc123_doc.md"),
			want:      "5/abc123_doc.md",
		},
		{
			uploadDir: uploadDir,
			filePath:  filepath.Join(uploadDir, "12", "nested", "folder", "hash_file.pdf"),
			want:      "12/nested/folder/hash_file.pdf",
		},
		{
			uploadDir: uploadDir,
			filePath:  filepath.Join("other", "dir", "fallback.txt"),
			want:      "fallback.txt",
		},
		{
			uploadDir: "",
			filePath:  filepath.Join("some", "path", "doc.md"),
			want:      "doc.md",
		},
	}

	for _, tc := range cases {
		got := BuildObjectKey(tc.uploadDir, tc.filePath)
		if got != tc.want {
			t.Errorf("BuildObjectKey(%q, %q) = %q, want %q", tc.uploadDir, tc.filePath, got, tc.want)
		}
	}
}
