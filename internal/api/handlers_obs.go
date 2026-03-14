package api

import (
	"net/http"

	"github.com/gustycube/spyder/internal/config"
	"github.com/gustycube/spyder/internal/dns"
	"github.com/gustycube/spyder/internal/tlsinfo"
)

// --- Log streaming ---

func (s *Server) handleStreamLogs(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "log streaming not yet implemented")
}

func (s *Server) handleSetLogLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Level string `json:"level"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if req.Level == "" {
		writeError(w, http.StatusBadRequest, "MISSING_LEVEL", "level is required")
		return
	}

	patch := config.ConfigPatch{
		Logging: &config.LoggingPatch{
			Level: &req.Level,
		},
	}
	_, _ = s.RuntimeCfg.Apply(patch)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"level": req.Level,
	})
}

// --- Metrics ---

func (s *Server) handleGetMetricsJSON(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "use /metrics endpoint for Prometheus format",
		"metrics_endpoint": s.RuntimeCfg.Get().Telemetry.MetricsAddr + "/metrics",
	})
}

func (s *Server) handleResetMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "metrics reset is not supported; restart the process to reset counters",
	})
}

// --- Telemetry ---

func (s *Server) handleGetTelemetry(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, cfg.Telemetry)
}

func (s *Server) handleUpdateTelemetry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OTELEndpoint *string `json:"otel_endpoint"`
		OTELInsecure *bool   `json:"otel_insecure"`
		OTELService  *string `json:"otel_service"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	// Telemetry changes (except metrics_addr) require restart, but endpoint/insecure/service
	// are informational. We store them but warn.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "telemetry configuration changes require restart to take effect",
		"telemetry": s.RuntimeCfg.Get().Telemetry,
	})
}

// --- Health checkers ---

func (s *Server) handleGetCheckers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "health checkers are registered programmatically; use GET /health for status",
	})
}

func (s *Server) handleAddChecker(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "dynamic checker registration not supported via API")
}

func (s *Server) handleRemoveChecker(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "dynamic checker removal not supported via API")
}

// --- Output ---

func (s *Server) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, cfg.Output)
}

func (s *Server) handleUpdateOutput(w http.ResponseWriter, r *http.Request) {
	var patch config.OutputPatch
	if err := readJSON(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	cfgPatch := config.ConfigPatch{Output: &patch}
	warnings, err := s.RuntimeCfg.Apply(cfgPatch)
	if err != nil {
		writeError(w, http.StatusConflict, "APPLY_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"output":   s.RuntimeCfg.Get().Output,
		"warnings": warnings,
	})
}

// --- DNS resolve ---

func (s *Server) handleDNSResolve(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if host == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HOST", "host is required")
		return
	}

	ips, ns, cname, mx, txts := dns.ResolveAll(r.Context(), host)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"host":  host,
		"ips":   ips,
		"ns":    ns,
		"cname": cname,
		"mx":    mx,
		"txt":   txts,
	})
}

// --- TLS fetch ---

func (s *Server) handleTLSFetch(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if host == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HOST", "host is required")
		return
	}

	cert, err := tlsinfo.FetchCert(host)
	if err != nil {
		writeError(w, http.StatusBadGateway, "TLS_ERROR", err.Error())
		return
	}
	if cert == nil {
		writeError(w, http.StatusNotFound, "NO_CERT", "no certificate found")
		return
	}

	writeJSON(w, http.StatusOK, cert)
}

// --- HTTP Client ---

func (s *Server) handleGetHTTPClient(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, cfg.HTTPClient)
}

func (s *Server) handleUpdateHTTPClient(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "HTTP client config changes require restart")
}
