package pool

import (
	"context"
	"sync"
	"sync/atomic"
)

type WorkFunc func(ctx context.Context, host string)

type WorkerStats struct {
	Target int32 `json:"target"`
	Active int32 `json:"active"`
	Paused bool  `json:"paused"`
}

type Pool struct {
	tasks  <-chan string
	ctx    context.Context
	work   WorkFunc
	target atomic.Int32
	active atomic.Int32
	paused atomic.Bool
	mu     sync.Mutex
	cond   *sync.Cond
	shrink chan struct{}
	wg     sync.WaitGroup
}

func New(ctx context.Context, tasks <-chan string, work WorkFunc) *Pool {
	p := &Pool{
		tasks:  tasks,
		ctx:    ctx,
		work:   work,
		shrink: make(chan struct{}, 1024),
	}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *Pool) Scale(n int) {
	old := p.target.Swap(int32(n))
	diff := int32(n) - old
	if diff > 0 {
		for i := int32(0); i < diff; i++ {
			p.wg.Add(1)
			go p.worker()
		}
	} else {
		for i := int32(0); i < -diff; i++ {
			select {
			case p.shrink <- struct{}{}:
			default:
			}
		}
	}
}

func (p *Pool) Pause() {
	p.paused.Store(true)
}

func (p *Pool) Resume() {
	p.paused.Store(false)
	p.cond.Broadcast()
}

func (p *Pool) Stats() WorkerStats {
	return WorkerStats{
		Target: p.target.Load(),
		Active: p.active.Load(),
		Paused: p.paused.Load(),
	}
}

func (p *Pool) Wait() {
	p.wg.Wait()
}

func (p *Pool) worker() {
	defer p.wg.Done()
	p.active.Add(1)
	defer p.active.Add(-1)

	for {
		// Check pause
		p.mu.Lock()
		for p.paused.Load() {
			p.cond.Wait()
			if p.ctx.Err() != nil {
				p.mu.Unlock()
				return
			}
		}
		p.mu.Unlock()

		// Check shrink
		select {
		case <-p.shrink:
			return
		default:
		}

		// Pull task
		select {
		case host, ok := <-p.tasks:
			if !ok {
				return
			}
			p.work(p.ctx, host)
		case <-p.shrink:
			return
		case <-p.ctx.Done():
			return
		}
	}
}
