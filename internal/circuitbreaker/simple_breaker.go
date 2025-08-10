package circuitbreaker

import (
	"sync"
	"time"
)

// SimpleBreaker is a simpler implementation of circuit breaker
type SimpleBreaker struct {
	mu           sync.RWMutex
	state        State
	failures     uint32
	requests     uint32
	nextAttempt  time.Time
	threshold    uint32
	failureRatio float64
	timeout      time.Duration
	interval     time.Duration
	lastReset    time.Time
}

// NewSimpleBreaker creates a new simple circuit breaker
func NewSimpleBreaker(threshold uint32, failureRatio float64, timeout time.Duration) *SimpleBreaker {
	return &SimpleBreaker{
		state:        StateClosed,
		threshold:    threshold,
		failureRatio: failureRatio,
		timeout:      timeout,
		interval:     60 * time.Second,
		lastReset:    time.Now(),
	}
}

// Execute runs the given function if allowed
func (sb *SimpleBreaker) Execute(fn func() error) error {
	if !sb.allowRequest() {
		return ErrOpenState
	}

	err := fn()
	sb.recordResult(err == nil)
	return err
}

// allowRequest checks if a request should be allowed
func (sb *SimpleBreaker) allowRequest() bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	now := time.Now()

	// Reset counts if interval has passed
	if now.Sub(sb.lastReset) > sb.interval {
		sb.failures = 0
		sb.requests = 0
		sb.lastReset = now
		if sb.state == StateClosed {
			return true
		}
	}

	switch sb.state {
	case StateClosed:
		return true
	case StateOpen:
		if now.After(sb.nextAttempt) {
			sb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}

	return false
}

// recordResult records the result of a request
func (sb *SimpleBreaker) recordResult(success bool) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.requests++
	if !success {
		sb.failures++
	}

	now := time.Now()

	switch sb.state {
	case StateClosed:
		if sb.requests >= sb.threshold {
			failureRate := float64(sb.failures) / float64(sb.requests)
			if failureRate >= sb.failureRatio {
				sb.state = StateOpen
				sb.nextAttempt = now.Add(sb.timeout)
			}
		}
	case StateHalfOpen:
		if success {
			sb.state = StateClosed
			sb.failures = 0
			sb.requests = 0
			sb.lastReset = now
		} else {
			sb.state = StateOpen
			sb.nextAttempt = now.Add(sb.timeout)
		}
	}
}

// State returns the current state
func (sb *SimpleBreaker) State() State {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.state
}

// Counts returns current failure/request counts
func (sb *SimpleBreaker) Counts() (requests, failures uint32) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return sb.requests, sb.failures
}