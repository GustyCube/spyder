package rate

import (
	"context"
	"sync"
	"time"
	"golang.org/x/time/rate"
)

type PerHost struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter
	perSecond float64
	burst int
}

func New(perSecond float64, burst int) *PerHost {
	return &PerHost{m: map[string]*rate.Limiter{}, perSecond: perSecond, burst: burst}
}

func (p *PerHost) Allow(host string) bool {
	p.mu.Lock()
	lim, ok := p.m[host]
	if !ok { lim = rate.NewLimiter(rate.Limit(p.perSecond), p.burst); p.m[host] = lim }
	p.mu.Unlock()
	return lim.Allow()
}

func (p *PerHost) Wait(host string) {
	p.mu.Lock()
	lim, ok := p.m[host]
	if !ok { lim = rate.NewLimiter(rate.Limit(p.perSecond), p.burst); p.m[host] = lim }
	p.mu.Unlock()
	_ = lim.Wait(context.Background())
	time.Sleep(0)
}
