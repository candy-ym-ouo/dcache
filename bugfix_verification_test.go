package main

import (
	"context"
	"dcache/internal/cache"
	"dcache/internal/cluster"
	"dcache/internal/config"
	"dcache/internal/metrics"
	"dcache/internal/server"
	"net"
	"testing"
	"time"
)

func TestBug001ShutdownCountsOnlyLiveConnections(t *testing.T) {
	cfg := config.Default()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := ln.Addr().String()
	ln.Close()
	cfg.Addr = addr
	s := server.New(cfg, cache.New(10), cluster.New("n", addr), metrics.New())
	go s.ListenAndServe()
	var c net.Conn
	for i := 0; i < 30; i++ {
		c, e = net.Dial("tcp", addr)
		if e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
	time.Sleep(30 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if e = s.Shutdown(ctx); e != nil {
		t.Fatalf("shutdown: %v", e)
	}
}
