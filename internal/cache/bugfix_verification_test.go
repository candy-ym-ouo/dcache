package cache

import (
	"testing"
	"time"
)

func TestBug004StoreCloseStopsSweeper(t *testing.T) {
	s := New(2)
	s.Close()
	select {
	case <-s.stop:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("sweeper stop signal was not closed")
	}
}
