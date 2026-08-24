package main

import (
	"dcache/internal/protocol"
	"testing"
)

func TestBug003ResponseCloneOwnsPayload(t *testing.T) {
	original := protocol.Response{Value: []byte("stable")}
	clone := original.Clone()
	clone.Value[0] = 'X'
	if string(original.Value) != "stable" {
		t.Fatalf("clone mutation changed original: %q", original.Value)
	}
}
