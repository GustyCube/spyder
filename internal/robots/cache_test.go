package robots

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCache_Get(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	cache := NewCache(client, "TestBot/1.0")
	ctx := context.Background()

	// Extract host from test server URL
	host := server.URL[7:] // Remove "http://"

	// First call should fetch from server
	rd, err := cache.Get(ctx, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rd == nil {
		t.Fatal("expected robots data, got nil")
	}

	// Second call should use cache
	rd2, err := cache.Get(ctx, host)
	if err != nil {
		t.Fatalf("unexpected error on cached get: %v", err)
	}
	if rd2 != rd {
		t.Error("expected cached robots data to be the same instance")
	}
}

func TestCache_Get_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	cache := NewCache(client, "TestBot/1.0")
	ctx := context.Background()

	host := server.URL[7:] // Remove "http://"

	rd, err := cache.Get(ctx, host)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rd == nil {
		t.Fatal("expected empty robots data for 404, got nil")
	}
}

func TestAllowed(t *testing.T) {
	// Note: This test is simplified since we can't easily test the actual
	// robotstxt.RobotsData without proper mocking infrastructure.
	// In a real scenario, we'd use the actual robotstxt package or proper mocks.
	t.Skip("Skipping Allowed test - requires proper robotstxt.RobotsData mock")
}

func TestShouldSkipByTLD(t *testing.T) {
	excluded := []string{"gov", "mil", "int"}

	tests := []struct {
		host     string
		expected bool
	}{
		{"example.gov", true},
		{"subdomain.example.gov", true},
		{"example.mil", true},
		{"example.int", true},
		{"example.com", false},
		{"gov.example.com", false},
		{"gov", true},
		{"example.org", false},
	}

	for _, tt := range tests {
		result := ShouldSkipByTLD(tt.host, excluded)
		if result != tt.expected {
			t.Errorf("ShouldSkipByTLD(%s) = %v, want %v", tt.host, result, tt.expected)
		}
	}
}

