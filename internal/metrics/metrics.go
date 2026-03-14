package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	TasksTotal   = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "spyder_tasks_total", Help: "tasks processed"}, []string{"status"})
	EdgesTotal   = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "spyder_edges_total", Help: "edges emitted"}, []string{"type"})
	RobotsBlocks = prometheus.NewCounter(prometheus.CounterOpts{Name: "spyder_robots_blocked_total", Help: "robots.txt blocks"})
)

func init() {
	prometheus.MustRegister(TasksTotal, EdgesTotal, RobotsBlocks)
}

func Serve(addr string, mux *http.ServeMux, log *zap.SugaredLogger) {
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		log.Warn("metrics server stopped", "err", err)
	}
}

func ServeWithHealth(addr string, mux *http.ServeMux, log *zap.SugaredLogger) {
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		log.Warn("metrics server stopped", "err", err)
	}
}
