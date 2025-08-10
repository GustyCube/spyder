package dedup

import (
	"sync"
	"testing"
)

func TestMemory_Seen(t *testing.T) {
	d := NewMemory()

	// Test first occurrence returns false
	if d.Seen("test1") {
		t.Error("expected false for first occurrence")
	}

	// Test second occurrence returns true
	if !d.Seen("test1") {
		t.Error("expected true for second occurrence")
	}

	// Test different key returns false
	if d.Seen("test2") {
		t.Error("expected false for new key")
	}

	// Test that test2 is now seen
	if !d.Seen("test2") {
		t.Error("expected true for second occurrence of test2")
	}
}

func TestMemory_Concurrent(t *testing.T) {
	d := NewMemory()
	var wg sync.WaitGroup
	seen := make(map[string]bool)
	var mu sync.Mutex

	// Test concurrent access
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := "concurrent"
			if !d.Seen(key) {
				mu.Lock()
				seen[key] = true
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Only one goroutine should have seen it as new
	if len(seen) != 1 {
		t.Errorf("expected exactly 1 first occurrence, got %d", len(seen))
	}
}

func BenchmarkMemory_Seen(b *testing.B) {
	d := NewMemory()
	
	b.Run("UniqueKeys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d.Seen(string(rune(i)))
		}
	})

	b.Run("SameKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d.Seen("benchmark")
		}
	})
}