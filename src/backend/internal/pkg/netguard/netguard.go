package netguard

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

const defaultHTTPTimeout = 10 * time.Second

// IsPublicUnicastIP reports whether ip is a non-nil public unicast IP address.
func IsPublicUnicastIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	return !ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsInterfaceLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}

// NewSafeTransport creates an http.Transport whose Dialer.Control blocks connections
// to loopback, private, link-local, multicast, or unspecified IP addresses at dial time,
// mitigating SSRF and DNS rebinding attacks.
func NewSafeTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !IsPublicUnicastIP(ip) {
				return fmt.Errorf("blocked connection to non-public IP: %s", host)
			}
			return nil
		},
	}
	return &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// ValidateHTTPURL verifies that rawURL uses http or https and has a non-empty host.
func ValidateHTTPURL(rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return nil, errors.New("invalid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q: only http and https are allowed", u.Scheme)
	}
	return u, nil
}

// SafeCheckRedirect validates redirect targets to ensure only http/https schemes
// and public IP literals are followed, up to 5 hops.
func SafeCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("stopped after 5 redirects")
	}
	if req.URL == nil {
		return errors.New("redirect to nil URL blocked")
	}
	scheme := strings.ToLower(req.URL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("redirect to disallowed scheme %q blocked", req.URL.Scheme)
	}
	host := strings.TrimSpace(req.URL.Hostname())
	if host == "" {
		return errors.New("redirect to empty host blocked")
	}
	if ip := net.ParseIP(host); ip != nil && !IsPublicUnicastIP(ip) {
		return fmt.Errorf("redirect to non-public IP %s blocked", host)
	}
	return nil
}

// NewSafeHTTPClient creates an http.Client configured with NewSafeTransport and SafeCheckRedirect.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     NewSafeTransport(),
		CheckRedirect: SafeCheckRedirect,
	}
}

// ReadBodyLimited reads up to maxBytes from r, returning an error if the body exceeds maxBytes.
func ReadBodyLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 2 << 20
	}
	b, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds limit of %d bytes", maxBytes)
	}
	return b, nil
}
