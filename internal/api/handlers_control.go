package api

import (
	"bufio"
	"net/http"
	"strings"
)

func (s *Server) handleSubmitDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host string `json:"host"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	req.Host = strings.TrimSpace(req.Host)
	if req.Host == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HOST", "host is required")
		return
	}

	select {
	case s.TaskCh <- req.Host:
		writeJSON(w, http.StatusAccepted, map[string]interface{}{
			"accepted": true,
			"host":     req.Host,
		})
	default:
		writeError(w, http.StatusServiceUnavailable, "QUEUE_FULL", "task channel is full")
	}
}

func (s *Server) handleSubmitDomainsBulk(w http.ResponseWriter, r *http.Request) {
	var hosts []string

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/plain") {
		scanner := bufio.NewScanner(r.Body)
		for scanner.Scan() {
			h := strings.TrimSpace(scanner.Text())
			if h != "" && !strings.HasPrefix(h, "#") {
				hosts = append(hosts, h)
			}
		}
		if err := scanner.Err(); err != nil {
			writeError(w, http.StatusBadRequest, "READ_ERROR", err.Error())
			return
		}
	} else {
		if err := readJSON(r, &hosts); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
			return
		}
	}

	accepted := 0
	rejected := 0
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		select {
		case s.TaskCh <- h:
			accepted++
		default:
			rejected++
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"accepted": accepted,
		"rejected": rejected,
		"total":    len(hosts),
	})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	s.Pool.Pause()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "paused",
	})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	s.Pool.Resume()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "resumed",
	})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "stopping",
	})
	// Cancel after response is sent
	go s.Cancel()
}
