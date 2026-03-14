package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"go.uber.org/zap"
)

// APIRateLimiter wraps a token bucket rate limiter for the API.
type APIRateLimiter struct {
	mu      sync.RWMutex
	limiter *rate.Limiter
}

func NewAPIRateLimiter(ratePerSec, burst int) *APIRateLimiter {
	return &APIRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(ratePerSec), burst),
	}
}

func (rl *APIRateLimiter) SetRate(ratePerSec, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limiter.SetLimit(rate.Limit(ratePerSec))
	rl.limiter.SetBurst(burst)
}

func (rl *APIRateLimiter) Allow() bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.limiter.Allow()
}

// RateLimitMiddleware returns 429 if rate limit exceeded.
func RateLimitMiddleware(rl *APIRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates API key and checks scope.
func AuthMiddleware(ks *KeyStore, requiredScope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		key, ok := ks.Lookup(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid API key")
			return
		}
		if !ScopeAllows(key.Scope, requiredScope) {
			writeError(w, http.StatusForbidden, "FORBIDDEN", fmt.Sprintf("scope %q insufficient, need %q", key.Scope, requiredScope))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs method, path, status, duration.
func LoggingMiddleware(log *zap.SugaredLogger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Debugw("api request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// writeError sends a JSON error response.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
