package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"dcache/internal/cache"
)

type Status struct {
	Path           string
	Interval       time.Duration
	LastSavedAt    time.Time
	LastRestoredAt time.Time
	LastError      string
	Saves          uint64
	RestoredItems  int
	Running        bool
}

type Manager struct {
	store    *cache.Store
	path     string
	interval time.Duration
	saveMu   sync.Mutex
	mu       sync.RWMutex
	status   Status
}

func New(store *cache.Store, path string, interval time.Duration) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if path == "" {
		return nil, fmt.Errorf("snapshot path is required")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("snapshot interval must be positive")
	}
	m := &Manager{store: store, path: path, interval: interval}
	m.status.Path = path
	m.status.Interval = interval
	return m, nil
}

func (m *Manager) Save(ctx context.Context) (Metadata, error) {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	meta, err := Save(ctx, m.path, m.store.SnapshotItems())
	m.mu.Lock()
	if err != nil {
		m.status.LastError = err.Error()
	} else {
		m.status.LastSavedAt = meta.CreatedAt
		m.status.LastError = ""
		m.status.Saves++
	}
	m.mu.Unlock()
	return meta, err
}

func (m *Manager) Restore(ctx context.Context) (Metadata, error) {
	items, meta, err := Load(ctx, m.path)
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, nil
	}
	if err != nil {
		m.setError(err)
		return Metadata{}, err
	}
	restored, skipped := m.store.RestoreSnapshot(items)
	meta.Items = restored
	m.mu.Lock()
	m.status.LastRestoredAt = time.Now().UTC()
	m.status.RestoredItems = restored
	m.status.LastError = ""
	m.mu.Unlock()
	if skipped > 0 {
		return meta, fmt.Errorf("snapshot contains %d items beyond store capacity", skipped)
	}
	return meta, nil
}

func (m *Manager) Run(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	m.setRunning(true)
	defer m.setRunning(false)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			_, _ = m.Save(ctx)
		}
	}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setError(err error) {
	m.mu.Lock()
	m.status.LastError = err.Error()
	m.mu.Unlock()
}

func (m *Manager) setRunning(running bool) {
	m.mu.Lock()
	m.status.Running = running
	m.mu.Unlock()
}
