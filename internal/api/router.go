package api

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/gustycube/spyder/internal/config"
	"github.com/gustycube/spyder/internal/dedup"
	"github.com/gustycube/spyder/internal/discover"
	"github.com/gustycube/spyder/internal/emit"
	"github.com/gustycube/spyder/internal/health"
	"github.com/gustycube/spyder/internal/httpclient"
	"github.com/gustycube/spyder/internal/hub"
	"github.com/gustycube/spyder/internal/pool"
	ratepkg "github.com/gustycube/spyder/internal/rate"
	"github.com/gustycube/spyder/internal/robots"
)

// Server holds all dependencies for the API layer.
type Server struct {
	RuntimeCfg  *config.RuntimeConfig
	Pool        *pool.Pool
	Hub         *hub.Hub
	Emitter     *emit.Emitter
	Dedup       dedup.Interface
	Sink        discover.Sink
	RateLimiter *ratepkg.PerHost
	HTTPClient  *httpclient.ResilientClient
	RobotsCache *robots.Cache
	KeyStore    *KeyStore
	QueryStore  QueryStore
	Health      *health.Handler
	Log         *zap.SugaredLogger
	Cancel      context.CancelFunc
	StartTime   time.Time
	apiRL       *APIRateLimiter
	TaskCh      chan<- string
}

// NewServer creates a new API server with the given dependencies.
func NewServer(
	rtCfg *config.RuntimeConfig,
	p *pool.Pool,
	h *hub.Hub,
	e *emit.Emitter,
	dd dedup.Interface,
	sink discover.Sink,
	rl *ratepkg.PerHost,
	hc *httpclient.ResilientClient,
	rc *robots.Cache,
	ks *KeyStore,
	qs QueryStore,
	hh *health.Handler,
	log *zap.SugaredLogger,
	cancel context.CancelFunc,
	taskCh chan<- string,
) *Server {
	cfg := rtCfg.Get()
	return &Server{
		RuntimeCfg:  rtCfg,
		Pool:        p,
		Hub:         h,
		Emitter:     e,
		Dedup:       dd,
		Sink:        sink,
		RateLimiter: rl,
		HTTPClient:  hc,
		RobotsCache: rc,
		KeyStore:    ks,
		QueryStore:  qs,
		Health:      hh,
		Log:         log,
		Cancel:      cancel,
		StartTime:   time.Now(),
		apiRL:       NewAPIRateLimiter(cfg.API.RateLimit, cfg.API.RateBurst),
		TaskCh:      taskCh,
	}
}

// SetAPIRate updates the API rate limiter settings.
func (s *Server) SetAPIRate(ratePerSec, burst int) {
	s.apiRL.SetRate(ratePerSec, burst)
}

