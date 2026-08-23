package cache

// lru is protected by Store.mu. The head is most recently used and the tail
// is the eviction candidate. Removing an item always clears its links, which
// prevents stale pointers when an expired item is deleted by the sweeper.

type lru struct {
	head, tail *Item
	size       int
}

func (l *lru) add(i *Item) {
	i.prev = nil
	i.next = l.head
	if l.head != nil {
		l.head.prev = i
	} else {
		l.tail = i
	}
	l.head = i
	l.size++
}
func (l *lru) remove(i *Item) {
	if i.prev != nil {
		i.prev.next = i.next
	} else {
		l.head = i.next
	}
	if i.next != nil {
		i.next.prev = i.prev
	} else {
		l.tail = i.prev
	}
	i.prev = nil
	i.next = nil
	l.size--
}
func (l *lru) touch(i *Item) {
	if l.head == i {
		return
	}
	l.remove(i)
	l.add(i)
}
func (l *lru) pop() *Item {
	if l.tail == nil {
		return nil
	}
	i := l.tail
	l.remove(i)
	return i
}
func (l *lru) front() *Item { return l.head }
func (l *lru) back() *Item  { return l.tail }
func (l *lru) Len() int     { return l.size }
func (l *lru) clear()       { l.head = nil; l.tail = nil; l.size = 0 }
func (l *lru) values() []string {
	out := []string{}
	for i := l.head; i != nil; i = i.next {
		out = append(out, i.Key)
	}
	return out
}
