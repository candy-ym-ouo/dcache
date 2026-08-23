package metrics

import (
	"sync/atomic"
	"time"
)

// Metrics uses atomics because request handlers run concurrently. Snapshots
// are approximate by design, but every individual counter update is visible
// without taking the cache mutex. HitRate is expressed as a percentage and
// returns zero when no requests have been observed. Reset is useful for the
// small administration endpoint and for repeatable demonstrations.

type Metrics struct{ requests, hits, misses, latencyNs, maxLatencyNs uint64 }

func New() *Metrics         { return &Metrics{} }
func (m *Metrics) Request() { atomic.AddUint64(&m.requests, 1) }
func (m *Metrics) Hit()     { atomic.AddUint64(&m.hits, 1) }
func (m *Metrics) Miss()    { atomic.AddUint64(&m.misses, 1) }
func (m *Metrics) Observe(d time.Duration) {
	if d < 0 {
		d = 0
	}
	ns := uint64(d)
	atomic.AddUint64(&m.latencyNs, ns)
	for {
		old := atomic.LoadUint64(&m.maxLatencyNs)
		if ns <= old || atomic.CompareAndSwapUint64(&m.maxLatencyNs, old, ns) {
			break
		}
	}
}
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
	m.requests = 0
	m.hits = 0
	m.misses = 0
	atomic.StoreUint64(&m.latencyNs, 0)
	atomic.StoreUint64(&m.maxLatencyNs, 0)
}

type Snapshot struct {
	Requests, Hits, Misses                   uint64
	HitRate, MissRate                        float64
	TotalLatency, MaxLatency, AverageLatency time.Duration
}

func (m *Metrics) Details() Snapshot {
	r, h, miss := m.Snapshot()
	total := time.Duration(atomic.LoadUint64(&m.latencyNs))
	max := time.Duration(atomic.LoadUint64(&m.maxLatencyNs))
	avg := time.Duration(0)
	if r > 0 {
		avg = total / time.Duration(r)
	}
	return Snapshot{r, h, miss, m.HitRate(), m.MissRate(), total, max, avg}
}
func (m *Metrics) Add(requests, hits, misses uint64) {
	atomic.AddUint64(&m.requests, requests)
	atomic.AddUint64(&m.hits, hits)
	atomic.AddUint64(&m.misses, misses)
}
