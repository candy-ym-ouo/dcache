package web

import (
	"context"
	"dcache/internal/cache"
	"dcache/internal/cluster"
	"dcache/internal/metrics"
	"embed"
	"errors"
	"io/fs"
	"net/http"
)

//go:embed assets/*
var assets embed.FS

type Server struct {
	http    *http.Server
	store   *cache.Store
	cluster *cluster.Cluster
	metrics *metrics.Metrics
}

// New validates its dependencies and rejects construction when any required
// collaborator is missing, so handlers can assume non-nil wiring. Returning an
// error instead of panicking keeps the failure local to the caller and avoids
// relying on package-level defaults to paper over a misconfigured assembly.
func New(addr string, s *cache.Store, c *cluster.Cluster, m *metrics.Metrics) (*Server, error) {
	if s == nil {
		return nil, errors.New("web: store dependency must not be nil")
	}
	if c == nil {
		return nil, errors.New("web: cluster dependency must not be nil")
	}
	if m == nil {
		return nil, errors.New("web: metrics dependency must not be nil")
	}
	x := &Server{store: s, cluster: c, metrics: m}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", x.status)
	x.registerRoutes(mux)
	mux.HandleFunc("/api/summary", x.summary)
	mux.HandleFunc("/api/node", x.nodeInfo)
	mux.HandleFunc("/api/key-exists", x.keyExists)
	mux.HandleFunc("/api/key-ttl", x.keyTTL)
	mux.HandleFunc("/api/migration", x.migration)
	mux.HandleFunc("/api/version", x.version)
	mux.HandleFunc("/api/metrics", x.metricsHandler)
	mux.HandleFunc("/api/topology", x.topology)
	mux.HandleFunc("/api/reset-metrics", x.resetMetrics)
	mux.HandleFunc("/api/flush", x.flush)
	mux.HandleFunc("/api/delete-prefix", x.deletePrefix)
	mux.HandleFunc("/api/capacity", x.capacity)
	mux.HandleFunc("/api/live", x.live)
	staticFS, err := fs.Sub(assets, "assets")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	x.http = &http.Server{Addr: addr, Handler: mux}
	return x, nil
}
func (s *Server) ListenAndServe() error {
	e := s.http.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }
func (s *Server) HTTPServer() *http.Server           { return s.http }
func (s *Server) Store() *cache.Store                { return s.store }
func (s *Server) Cluster() *cluster.Cluster          { return s.cluster }
func (s *Server) Metrics() *metrics.Metrics          { return s.metrics }
