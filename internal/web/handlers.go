package web

import (
	"encoding/json"
	"net/http"
)

// dependenciesReady reports whether the injected collaborators are wired. It
// is the last line of defense against a nil-pointer panic in a handler when a
// Server was assembled without going through New (for example a struct-literal
// in an embedded app or a test). Such a request fails cleanly with a 503
// rather than crashing the process.
func (s *Server) dependenciesReady(w http.ResponseWriter) bool {
	if s == nil || s.store == nil || s.cluster == nil || s.metrics == nil {
		http.Error(w, "service dependencies unavailable", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	k, h, m, e, x := s.store.Stats()
	req, _, _ := s.metrics.Snapshot()
	json.NewEncoder(w).Encode(map[string]any{"node": s.cluster.Self(), "members": s.cluster.Members(), "keys": k, "hits": h, "misses": m, "evicted": e, "expired": x, "requests": req})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) members(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	writeJSON(w, s.cluster.Members())
}
func (s *Server) keys(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	prefix := r.URL.Query().Get("prefix")
	writeJSON(w, map[string]any{"keys": s.store.Keys(prefix)})
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	writeJSON(w, map[string]any{"ok": true, "node": s.cluster.Self().ID()})
}
func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/members", s.members)
	mux.HandleFunc("/api/keys", s.keys)
	mux.HandleFunc("/healthz", s.health)
}
func (s *Server) metricsJSON() map[string]any {
	d := s.metrics.Details()
	return map[string]any{"requests": d.Requests, "hits": d.Hits, "misses": d.Misses, "hitRate": d.HitRate, "missRate": d.MissRate, "totalLatencyMs": d.TotalLatency.Milliseconds(), "averageLatencyMs": d.AverageLatency.Milliseconds(), "maxLatencyMs": d.MaxLatency.Milliseconds()}
}
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	writeJSON(w, s.metricsJSON())
}
func (s *Server) topology(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	nodes := s.cluster.Members()
	writeJSON(w, map[string]any{"count": len(nodes), "nodes": nodes})
}
func (s *Server) resetMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	s.metrics.Reset()
	writeJSON(w, map[string]bool{"ok": true})
}
func (s *Server) flush(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	s.store.Flush()
	writeJSON(w, map[string]bool{"ok": true})
}
func (s *Server) deletePrefix(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	n := s.store.DeletePrefix(r.URL.Query().Get("prefix"))
	writeJSON(w, map[string]int{"deleted": n})
}
func (s *Server) capacity(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	writeJSON(w, map[string]any{"keys": s.store.Len(), "capacity": s.store.Capacity(), "usage": s.store.Usage(), "bytes": s.store.SizeBytes()})
}
func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	writeJSON(w, map[string]any{"alive": true, "members": s.cluster.HealthyCount()})
}
func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	k, h, m, e, x := s.store.Stats()
	writeJSON(w, map[string]any{"keys": k, "hits": h, "misses": m, "evicted": e, "expired": x, "members": s.cluster.Count()})
}
func (s *Server) nodeInfo(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	n := s.cluster.Self()
	writeJSON(w, map[string]any{"name": n.Name, "addr": n.Addr, "alive": n.Alive})
}
func (s *Server) keyExists(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	key := r.URL.Query().Get("key")
	writeJSON(w, map[string]bool{"exists": s.store.Has(key)})
}
func (s *Server) keyTTL(w http.ResponseWriter, r *http.Request) {
	if !s.dependenciesReady(w) {
		return
	}
	key := r.URL.Query().Get("key")
	writeJSON(w, map[string]int64{"ttl": s.store.TTL(key)})
}
func (s *Server) migration(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"state": "IDLE", "done": 0, "total": 0})
}
func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"service": "dcache", "version": "1.0"})
}
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
}
