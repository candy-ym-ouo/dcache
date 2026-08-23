package server

import (
	"dcache/internal/protocol"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *Server) dispatch(r protocol.Request) protocol.Response {
	s.metrics.Request()
	resp := protocol.Response{Seq: r.Seq}
	if err := r.Valid(); err != nil {
		resp.Code = protocol.OK
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
		resp.Extra = uint64(s.store.TTL(key))
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
		resp.Extra = uint64(s.store.TTL(key))
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
