package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := New(&Config{
		Threshold:    3,
		FailureRatio: 0.6,
		Timeout:      time.Second,
		Interval:     time.Minute,
	})

	// Circuit should start closed
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed, got %v", cb.State())
	}

	// Successful requests should keep it closed
	for i := 0; i < 5; i++ {
		err := cb.Execute(func() error { return nil })
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpensOnFailures(t *testing.T) {
	cb := NewSimpleBreaker(3, 0.6, 100*time.Millisecond)

	testErr := errors.New("test error")

	// First two failures
	cb.Execute(func() error { return testErr })
	cb.Execute(func() error { return testErr })
	
	// Should still be closed (below threshold)
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed below threshold, got %v", cb.State())
	}

	// Third failure should open the circuit (2/3 = 0.66 > 0.6)
	cb.Execute(func() error { return testErr })
	
	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after failures, got %v", cb.State())
	}

	// Should reject requests when open
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrOpenState) {
		t.Errorf("expected ErrOpenState, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	cb := New(&Config{
		Threshold:    2,
		FailureRatio: 0.5,
		Timeout:      50 * time.Millisecond,
		Interval:     time.Minute,
		MaxRequests:  2,
	})

	testErr := errors.New("test error")

	// Open the circuit
	cb.Execute(func() error { return testErr })
	cb.Execute(func() error { return testErr })

	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen, got %v", cb.State())
	}

	// Wait for timeout to transition to half-open
	time.Sleep(60 * time.Millisecond)

	// Should allow limited requests in half-open
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("unexpected error in half-open: %v", err)
	}

	// Should still be half-open
	if cb.State() != StateHalfOpen {
		t.Errorf("expected StateHalfOpen, got %v", cb.State())
	}

	// Another success should close it
	cb.Execute(func() error { return nil })
	
	if cb.State() != StateClosed {
		t.Errorf("expected StateClosed after recovery, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cb := New(&Config{
		Threshold:    2,
		FailureRatio: 0.5,
		Timeout:      50 * time.Millisecond,
		Interval:     time.Minute,
	})

	testErr := errors.New("test error")

	// Open the circuit
	cb.Execute(func() error { return testErr })
	cb.Execute(func() error { return testErr })

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Failure in half-open should reopen
	cb.Execute(func() error { return testErr })
	
	if cb.State() != StateOpen {
		t.Errorf("expected StateOpen after half-open failure, got %v", cb.State())
	}
}

func TestHostBreaker(t *testing.T) {
	hb := NewHostBreaker(&Config{
		Threshold:    2,
		FailureRatio: 0.5,
		Timeout:      50 * time.Millisecond,
	})

	testErr := errors.New("test error")

	// Different hosts should have independent breakers
	hb.Execute("host1", func() error { return testErr })
	hb.Execute("host1", func() error { return testErr })
	hb.Execute("host2", func() error { return nil })

	if hb.State("host1") != StateOpen {
		t.Errorf("expected host1 to be open")
	}

	if hb.State("host2") != StateClosed {
		t.Errorf("expected host2 to be closed")
	}

	// Stats should show both hosts
	stats := hb.Stats()
	if len(stats) != 2 {
		t.Errorf("expected 2 hosts in stats, got %d", len(stats))
	}

	// Reset should clear the breaker
	hb.Reset("host1")
	
	err := hb.Execute("host1", func() error { return nil })
	if err != nil {
		t.Errorf("unexpected error after reset: %v", err)
	}
}

func TestExecuteWithRetry(t *testing.T) {
	cb := New(&Config{
		Threshold:    10,
		FailureRatio: 0.5,
	})

	attempts := 0
	err := ExecuteWithRetry(cb, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, 5, 10*time.Millisecond)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestExecuteWithRetry_CircuitOpen(t *testing.T) {
	cb := New(&Config{
		Threshold:    2,
		FailureRatio: 0.5,
	})

	testErr := errors.New("test error")

	// Open the circuit
	cb.Execute(func() error { return testErr })
	cb.Execute(func() error { return testErr })

	// Should not retry when circuit is open
	attempts := 0
	err := ExecuteWithRetry(cb, func() error {
		attempts++
		return nil
	}, 5, 10*time.Millisecond)

	if !errors.Is(err, ErrOpenState) {
		t.Errorf("expected ErrOpenState, got %v", err)
	}

	if attempts != 0 {
		t.Errorf("expected 0 attempts when circuit open, got %d", attempts)
	}
}