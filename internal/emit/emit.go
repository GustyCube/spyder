package emit

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
)

type Edge struct {
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	Target     string    `json:"target"`
	ObservedAt time.Time `json:"observed_at"`
	ProbeID    string    `json:"probe_id"`
	RunID      string    `json:"run_id"`
}

type NodeDomain struct {
	Host      string    `json:"host"`
	Apex      string    `json:"apex"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type NodeIP struct {
	IP        string    `json:"ip"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type NodeCert struct {
	SPKI      string    `json:"spki_sha256"`
	SubjectCN string    `json:"subject_cn"`
	IssuerCN  string    `json:"issuer_cn"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

type Batch struct {
	ProbeID string       `json:"probe_id"`
	RunID   string       `json:"run_id"`
	NodesD  []NodeDomain `json:"nodes_domain"`
	NodesIP []NodeIP     `json:"nodes_ip"`
	NodesC  []NodeCert   `json:"nodes_cert"`
	Edges   []Edge       `json:"edges"`
}

type Emitter struct {
	ingest    string
	probeID   string
	runID     string
	batchMax  int
	flushEvery time.Duration
	spoolDir  string
	client    *http.Client
	mu        sync.Mutex
	acc       Batch
}

func NewEmitter(ingest, probeID, runID string, batchMax int, flushEvery time.Duration, spoolDir, mtlsCert, mtlsKey, mtlsCA string) *Emitter {
	tr := &http.Transport{TLSClientConfig: &tls.Config{}}
	if mtlsCert != "" && mtlsKey != "" {
		cert, err := tls.LoadX509KeyPair(mtlsCert, mtlsKey)
		if err == nil {
			tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
		}
	}
	_ = os.MkdirAll(spoolDir, 0o755)
	return &Emitter{
		ingest: ingest, probeID: probeID, runID: runID,
		batchMax: batchMax, flushEvery: flushEvery, spoolDir: spoolDir,
		client: &http.Client{Transport: tr, Timeout: 20 * time.Second},
		acc: Batch{ProbeID: probeID, RunID: runID},
	}
}

func (e *Emitter) Run(ctx context.Context, in <-chan Batch, log *zap.SugaredLogger) {
	t := time.NewTimer(e.flushEvery)
	for {
		select {
		case b, ok := <-in:
			if !ok { return }
			e.append(b)
			if len(e.acc.Edges) >= e.batchMax || (len(e.acc.NodesD)+len(e.acc.NodesIP)+len(e.acc.NodesC)) >= e.batchMax/2 {
				e.flush(log)
				if !t.Stop() { select { case <-t.C: default: } }
				t.Reset(e.flushEvery)
			}
		case <-t.C:
			e.flush(log)
			t.Reset(e.flushEvery)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Emitter) append(b Batch) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.acc.NodesD = append(e.acc.NodesD, b.NodesD...)
	e.acc.NodesIP = append(e.acc.NodesIP, b.NodesIP...)
	e.acc.NodesC = append(e.acc.NodesC, b.NodesC...)
	e.acc.Edges = append(e.acc.Edges, b.Edges...)
}

func (e *Emitter) flush(log *zap.SugaredLogger) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.acc.Edges)+len(e.acc.NodesD)+len(e.acc.NodesIP)+len(e.acc.NodesC) == 0 { return }
	if e.ingest == "" {
		_ = json.NewEncoder(os.Stdout).Encode(e.acc)
	} else {
		if err := e.post(e.acc); err != nil {
			log.Warn("ingest failed, spooling", "err", err)
			e.spool(e.acc, log)
		}
	}
	e.acc = Batch{ProbeID: e.probeID, RunID: e.runID}
}

func (e *Emitter) post(b Batch) error {
	buf := &bytes.Buffer{}
	_ = json.NewEncoder(buf).Encode(b)
	op := func() error {
		req, _ := http.NewRequest("POST", e.ingest, bytes.NewReader(buf.Bytes()))
		req.Header.Set("Content-Type", "application/json")
		resp, err := e.client.Do(req)
		if err != nil { return err }
		io.Copy(io.Discard, resp.Body); resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("bad status: %d", resp.StatusCode)
		}
		return nil
	}
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 30 * time.Second
	return backoff.Retry(op, bo)
}

func (e *Emitter) spool(b Batch, log *zap.SugaredLogger) {
	name := time.Now().UTC().Format("20060102T150405.000000000") + ".json"
	path := filepath.Join(e.spoolDir, name)
	f, err := os.Create(path); if err != nil { log.Error("spool create", "err", err); return }
	defer f.Close()
	_ = json.NewEncoder(f).Encode(b)
}

func (e *Emitter) Drain(log *zap.SugaredLogger) {
	e.flush(log)
	// attempt to resend spooled files
	entries, _ := os.ReadDir(e.spoolDir)
	for _, ent := range entries {
		p := filepath.Join(e.spoolDir, ent.Name())
		f, err := os.Open(p); if err != nil { continue }
		var b Batch
		if err := json.NewDecoder(f).Decode(&b); err == nil {
			if e.ingest == "" || e.post(b) == nil {
				_ = f.Close(); _ = os.Remove(p); continue
			}
		}
		_ = f.Close()
	}
}
