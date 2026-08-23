package server

// Server owns the listener and coordinates connection goroutines. Shutdown
// closes the listener first, then waits for active sessions up to the caller's
// context deadline. Storage and cluster dependencies are injected for tests.
import (
	"context"
	"dcache/internal/cache"
	"dcache/internal/cluster"
	"dcache/internal/config"
	"dcache/internal/metrics"
	"net"
	"sync"
)

type Server struct {
	cfg     config.Config
	store   *cache.Store
	cluster *cluster.Cluster
	metrics *metrics.Metrics
	ln      net.Listener
	wg      sync.WaitGroup
	stop    chan struct{}
}

func (s *Server) Addr() string {
	if s.ln != nil {
		return s.ln.Addr().String()
	}
	return s.cfg.Addr
}
func (s *Server) Store() *cache.Store     { return s.store }
func (s *Server) Members() []cluster.Node { return s.cluster.Members() }
func (s *Server) Active() bool {
	select {
	case <-s.stop:
		return false
	default:
		return true
	}
}
func (s *Server) Config() config.Config     { return s.cfg }
func (s *Server) Metrics() *metrics.Metrics { return s.metrics }
func (s *Server) Cluster() *cluster.Cluster { return s.cluster }
func (s *Server) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}
func (s *Server) Wait() { s.wg.Wait() }

func New(c config.Config, s *cache.Store, g *cluster.Cluster, m *metrics.Metrics) *Server {
	return &Server{cfg: c, store: s, cluster: g, metrics: m, stop: make(chan struct{})}
}
func (s *Server) ListenAndServe() error {
	ln, e := net.Listen("tcp", s.cfg.Addr)
	if e != nil {
		return e
	}
	s.ln = ln
	for {
		c, e := ln.Accept()
		if e != nil {
			select {
			case <-s.stop:
				return nil
			default:
				return e
			}
		}
		s.wg.Add(1)
		go func() { s.handle(c) }()
	}
}
func (s *Server) Shutdown(ctx context.Context) error {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	if s.ln != nil {
		_ = s.ln.Close()
	}
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
