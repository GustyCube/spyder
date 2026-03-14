package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gustycube/spyder/internal/hub"
	"go.uber.org/zap"
)

// Handler serves the dashboard UI and SSE events.
type Handler struct {
	hub *hub.Hub
	log *zap.SugaredLogger
}

// NewHandler creates a new dashboard handler.
func NewHandler(h *hub.Hub, log *zap.SugaredLogger) *Handler {
	return &Handler{hub: h, log: log}
}

// RegisterRoutes registers dashboard routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/events", h.ServeSSE)
	mux.HandleFunc("/api/stats", h.ServeStats)
	mux.HandleFunc("/api/history", h.ServeHistory)
	mux.Handle("/", http.FileServer(http.FS(staticFS())))
}

// ServeSSE streams Server-Sent Events to the client.
func (h *Handler) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	id, ch := h.hub.Subscribe()
	defer h.hub.Unsubscribe(id)

	h.log.Debugw("sse client connected", "id", id)

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-ping.C:
			fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
			flusher.Flush()

		case <-r.Context().Done():
			h.log.Debugw("sse client disconnected", "id", id)
			return
		}
	}
}

// ServeStats returns current stats as JSON.
func (h *Handler) ServeStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.hub.GetStats())
}

// ServeHistory returns all stored batch events for late-joining clients.
func (h *Handler) ServeHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	history := h.hub.GetHistory()
	if history == nil {
		history = []hub.BatchEvent{}
	}
	json.NewEncoder(w).Encode(history)
}
