package cache

import (
	"testing"
)

func TestBug010SnapshotOwnsValues(t *testing.T) {
	s := New(2)
	defer s.Close()
	if e := s.Set("k", []byte("stable"), 0); e != nil {
		t.Fatal(e)
	}
	items := s.SnapshotItems()
	items[0].Value[0] = 'X'
	v, ok := s.Get("k")
	if !ok || string(v) != "stable" {
		t.Fatalf("value=%q ok=%v", v, ok)
	}
}
