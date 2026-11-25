package netguard

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsPublicUnicastIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false},
		{"0.0.0.0", false},
		{"::", false},
		{"224.0.0.1", false},
		{"fc00::1", false},
		{"fe80::1", false},
	}

	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if got := IsPublicUnicastIP(ip); got != tc.want {
			t.Errorf("IsPublicUnicastIP(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
	if IsPublicUnicastIP(nil) {
		t.Error("IsPublicUnicastIP(nil) = true, want false")
	}
}

func TestSafeTransportBlocksNonPublicIPs(t *testing.T) {
	tr := NewSafeTransport()
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.1:80", "169.254.169.254:80", "192.168.1.1:443", "0.0.0.0:80", "[::1]:80"} {
		if _, err := tr.DialContext(context.Background(), "tcp", addr); err == nil {
			t.Fatalf("expected NewSafeTransport to block non-public IP %s", addr)
		}
	}
}

func TestSafeHTTPClientBlocksLoopbackServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewSafeHTTPClient(2 * time.Second)
	if _, err := client.Get(srv.URL); err == nil {
		t.Fatalf("expected NewSafeHTTPClient to block loopback server %s", srv.URL)
	}
}

func TestSafeCheckRedirect(t *testing.T) {
	reqFile, _ := http.NewRequest(http.MethodGet, "file:///etc/passwd", nil)
	if err := SafeCheckRedirect(reqFile, nil); err == nil {
		t.Fatal("expected file:// redirect to be blocked")
	}

	reqLocal, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/admin", nil)
	if err := SafeCheckRedirect(reqLocal, nil); err == nil {
		t.Fatal("expected 127.0.0.1 redirect to be blocked")
	}

	reqMeta, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	if err := SafeCheckRedirect(reqMeta, nil); err == nil {
		t.Fatal("expected metadata IP redirect to be blocked")
	}

	reqPublic, _ := http.NewRequest(http.MethodGet, "https://example.com/ok", nil)
	if err := SafeCheckRedirect(reqPublic, make([]*http.Request, 5)); err == nil {
		t.Fatal("expected >5 redirects to be blocked")
	}
	if err := SafeCheckRedirect(reqPublic, nil); err != nil {
		t.Fatalf("expected valid https redirect to pass, got %v", err)
	}
}

func TestReadBodyLimited(t *testing.T) {
	data, err := ReadBodyLimited(strings.NewReader("hello"), 10)
	if err != nil || string(data) != "hello" {
		t.Fatalf("unexpected ReadBodyLimited result: %q, %v", string(data), err)
	}
	if _, err := ReadBodyLimited(strings.NewReader("01234567890"), 10); err == nil {
		t.Fatal("expected error when body exceeds limit")
	}
}
