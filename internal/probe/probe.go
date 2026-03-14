package probe

import "go.opentelemetry.io/otel"

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gustycube/spyder/internal/dedup"
	"github.com/gustycube/spyder/internal/discover"
	"github.com/gustycube/spyder/internal/dns"
	"github.com/gustycube/spyder/internal/emit"
	"github.com/gustycube/spyder/internal/extract"
	"github.com/gustycube/spyder/internal/httpclient"
	"github.com/gustycube/spyder/internal/metrics"
	"github.com/gustycube/spyder/internal/rate"
	"github.com/gustycube/spyder/internal/robots"
	"github.com/gustycube/spyder/internal/tlsinfo"
	"go.uber.org/zap"
)

// ProbeConfig holds runtime-tunable configuration for the Probe.
type ProbeConfig struct {
	UA           string
	ProbeID      string
	RunID        string
	ExcludeTLDs  []string
	HTTPTimeout  time.Duration
	TLSTimeout   time.Duration
	BodyMaxBytes int64
}

type Probe struct {
	cfg     ProbeConfig
	mu      sync.RWMutex
	dedup   dedup.Interface
	sink    discover.Sink
	out     chan<- emit.Batch
	hc      *httpclient.ResilientClient
	rob     *robots.Cache
	ratelim *rate.PerHost
	log     *zap.SugaredLogger
}

func New(cfg ProbeConfig, d dedup.Interface, sink discover.Sink, out chan<- emit.Batch, rl *rate.PerHost, hc *httpclient.ResilientClient, rob *robots.Cache, log *zap.SugaredLogger) *Probe {
	return &Probe{
		cfg:     cfg,
		dedup:   d,
		sink:    sink,
		out:     out,
		hc:      hc,
		rob:     rob,
		ratelim: rl,
		log:     log,
	}
}

// SetExcludeTLDs updates the excluded TLDs in a thread-safe manner.
func (p *Probe) SetExcludeTLDs(tlds []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg.ExcludeTLDs = tlds
}

// SetUA updates the user-agent string in a thread-safe manner.
func (p *Probe) SetUA(ua string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg.UA = ua
}

