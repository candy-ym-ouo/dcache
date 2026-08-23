package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dcache/internal/cache"
	"dcache/internal/cluster"
	"dcache/internal/config"
	"dcache/internal/metrics"
	"dcache/internal/server"
	webpkg "dcache/internal/web"
)

func main() {
	cfg := config.Default()
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "TCP listen address")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "node name")
	flag.StringVar(&cfg.HTTPAddr, "http", cfg.HTTPAddr, "HTTP listen address")
	flag.IntVar(&cfg.MaxKeys, "max-keys", cfg.MaxKeys, "maximum keys")
	flag.Parse()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	store := cache.New(cfg.MaxKeys)
	stats := metrics.New()
	group := cluster.New(cfg.Name, cfg.Addr)
	tcp := server.New(cfg, store, group, stats)
	httpSrv := webpkg.New(cfg.HTTPAddr, store, group, stats)
	go func() {
		if err := tcp.ListenAndServe(); err != nil {
			log.Printf("tcp: %v", err)
		}
	}()
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil {
			log.Printf("http: %v", err)
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	ctx = context.Background()
	_ = tcp.Shutdown(ctx)
	_ = httpSrv.Shutdown(ctx)
	store.Close()
}
