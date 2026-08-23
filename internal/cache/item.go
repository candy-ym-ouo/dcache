package cache

import "time"

type Item struct {
	Key        string
	Value      []byte
	ExpiresAt  time.Time
	prev, next *Item
}

func (i *Item) Expired(now time.Time) bool { return !i.ExpiresAt.IsZero() && !now.Before(i.ExpiresAt) }
func (i *Item) TTL(now time.Time) int64 {
	if i.ExpiresAt.IsZero() {
		return -1
	}
	d := time.Until(i.ExpiresAt)
	if d <= 0 {
		return -2
	}
	return int64(d / time.Second)
}
func (i *Item) Clone() Item {
	return Item{Key: i.Key, Value: append([]byte(nil), i.Value...), ExpiresAt: i.ExpiresAt}
}
func (i *Item) Size() int        { return len(i.Key) + len(i.Value) }
func (i *Item) Persistent() bool { return i.ExpiresAt.IsZero() }
func (i *Item) Remaining(now time.Time) time.Duration {
	if i.Persistent() {
		return 0
	}
	d := time.Until(i.ExpiresAt)
	if d < 0 {
		return 0
	}
	return d
}
func (i *Item) ValueString() string { return string(i.Value) }
func (i *Item) KeyValue() string    { return i.Key + "=" + string(i.Value) }
func (i *Item) WithTTL(ttl time.Duration) Item {
	n := i.Clone()
	if ttl > 0 {
		n.ExpiresAt = time.Now().Add(ttl)
	} else {
		n.ExpiresAt = time.Time{}
	}
	return n
}
