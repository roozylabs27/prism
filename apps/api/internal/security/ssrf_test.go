package security

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"0.0.0.0", true},
		{"::1", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"142.250.190.46", false}, // Google public IP
		{"104.244.42.1", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %s", tt.ip)
		}
		got := IsPrivateIP(ip)
		if got != tt.expected {
			t.Errorf("IsPrivateIP(%s) = %v, expected %v", tt.ip, got, tt.expected)
		}
	}
}

func TestValidateOutboundURL_BlockedTargets(t *testing.T) {
	_ = os.Unsetenv("ALLOW_INTERNAL_SSRF")
	_ = os.Unsetenv("IS_DEVELOPMENT")

	blockedURLs := []string{
		"http://localhost:8080/api",
		"http://127.0.0.1:5432",
		"http://10.0.0.5/admin",
		"http://172.16.1.1/secret",
		"http://192.168.1.100:3000",
		"http://169.254.169.254/latest/meta-data/",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://database.local:5432",
		"ftp://example.com/file",
		"file:///etc/passwd",
		"gopher://example.com",
	}

	for _, rawURL := range blockedURLs {
		err := ValidateOutboundURL(rawURL)
		if err == nil {
			t.Errorf("expected ValidateOutboundURL(%q) to be blocked, but it was allowed", rawURL)
		}
	}
}

func TestNewSafeHTTPClient_Construction(t *testing.T) {
	client := NewSafeHTTPClient(5 * time.Second)
	if client == nil {
		t.Fatal("expected non-nil safe HTTP client")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("expected client timeout 5s, got %v", client.Timeout)
	}
	if client.Transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestSafeCheckRedirect_StripsAuthHeadersCrossHost(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")

	originReq := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.provider-a.com", Path: "/v1/chat"}}

	t.Run("strips credentials on cross-host redirect", func(t *testing.T) {
		nextReq := &http.Request{
			URL: &url.URL{Scheme: "https", Host: "evil.example.org", Path: "/collect"},
			Header: http.Header{
				"Authorization":       {"Bearer secret"},
				"Proxy-Authorization": {"Basic xyz"},
				"X-Api-Key":           {"ant-secret"},
				"X-Goog-Api-Key":      {"AIzaSecret"},
				"Cookie":              {"sid=abc"},
				"Content-Type":        {"application/json"},
			},
		}
		if err := SafeCheckRedirect(nextReq, []*http.Request{originReq}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, h := range []string{"Authorization", "Proxy-Authorization", "X-Api-Key", "X-Goog-Api-Key", "Cookie"} {
			if got := nextReq.Header.Get(h); got != "" {
				t.Errorf("expected cross-host redirect header %s stripped, got %q", h, got)
			}
		}
		if got := nextReq.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected safe header Content-Type preserved, got %q", got)
		}
	})

	t.Run("preserves Authorization on same-origin redirect", func(t *testing.T) {
		sameReq := &http.Request{
			URL:    &url.URL{Scheme: "https", Host: "api.provider-a.com", Path: "/v2/chat"},
			Header: http.Header{"Authorization": {"Bearer secret"}},
		}
		if err := SafeCheckRedirect(sameReq, []*http.Request{originReq}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := sameReq.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("expected same-origin Authorization preserved, got %q", got)
		}
	})
}
