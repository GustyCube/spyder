package discover

import (
	"context"
	"sync/atomic"

	"github.com/gustycube/spyder/internal/dedup"
	"github.com/gustycube/spyder/internal/queue"
)

// Sink receives discovered domains and feeds them back for crawling.
type Sink interface {
	// Submit sends a discovered domain for future crawling.
	// Returns true if the domain was new and enqueued.
	Submit(ctx context.Context, host string) bool
	// Discovered returns the total number of domains submitted.
	Discovered() int64
	// SetMaxDomains updates the maximum number of domains to discover.
	SetMaxDomains(n int64)
}

// NoopSink discards all discovered domains (non-recursive mode).
type NoopSink struct{}

func (NoopSink) Submit(context.Context, string) bool { return false }
func (NoopSink) Discovered() int64                   { return 0 }
func (NoopSink) SetMaxDomains(int64)                 {}

// ChannelSink feeds discovered domains into a channel for in-memory crawling.
type ChannelSink struct {
	ch        chan<- string
	dedup     dedup.Interface
	maxDomains atomic.Int64
	count     atomic.Int64
}

// NewChannelSink creates a sink that sends to the given channel.
// maxDomains limits total discoveries (0 = unlimited).
func NewChannelSink(ch chan<- string, d dedup.Interface, maxDomains int64) *ChannelSink {
	s := &ChannelSink{ch: ch, dedup: d}
	s.maxDomains.Store(maxDomains)
	return s
}

func (s *ChannelSink) Submit(ctx context.Context, host string) bool {
	max := s.maxDomains.Load()
	if max > 0 && s.count.Load() >= max {
		return false
	}
	if s.dedup.Seen("discovered|" + host) {
		return false
	}
	s.count.Add(1)
	select {
	case s.ch <- host:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *ChannelSink) Discovered() int64 {
	return s.count.Load()
}

func (s *ChannelSink) SetMaxDomains(n int64) {
	s.maxDomains.Store(n)
}

// RedisSink feeds discovered domains into a Redis queue.
type RedisSink struct {
	q          *queue.RedisQueue
	dedup      dedup.Interface
	maxDomains atomic.Int64
	count      atomic.Int64
}

// NewRedisSink creates a sink that pushes to a Redis queue.
func NewRedisSink(q *queue.RedisQueue, d dedup.Interface, maxDomains int64) *RedisSink {
	s := &RedisSink{q: q, dedup: d}
	s.maxDomains.Store(maxDomains)
	return s
}

func (s *RedisSink) Submit(ctx context.Context, host string) bool {
	max := s.maxDomains.Load()
	if max > 0 && s.count.Load() >= max {
		return false
	}
	if s.dedup.Seen("discovered|" + host) {
		return false
	}
	if err := s.q.Seed(ctx, host); err != nil {
		return false
	}
	s.count.Add(1)
	return true
}

func (s *RedisSink) Discovered() int64 {
	return s.count.Load()
}

func (s *RedisSink) SetMaxDomains(n int64) {
	s.maxDomains.Store(n)
}
