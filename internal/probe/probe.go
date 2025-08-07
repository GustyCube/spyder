package probe

import "go.opentelemetry.io/otel"

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gustycube/spyder-probe/internal/dedup"
	"github.com/gustycube/spyder-probe/internal/dns"
	"github.com/gustycube/spyder-probe/internal/emit"
	"github.com/gustycube/spyder-probe/internal/extract"
	"github.com/gustycube/spyder-probe/internal/httpclient"
	"github.com/gustycube/spyder-probe/internal/rate"
	"github.com/gustycube/spyder-probe/internal/robots"
	"github.com/gustycube/spyder-probe/internal/tlsinfo"
	"github.com/gustycube/spyder-probe/internal/metrics"
	"go.uber.org/zap"
)

type Probe struct {
	ua       string
	probeID  string
	runID    string
	excluded []string
	dedup    dedup.Interface
	out      chan<- emit.Batch
	hc       *http.Client
	rob      *robots.Cache
	ratelim  *rate.PerHost
	log      *zap.SugaredLogger
}

func New(ua, probeID, runID string, excluded []string, d dedup.Interface, out chan<- emit.Batch, log *zap.SugaredLogger) *Probe {
	hc := httpclient.Default()
	return &Probe{
		ua: ua, probeID: probeID, runID: runID, excluded: excluded, dedup: d, out: out,
		hc: hc, rob: robots.NewCache(hc, ua), ratelim: rate.New(1.0, 1), log: log,
	}
}

func (p *Probe) Run(ctx context.Context, tasks <-chan string, workers int) {
	done := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			for host := range tasks {
				p.CrawlOne(ctx, host)
				metrics.TasksTotal.WithLabelValues("ok").Inc()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < workers; i++ { <-done }
}

func (p *Probe) CrawlOne(ctx context.Context, host string) {
	tr := otel.Tracer("spyder/probe")
	ctx, span := tr.Start(ctx, "CrawlOne")
	defer span.End()
	now := time.Now().UTC()
	var nodesD []emit.NodeDomain
	var nodesIP []emit.NodeIP
	var nodesC []emit.NodeCert
	var edges []emit.Edge

	ap := extract.Apex(host)
	nodesD = append(nodesD, emit.NodeDomain{Host: host, Apex: ap, FirstSeen: now, LastSeen: now})

	ips, ns, cname, mx, _ := dns.ResolveAll(ctx, host)
	for _, ip := range ips {
		if !p.dedup.Seen("nodeip|"+ip) { nodesIP = append(nodesIP, emit.NodeIP{IP: ip, FirstSeen: now, LastSeen: now}) }
		k := "edge|"+host+"|RESOLVES_TO|"+ip
		if !p.dedup.Seen(k) { edges = append(edges, emit.Edge{Type: "RESOLVES_TO", Source: host, Target: ip, ObservedAt: now, ProbeID: p.probeID, RunID: p.runID}); metrics.EdgesTotal.WithLabelValues("RESOLVES_TO").Inc() }
	}
	for _, n := range ns {
		if !p.dedup.Seen("domain|"+n) { nodesD = append(nodesD, emit.NodeDomain{Host: n, Apex: extract.Apex(n), FirstSeen: now, LastSeen: now}) }
		k := "edge|"+host+"|USES_NS|"+n
		if !p.dedup.Seen(k) { edges = append(edges, emit.Edge{Type: "USES_NS", Source: host, Target: n, ObservedAt: now, ProbeID: p.probeID, RunID: p.runID}); metrics.EdgesTotal.WithLabelValues("USES_NS").Inc() }
	}
	if cname != "" {
		if !p.dedup.Seen("domain|"+cname) { nodesD = append(nodesD, emit.NodeDomain{Host: cname, Apex: extract.Apex(cname), FirstSeen: now, LastSeen: now}) }
		k := "edge|"+host+"|ALIAS_OF|"+cname
		if !p.dedup.Seen(k) { edges = append(edges, emit.Edge{Type: "ALIAS_OF", Source: host, Target: cname, ObservedAt: now, ProbeID: p.probeID, RunID: p.runID}) }
	}
	for _, m := range mx {
		if !p.dedup.Seen("domain|"+m) { nodesD = append(nodesD, emit.NodeDomain{Host: m, Apex: extract.Apex(m), FirstSeen: now, LastSeen: now}) }
		k := "edge|"+host+"|USES_MX|"+m
		if !p.dedup.Seen(k) { edges = append(edges, emit.Edge{Type: "USES_MX", Source: host, Target: m, ObservedAt: now, ProbeID: p.probeID, RunID: p.runID}); metrics.EdgesTotal.WithLabelValues("USES_MX").Inc() }
	}

	// Policy
	if robots.ShouldSkipByTLD(host, p.excluded) {
		p.flush(nodesD, nodesIP, nodesC, edges)
		return
	}
	rd, _ := p.rob.Get(ctx, host)
	if !robots.Allowed(rd, p.ua, "/") {
		metrics.RobotsBlocks.Inc()
		p.flush(nodesD, nodesIP, nodesC, edges)
		return
	}

	// Per-host rate limit
	p.ratelim.Wait(host)

	// GET root HTML
	root := &url.URL{Scheme: "https", Host: host, Path: "/"}
	req, _ := http.NewRequestWithContext(ctx, "GET", root.String(), nil)
	req.Header.Set("User-Agent", p.ua)
	resp, err := p.hc.Do(req)
	if err == nil {
		ct := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.Contains(ct, "text/html") && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			body := io.LimitReader(resp.Body, 512*1024)
			links, _ := extract.ParseLinks(root, body)
			outs := extract.ExternalDomains(host, links)
			for _, h := range outs {
				if !p.dedup.Seen("domain|"+h) { nodesD = append(nodesD, emit.NodeDomain{Host: h, Apex: extract.Apex(h), FirstSeen: now, LastSeen: now}) }
				k := "edge|"+host+"|LINKS_TO|"+h
				if !p.dedup.Seen(k) { edges = append(edges, emit.Edge{Type: "LINKS_TO", Source: host, Target: h, ObservedAt: now, ProbeID: p.probeID, RunID: p.runID}); metrics.EdgesTotal.WithLabelValues("LINKS_TO").Inc() }
			}
		}
		io.Copy(io.Discard, resp.Body); resp.Body.Close()
	}

	if cert, err := tlsinfo.FetchCert(host); err == nil && cert != nil {
		if !p.dedup.Seen("cert|"+cert.SPKI) { nodesC = append(nodesC, *cert) }
		k := "edge|"+host+"|USES_CERT|"+cert.SPKI
		if !p.dedup.Seen(k) { edges = append(edges, emit.Edge{Type: "USES_CERT", Source: host, Target: cert.SPKI, ObservedAt: now, ProbeID: p.probeID, RunID: p.runID}); metrics.EdgesTotal.WithLabelValues("USES_CERT").Inc() }
	}

	p.flush(nodesD, nodesIP, nodesC, edges)
}

func (p *Probe) flush(nd []emit.NodeDomain, ni []emit.NodeIP, nc []emit.NodeCert, e []emit.Edge) {
	if len(nd)+len(ni)+len(nc)+len(e) == 0 { return }
	p.out <- emit.Batch{ProbeID: p.probeID, RunID: p.runID, NodesD: nd, NodesIP: ni, NodesC: nc, Edges: e}
}
