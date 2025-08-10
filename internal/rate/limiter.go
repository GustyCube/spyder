package rate

import (
	"context"
	"sync"
	"time"
	"golang.org/x/time/rate"
)

type PerHost struct {
	mu sync.Mutex
	m  map[string]*limitEntry
	perSecond float64
	burst int
	maxEntries int
}

type limitEntry struct {
	limiter *rate.Limiter
	lastUsed time.Time
}

func New(perSecond float64, burst int) *PerHost {
	ph := &PerHost{
		m: make(map[string]*limitEntry), 
		perSecond: perSecond, 
		burst: burst,
		maxEntries: 10000, // Prevent unlimited growth
	}
	
	// Start cleanup goroutine
	go ph.cleanup()
	return ph
}

func (p *PerHost) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		p.mu.Lock()
		if len(p.m) > p.maxEntries {
			// Remove entries older than 1 hour
			cutoff := time.Now().Add(-1 * time.Hour)
			for host, entry := range p.m {
				if entry.lastUsed.Before(cutoff) {
					delete(p.m, host)
				}
			}
		}
		p.mu.Unlock()
	}
}

func (p *PerHost) Allow(host string) bool {
	p.mu.Lock()
	entry, ok := p.m[host]
	if !ok { 
		entry = &limitEntry{
			limiter: rate.NewLimiter(rate.Limit(p.perSecond), p.burst),
			lastUsed: time.Now(),
		}
		p.m[host] = entry
	} else {
		entry.lastUsed = time.Now()
	}
	p.mu.Unlock()
	return entry.limiter.Allow()
}

func (p *PerHost) Wait(host string) {
	p.mu.Lock()
	entry, ok := p.m[host]
	if !ok { 
		entry = &limitEntry{
			limiter: rate.NewLimiter(rate.Limit(p.perSecond), p.burst),
			lastUsed: time.Now(),
		}
		p.m[host] = entry
	} else {
		entry.lastUsed = time.Now()
	}
	p.mu.Unlock()
	_ = entry.limiter.Wait(context.Background())
}
