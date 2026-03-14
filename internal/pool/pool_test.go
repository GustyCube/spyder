package pool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolScaleUp(t *testing.T) {
	tasks := make(chan string, 100)
	var processed atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := New(ctx, tasks, func(ctx context.Context, host string) {
		processed.Add(1)
	})
	p.Scale(4)

	for i := 0; i < 20; i++ {
		tasks <- "domain.com"
	}
	time.Sleep(200 * time.Millisecond)

	stats := p.Stats()
	if stats.Target != 4 {
		t.Errorf("target = %d", stats.Target)
	}
	if processed.Load() < 10 {
		t.Errorf("processed only %d", processed.Load())
	}
}

func TestPoolPauseResume(t *testing.T) {
	tasks := make(chan string, 100)
	var processed atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := New(ctx, tasks, func(ctx context.Context, host string) {
		processed.Add(1)
		time.Sleep(10 * time.Millisecond)
	})
	p.Scale(2)

	for i := 0; i < 10; i++ {
		tasks <- "domain.com"
	}
	time.Sleep(50 * time.Millisecond)

	p.Pause()
	countAtPause := processed.Load()
	time.Sleep(100 * time.Millisecond)
	countAfterPause := processed.Load()

	if countAfterPause-countAtPause > 2 {
		t.Errorf("processed %d items while paused", countAfterPause-countAtPause)
	}

	// Add fresh tasks so resumed workers have something to process
	for i := 0; i < 10; i++ {
		tasks <- "domain.com"
	}
	p.Resume()
	time.Sleep(200 * time.Millisecond)
	if processed.Load() <= countAfterPause {
		t.Error("didn't resume processing")
	}
}

func TestPoolScaleDown(t *testing.T) {
	tasks := make(chan string, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := New(ctx, tasks, func(ctx context.Context, host string) {
		time.Sleep(50 * time.Millisecond)
	})
	p.Scale(8)
	time.Sleep(50 * time.Millisecond)

	p.Scale(2)
	time.Sleep(200 * time.Millisecond)

	stats := p.Stats()
	if stats.Target != 2 {
		t.Errorf("target = %d", stats.Target)
	}
	if stats.Active > 4 {
		t.Errorf("active = %d, expected <= 4", stats.Active)
	}
}
