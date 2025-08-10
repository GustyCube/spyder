package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gustycube/spyder-probe/internal/logging"
)

// Status represents the health status of a component
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a health check for a component
type Check struct {
	Name        string        `json:"name"`
	Status      Status        `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Duration    time.Duration `json:"duration_ms"`
}

// Response represents the overall health response
type Response struct {
	Status    Status            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    []Check           `json:"checks"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Checker defines the interface for health checks
type Checker interface {
	Check(ctx context.Context) Check
}

// Handler manages health and readiness checks
type Handler struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	metadata map[string]string
	logger   *logging.Logger
	ready    bool
}

// NewHandler creates a new health handler
func NewHandler(logger *logging.Logger) *Handler {
	return &Handler{
		checkers: make(map[string]Checker),
		metadata: make(map[string]string),
		logger:   logger,
		ready:    false,
	}
}

// RegisterChecker adds a health checker
func (h *Handler) RegisterChecker(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// SetMetadata sets metadata for the health response
func (h *Handler) SetMetadata(key, value string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.metadata[key] = value
}

// SetReady marks the service as ready
func (h *Handler) SetReady(ready bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ready = ready
}

// IsReady returns the readiness status
func (h *Handler) IsReady() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.ready
}

// HealthHandler handles health check requests
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	metadata := make(map[string]string, len(h.metadata))
	for k, v := range h.metadata {
		metadata[k] = v
	}
	h.mu.RUnlock()

	response := Response{
		Timestamp: time.Now(),
		Checks:    []Check{},
		Metadata:  metadata,
	}

	overallStatus := StatusHealthy
	
	// Run all health checks
	for name, checker := range checkers {
		check := checker.Check(ctx)
		check.Name = name
		response.Checks = append(response.Checks, check)

		// Update overall status
		if check.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if check.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	response.Status = overallStatus

	// Set appropriate HTTP status code
	statusCode := http.StatusOK
	if overallStatus == StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if overallStatus == StatusDegraded {
		statusCode = http.StatusOK // Still return 200 for degraded
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// ReadinessHandler handles readiness check requests
func (h *Handler) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	ready := h.ready
	metadata := make(map[string]string, len(h.metadata))
	for k, v := range h.metadata {
		metadata[k] = v
	}
	h.mu.RUnlock()

	response := map[string]interface{}{
		"ready":     ready,
		"timestamp": time.Now(),
		"metadata":  metadata,
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// LivenessHandler handles liveness check requests (always returns OK if service is running)
func (h *Handler) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"alive":     true,
		"timestamp": time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// RedisChecker checks Redis connectivity
type RedisChecker struct {
	addr string
	checkFunc func() error
}

// NewRedisChecker creates a new Redis health checker
func NewRedisChecker(addr string, checkFunc func() error) *RedisChecker {
	return &RedisChecker{
		addr: addr,
		checkFunc: checkFunc,
	}
}

// Check performs the Redis health check
func (c *RedisChecker) Check(ctx context.Context) Check {
	start := time.Now()
	
	if c.checkFunc == nil {
		return Check{
			Status:      StatusHealthy,
			Message:     "Redis not configured",
			LastChecked: time.Now(),
			Duration:    time.Since(start) / time.Millisecond,
		}
	}

	err := c.checkFunc()
	duration := time.Since(start)

	if err != nil {
		return Check{
			Status:      StatusUnhealthy,
			Message:     "Redis connection failed: " + err.Error(),
			LastChecked: time.Now(),
			Duration:    duration / time.Millisecond,
		}
	}

	return Check{
		Status:      StatusHealthy,
		Message:     "Redis connection OK",
		LastChecked: time.Now(),
		Duration:    duration / time.Millisecond,
	}
}

// WorkerPoolChecker checks worker pool status
type WorkerPoolChecker struct {
	getActiveWorkers func() int
	maxWorkers       int
}

// NewWorkerPoolChecker creates a new worker pool health checker
func NewWorkerPoolChecker(getActiveWorkers func() int, maxWorkers int) *WorkerPoolChecker {
	return &WorkerPoolChecker{
		getActiveWorkers: getActiveWorkers,
		maxWorkers:       maxWorkers,
	}
}

// Check performs the worker pool health check
func (c *WorkerPoolChecker) Check(ctx context.Context) Check {
	start := time.Now()
	activeWorkers := c.getActiveWorkers()
	
	status := StatusHealthy
	message := "Worker pool operating normally"
	
	utilizationPct := float64(activeWorkers) / float64(c.maxWorkers) * 100
	
	if utilizationPct > 90 {
		status = StatusDegraded
		message = "Worker pool near capacity"
	} else if activeWorkers == 0 {
		status = StatusDegraded
		message = "No active workers"
	}

	return Check{
		Status:      status,
		Message:     message,
		LastChecked: time.Now(),
		Duration:    time.Since(start) / time.Millisecond,
	}
}