// RegisterRoutes registers all API routes on the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Middleware helpers
	read := func(h http.HandlerFunc) http.Handler {
		return LoggingMiddleware(s.Log,
			RateLimitMiddleware(s.apiRL,
				AuthMiddleware(s.KeyStore, "read", http.HandlerFunc(h))))
	}
	write := func(h http.HandlerFunc) http.Handler {
		return LoggingMiddleware(s.Log,
			RateLimitMiddleware(s.apiRL,
				AuthMiddleware(s.KeyStore, "write", http.HandlerFunc(h))))
	}
	admin := func(h http.HandlerFunc) http.Handler {
		return LoggingMiddleware(s.Log,
			RateLimitMiddleware(s.apiRL,
				AuthMiddleware(s.KeyStore, "admin", http.HandlerFunc(h))))
	}

	// --- Control ---
	mux.Handle("POST /api/v1/domains", write(s.handleSubmitDomain))
	mux.Handle("POST /api/v1/domains/bulk", write(s.handleSubmitDomainsBulk))
	mux.Handle("POST /api/v1/control/pause", write(s.handlePause))
	mux.Handle("POST /api/v1/control/resume", write(s.handleResume))
	mux.Handle("POST /api/v1/control/stop", admin(s.handleStop))

	// --- Config ---
	mux.Handle("GET /api/v1/config", read(s.handleGetConfig))
	mux.Handle("PATCH /api/v1/config", write(s.handlePatchConfig))

	// --- Stats & Status ---
	mux.Handle("GET /api/v1/stats", read(s.handleGetStats))
	mux.Handle("GET /api/v1/stats/history", read(s.handleGetStatsHistory))
	mux.Handle("GET /api/v1/status", read(s.handleGetStatus))

	// --- Workers ---
	mux.Handle("GET /api/v1/workers", read(s.handleGetWorkers))
	mux.Handle("POST /api/v1/workers/scale", write(s.handleScaleWorkers))

	// --- Circuit Breakers ---
	mux.Handle("GET /api/v1/breakers", read(s.handleGetBreakers))
	mux.Handle("POST /api/v1/breakers/reset", write(s.handleResetAllBreakers))
	mux.Handle("POST /api/v1/breakers/{host}/reset", write(s.handleResetBreaker))

	// --- Rate Limits ---
	mux.Handle("GET /api/v1/ratelimits", read(s.handleGetRateLimits))
	mux.Handle("PUT /api/v1/ratelimits", write(s.handleSetRateLimits))
	mux.Handle("POST /api/v1/ratelimits/{host}/reset", write(s.handleResetRateLimit))

	// --- Dedup ---
	mux.Handle("GET /api/v1/dedup/{key}", read(s.handleCheckDedup))
	mux.Handle("DELETE /api/v1/dedup", admin(s.handleClearDedup))
	mux.Handle("DELETE /api/v1/dedup/{key}", write(s.handleDeleteDedupKey))

	// --- Robots ---
	mux.Handle("GET /api/v1/robots", read(s.handleGetRobots))
	mux.Handle("DELETE /api/v1/robots", write(s.handleClearRobots))

	// --- Spool ---
	mux.Handle("GET /api/v1/spool", read(s.handleGetSpool))
	mux.Handle("POST /api/v1/spool/retry", write(s.handleRetrySpool))
	mux.Handle("DELETE /api/v1/spool", write(s.handleClearSpool))

	// --- Emitter ---
	mux.Handle("GET /api/v1/emitter", read(s.handleGetEmitter))
	mux.Handle("POST /api/v1/emitter/flush", write(s.handleFlushEmitter))

	// --- Excludes ---
	mux.Handle("GET /api/v1/excludes", read(s.handleGetExcludes))
	mux.Handle("PUT /api/v1/excludes", write(s.handleSetExcludes))
	mux.Handle("POST /api/v1/excludes", write(s.handleAddExclude))
	mux.Handle("DELETE /api/v1/excludes/{tld}", write(s.handleRemoveExclude))

	// --- Discovery ---
	mux.Handle("GET /api/v1/discovery", read(s.handleGetDiscovery))
	mux.Handle("PATCH /api/v1/discovery", write(s.handleUpdateDiscovery))

	// --- Queue ---
	mux.Handle("GET /api/v1/queue", read(s.handleGetQueue))
	mux.Handle("DELETE /api/v1/queue", write(s.handleClearQueue))
	mux.Handle("DELETE /api/v1/queue/{host}", write(s.handleRemoveFromQueue))

	// --- Observability ---
	mux.Handle("GET /api/v1/logs/stream", read(s.handleStreamLogs))
	mux.Handle("PUT /api/v1/logs/level", write(s.handleSetLogLevel))
	mux.Handle("GET /api/v1/metrics", read(s.handleGetMetricsJSON))
	mux.Handle("POST /api/v1/metrics/reset", admin(s.handleResetMetrics))
	mux.Handle("GET /api/v1/telemetry", read(s.handleGetTelemetry))
	mux.Handle("PATCH /api/v1/telemetry", write(s.handleUpdateTelemetry))
	mux.Handle("GET /api/v1/health/checkers", read(s.handleGetCheckers))
	mux.Handle("POST /api/v1/health/checkers", write(s.handleAddChecker))
	mux.Handle("DELETE /api/v1/health/checkers/{name}", write(s.handleRemoveChecker))
	mux.Handle("GET /api/v1/output", read(s.handleGetOutput))
	mux.Handle("PATCH /api/v1/output", write(s.handleUpdateOutput))

	// --- Diagnostics ---
	mux.Handle("GET /api/v1/dns/{host}", read(s.handleDNSResolve))
	mux.Handle("GET /api/v1/tls/{host}", read(s.handleTLSFetch))
	mux.Handle("GET /api/v1/httpclient", read(s.handleGetHTTPClient))
	mux.Handle("PATCH /api/v1/httpclient", write(s.handleUpdateHTTPClient))

	// --- API Keys ---
	mux.Handle("GET /api/v1/keys", admin(s.handleListKeys))
	mux.Handle("POST /api/v1/keys", admin(s.handleCreateKey))
	mux.Handle("DELETE /api/v1/keys/{key}", admin(s.handleDeleteKey))

	// --- Query ---
	mux.Handle("GET /api/v1/query/domains", read(s.handleListDomains))
	mux.Handle("GET /api/v1/query/domains/{host}", read(s.handleGetDomain))
	mux.Handle("GET /api/v1/query/edges", read(s.handleListEdges))
	mux.Handle("GET /api/v1/query/ips", read(s.handleListIPs))
	mux.Handle("GET /api/v1/query/certs", read(s.handleListCerts))
}
