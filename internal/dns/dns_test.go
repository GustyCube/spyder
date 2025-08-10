package dns

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestResolveAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test with a well-known domain
	ips, nsHosts, cname, mxHosts, _ := ResolveAll(ctx, "google.com")

	// Google.com should have IP addresses
	if len(ips) == 0 {
		t.Error("expected at least one IP address for google.com")
	}

	// Should have NS records
	if len(nsHosts) == 0 {
		t.Error("expected at least one NS record for google.com")
	}

	// Check that trailing dots are removed from NS hosts
	for _, ns := range nsHosts {
		if strings.HasSuffix(ns, ".") {
			t.Errorf("NS host should not have trailing dot: %s", ns)
		}
	}

	// Check CNAME doesn't have trailing dot
	if strings.HasSuffix(cname, ".") {
		t.Error("CNAME should not have trailing dot")
	}

	// MX hosts shouldn't have trailing dots
	for _, mx := range mxHosts {
		if strings.HasSuffix(mx, ".") {
			t.Errorf("MX host should not have trailing dot: %s", mx)
		}
	}
}

func TestResolveAll_InvalidDomain(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Test with invalid domain
	ips, nsHosts, _, mxHosts, txts := ResolveAll(ctx, "this-domain-definitely-does-not-exist-123456789.com")

	// Should return empty results, not panic
	if len(ips) != 0 {
		t.Errorf("expected no IPs for invalid domain, got %v", ips)
	}
	if len(nsHosts) != 0 {
		t.Errorf("expected no NS records for invalid domain, got %v", nsHosts)
	}
	if len(mxHosts) != 0 {
		t.Errorf("expected no MX records for invalid domain, got %v", mxHosts)
	}
	if len(txts) != 0 {
		t.Errorf("expected no TXT records for invalid domain, got %v", txts)
	}
}

func TestResolveAll_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should handle cancelled context gracefully
	ips, _, _, _, _ := ResolveAll(ctx, "google.com")
	
	// With cancelled context, lookups should fail and return empty
	if len(ips) != 0 {
		t.Error("expected no results with cancelled context")
	}
}

func BenchmarkResolveAll(b *testing.B) {
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ResolveAll(ctx, "example.com")
	}
}