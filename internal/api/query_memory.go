package api

import (
	"context"
	"strconv"
	"strings"

	"github.com/gustycube/spyder/internal/emit"
	"github.com/gustycube/spyder/internal/hub"
)

// MemoryQueryStore implements QueryStore by scanning hub history in memory.
type MemoryQueryStore struct {
	hub *hub.Hub
}

// NewMemoryQueryStore creates a query store backed by the hub's batch history.
func NewMemoryQueryStore(h *hub.Hub) *MemoryQueryStore {
	return &MemoryQueryStore{hub: h}
}

func (m *MemoryQueryStore) collectDomains() []emit.NodeDomain {
	history := m.hub.GetHistory()
	seen := make(map[string]bool)
	var result []emit.NodeDomain
	for _, be := range history {
		for _, d := range be.Domains {
			if !seen[d] {
				seen[d] = true
				result = append(result, emit.NodeDomain{Host: d, FirstSeen: be.Timestamp, LastSeen: be.Timestamp})
			}
		}
	}
	return result
}

func (m *MemoryQueryStore) collectEdges() []emit.Edge {
	history := m.hub.GetHistory()
	var result []emit.Edge
	for _, be := range history {
		result = append(result, be.Edges...)
	}
	return result
}

func (m *MemoryQueryStore) collectIPs() []emit.NodeIP {
	history := m.hub.GetHistory()
	seen := make(map[string]bool)
	var result []emit.NodeIP
	for _, be := range history {
		for _, ip := range be.IPs {
			if !seen[ip] {
				seen[ip] = true
				result = append(result, emit.NodeIP{IP: ip, FirstSeen: be.Timestamp, LastSeen: be.Timestamp})
			}
		}
	}
	return result
}

func (m *MemoryQueryStore) collectCerts() []emit.NodeCert {
	history := m.hub.GetHistory()
	seen := make(map[string]bool)
	var result []emit.NodeCert
	for _, be := range history {
		for _, c := range be.Certs {
			if !seen[c.SPKI] {
				seen[c.SPKI] = true
				result = append(result, emit.NodeCert{
					SPKI:      c.SPKI,
					SubjectCN: c.SubjectCN,
					IssuerCN:  c.IssuerCN,
				})
			}
		}
	}
	return result
}

func parseCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	n, err := strconv.Atoi(cursor)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func (m *MemoryQueryStore) ListDomains(_ context.Context, filter DomainFilter) (*Page[emit.NodeDomain], error) {
	all := m.collectDomains()

	// Apply filters
	var filtered []emit.NodeDomain
	for _, d := range all {
		if filter.Apex != "" && !strings.HasSuffix(d.Host, "."+filter.Apex) && d.Host != filter.Apex {
			continue
		}
		if filter.Search != "" && !strings.Contains(d.Host, filter.Search) {
			continue
		}
		filtered = append(filtered, d)
	}

	start := parseCursor(filter.Cursor)
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}

	page := &Page[emit.NodeDomain]{
		Items: filtered[start:end],
		Total: int64(len(filtered)),
		Store: "memory",
	}
	if page.Items == nil {
		page.Items = []emit.NodeDomain{}
	}
	if end < len(filtered) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (m *MemoryQueryStore) GetDomain(_ context.Context, host string) (*DomainDetail, error) {
	domains := m.collectDomains()
	edges := m.collectEdges()

	for _, d := range domains {
		if d.Host == host {
			var related []emit.Edge
			for _, e := range edges {
				if e.Source == host || e.Target == host {
					related = append(related, e)
				}
			}
			if related == nil {
				related = []emit.Edge{}
			}
			return &DomainDetail{NodeDomain: d, Edges: related}, nil
		}
	}
	return nil, nil
}

func (m *MemoryQueryStore) ListEdges(_ context.Context, filter EdgeFilter) (*Page[emit.Edge], error) {
	all := m.collectEdges()

	var filtered []emit.Edge
	for _, e := range all {
		if filter.Type != "" && e.Type != filter.Type {
			continue
		}
		if filter.Source != "" && e.Source != filter.Source {
			continue
		}
		if filter.Target != "" && e.Target != filter.Target {
			continue
		}
		filtered = append(filtered, e)
	}

	start := parseCursor(filter.Cursor)
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}

	page := &Page[emit.Edge]{
		Items: filtered[start:end],
		Total: int64(len(filtered)),
		Store: "memory",
	}
	if page.Items == nil {
		page.Items = []emit.Edge{}
	}
	if end < len(filtered) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (m *MemoryQueryStore) ListIPs(_ context.Context, filter IPFilter) (*Page[emit.NodeIP], error) {
	all := m.collectIPs()

	var filtered []emit.NodeIP
	for _, ip := range all {
		if filter.Search != "" && !strings.Contains(ip.IP, filter.Search) {
			continue
		}
		filtered = append(filtered, ip)
	}

	start := parseCursor(filter.Cursor)
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}

	page := &Page[emit.NodeIP]{
		Items: filtered[start:end],
		Total: int64(len(filtered)),
		Store: "memory",
	}
	if page.Items == nil {
		page.Items = []emit.NodeIP{}
	}
	if end < len(filtered) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (m *MemoryQueryStore) ListCerts(_ context.Context, filter CertFilter) (*Page[emit.NodeCert], error) {
	all := m.collectCerts()

	var filtered []emit.NodeCert
	for _, c := range all {
		if filter.Search != "" && !strings.Contains(c.SubjectCN, filter.Search) && !strings.Contains(c.IssuerCN, filter.Search) {
			continue
		}
		filtered = append(filtered, c)
	}

	start := parseCursor(filter.Cursor)
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + filter.Limit
	if end > len(filtered) {
		end = len(filtered)
	}

	page := &Page[emit.NodeCert]{
		Items: filtered[start:end],
		Total: int64(len(filtered)),
		Store: "memory",
	}
	if page.Items == nil {
		page.Items = []emit.NodeCert{}
	}
	if end < len(filtered) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}
