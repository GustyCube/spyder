package rate

import (
	"sync"
	"testing"
	"time"
)

func TestPerHost_Allow(t *testing.T) {
	limiter := New(10.0, 5, 10000, 5*time.Minute, 1*time.Hour) // 10 per second, burst of 5

	// Test burst allowance
	for i := 0; i < 5; i++ {
		if !limiter.Allow("host1") {
			t.Errorf("expected Allow to return true for burst request %d", i+1)
		}
	}

	// Next request should be rate limited
	if limiter.Allow("host1") {
		t.Error("expected Allow to return false after burst exhausted")
	}

	// Different host should have its own limit
	if !limiter.Allow("host2") {
		t.Error("expected Allow to return true for different host")
	}
}

func TestPerHost_Wait(t *testing.T) {
	limiter := New(100.0, 1, 10000, 5*time.Minute, 1*time.Hour) // 100 per second, burst of 1

	start := time.Now()
	limiter.Wait("host1")
	limiter.Wait("host1")
	duration := time.Since(start)

	// Second wait should have delayed approximately 10ms (1/100 second)
	if duration < 5*time.Millisecond {
		t.Errorf("expected Wait to delay, got %v", duration)
	}
}

func TestPerHost_Concurrent(t *testing.T) {
	limiter := New(1000.0, 10, 10000, 5*time.Minute, 1*time.Hour)
	var wg sync.WaitGroup
	allowed := 0
	var mu sync.Mutex

	// Test concurrent access for same host
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("concurrent-host") {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Should allow around burst size initially
	if allowed == 0 {
		t.Error("expected some requests to be allowed")
	}
	if allowed > 15 { // Some tolerance for timing
		t.Errorf("expected rate limiting to apply, but %d requests were allowed", allowed)
	}
}

func TestPerHost_MultipleHosts(t *testing.T) {
	limiter := New(10.0, 2, 10000, 5*time.Minute, 1*time.Hour)
	hosts := []string{"host1", "host2", "host3"}

	// Each host should get its own burst allowance
	for _, host := range hosts {
		allowed := 0
		for i := 0; i < 5; i++ {
			if limiter.Allow(host) {
				allowed++
			}
		}
		if allowed != 2 {
			t.Errorf("expected 2 requests allowed for %s, got %d", host, allowed)
		}
	}
}

func TestPerHost_SetRate(t *testing.T) {
	limiter := New(10.0, 2, 10000, 5*time.Minute, 1*time.Hour)

	// Use burst for host1
	limiter.Allow("host1")
	limiter.Allow("host1")
	if limiter.Allow("host1") {
		t.Error("expected Allow to return false after burst exhausted")
	}

	// Change rate and burst
	limiter.SetRate(10.0, 5)

	// New host should get new burst
	for i := 0; i < 5; i++ {
		if !limiter.Allow("host2") {
			t.Errorf("expected Allow to return true for new rate burst request %d", i+1)
		}
	}
}

func TestPerHost_Stats(t *testing.T) {
	limiter := New(10.0, 5, 10000, 5*time.Minute, 1*time.Hour)
	limiter.Allow("host1")
	limiter.Allow("host2")

	stats := limiter.Stats()
	if len(stats) != 2 {
		t.Errorf("expected 2 hosts in stats, got %d", len(stats))
	}
	if _, ok := stats["host1"]; !ok {
		t.Error("expected host1 in stats")
	}
	if _, ok := stats["host2"]; !ok {
		t.Error("expected host2 in stats")
	}
}

func BenchmarkPerHost_Allow(b *testing.B) {
	limiter := New(1000000.0, 1000000, 10000, 5*time.Minute, 1*time.Hour) // High limits to avoid blocking

	b.Run("SingleHost", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			limiter.Allow("benchmark-host")
		}
	})

	b.Run("MultipleHosts", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			limiter.Allow(string(rune(i % 100)))
		}
	})
}
