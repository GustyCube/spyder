package types

import "time"

// Edge represents a relationship between entities
type Edge struct {
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	ObservedAt time.Time `json:"observed_at"`
	ProbeID    string    `json:"probe_id"`
	RunID      string    `json:"run_id"`
}

// NodeDomain represents a domain entity
type NodeDomain struct {
	Host      string    `json:"host"`
	Apex      string    `json:"apex"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// NodeIP represents an IP address entity
type NodeIP struct {
	IP        string    `json:"ip"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// NodeCert represents a certificate entity
type NodeCert struct {
	SPKI      string    `json:"spki_sha256"`
	SubjectCN string    `json:"subject_cn"`
	IssuerCN  string    `json:"issuer_cn"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

// Batch represents a collection of nodes and edges
type Batch struct {
	ProbeID     string       `json:"probe_id"`
	RunID       string       `json:"run_id"`
	BatchID     string       `json:"batch_id"`
	Timestamp   time.Time    `json:"timestamp"`
	NodesDomain []NodeDomain `json:"nodes_domain"`
	NodesIP     []NodeIP     `json:"nodes_ip"`
	NodesCert   []NodeCert   `json:"nodes_cert"`
	Edges       []Edge       `json:"edges"`
}