package cache

// Store combines a map for O(1) lookup with the LRU list for bounded memory.
// All public operations copy byte slices so callers cannot mutate entries
// behind the store's locks. The sweeper performs lazy-safe expiry cleanup.
import (
	"sort"
	"strconv"
	"strings"
	"time"
)

type Snapshot struct {
	Keys                           int
	Hits, Misses, Evicted, Expired uint64
}

// PutIfAbsent stores a value only when key is not present.
func (s *Store) PutIfAbsent(key string, value []byte, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[key]; ok {
		return false
	}
	for len(s.items) >= s.max {
		if old := s.order.pop(); old != nil {
			delete(s.items, old.Key)
			s.evicted++
		} else {
			break
		}
	}
	i := &Item{Key: key, Value: append([]byte(nil), value...)}
	if ttl > 0 {
		i.ExpiresAt = time.Now().Add(ttl)
	}
	s.items[key] = i
	s.order.add(i)
	return true
}

// Increment changes an integer value atomically and returns the new number.
func (s *Store) Increment(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	if i := s.items[key]; i != nil && !i.Expired(time.Now()) {
		var err error
		n, err = strconv.ParseInt(string(i.Value), 10, 64)
		if err != nil {
			return 0, err
		}
		n += delta
		i.Value = []byte(strconv.FormatInt(n, 10))
		s.order.touch(i)
		return n, nil
	}
	n = delta
	if len(s.items) >= s.max {
		if old := s.order.pop(); old != nil {
			delete(s.items, old.Key)
			s.evicted++
		}
	}
	i := &Item{Key: key, Value: []byte(strconv.FormatInt(n, 10))}
	s.items[key] = i
	s.order.add(i)
	return n, nil
}

// Len reports the number of live entries.
func (s *Store) Len() int { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.items) }

// SnapshotItems copies live entries for migration or inspection.
func (s *Store) SnapshotItems() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Item, 0, len(s.items))
	now := time.Now()
	for _, i := range s.items {
		if !i.Expired(now) {
			out = append(out, Item{Key: i.Key, Value: i.Value, ExpiresAt: i.ExpiresAt})
		}
	}
	return out
}

// DeletePrefix removes every key beginning with prefix.
func (s *Store) DeletePrefix(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for k, i := range s.items {
		if strings.HasPrefix(k, prefix) {
			delete(s.items, k)
			s.order.remove(i)
			n++
		}
	}
	return n
}

// Touch refreshes LRU order without changing value or TTL.
func (s *Store) Touch(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.items[key]; i != nil && !i.Expired(time.Now()) {
		s.order.touch(i)
		return true
	}
	return false
}
func (s *Store) SetMany(values map[string][]byte) int {
	n := 0
	for k, v := range values {
		if s.Set(k, v, 0) == nil {
			n++
		}
	}
	return n
}
func (s *Store) DeleteMany(keys []string) int {
	n := 0
	for _, k := range keys {
		if s.Del(k) {
			n++
		}
	}
	return n
}
func (s *Store) GetMany(keys []string) map[string][]byte {
	out := map[string][]byte{}
	for _, k := range keys {
		if v, ok := s.Get(k); ok {
			out[k] = v
		}
	}
	return out
}
func (s *Store) SizeBytes() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, i := range s.items {
		n += i.Size()
	}
	return n
}
func (s *Store) Capacity() int { return s.max }
func (s *Store) Usage() float64 {
	if s.max == 0 {
		return 0
	}
	return float64(s.Len()) * 100 / float64(s.max)
}
func (s *Store) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.items[key]
	return ok && !i.Expired(time.Now())
}
func (s *Store) Copy(key string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.items[key]
	if !ok || i.Expired(time.Now()) {
		return Item{}, false
	}
	return i.Clone(), true
}
func (s *Store) KeysSorted(prefix string) []string {
	keys := s.Keys(prefix)
	sort.Strings(keys)
	return keys
}
func (s *Store) ForEach(fn func(Item) bool) {
	for _, i := range s.SnapshotItems() {
		if !fn(i) {
			return
		}
	}
}
func (s *Store) Update(key string, fn func([]byte) []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.items[key]
	if !ok || i.Expired(time.Now()) {
		return false
	}
	i.Value = fn(append([]byte(nil), i.Value...))
	s.order.touch(i)
	return true
}
func (s *Store) GetWithTTL(key string) ([]byte, int64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.items[key]
	if !ok || i.Expired(time.Now()) {
		return nil, -2, false
	}
	s.order.touch(i)
	s.hits++
	return append([]byte(nil), i.Value...), i.TTL(time.Now()), true
}
func (s *Store) CountPrefix(prefix string) int { return len(s.Keys(prefix)) }
func (s *Store) AllValues() [][]byte {
	items := s.SnapshotItems()
	out := make([][]byte, 0, len(items))
	for _, i := range items {
		out = append(out, i.Value)
	}
	return out
}
func (s *Store) KeysBySize(limit int) []string {
	out := []string{}
	for _, i := range s.SnapshotItems() {
		if i.Size() <= limit {
			out = append(out, i.Key)
		}
	}
	return out
}
func (s *Store) RemoveExpiredNow() int { return s.ClearExpired() }

func (s *Store) Snapshot() Snapshot { k, h, m, e, x := s.Stats(); return Snapshot{k, h, m, e, x} }
