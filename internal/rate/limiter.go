package rate

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// LimiterStats holds per-host rate limiter statistics.
type LimiterStats struct {
	Host      string  `json:"host"`
	Remaining float64 `json:"remaining"`
}

type PerHost struct {
	mu              sync.Mutex
	m               map[string]*limitEntry
	perSecond       float64
	burst           int
	maxEntries      int
	cleanupInterval time.Duration
	entryTTL        time.Duration
	done            chan struct{}
}

type limitEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func New(perSecond float64, burst int, maxEntries int, cleanupInterval, entryTTL time.Duration) *PerHost {
	ph := &PerHost{
		m:               make(map[string]*limitEntry),
		perSecond:       perSecond,
		burst:           burst,
		maxEntries:      maxEntries,
		cleanupInterval: cleanupInterval,
		entryTTL:        entryTTL,
		done:            make(chan struct{}),
	}

	go ph.cleanup()
	return ph
}

func (p *PerHost) Close() {
	close(p.done)
}

func (p *PerHost) cleanup() {
	ticker := time.NewTicker(p.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()
			cutoff := time.Now().Add(-p.entryTTL)
			for host, entry := range p.m {
				if entry.lastUsed.Before(cutoff) {
					delete(p.m, host)
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *PerHost) getOrCreate(host string) *limitEntry {
	entry, ok := p.m[host]
	if !ok {
		entry = &limitEntry{
			limiter:  rate.NewLimiter(rate.Limit(p.perSecond), p.burst),
			lastUsed: time.Now(),
		}
		p.m[host] = entry
	} else {
		entry.lastUsed = time.Now()
	}
	return entry
}

func (p *PerHost) Allow(host string) bool {
	p.mu.Lock()
	entry := p.getOrCreate(host)
	result := entry.limiter.Allow()
	p.mu.Unlock()
	return result
}

func (p *PerHost) Wait(host string) {
	p.mu.Lock()
	entry := p.getOrCreate(host)
	limiter := entry.limiter
	p.mu.Unlock()
	_ = limiter.Wait(context.Background())
}

// Stats returns per-host rate limiter statistics.
func (p *PerHost) Stats() map[string]LimiterStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	stats := make(map[string]LimiterStats, len(p.m))
	for host, entry := range p.m {
		stats[host] = LimiterStats{
			Host:      host,
			Remaining: float64(entry.limiter.Burst()) - entry.limiter.Tokens(),
		}
	}
	return stats
}

// SetRate updates the rate for newly created limiters. Existing limiters keep
// their rate until they expire.
func (p *PerHost) SetRate(perSecond float64, burst int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.perSecond = perSecond
	p.burst = burst
}
