package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open
	MaxRequests uint32

	// Interval is the cyclic period of the closed state
	// for the circuit breaker to clear the internal counts
	Interval time.Duration

	// Timeout is the period of the open state,
	// after which the state becomes half-open
	Timeout time.Duration

	// Threshold is the minimum number of requests needed
	// before evaluating the failures
	Threshold uint32

	// FailureRatio is the failure ratio threshold
	// above which the circuit breaker opens
	FailureRatio float64

	// OnStateChange is called whenever the state changes
	OnStateChange func(from, to State)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		MaxRequests:  1,
		Interval:     60 * time.Second,
		Timeout:      60 * time.Second,
		Threshold:    10,
		FailureRatio: 0.5,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *Config
	state  State
	mu     sync.RWMutex

	counts     counts
	expiry     time.Time
	lastOpened time.Time
}

// counts holds the request counts
type counts struct {
	requests uint32
	total    uint32
	failures uint32
}

// New creates a new circuit breaker
func New(config *Config) *CircuitBreaker {
	if config == nil {
		config = DefaultConfig()
	}

	if config.MaxRequests == 0 {
		config.MaxRequests = 1
	}

	if config.Interval == 0 {
		config.Interval = 60 * time.Second
	}

	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Counts returns the current counts
func (cb *CircuitBreaker) Counts() (requests, total, failures uint32) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.counts.requests, cb.counts.total, cb.counts.failures
}

// Execute runs the given function if the circuit breaker allows it
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	err := fn()
	cb.afterRequest(err == nil)
	return err
}

// beforeRequest checks if request is allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	switch state {
	case StateOpen:
		return ErrOpenState
	case StateHalfOpen:
		if cb.counts.requests >= cb.config.MaxRequests {
			return ErrTooManyRequests
		}
	}

	cb.counts.requests++
	cb.setState(state, generation)

	return nil
}

// afterRequest records the result
func (cb *CircuitBreaker) afterRequest(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	switch state {
	case StateClosed:
		cb.onClosed(now, success)
	case StateHalfOpen:
		cb.onHalfOpen(now, success)
	}

	cb.setState(state, generation)
}

// currentState returns the current state
func (cb *CircuitBreaker) currentState(now time.Time) (State, time.Time) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}

	return cb.state, cb.expiry
}

// setState updates the state
func (cb *CircuitBreaker) setState(state State, until time.Time) {
	if cb.state == state {
		cb.expiry = until
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(until)

	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(prev, state)
	}
}

// toNewGeneration resets counts for a new generation
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.counts = counts{}

	switch cb.state {
	case StateClosed:
		cb.expiry = now.Add(cb.config.Interval)
	case StateOpen:
		cb.expiry = now.Add(cb.config.Timeout)
	default:
		cb.expiry = time.Time{}
	}
}

// onClosed handles closed state logic
func (cb *CircuitBreaker) onClosed(now time.Time, success bool) {
	cb.counts.total++
	if !success {
		cb.counts.failures++
	}

	if cb.counts.total >= cb.config.Threshold {
		failureRatio := float64(cb.counts.failures) / float64(cb.counts.total)
		if failureRatio >= cb.config.FailureRatio {
			cb.lastOpened = now
			cb.setState(StateOpen, now)
		}
	}
}

// onHalfOpen handles half-open state logic
func (cb *CircuitBreaker) onHalfOpen(now time.Time, success bool) {
	if success {
		cb.counts.total++
		if cb.counts.total >= cb.config.Threshold {
			cb.setState(StateClosed, now)
		}
	} else {
		cb.setState(StateOpen, now)
	}
}

// Errors
var (
	ErrOpenState       = errors.New("circuit breaker is open")
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// HostBreaker manages circuit breakers per host
type HostBreaker struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
	config   *Config
}

// NewHostBreaker creates a new per-host circuit breaker
func NewHostBreaker(config *Config) *HostBreaker {
	if config == nil {
		config = DefaultConfig()
	}
	return &HostBreaker{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
	}
}

// Execute runs the function with circuit breaker for the given host
func (hb *HostBreaker) Execute(host string, fn func() error) error {
	breaker := hb.getBreaker(host)
	return breaker.Execute(fn)
}

// getBreaker gets or creates a circuit breaker for a host
func (hb *HostBreaker) getBreaker(host string) *CircuitBreaker {
	hb.mu.RLock()
	breaker, exists := hb.breakers[host]
	hb.mu.RUnlock()

	if exists {
		return breaker
	}

	hb.mu.Lock()
	defer hb.mu.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := hb.breakers[host]; exists {
		return breaker
	}

	breaker = New(hb.config)
	hb.breakers[host] = breaker
	return breaker
}

// State returns the state for a specific host
func (hb *HostBreaker) State(host string) State {
	breaker := hb.getBreaker(host)
	return breaker.State()
}

// Stats returns statistics for all hosts
func (hb *HostBreaker) Stats() map[string]struct {
	State    string
	Requests uint32
	Failures uint32
} {
	hb.mu.RLock()
	defer hb.mu.RUnlock()

	stats := make(map[string]struct {
		State    string
		Requests uint32
		Failures uint32
	})

	for host, breaker := range hb.breakers {
		requests, _, failures := breaker.Counts()
		stats[host] = struct {
			State    string
			Requests uint32
			Failures uint32
		}{
			State:    breaker.State().String(),
			Requests: requests,
			Failures: failures,
		}
	}

	return stats
}

// Reset resets the circuit breaker for a specific host
func (hb *HostBreaker) Reset(host string) {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	delete(hb.breakers, host)
}

// ResetAll resets all circuit breakers
func (hb *HostBreaker) ResetAll() {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.breakers = make(map[string]*CircuitBreaker)
}

// ExecuteWithRetry executes with circuit breaker and retry logic
func ExecuteWithRetry(breaker *CircuitBreaker, fn func() error, maxRetries int, backoff time.Duration) error {
	var lastErr error
	
	for i := 0; i <= maxRetries; i++ {
		err := breaker.Execute(fn)
		
		if err == nil {
			return nil
		}
		
		// Don't retry if circuit is open
		if errors.Is(err, ErrOpenState) || errors.Is(err, ErrTooManyRequests) {
			return err
		}
		
		lastErr = err
		
		if i < maxRetries {
			// Exponential backoff with jitter
			wait := backoff * time.Duration(1<<uint(i))
			jitter := time.Duration(float64(wait) * 0.1)
			time.Sleep(wait + jitter)
		}
	}
	
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}