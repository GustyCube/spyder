package hub

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gustycube/spyder/internal/emit"
	"go.uber.org/zap"
)

// Event is sent to SSE subscribers.
type Event struct {
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Payload json.RawMessage `json:"payload"`
}

// BatchEvent is the payload for batch events.
type BatchEvent struct {
	ProbeID   string      `json:"probe_id"`
	RunID     string      `json:"run_id"`
	Domains   []string    `json:"domains"`
	IPs       []string    `json:"ips"`
	Certs     []CertInfo  `json:"certs"`
	Edges     []emit.Edge `json:"edges"`
	Stats     Stats       `json:"stats"`
	Timestamp time.Time   `json:"timestamp"`
}

// CertInfo is a minimal cert representation for the dashboard.
type CertInfo struct {
	SubjectCN string `json:"subject_cn"`
	IssuerCN  string `json:"issuer_cn"`
	SPKI      string `json:"spki"`
}

// Stats holds running statistics.
type Stats struct {
	DomainsTotal  int64   `json:"domains_total"`
	EdgesTotal    int64   `json:"edges_total"`
	IPsTotal      int64   `json:"ips_total"`
	CertsTotal    int64   `json:"certs_total"`
	DomainsPerSec float64 `json:"domains_per_sec"`
}

// Hub fans out emit.Batch from the probe to both the emitter and SSE subscribers.
type Hub struct {
	in  <-chan emit.Batch
	out chan<- emit.Batch

	mu   sync.RWMutex
	subs map[int64]chan Event
	next atomic.Int64

	stats Stats

	// history ring buffer of recent BatchEvents for late-joining clients
	history    []BatchEvent
	historyLen int
	historyIdx int

	// throughput: ring of per-second domain counts
	ring    [60]int64
	ringIdx int
}

// New creates a hub that reads from in and forwards to out.
func New(in <-chan emit.Batch, out chan<- emit.Batch, historySize int) *Hub {
	return &Hub{
		in:      in,
		out:     out,
		subs:    make(map[int64]chan Event),
		history: make([]BatchEvent, historySize),
	}
}

// Run starts the fan-out loop. Blocks until in is closed or ctx is cancelled.
func (h *Hub) Run(ctx context.Context, log *zap.SugaredLogger) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case b, ok := <-h.in:
			if !ok {
				return
			}
			// Forward to emitter (blocking is fine, emitter channel is buffered)
			select {
			case h.out <- b:
			case <-ctx.Done():
				return
			}
			h.broadcast(b)

		case <-ticker.C:
			h.advanceRing()

		case <-ctx.Done():
			return
		}
	}
}

func (h *Hub) broadcast(b emit.Batch) {
	historySize := len(h.history)

	// Build event outside lock
	domains := make([]string, len(b.NodesD))
	for i, d := range b.NodesD {
		domains[i] = d.Host
	}
	ips := make([]string, len(b.NodesIP))
	for i, ip := range b.NodesIP {
		ips[i] = ip.IP
	}
	certs := make([]CertInfo, len(b.NodesC))
	for i, c := range b.NodesC {
		certs[i] = CertInfo{SubjectCN: c.SubjectCN, IssuerCN: c.IssuerCN, SPKI: c.SPKI}
	}

	// Update stats and store history under single lock
	h.mu.Lock()
	h.stats.DomainsTotal += int64(len(b.NodesD))
	h.stats.EdgesTotal += int64(len(b.Edges))
	h.stats.IPsTotal += int64(len(b.NodesIP))
	h.stats.CertsTotal += int64(len(b.NodesC))
	h.ring[h.ringIdx] += int64(len(b.NodesD))
	stats := h.stats
	stats.DomainsPerSec = h.calcThroughput()

	be := BatchEvent{
		ProbeID:   b.ProbeID,
		RunID:     b.RunID,
		Domains:   domains,
		IPs:       ips,
		Certs:     certs,
		Edges:     b.Edges,
		Stats:     stats,
		Timestamp: time.Now(),
	}

	h.history[h.historyIdx] = be
	h.historyIdx = (h.historyIdx + 1) % historySize
	if h.historyLen < historySize {
		h.historyLen++
	}
	h.mu.Unlock()

	payload, _ := json.Marshal(be)
	ev := Event{
		Type:    "batch",
		Seq:     h.next.Add(1),
		Payload: payload,
	}

	h.mu.RLock()
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default: // drop if subscriber is slow
		}
	}
	h.mu.RUnlock()
}

func (h *Hub) advanceRing() {
	h.mu.Lock()
	h.ringIdx = (h.ringIdx + 1) % len(h.ring)
	h.ring[h.ringIdx] = 0
	h.stats.DomainsPerSec = h.calcThroughput()
	h.mu.Unlock()
}

// calcThroughput returns average domains/sec over the last 10 seconds.
// Must be called with h.mu held.
func (h *Hub) calcThroughput() float64 {
	var sum int64
	for i := 1; i <= 10; i++ {
		idx := (h.ringIdx - i + len(h.ring)) % len(h.ring)
		sum += h.ring[idx]
	}
	return float64(sum) / 10.0
}

// Subscribe registers a new SSE subscriber. Returns an ID and a channel.
func (h *Hub) Subscribe() (int64, <-chan Event) {
	id := h.next.Add(1)
	ch := make(chan Event, 64)
	h.mu.Lock()
	h.subs[id] = ch
	h.mu.Unlock()
	return id, ch
}

// Unsubscribe removes a subscriber.
func (h *Hub) Unsubscribe(id int64) {
	h.mu.Lock()
	if ch, ok := h.subs[id]; ok {
		close(ch)
		delete(h.subs, id)
	}
	h.mu.Unlock()
}

// GetHistory returns all stored batch events in chronological order.
func (h *Hub) GetHistory() []BatchEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	historySize := len(h.history)

	if h.historyLen == 0 {
		return nil
	}

	result := make([]BatchEvent, h.historyLen)
	if h.historyLen < historySize {
		// Haven't wrapped yet — entries are 0..historyLen-1
		copy(result, h.history[:h.historyLen])
	} else {
		// Ring has wrapped — oldest is at historyIdx, read forward
		n := copy(result, h.history[h.historyIdx:])
		copy(result[n:], h.history[:h.historyIdx])
	}
	return result
}

// GetStats returns a snapshot of the current stats.
func (h *Hub) GetStats() Stats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	s := h.stats
	s.DomainsPerSec = h.calcThroughput()
	return s
}
