package cache

import "time"

func (s *Store) SetTTLSeconds(key string, seconds int64) bool {
	if seconds < 0 {
		return false
	}
	return s.Expire(key, time.Duration(seconds)*time.Second)
}

func (s *Store) ExpireAt(key string, at time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.items[key]; i != nil && !i.Expired(time.Now()) {
		i.ExpiresAt = at
		return true
	}
	return false
}
func (s *Store) ClearExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	now := time.Now()
	for k, i := range s.items {
		if i.Expired(now) {
			delete(s.items, k)
			s.order.remove(i)
			s.expired++
			n++
		}
	}
	return n
}
func (s *Store) ExpiringKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []string{}
	now := time.Now()
	for k, i := range s.items {
		if !i.ExpiresAt.IsZero() && !i.Expired(now) {
			out = append(out, k)
		}
	}
	return out
}
