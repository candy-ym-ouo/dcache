package main

import (
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBug008StatusEndpointHasStoreDependency(t *testing.T) {
	tcp := freeAddr008(t)
	httpAddr := freeAddr008(t)
	bin := filepath.Join(t.TempDir(), "dcache")
	if output, e := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); e != nil {
		t.Fatalf("build: %v\n%s", e, output)
	}
	cmd := exec.Command(bin, "-addr", tcp, "-http", httpAddr)
	if e := cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()
	var resp *http.Response
	var e error
	for i := 0; i < 80; i++ {
		resp, e = http.Get("http://" + httpAddr + "/api/status")
		if e == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}
func freeAddr008(t *testing.T) string {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	a := ln.Addr().String()
	ln.Close()
	return a
}
