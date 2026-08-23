package metrics

import "sync/atomic"

// Metrics uses atomics because request handlers run concurrently. Snapshots
// are approximate by design, but every individual counter update is visible
// without taking the cache mutex. HitRate is expressed as a percentage and
// returns zero when no requests have been observed. Reset is useful for the
// small administration endpoint and for repeatable demonstrations.

type Metrics struct{ requests, hits, misses uint64 }

func New() *Metrics         { return &Metrics{} }
func (m *Metrics) Request() { atomic.AddUint64(&m.requests, 1) }
func (m *Metrics) Hit()     { atomic.AddUint64(&m.hits, 1) }
func (m *Metrics) Miss()    { atomic.AddUint64(&m.misses, 1) }
func (m *Metrics) Snapshot() (uint64, uint64, uint64) {
	return atomic.LoadUint64(&m.requests), atomic.LoadUint64(&m.hits), atomic.LoadUint64(&m.misses)
}
func (m *Metrics) HitRate() float64 {
	r, h, _ := m.Snapshot()
	if r == 0 {
		return 0
	}
	return float64(h) * 100 / float64(r)
}
func (m *Metrics) MissRate() float64 {
	r, _, miss := m.Snapshot()
	if r == 0 {
		return 0
	}
	return float64(miss) * 100 / float64(r)
}
func (m *Metrics) Reset() {
	atomic.StoreUint64(&m.requests, 0)
	atomic.StoreUint64(&m.hits, 0)
	atomic.StoreUint64(&m.misses, 0)
}

type Snapshot struct {
	Requests, Hits, Misses uint64
	HitRate, MissRate      float64
}

func (m *Metrics) Details() Snapshot {
	r, h, miss := m.Snapshot()
	return Snapshot{r, h, miss, m.HitRate(), m.MissRate()}
}
func (m *Metrics) Add(requests, hits, misses uint64) {
	atomic.AddUint64(&m.requests, requests)
	atomic.AddUint64(&m.hits, hits)
	atomic.AddUint64(&m.misses, misses)
}
