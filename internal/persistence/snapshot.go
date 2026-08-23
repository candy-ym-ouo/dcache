package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"dcache/internal/cache"
)

const (
	formatVersion = 1
	maxFileSize   = 256 << 20
)

var (
	ErrChecksum          = errors.New("snapshot checksum mismatch")
	ErrUnsupportedFormat = errors.New("unsupported snapshot format")
)

type Metadata struct {
	Version   int
	CreatedAt time.Time
	Items     int
	Bytes     int64
}

type record struct {
	Key       string `json:"key"`
	Value     []byte `json:"value"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

type payload struct {
	Version   int      `json:"version"`
	CreatedAt int64    `json:"created_at"`
	Items     []record `json:"items"`
}

type envelope struct {
	Checksum string          `json:"checksum"`
	Payload  json.RawMessage `json:"payload"`
}

func Save(ctx context.Context, path string, items []cache.Item) (Metadata, error) {
	if path == "" {
		return Metadata{}, fmt.Errorf("snapshot path is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}

	now := time.Now().UTC()
	records := make([]record, 0, len(items))
	for _, item := range items {
		if item.Key == "" || item.Expired(now) {
			continue
		}
		r := record{Key: item.Key, Value: append([]byte(nil), item.Value...)}
		if !item.ExpiresAt.IsZero() {
			r.ExpiresAt = item.ExpiresAt.UnixNano()
		}
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Key < records[j].Key })

	body, err := json.Marshal(payload{Version: formatVersion, CreatedAt: now.UnixNano(), Items: records})
	if err != nil {
		return Metadata{}, fmt.Errorf("encode snapshot payload: %w", err)
	}
	sum := sha256.Sum256(body)
	data, err := json.Marshal(envelope{Checksum: hex.EncodeToString(sum[:]), Payload: body})
	if err != nil {
		return Metadata{}, fmt.Errorf("encode snapshot envelope: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	if err := writeAtomic(path, data); err != nil {
		return Metadata{}, err
	}
	return Metadata{Version: formatVersion, CreatedAt: now, Items: len(records), Bytes: int64(len(data))}, nil
}

func Load(ctx context.Context, path string) ([]cache.Item, Metadata, error) {
	if path == "" {
		return nil, Metadata{}, fmt.Errorf("snapshot path is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, Metadata{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("stat snapshot: %w", err)
	}
	if info.Size() > maxFileSize {
		return nil, Metadata{}, fmt.Errorf("snapshot exceeds %d bytes", maxFileSize)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("read snapshot: %w", err)
	}
	if int64(len(data)) > maxFileSize {
		return nil, Metadata{}, fmt.Errorf("snapshot exceeds %d bytes", maxFileSize)
	}
	if err := ctx.Err(); err != nil {
		return nil, Metadata{}, err
	}

	var outer envelope
	if err := json.Unmarshal(data, &outer); err != nil {
		return nil, Metadata{}, fmt.Errorf("decode snapshot envelope: %w", err)
	}
	expected, err := hex.DecodeString(outer.Checksum)
	if err != nil || len(expected) != sha256.Size {
		return nil, Metadata{}, ErrChecksum
	}
	actual := sha256.Sum256(outer.Payload)
	if !equalBytes(expected, actual[:]) {
		return nil, Metadata{}, ErrChecksum
	}

	var body payload
	if err := json.Unmarshal(outer.Payload, &body); err != nil {
		return nil, Metadata{}, fmt.Errorf("decode snapshot payload: %w", err)
	}
	if body.Version != formatVersion {
		return nil, Metadata{}, fmt.Errorf("%w: version %d", ErrUnsupportedFormat, body.Version)
	}
	now := time.Now()
	seen := make(map[string]struct{}, len(body.Items))
	items := make([]cache.Item, 0, len(body.Items))
	for _, r := range body.Items {
		if r.Key == "" {
			return nil, Metadata{}, fmt.Errorf("snapshot contains an empty key")
		}
		if _, ok := seen[r.Key]; ok {
			return nil, Metadata{}, fmt.Errorf("snapshot contains duplicate key %q", r.Key)
		}
		seen[r.Key] = struct{}{}
		item := cache.Item{Key: r.Key, Value: append([]byte(nil), r.Value...)}
		if r.ExpiresAt != 0 {
			item.ExpiresAt = time.Unix(0, r.ExpiresAt)
			if item.Expired(now) {
				continue
			}
		}
		items = append(items, item)
	}
	createdAt := time.Unix(0, body.CreatedAt).UTC()
	return items, Metadata{Version: body.Version, CreatedAt: createdAt, Items: len(items), Bytes: info.Size()}, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".dcache-snapshot-*")
	if err != nil {
		return fmt.Errorf("create temporary snapshot: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("set snapshot permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync snapshot: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close snapshot: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	committed = true
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
