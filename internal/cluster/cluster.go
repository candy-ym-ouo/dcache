package cluster

import (
	"dcache/internal/hash"
	"sync"
)

type Cluster struct {
	mu    sync.RWMutex
	self  Node
	nodes map[string]Node
	ring  *hash.Ring
}

func New(name, addr string) *Cluster {
	c := &Cluster{self: Node{name, addr, true}, nodes: nil, ring: hash.New(160)}
	c.nodes[addr] = c.self
	c.ring.Add(addr)
	return c
}
func (c *Cluster) Self() Node { c.mu.RLock(); defer c.mu.RUnlock(); return c.self }
func (c *Cluster) Add(n Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[n.Addr] = n
	c.ring.Add(n.Addr)
}
func (c *Cluster) Remove(addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.nodes, addr)
	c.ring.Remove(addr)
}
func (c *Cluster) Owner(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ring.Get(key)
}
func (c *Cluster) Members() []Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		out = append(out, n)
	}
	return out
}
func (c *Cluster) Count() int { c.mu.RLock(); defer c.mu.RUnlock(); return len(c.nodes) }
func (c *Cluster) Contains(addr string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.nodes[addr]
	return ok
}
func (c *Cluster) SetAlive(addr string, alive bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n, ok := c.nodes[addr]; ok {
		n.Alive = alive
		c.nodes[addr] = n
	}
}
func (c *Cluster) Names() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := []string{}
	for _, n := range c.nodes {
		out = append(out, n.Name)
	}
	return out
}
func (c *Cluster) Addresses() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := []string{}
	for a := range c.nodes {
		out = append(out, a)
	}
	return out
}
func (c *Cluster) AddMany(nodes []Node) {
	for _, n := range nodes {
		c.Add(n)
	}
}
func (c *Cluster) RemoveMany(addrs []string) {
	for _, a := range addrs {
		c.Remove(a)
	}
}
func (c *Cluster) Node(addr string) (Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	n, ok := c.nodes[addr]
	return n, ok
}
func (c *Cluster) Upsert(n Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.nodes[n.Addr]; ok && old.Name == n.Name && old.Alive == n.Alive {
		return
	}
	c.nodes[n.Addr] = n
	c.ring.Add(n.Addr)
}
func (c *Cluster) Snapshot() map[string]Node {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := map[string]Node{}
	for a, n := range c.nodes {
		out[a] = n
	}
	return out
}
func (c *Cluster) Route(key string) (Node, bool) { a := c.Owner(key); return c.Node(a) }
func (c *Cluster) HealthyMembers() []Node {
	out := []Node{}
	for _, n := range c.Members() {
		if n.Healthy() {
			out = append(out, n)
		}
	}
	return out
}
func (c *Cluster) AliveAddresses() []string {
	out := []string{}
	for _, n := range c.HealthyMembers() {
		out = append(out, n.Addr)
	}
	return out
}
func (c *Cluster) IsSingle() bool      { return c.Count() == 1 }
func (c *Cluster) SelfAddress() string { return c.Self().Addr }
func (c *Cluster) SelfName() string    { return c.Self().Name }
func (c *Cluster) RingNodes() []string { return c.ring.Nodes() }
