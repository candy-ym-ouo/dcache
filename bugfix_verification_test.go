package main

import (
	"bufio"
	"dcache/internal/client"
	"dcache/internal/protocol"
	"net"
	"testing"
)

func TestBug002ClientPreservesNonNotFoundResponse(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c == nil {
			return
		}
		defer c.Close()
		req, e := protocol.DecodeRequest(bufio.NewReader(c))
		if e == nil {
			_ = (protocol.Response{Seq: req.Seq, Code: protocol.ErrBadArg, Value: []byte("bad argument")}).Encode(c)
		}
	}()
	v, e := client.New(ln.Addr().String()).Get("k")
	if e != nil {
		t.Fatal(e)
	}
	if v != "bad argument" {
		t.Fatalf("response value=%q", v)
	}
}
