package cluster

import "sync/atomic"

type Migration struct{ total, done uint64 }

func (m *Migration) Start(total uint64) {
	atomic.StoreUint64(&m.total, total)
	atomic.StoreUint64(&m.done, 0)
}
func (m *Migration) Advance() { atomic.AddUint64(&m.done, 1) }
func (m *Migration) Progress() (uint64, uint64) {
	return atomic.LoadUint64(&m.total), atomic.LoadUint64(&m.done)
}
func (m *Migration) Reset()         { atomic.StoreUint64(&m.total, 0); atomic.StoreUint64(&m.done, 0) }
func (m *Migration) Complete() bool { t, d := m.Progress(); return t > 0 && d >= t }
func (m *Migration) Percent() float64 {
	t, d := m.Progress()
	if t == 0 {
		return 100
	}
	return float64(d) * 100 / float64(t)
}
func (m *Migration) Remaining() uint64 {
	t, d := m.Progress()
	if d >= t {
		return 0
	}
	return t - d
}
func (m *Migration) Started() bool      { t, _ := m.Progress(); return t > 0 }
func (m *Migration) AddTotal(n uint64)  { atomic.AddUint64(&m.total, n) }
func (m *Migration) AdvanceBy(n uint64) { atomic.AddUint64(&m.done, n) }
func (m *Migration) State() string {
	if !m.Started() {
		return "IDLE"
	}
	if m.Complete() {
		return "DONE"
	}
	return "MIGRATING"
}
