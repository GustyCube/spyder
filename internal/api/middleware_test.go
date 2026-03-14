package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gustycube/spyder/internal/config"
)

func TestAuthMiddleware401(t *testing.T) {
	ks := NewKeyStore([]config.APIKey{{Key: "valid", Scope: "admin"}})
	handler := AuthMiddleware(ks, "read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 401 { t.Errorf("status = %d, want 401", rec.Code) }
}

func TestAuthMiddleware403(t *testing.T) {
	ks := NewKeyStore([]config.APIKey{{Key: "readkey", Scope: "read"}})
	handler := AuthMiddleware(ks, "admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer readkey")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 403 { t.Errorf("status = %d, want 403", rec.Code) }
}

func TestAuthMiddlewareSuccess(t *testing.T) {
	ks := NewKeyStore([]config.APIKey{{Key: "adminkey", Scope: "admin"}})
	handler := AuthMiddleware(ks, "read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer adminkey")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Errorf("status = %d, want 200", rec.Code) }
}

func TestRateLimitMiddleware429(t *testing.T) {
	rl := NewAPIRateLimiter(1, 1)
	handler := RateLimitMiddleware(rl, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	// First request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 { t.Errorf("first request: status = %d", rec.Code) }

	// Second request should be rate limited
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 429 { t.Errorf("second request: status = %d, want 429", rec.Code) }
}
