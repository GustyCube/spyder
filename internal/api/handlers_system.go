package api

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gustycube/spyder/internal/config"
	"github.com/gustycube/spyder/internal/hub"
)

// --- Stats ---

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Hub.GetStats())
}

func (s *Server) handleGetStatsHistory(w http.ResponseWriter, r *http.Request) {
	history := s.Hub.GetHistory()
	if len(history) == 0 {
		writeJSON(w, http.StatusOK, []hub.BatchEvent{})
		return
	}
	writeJSON(w, http.StatusOK, history)
}

// --- Status ---

func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	ws := s.Pool.Stats()
	edges, nodes := s.Emitter.PendingStats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"uptime_seconds": time.Since(s.StartTime).Seconds(),
		"workers":        ws,
		"pending_edges":  edges,
		"pending_nodes":  nodes,
		"discovered":     s.Sink.Discovered(),
	})
}

// --- Workers ---

func (s *Server) handleGetWorkers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Pool.Stats())
}

func (s *Server) handleScaleWorkers(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count int `json:"count"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if req.Count < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_COUNT", "count must be non-negative")
		return
	}
	s.Pool.Scale(req.Count)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"scaled_to": req.Count,
		"workers":   s.Pool.Stats(),
	})
}

// --- Circuit Breakers ---

func (s *Server) handleGetBreakers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.HTTPClient.Stats())
}

func (s *Server) handleResetBreaker(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if host == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HOST", "host is required")
		return
	}
	s.HTTPClient.ResetBreaker(host)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reset": host,
	})
}

func (s *Server) handleResetAllBreakers(w http.ResponseWriter, r *http.Request) {
	// Reset all known breakers by iterating stats
	stats := s.HTTPClient.Stats()
	for host := range stats {
		s.HTTPClient.ResetBreaker(host)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reset": "all",
		"count": len(stats),
	})
}

// --- Rate Limits ---

func (s *Server) handleGetRateLimits(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"per_host_rate":  cfg.Crawling.RatePerHost,
		"per_host_burst": cfg.Crawling.RateBurst,
		"hosts":          s.RateLimiter.Stats(),
	})
}

func (s *Server) handleSetRateLimits(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RatePerHost *float64 `json:"rate_per_host"`
		RateBurst   *int     `json:"rate_burst"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	cfg := s.RuntimeCfg.Get()
	rateVal := cfg.Crawling.RatePerHost
	burstVal := cfg.Crawling.RateBurst
	if req.RatePerHost != nil {
		rateVal = *req.RatePerHost
	}
	if req.RateBurst != nil {
		burstVal = *req.RateBurst
	}

	s.RateLimiter.SetRate(rateVal, burstVal)

	// Also update runtime config
	patch := config.ConfigPatch{
		Crawling: &config.CrawlingPatch{
			RatePerHost: req.RatePerHost,
			RateBurst:   req.RateBurst,
		},
	}
	_, _ = s.RuntimeCfg.Apply(patch)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rate_per_host": rateVal,
		"rate_burst":    burstVal,
	})
}

func (s *Server) handleResetRateLimit(w http.ResponseWriter, r *http.Request) {
	host := r.PathValue("host")
	if host == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HOST", "host is required")
		return
	}
	// PerHost doesn't have a per-host reset; return a helpful message
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "per-host rate limiter entries expire automatically via TTL",
		"host":    host,
	})
}

// --- Dedup ---

func (s *Server) handleCheckDedup(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "MISSING_KEY", "key is required")
		return
	}
	seen := s.Dedup.Seen(key)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"key":  key,
		"seen": seen,
	})
}

type dedupClearer interface {
	Clear()
}

type dedupDeleter interface {
	Delete(key string)
}

func (s *Server) handleClearDedup(w http.ResponseWriter, r *http.Request) {
	if c, ok := s.Dedup.(dedupClearer); ok {
		c.Clear()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cleared": true,
		})
		return
	}
	writeError(w, http.StatusNotImplemented, "NOT_SUPPORTED", "clear not supported with current dedup backend")
}

func (s *Server) handleDeleteDedupKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if d, ok := s.Dedup.(dedupDeleter); ok {
		d.Delete(key)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"deleted": key,
		})
		return
	}
	writeError(w, http.StatusNotImplemented, "NOT_SUPPORTED", "delete not supported with current dedup backend")
}

// --- Robots ---

func (s *Server) handleGetRobots(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "robots.txt cache is an LRU; individual entries cannot be listed",
		"status":  "active",
	})
}

func (s *Server) handleClearRobots(w http.ResponseWriter, r *http.Request) {
	// LRU cache doesn't expose a Purge — rebuild would require access to internals
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "robots cache clear is not supported; entries expire via TTL",
	})
}

