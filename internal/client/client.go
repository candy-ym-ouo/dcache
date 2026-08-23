package client

import (
	"bufio"
	"context"
	"dcache/internal/protocol"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	addr    string
	timeout time.Duration
	seq     uint32
}

func New(addr string) *Client { return &Client{addr: addr, timeout: 3 * time.Second} }
func (c *Client) Do(cmd protocol.Command, key string, value []byte, extra uint64) (protocol.Response, error) {
	return c.DoContext(context.Background(), cmd, key, value, extra)
}
func (c *Client) DoContext(ctx context.Context, cmd protocol.Command, key string, value []byte, extra uint64) (protocol.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dialer := net.Dialer{Timeout: c.timeout}
	conn, e := dialer.DialContext(ctx, "tcp", c.addr)
	if e != nil {
		return protocol.Response{}, e
	}
	defer conn.Close()
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
	r := protocol.Request{Seq: atomic.AddUint32(&c.seq, 1), Cmd: cmd, Key: []byte(key), Value: value, Extra: extra}
	if e = r.Encode(conn); e != nil {
		return protocol.Response{}, e
	}
	return protocol.DecodeResponse(bufio.NewReader(conn))
}
func (c *Client) Set(k, v string, ttl uint64) error {
	r, e := c.Do(protocol.CmdSet, k, []byte(v), ttl)
	if e == nil && r.Code != protocol.OK {
		e = fmt.Errorf("server error %d", r.Code)
	}
	return e
}
func (c *Client) Get(k string) (string, error) {
	r, e := c.Do(protocol.CmdGet, k, nil, 0)
	if e != nil {
		return "", e
	}
	if r.Code != protocol.OK {
		return "", nil
	}
	return string(r.Value), nil
}
func (c *Client) Del(k string) error {
	r, e := c.Do(protocol.CmdDel, k, nil, 0)
	if e == nil && r.Code != protocol.OK && r.Code != protocol.ErrNotFound {
		e = fmt.Errorf("server error %d", r.Code)
	}
	return e
}
func (c *Client) Exists(k string) (bool, error) {
	r, e := c.Do(protocol.CmdExists, k, nil, 0)
	return r.Extra == 1, e
}
func (c *Client) Expire(k string, seconds int64) error {
	r, e := c.Do(protocol.CmdExpire, k, nil, uint64(seconds))
	if e == nil && r.Code != protocol.OK {
		e = fmt.Errorf("server error %d", r.Code)
	}
	return e
}
func (c *Client) TTL(k string) (int64, error) {
	r, e := c.Do(protocol.CmdTTL, k, nil, 0)
	return int64(r.Extra), e
}
func (c *Client) Persist(k string) error {
	r, e := c.Do(protocol.CmdPersist, k, nil, 0)
	if e == nil && r.Code != protocol.OK {
		e = fmt.Errorf("server error %d", r.Code)
	}
	return e
}
func (c *Client) Keys(prefix string) ([]string, error) {
	r, e := c.Do(protocol.CmdKeys, prefix, nil, 0)
	if e != nil {
		return nil, e
	}
	if len(r.Value) == 0 {
		return []string{}, nil
	}
	return strings.Split(string(r.Value), "\n"), nil
}
func (c *Client) Flush() error { _, e := c.Do(protocol.CmdFlush, "", nil, 0); return e }
func (c *Client) Ping() error {
	r, e := c.Do(protocol.CmdPing, "", nil, 0)
	if e == nil && string(r.Value) != "PONG" {
		return fmt.Errorf("unexpected ping")
	}
	return e
}
func (c *Client) Address() string        { return c.addr }
func (c *Client) Timeout() time.Duration { return c.timeout }
func (c *Client) SetTimeout(d time.Duration) {
	if d > 0 {
		c.timeout = d
	}
}
func (c *Client) DoString(cmd protocol.Command, key, value string, extra uint64) (string, error) {
	r, e := c.Do(cmd, key, []byte(value), extra)
	return string(r.Value), e
}
func (c *Client) Info() (string, error)    { return c.DoString(protocol.CmdInfo, "", "", 0) }
func (c *Client) Stats() (string, error)   { return c.DoString(protocol.CmdStats, "", "", 0) }
func (c *Client) Members() (string, error) { return c.DoString(protocol.CmdMembers, "", "", 0) }
func (c *Client) Execute(cmd string, key string, args ...string) (string, error) {
	cmt, e := protocol.ParseCommand(cmd)
	if e != nil {
		return "", e
	}
	var v string
	if len(args) > 0 {
		v = args[0]
	}
	return c.DoString(cmt, key, v, 0)
}
func (c *Client) SetBytes(k string, v []byte, ttl uint64) error {
	r, e := c.Do(protocol.CmdSet, k, v, ttl)
	if e == nil && r.Code != protocol.OK {
		e = fmt.Errorf("server error %d", r.Code)
	}
	return e
}
func (c *Client) GetBytes(k string) ([]byte, error) {
	r, e := c.Do(protocol.CmdGet, k, nil, 0)
	if e != nil {
		return nil, e
	}
	if r.Code == protocol.ErrNotFound {
		return nil, nil
	}
	return r.Value, nil
}
func (c *Client) Close() error { return nil }