func (p *Probe) CrawlOne(ctx context.Context, host string) {
	tr := otel.Tracer("spyder/probe")
	ctx, span := tr.Start(ctx, "CrawlOne")
	defer span.End()

	// Read config under lock
	p.mu.RLock()
	ua := p.cfg.UA
	probeID := p.cfg.ProbeID
	runID := p.cfg.RunID
	excluded := p.cfg.ExcludeTLDs
	httpTimeout := p.cfg.HTTPTimeout
	bodyMaxBytes := p.cfg.BodyMaxBytes
	p.mu.RUnlock()

	now := time.Now().UTC()
	var nodesD []emit.NodeDomain
	var nodesIP []emit.NodeIP
	var nodesC []emit.NodeCert
	var edges []emit.Edge

	ap := extract.Apex(host)
	nodesD = append(nodesD, emit.NodeDomain{Host: host, Apex: ap, FirstSeen: now, LastSeen: now})

	ips, ns, cname, mx, _ := dns.ResolveAll(ctx, host)
	for _, ip := range ips {
		if !p.dedup.Seen(emit.DedupNodeIP + ip) {
			nodesIP = append(nodesIP, emit.NodeIP{IP: ip, FirstSeen: now, LastSeen: now})
		}
		k := emit.DedupEdge + host + "|" + emit.EdgeResolvesTo + "|" + ip
		if !p.dedup.Seen(k) {
			edges = append(edges, emit.Edge{Type: emit.EdgeResolvesTo, Source: host, Target: ip, ObservedAt: now, ProbeID: probeID, RunID: runID})
			metrics.EdgesTotal.WithLabelValues(emit.EdgeResolvesTo).Inc()
		}
	}
	for _, n := range ns {
		if !p.dedup.Seen(emit.DedupDomain + n) {
			nodesD = append(nodesD, emit.NodeDomain{Host: n, Apex: extract.Apex(n), FirstSeen: now, LastSeen: now})
			p.sink.Submit(ctx, n)
		}
		k := emit.DedupEdge + host + "|" + emit.EdgeUsesNS + "|" + n
		if !p.dedup.Seen(k) {
			edges = append(edges, emit.Edge{Type: emit.EdgeUsesNS, Source: host, Target: n, ObservedAt: now, ProbeID: probeID, RunID: runID})
			metrics.EdgesTotal.WithLabelValues(emit.EdgeUsesNS).Inc()
		}
	}
	if cname != "" {
		if !p.dedup.Seen(emit.DedupDomain + cname) {
			nodesD = append(nodesD, emit.NodeDomain{Host: cname, Apex: extract.Apex(cname), FirstSeen: now, LastSeen: now})
			p.sink.Submit(ctx, cname)
		}
		k := emit.DedupEdge + host + "|" + emit.EdgeAliasOf + "|" + cname
		if !p.dedup.Seen(k) {
			edges = append(edges, emit.Edge{Type: emit.EdgeAliasOf, Source: host, Target: cname, ObservedAt: now, ProbeID: probeID, RunID: runID})
		}
	}
	for _, m := range mx {
		if !p.dedup.Seen(emit.DedupDomain + m) {
			nodesD = append(nodesD, emit.NodeDomain{Host: m, Apex: extract.Apex(m), FirstSeen: now, LastSeen: now})
			p.sink.Submit(ctx, m)
		}
		k := emit.DedupEdge + host + "|" + emit.EdgeUsesMX + "|" + m
		if !p.dedup.Seen(k) {
			edges = append(edges, emit.Edge{Type: emit.EdgeUsesMX, Source: host, Target: m, ObservedAt: now, ProbeID: probeID, RunID: runID})
			metrics.EdgesTotal.WithLabelValues(emit.EdgeUsesMX).Inc()
		}
	}

	// Policy
	if robots.ShouldSkipByTLD(host, excluded) {
		p.flush(probeID, runID, nodesD, nodesIP, nodesC, edges)
		return
	}
	rd, err := p.rob.Get(ctx, host)
	if err != nil {
		p.log.Debugw("robots.txt fetch", "host", host, "err", err)
	}
	if !robots.Allowed(rd, ua, "/") {
		metrics.RobotsBlocks.Inc()
		p.flush(probeID, runID, nodesD, nodesIP, nodesC, edges)
		return
	}

	// Per-host rate limit
	p.ratelim.Wait(host)

	// GET root HTML with separate timeout context
	root := &url.URL{Scheme: "https", Host: host, Path: "/"}
	httpCtx, cancel := context.WithTimeout(ctx, httpTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(httpCtx, "GET", root.String(), nil)
	if err != nil {
		p.log.Warnw("create request", "host", host, "err", err)
		p.flush(probeID, runID, nodesD, nodesIP, nodesC, edges)
		return
	}
	req.Header.Set("User-Agent", ua)
	resp, err := p.hc.Do(req)
	if err == nil {
		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.Contains(ct, "text/html") && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body := io.LimitReader(resp.Body, bodyMaxBytes)
			links, err := extract.ParseLinks(root, body)
			if err != nil {
				p.log.Debugw("parse links", "host", host, "err", err)
			}
			outs := extract.ExternalDomains(host, links)
			for _, h := range outs {
				if !p.dedup.Seen(emit.DedupDomain + h) {
					nodesD = append(nodesD, emit.NodeDomain{Host: h, Apex: extract.Apex(h), FirstSeen: now, LastSeen: now})
					p.sink.Submit(ctx, h)
				}
				k := emit.DedupEdge + host + "|" + emit.EdgeLinksTo + "|" + h
				if !p.dedup.Seen(k) {
					edges = append(edges, emit.Edge{Type: emit.EdgeLinksTo, Source: host, Target: h, ObservedAt: now, ProbeID: probeID, RunID: runID})
					metrics.EdgesTotal.WithLabelValues(emit.EdgeLinksTo).Inc()
				}
			}
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	if cert, err := tlsinfo.FetchCert(host); err == nil && cert != nil {
		if !p.dedup.Seen(emit.DedupCert + cert.SPKI) {
			nodesC = append(nodesC, *cert)
		}
		k := emit.DedupEdge + host + "|" + emit.EdgeUsesCert + "|" + cert.SPKI
		if !p.dedup.Seen(k) {
			edges = append(edges, emit.Edge{Type: emit.EdgeUsesCert, Source: host, Target: cert.SPKI, ObservedAt: now, ProbeID: probeID, RunID: runID})
			metrics.EdgesTotal.WithLabelValues(emit.EdgeUsesCert).Inc()
		}
	}

	p.flush(probeID, runID, nodesD, nodesIP, nodesC, edges)
}

func (p *Probe) flush(probeID, runID string, nd []emit.NodeDomain, ni []emit.NodeIP, nc []emit.NodeCert, e []emit.Edge) {
	if len(nd)+len(ni)+len(nc)+len(e) == 0 {
		return
	}
	p.out <- emit.Batch{ProbeID: probeID, RunID: runID, NodesD: nd, NodesIP: ni, NodesC: nc, Edges: e}
}