// --- Spool ---

func (s *Server) handleGetSpool(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	dir := cfg.Output.SpoolDir

	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"spool_dir": dir,
			"files":     []string{},
			"count":     0,
		})
		return
	}

	files := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		f := map[string]interface{}{
			"name": e.Name(),
		}
		if info != nil {
			f["size"] = info.Size()
			f["modified"] = info.ModTime()
		}
		files = append(files, f)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"spool_dir": dir,
		"files":     files,
		"count":     len(files),
	})
}

func (s *Server) handleRetrySpool(w http.ResponseWriter, r *http.Request) {
	s.Emitter.Drain(s.Log)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "spool retry initiated",
	})
}

func (s *Server) handleClearSpool(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	dir := cfg.Output.SpoolDir

	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"cleared": 0,
		})
		return
	}

	cleared := 0
	for _, e := range entries {
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			cleared++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cleared": cleared,
	})
}

// --- Emitter ---

func (s *Server) handleGetEmitter(w http.ResponseWriter, r *http.Request) {
	edges, nodes := s.Emitter.PendingStats()
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pending_edges": edges,
		"pending_nodes": nodes,
		"ingest":        cfg.Output.Ingest,
		"spool_dir":     cfg.Output.SpoolDir,
	})
}

func (s *Server) handleFlushEmitter(w http.ResponseWriter, r *http.Request) {
	s.Emitter.ForceFlush(r.Context(), s.Log)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"flushed": true,
	})
}

// --- Excludes ---

func (s *Server) handleGetExcludes(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"exclude_tlds": cfg.ExcludeTLDs,
	})
}

func (s *Server) handleSetExcludes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExcludeTLDs []string `json:"exclude_tlds"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	patch := config.ConfigPatch{ExcludeTLDs: &req.ExcludeTLDs}
	_, _ = s.RuntimeCfg.Apply(patch)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"exclude_tlds": req.ExcludeTLDs,
	})
}

func (s *Server) handleAddExclude(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TLD string `json:"tld"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	if req.TLD == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TLD", "tld is required")
		return
	}

	cfg := s.RuntimeCfg.Get()
	tlds := make([]string, len(cfg.ExcludeTLDs))
	copy(tlds, cfg.ExcludeTLDs)

	// Check if already present
	for _, t := range tlds {
		if t == req.TLD {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"exclude_tlds": tlds,
				"message":      "already excluded",
			})
			return
		}
	}

	tlds = append(tlds, req.TLD)
	patch := config.ConfigPatch{ExcludeTLDs: &tlds}
	_, _ = s.RuntimeCfg.Apply(patch)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"exclude_tlds": tlds,
	})
}

func (s *Server) handleRemoveExclude(w http.ResponseWriter, r *http.Request) {
	tld := r.PathValue("tld")
	if tld == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TLD", "tld is required")
		return
	}

	cfg := s.RuntimeCfg.Get()
	var tlds []string
	found := false
	for _, t := range cfg.ExcludeTLDs {
		if t == tld {
			found = true
			continue
		}
		tlds = append(tlds, t)
	}

	if !found {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "TLD not in exclude list")
		return
	}

	patch := config.ConfigPatch{ExcludeTLDs: &tlds}
	_, _ = s.RuntimeCfg.Apply(patch)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"exclude_tlds": tlds,
	})
}

// --- Discovery ---

func (s *Server) handleGetDiscovery(w http.ResponseWriter, r *http.Request) {
	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"discovered":  s.Sink.Discovered(),
		"max_domains": cfg.Crawling.MaxDomains,
		"continuous":  cfg.Crawling.Continuous,
	})
}

func (s *Server) handleUpdateDiscovery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaxDomains *int  `json:"max_domains"`
		Continuous *bool `json:"continuous"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.MaxDomains != nil {
		s.Sink.SetMaxDomains(int64(*req.MaxDomains))
	}

	patch := config.ConfigPatch{
		Crawling: &config.CrawlingPatch{
			MaxDomains: req.MaxDomains,
			Continuous: req.Continuous,
		},
	}
	_, _ = s.RuntimeCfg.Apply(patch)

	cfg := s.RuntimeCfg.Get()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"discovered":  s.Sink.Discovered(),
		"max_domains": cfg.Crawling.MaxDomains,
		"continuous":  cfg.Crawling.Continuous,
	})
}

// --- Queue ---

func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "queue introspection not available")
}

func (s *Server) handleClearQueue(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "queue clear not available")
}

func (s *Server) handleRemoveFromQueue(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_AVAILABLE", "queue removal not available")
}
