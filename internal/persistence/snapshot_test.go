package persistence

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dcache/internal/cache"
)

func TestSnapshotRoundTripPreservesValuesAndExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.snapshot")
	expires := time.Now().Add(10 * time.Minute).Truncate(time.Millisecond)
	items := []cache.Item{
		{Key: "persistent", Value: []byte("alpha")},
		{Key: "temporary", Value: []byte("beta"), ExpiresAt: expires},
		{Key: "expired", Value: []byte("old"), ExpiresAt: time.Now().Add(-time.Second)},
	}
	meta, err := Save(context.Background(), path, items)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Items != 2 || meta.Bytes == 0 {
		t.Fatalf("unexpected save metadata: %+v", meta)
	}
	loaded, loadedMeta, err := Load(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if loadedMeta.Items != 2 || len(loaded) != 2 {
		t.Fatalf("unexpected loaded items: %+v", loadedMeta)
	}
	if loaded[0].Key != "persistent" || string(loaded[0].Value) != "alpha" {
		t.Fatalf("unexpected persistent item: %+v", loaded[0])
	}
	if loaded[1].Key != "temporary" || string(loaded[1].Value) != "beta" || !loaded[1].ExpiresAt.Equal(expires) {
		t.Fatalf("unexpected temporary item: %+v", loaded[1])
	}
}

func TestSnapshotRejectsTamperedPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.snapshot")
	_, err := Save(context.Background(), path, []cache.Item{{Key: "key", Value: []byte("value")}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte("dmFsdWU="), []byte("eGFsdWU="), 1)
	if bytes.Equal(data, tampered) {
		t.Fatal("test payload was not changed")
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = Load(context.Background(), path)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("expected checksum error, got %v", err)
	}
}

func TestManagerRestoresAndSerializesConcurrentSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cache.snapshot")
	source := cache.New(10)
	defer source.Close()
	if err := source.Set("one", []byte("1"), 0); err != nil {
		t.Fatal(err)
	}
	manager, err := New(source, path, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := manager.Save(context.Background()); err != nil {
				t.Errorf("save: %v", err)
			}
		}()
	}
	wg.Wait()
	if status := manager.Status(); status.Saves != 8 || status.LastError != "" {
		t.Fatalf("unexpected manager status: %+v", status)
	}

	target := cache.New(10)
	defer target.Close()
	restorer, err := New(target, path, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := restorer.Restore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	value, ok := target.Get("one")
	if meta.Items != 1 || !ok || string(value) != "1" {
		t.Fatalf("restore failed: meta=%+v value=%q ok=%v", meta, value, ok)
	}
}

func TestManagerRunSavesOnIntervalAndStops(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.snapshot")
	store := cache.New(10)
	defer store.Close()
	_ = store.Set("key", []byte("value"), 0)
	manager, err := New(store, path, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for manager.Status().Saves == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.Status().Saves == 0 {
		t.Fatal("periodic snapshot was not saved")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if manager.Status().Running {
		t.Fatal("manager still reports running")
	}
}
