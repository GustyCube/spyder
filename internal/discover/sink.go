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
}

// NoopSink discards all discovered domains (non-recursive mode).
type NoopSink struct{}

func (NoopSink) Submit(context.Context, string) bool { return false }
func (NoopSink) Discovered() int64                   { return 0 }

// ChannelSink feeds discovered domains into a channel for in-memory crawling.
type ChannelSink struct {
	ch        chan<- string
	dedup     dedup.Interface
	maxDomains int64
	count     atomic.Int64
}

// NewChannelSink creates a sink that sends to the given channel.
// maxDomains limits total discoveries (0 = unlimited).
func NewChannelSink(ch chan<- string, d dedup.Interface, maxDomains int64) *ChannelSink {
	return &ChannelSink{ch: ch, dedup: d, maxDomains: maxDomains}
}

func (s *ChannelSink) Submit(ctx context.Context, host string) bool {
	if s.maxDomains > 0 && s.count.Load() >= s.maxDomains {
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

// RedisSink feeds discovered domains into a Redis queue.
type RedisSink struct {
	q          *queue.RedisQueue
	dedup      dedup.Interface
	maxDomains int64
	count      atomic.Int64
}

// NewRedisSink creates a sink that pushes to a Redis queue.
func NewRedisSink(q *queue.RedisQueue, d dedup.Interface, maxDomains int64) *RedisSink {
	return &RedisSink{q: q, dedup: d, maxDomains: maxDomains}
}

func (s *RedisSink) Submit(ctx context.Context, host string) bool {
	if s.maxDomains > 0 && s.count.Load() >= s.maxDomains {
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
