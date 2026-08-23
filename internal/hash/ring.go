package hash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Ring stores sorted CRC32 points and a reverse owner map. Virtual replicas
// smooth uneven key distributions while preserving consistent-hash behavior:
// adding a node only changes the interval immediately preceding its points.
// Rebuilding after removal is deterministic because node names and replica
// indexes are the complete source of point identities. Empty rings return an
// empty owner instead of panicking, which simplifies bootstrap and shutdown.

type Ring struct {
	replicas int
	points   []uint32
	owners   map[uint32]string
	nodes    map[string]bool
}

func New(replicas int) *Ring {
	if replicas < 1 {
		replicas = 160
	}
	return &Ring{replicas: replicas, owners: map[uint32]string{}, nodes: map[string]bool{}}
}
func (r *Ring) Add(node string) {
	if r.nodes[node] {
		return
	}
	r.nodes[node] = true
	for i := 0; i < r.replicas; i++ {
		p := crc32.ChecksumIEEE([]byte(node + ":" + strconv.Itoa(i)))
		r.points = append(r.points, p)
		r.owners[p] = node
	}
	sort.Slice(r.points, func(i, j int) bool { return r.points[i] < r.points[j] })
}
func (r *Ring) Remove(node string) {
	if !r.nodes[node] {
		return
	}
	delete(r.nodes, node)
	r.points = nil
	r.owners = map[uint32]string{}
	for n := range r.nodes {
		for i := 0; i < r.replicas; i++ {
			p := crc32.ChecksumIEEE([]byte(n + ":" + strconv.Itoa(i)))
			r.points = append(r.points, p)
			r.owners[p] = n
		}
	}
	sort.Slice(r.points, func(i, j int) bool { return r.points[i] < r.points[j] })
}
func (r *Ring) Get(key string) string {
	if len(r.points) == 0 {
		return ""
	}
	p := crc32.ChecksumIEEE([]byte(key))
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i] >= p })
	if i == len(r.points) {
		i = 0
	}
	return r.owners[r.points[i]]
}
func (r *Ring) Nodes() []string {
	out := make([]string, 0, len(r.nodes))
	for n := range r.nodes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
func (r *Ring) Has(node string) bool { return r.nodes[node] }
func (r *Ring) Size() int            { return len(r.points) }
func (r *Ring) Distribution(keys []string) map[string]int {
	out := map[string]int{}
	for n := range r.nodes {
		out[n] = 0
	}
	for _, k := range keys {
		if n := r.Get(k); n != "" {
			out[n]++
		}
	}
	return out
}
func (r *Ring) Owners() map[uint32]string {
	out := make(map[uint32]string, len(r.owners))
	for p, n := range r.owners {
		out[p] = n
	}
	return out
}
func (r *Ring) ReplicaCount() int { return r.replicas }
func (r *Ring) Empty() bool       { return len(r.points) == 0 }
func (r *Ring) LookupMany(keys []string) map[string][]string {
	out := map[string][]string{}
	for _, k := range keys {
		n := r.Get(k)
		out[n] = append(out[n], k)
	}
	return out
}
func (r *Ring) AddMany(nodes []string) {
	for _, n := range nodes {
		r.Add(n)
	}
}
func (r *Ring) RemoveMany(nodes []string) {
	for _, n := range nodes {
		r.Remove(n)
	}
}
func (r *Ring) Rebuild(nodes []string) {
	r.points = nil
	r.owners = map[uint32]string{}
	r.nodes = map[string]bool{}
	r.AddMany(nodes)
}
func (r *Ring) Copy() *Ring {
	x := New(r.replicas)
	for n := range r.nodes {
		x.Add(n)
	}
	return x
}
func (r *Ring) SameOwner(a, b string) bool { return r.Get(a) == r.Get(b) }
func (r *Ring) OwnersFor(keys []string) map[string]string {
	out := map[string]string{}
	for _, k := range keys {
		out[k] = r.Get(k)
	}
	return out
}
func (r *Ring) NodeLoad(keys []string, node string) int {
	n := 0
	for _, k := range keys {
		if r.Get(k) == node {
			n++
		}
	}
	return n
}
func (r *Ring) Balance(keys []string) float64 {
	if len(r.nodes) == 0 {
		return 0
	}
	d := r.Distribution(keys)
	min, max := len(keys), 0
	for _, n := range d {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	if len(keys) == 0 {
		return 0
	}
	return float64(max-min) * 100 / float64(len(keys))
}
