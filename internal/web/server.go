package web

import (
	"context"
	"dcache/internal/cache"
	"dcache/internal/cluster"
	"dcache/internal/metrics"
	"embed"
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

func New(addr string, s *cache.Store, c *cluster.Cluster, m *metrics.Metrics) *Server {
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
	staticFS, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	x.http = &http.Server{Addr: addr, Handler: mux}
	return x
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
