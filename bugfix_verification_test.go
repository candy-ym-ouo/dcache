package main

import (
	"context"
	"dcache/internal/cache"
	"dcache/internal/cluster"
	"dcache/internal/config"
	"dcache/internal/metrics"
	"dcache/internal/server"
	"testing"
	"time"
)

func TestBug007ShutdownWaitGroupMatchesConnections(t *testing.T) {
	cfg := config.Default()
	s := server.New(cfg, cache.New(2), cluster.New("n", cfg.Addr), metrics.New())
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if e := s.Shutdown(ctx); e != nil {
		t.Fatalf("shutdown: %v", e)
	}
}
