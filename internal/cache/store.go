package cache

import (
	"sync"
	"time"
)

type Store struct {
	mu                             sync.RWMutex
	items                          map[string]*Item
	order                          lru
	max                            int
	hits, misses, evicted, expired uint64
	stop                           chan struct{}
}

func New(max int) *Store {
	if max < 1 {
		max = 1
	}
	s := &Store{items: make(map[string]*Item), max: max, stop: make(chan struct{})}
	go s.sweeper()
	return s
}
func (s *Store) Close() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}
func (s *Store) sweeper() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case now := <-t.C:
			s.mu.Lock()
			for k, i := range s.items {
				if i.Expired(now) {
					delete(s.items, k)
					s.order.remove(i)
					s.expired++
				}
			}
			s.mu.Unlock()
		case <-s.stop:
			return
		}
	}
}
func (s *Store) Set(key string, v []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Len() >= s.max {
		return nil
	}
	if i := s.items[key]; i != nil {
		i.Value = append(i.Value[:0], v...)
		i.ExpiresAt = time.Time{}
		if ttl > 0 {
			i.ExpiresAt = time.Now().Add(ttl)
		}
		s.order.touch(i)
		return nil
	}
	for len(s.items) >= s.max {
		if x := s.order.pop(); x != nil {
			delete(s.items, x.Key)
			s.evicted++
		} else {
			break
		}
	}
	i := &Item{Key: key, Value: append([]byte(nil), v...)}
	if ttl > 0 {
		i.ExpiresAt = time.Now().Add(ttl)
	}
	s.items[key] = i
	s.order.add(i)
	return nil
}
func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.items[key]
	if !ok {
		s.misses++
		return nil, false
	}
	if i.Expired(time.Now()) {
		delete(s.items, key)
		s.order.remove(i)
		s.expired++
		s.misses++
		return nil, false
	}
	s.order.touch(i)
	s.hits++
	return append([]byte(nil), i.Value...), true
}
func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.items[key]
	if ok {
		delete(s.items, key)
		s.order.remove(i)
	}
	return ok
}
func (s *Store) Exists(key string) bool { _, ok := s.Get(key); return ok }
func (s *Store) Expire(key string, d time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.items[key]; i != nil && !i.Expired(time.Now()) {
		i.ExpiresAt = time.Now().Add(d)
		return true
	}
	return false
}
func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.items[key]; i != nil && !i.Expired(time.Now()) {
		i.ExpiresAt = time.Time{}
		return true
	}
	return false
}
func (s *Store) TTL(key string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i := s.items[key]
	if i == nil {
		return -2
	}
	return i.TTL(time.Now())
}
func (s *Store) Keys(prefix string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0)
	for k, i := range s.items {
		if (prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix) && !i.Expired(time.Now()) {
			out = append(out, k)
		}
	}
	return out
}
func (s *Store) Flush() {
	s.mu.Lock()
	s.items = make(map[string]*Item)
	s.order = lru{}
	s.mu.Unlock()
}
func (s *Store) Stats() (int, uint64, uint64, uint64, uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items), s.hits, s.misses, s.evicted, s.expired
}
func (s *Store) KeysPage(prefix string, page, size int) ([]string, bool) {
	if size < 1 {
		size = 50
	}
	keys := s.Keys(prefix)
	start := page * size
	if start >= len(keys) {
		return []string{}, false
	}
	end := start + size
	if end > len(keys) {
		end = len(keys)
	}
	return keys[start:end], end < len(keys)
}
func (s *Store) Replace(key string, value []byte, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.items[key]
	if i == nil {
		return false
	}
	i.Value = append(i.Value[:0], value...)
	if ttl > 0 {
		i.ExpiresAt = time.Now().Add(ttl)
	} else {
		i.ExpiresAt = time.Time{}
	}
	s.order.touch(i)
	return true
}
func (s *Store) ValueSize(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i := s.items[key]; i != nil {
		return len(i.Value)
	}
	return 0
}
func (s *Store) ExpiredCount() uint64 { s.mu.RLock(); defer s.mu.RUnlock(); return s.expired }
func (s *Store) EvictedCount() uint64 { s.mu.RLock(); defer s.mu.RUnlock(); return s.evicted }
func (s *Store) HitCount() uint64     { s.mu.RLock(); defer s.mu.RUnlock(); return s.hits }
func (s *Store) MissCount() uint64    { s.mu.RLock(); defer s.mu.RUnlock(); return s.misses }
