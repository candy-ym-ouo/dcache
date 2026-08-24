package server

import (
	"dcache/internal/protocol"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *Server) dispatch(r protocol.Request) protocol.Response {
	started := time.Now()
	defer func() { s.metrics.Observe(time.Since(started)) }()
	s.metrics.Request()
	resp := protocol.Response{Seq: r.Seq}
	if err := r.Valid(); err != nil {
		resp.Code = protocol.ErrCode(err)
		resp.Value = []byte(err.Error())
		return resp
	}
	key := string(r.Key)
	switch r.Cmd {
	case protocol.CmdSet:
		e := s.store.Set(key, r.Value, time.Duration(r.Extra)*time.Second)
		if e != nil {
			resp.Code = protocol.ErrFull
		}
	case protocol.CmdGet:
		if v, ok := s.store.Get(key); ok {
			resp.Value = v
			s.metrics.Hit()
		} else {
			resp.Code = protocol.ErrNotFound
			s.metrics.Miss()
		}
	case protocol.CmdDel:
		if !s.store.Del(key) {
			resp.Code = protocol.ErrNotFound
		}
	case protocol.CmdExists:
		if s.store.Exists(key) {
			resp.Extra = 1
		}
	case protocol.CmdExpire:
		if !s.store.SetTTLSeconds(key, int64(r.Extra)) {
			resp.Code = protocol.ErrNotFound
		}
	case protocol.CmdTTL:
		// Extra carries the response Code on the wire, so the TTL value must
		// travel in Value as a signed 8-byte integer: remaining seconds for a
		// live key, -1 for a persistent key, -2 for a missing key. Encoding the
		// sentinel through Extra would collide with the Code field and let the
		// three states drift to the same value on the client.
		resp.Value = protocol.EncodeTTL(s.store.TTL(key))
	case protocol.CmdPersist:
		if !s.store.Persist(key) {
			resp.Code = protocol.ErrNotFound
		}
	case protocol.CmdKeys:
		resp.Value = []byte(strings.Join(s.store.Keys(key), "\n"))
	case protocol.CmdFlush:
		s.store.Flush()
	case protocol.CmdInfo:
		n := s.cluster.Self()
		resp.Value = []byte(fmt.Sprintf("name=%s addr=%s", n.Name, n.Addr))
	case protocol.CmdStats:
		k, h, m, e, x := s.store.Stats()
		resp.Value = []byte(fmt.Sprintf("keys=%d hits=%d misses=%d evicted=%d expired=%d", k, h, m, e, x))
	case protocol.CmdMembers:
		for _, n := range s.cluster.Members() {
			resp.Value = append(resp.Value, []byte(n.Name+" "+n.Addr+"\n")...)
		}
	case protocol.CmdPing:
		resp.Value = []byte("PONG")
	default:
		resp.Code = protocol.ErrBadCmd
	}
	return resp
}
func parseInt(b []byte) (int64, error) { return strconv.ParseInt(string(b), 10, 64) }
func (s *Server) commandSet(r protocol.Request) protocol.Code {
	if e := s.store.Set(string(r.Key), r.Value, time.Duration(r.Extra)*time.Second); e != nil {
		return protocol.ErrFull
	}
	return protocol.OK
}
func (s *Server) commandGet(key string) ([]byte, protocol.Code) {
	v, ok := s.store.Get(key)
	if !ok {
		return nil, protocol.ErrNotFound
	}
	return v, protocol.OK
}
func (s *Server) commandDelete(key string) protocol.Code {
	if !s.store.Del(key) {
		return protocol.ErrNotFound
	}
	return protocol.OK
}
func (s *Server) validateOwnership(key string) protocol.Code {
	owner := s.cluster.Owner(key)
	if owner != "" && owner != s.cluster.Self().Addr {
		return protocol.ErrMoved
	}
	return protocol.OK
}
func (s *Server) dispatchRead(key string, cmd protocol.Command) protocol.Response {
	resp := protocol.Response{}
	if code := s.validateOwnership(key); code != protocol.OK {
		resp.Code = code
		return resp
	}
	switch cmd {
	case protocol.CmdGet:
		resp.Value, resp.Code = s.commandGet(key)
	case protocol.CmdExists:
		if s.store.Exists(key) {
			resp.Extra = 1
		}
	default:
		// TTL travels in Value as a signed integer so it cannot collide with
		// the Code carried in Extra (see dispatch).
		resp.Value = protocol.EncodeTTL(s.store.TTL(key))
	}
	return resp
}
func (s *Server) dispatchMutation(r protocol.Request) protocol.Code {
	switch r.Cmd {
	case protocol.CmdSet:
		return s.commandSet(r)
	case protocol.CmdDel:
		return s.commandDelete(string(r.Key))
	case protocol.CmdExpire:
		if !s.store.SetTTLSeconds(string(r.Key), int64(r.Extra)) {
			return protocol.ErrNotFound
		}
	case protocol.CmdPersist:
		if !s.store.Persist(string(r.Key)) {
			return protocol.ErrNotFound
		}
	case protocol.CmdFlush:
		s.store.Flush()
	}
	return protocol.OK
}
