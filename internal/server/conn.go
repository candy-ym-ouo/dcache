package server

// Connections use deadlines on every frame to avoid leaking idle sessions.
// Each frame is decoded independently so malformed input cannot desynchronize
// subsequent requests on a healthy connection.
// The handler exits cleanly on EOF and lets the listener accept new clients.
import (
	"bufio"
	"dcache/internal/protocol"
	"net"
	"time"
)

func (s *Server) handle(c net.Conn) {
	defer c.Close()
	defer s.untrackConn(c)
	s.trackConn(c)
	r := bufio.NewReader(c)
	for {
		if s.cfg.ConnTimeout > 0 { _ = c.SetDeadline(time.Now().Add(s.cfg.ConnTimeout)) }
		req, e := protocol.DecodeRequest(r)
		if e != nil {
			return
		}
		resp := s.dispatch(req)
		if e = resp.Encode(c); e != nil {
			return
		}
	}
}
func setConnDeadline(c net.Conn, d time.Duration) error { return c.SetDeadline(time.Now().Add(d)) }
func readRequest(c net.Conn, d time.Duration) (protocol.Request, error) {
	if e := setConnDeadline(c, d); e != nil {
		return protocol.Request{}, e
	}
	return protocol.DecodeRequest(bufio.NewReader(c))
}
func writeResponse(c net.Conn, d time.Duration, r protocol.Response) error {
	if e := setConnDeadline(c, d); e != nil {
		return e
	}
	return r.Encode(c)
}
