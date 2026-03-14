package api

import (
	"context"

	"github.com/gustycube/spyder/internal/emit"
)

// Page is a generic paginated response.
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	Total      int64  `json:"total,omitempty"`
	Store      string `json:"store"`
}

// DomainFilter holds query parameters for listing domains.
type DomainFilter struct {
	Apex   string
	Search string
	Cursor string
	Limit  int
}

// EdgeFilter holds query parameters for listing edges.
type EdgeFilter struct {
	Type   string
	Source string
	Target string
	Cursor string
	Limit  int
}

// IPFilter holds query parameters for listing IPs.
type IPFilter struct {
	Search string
	Cursor string
	Limit  int
}

// CertFilter holds query parameters for listing certificates.
type CertFilter struct {
	Search string
	Cursor string
	Limit  int
}

// DomainDetail is a domain with its edges.
type DomainDetail struct {
	emit.NodeDomain
	Edges []emit.Edge `json:"edges"`
}

// QueryStore is the interface for querying discovered data.
type QueryStore interface {
	ListDomains(ctx context.Context, filter DomainFilter) (*Page[emit.NodeDomain], error)
	GetDomain(ctx context.Context, host string) (*DomainDetail, error)
	ListEdges(ctx context.Context, filter EdgeFilter) (*Page[emit.Edge], error)
	ListIPs(ctx context.Context, filter IPFilter) (*Page[emit.NodeIP], error)
	ListCerts(ctx context.Context, filter CertFilter) (*Page[emit.NodeCert], error)
}
