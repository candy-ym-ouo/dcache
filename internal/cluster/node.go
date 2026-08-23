package cluster

import "fmt"

type Node struct {
	Name, Addr string
	Alive      bool
}

func (n Node) Healthy() bool          { return n.Alive || n.Addr != "" }
func (n Node) ID() string             { return n.Name + "@" + n.Addr }
func (n Node) String() string         { return n.ID() }
func (n Node) IsSelf(other Node) bool { return n.Addr == other.Addr }
func (n Node) Clone() Node            { return Node{Name: n.Name, Addr: n.Addr, Alive: n.Alive} }
func (n *Node) MarkAlive()            { n.Alive = true }
func (n *Node) MarkDead()             { n.Alive = false }
func (n Node) Metadata() map[string]string {
	return map[string]string{"name": n.Name, "addr": n.Addr, "alive": fmt.Sprint(n.Alive)}
}
