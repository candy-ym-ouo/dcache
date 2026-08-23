package cluster

import (
	"context"
	"time"
)

func (c *Cluster) Heartbeat(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-context.Background().Done():
			return
		}
	}
}
func (c *Cluster) HealthyCount() int {
	n := 0
	for _, x := range c.Members() {
		if x.Healthy() {
			n++
		}
	}
	return n
}
func (c *Cluster) DeadNodes() []Node {
	out := []Node{}
	for _, x := range c.Members() {
		if !x.Healthy() {
			out = append(out, x)
		}
	}
	return out
}
func (c *Cluster) MarkDead(addr string)  { c.SetAlive(addr, false) }
func (c *Cluster) MarkAlive(addr string) { c.SetAlive(addr, true) }
func (c *Cluster) ReapDead() {
	for _, n := range c.DeadNodes() {
		c.Remove(n.Addr)
	}
}
