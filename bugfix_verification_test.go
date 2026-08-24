package main

import (
	"context"
	"dcache/internal/cache"
	"dcache/internal/client"
	"dcache/internal/cluster"
	"dcache/internal/config"
	"dcache/internal/metrics"
	"dcache/internal/server"
	"net"
	"testing"
	"time"
)

func TestBug009TTLWireValueDoesNotDrift(t *testing.T) {
	cfg := config.Default()
	cfg.Addr = freeAddr009(t)
	s := server.New(cfg, cache.New(10), cluster.New("n", cfg.Addr), metrics.New())
	go s.ListenAndServe()
	defer s.Shutdown(context.Background())
	for i := 0; i < 50; i++ {
		if _, e := client.New(cfg.Addr).Do(13, "", nil, 0); e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	c := client.New(cfg.Addr)
	if e := c.Set("persistent-key", "v", 0); e != nil {
		t.Fatal(e)
	}
	persistentTTL, e := c.TTL("persistent-key")
	if e != nil {
		t.Fatal(e)
	}
	if persistentTTL != -1 {
		t.Fatalf("persistent ttl=%d", persistentTTL)
	}
	missingTTL, e := c.TTL("missing-key")
	if e != nil {
		t.Fatal(e)
	}
	if missingTTL != -2 {
		t.Fatalf("missing ttl=%d", missingTTL)
	}
	if e := c.Set("ttl-key", "v", 3); e != nil {
		t.Fatal(e)
	}
	ttl, e := c.TTL("ttl-key")
	if e != nil {
		t.Fatal(e)
	}
	if ttl < 0 || ttl > 3 {
		t.Fatalf("ttl=%d", ttl)
	}
}
func freeAddr009(t *testing.T) string {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	a := ln.Addr().String()
	ln.Close()
	return a
}
