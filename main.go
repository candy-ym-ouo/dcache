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
	"dcache/internal/persistence"
	"dcache/internal/server"
	webpkg "dcache/internal/web"
)

func main() {
	cfg := config.Default()
	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "TCP listen address")
	flag.StringVar(&cfg.Name, "name", cfg.Name, "node name")
	flag.StringVar(&cfg.HTTPAddr, "http", cfg.HTTPAddr, "HTTP listen address")
	flag.IntVar(&cfg.MaxKeys, "max-keys", cfg.MaxKeys, "maximum keys")
	flag.StringVar(&cfg.SnapshotPath, "snapshot", cfg.SnapshotPath, "cache snapshot file (disabled when empty)")
	flag.DurationVar(&cfg.SnapshotInterval, "snapshot-interval", cfg.SnapshotInterval, "automatic snapshot interval")
	flag.Parse()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	store := cache.New(cfg.MaxKeys)
	var snapshots *persistence.Manager
	snapshotCtx, stopSnapshots := context.WithCancel(context.Background())
	defer stopSnapshots()
	if cfg.SnapshotPath != "" {
		var err error
		snapshots, err = persistence.New(store, cfg.SnapshotPath, cfg.SnapshotInterval)
		if err != nil {
			log.Fatal(err)
		}
		if meta, err := snapshots.Restore(snapshotCtx); err != nil {
			log.Printf("snapshot restore: %v", err)
		} else if meta.Items > 0 {
			log.Printf("restored %d keys from snapshot", meta.Items)
		}
		go func() {
			if err := snapshots.Run(snapshotCtx); err != nil {
				log.Printf("snapshot manager: %v", err)
			}
		}()
	}
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
	_ = tcp.Shutdown(ctx)
	tcp.Wait()
	_ = httpSrv.Shutdown(ctx)
	stopSnapshots()
	if snapshots != nil {
		saveCtx, cancelSave := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		if _, err := snapshots.Save(saveCtx); err != nil {
			log.Printf("final snapshot: %v", err)
		}
		cancelSave()
	}
	store.Close()
}
