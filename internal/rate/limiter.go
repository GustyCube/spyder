package rate

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type PerHost struct {
	mu         sync.Mutex
	m          map[string]*limitEntry
	perSecond  float64
	burst      int
	maxEntries int
	done       chan struct{}
}

type limitEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func New(perSecond float64, burst int) *PerHost {
	ph := &PerHost{
		m:          make(map[string]*limitEntry),
		perSecond:  perSecond,
		burst:      burst,
		maxEntries: 10000,
		done:       make(chan struct{}),
	}

	go ph.cleanup()
	return ph
}

func (p *PerHost) Close() {
	close(p.done)
}

func (p *PerHost) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.mu.Lock()
			cutoff := time.Now().Add(-1 * time.Hour)
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
