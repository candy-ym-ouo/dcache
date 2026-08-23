package config

import (
	"fmt"
	"time"
)

// Config is deliberately plain data. Flag parsing happens in main, while
// validation remains reusable by tests and embedding applications. Defaults
// keep a single-node demo useful without any external configuration file.
// Addresses are passed directly to net.Listen, so callers can use IPv4,
// IPv6, or a unix-compatible resolver. MaxKeys controls the local LRU bound;
// it is a safety limit rather than a memory measurement. Timeouts are finite
// by default to ensure dead peers do not retain handler goroutines forever.
// ShutdownTimeout gives in-flight requests a short graceful completion window.

type Config struct {
	Addr, Name, HTTPAddr, Seed, LogLevel        string
	MaxKeys                                     int
	ConnTimeout, ShutdownTimeout, SweepInterval time.Duration
}

func Default() Config {
	return Config{Addr: "127.0.0.1:7301", Name: "node-1", HTTPAddr: "127.0.0.1:8080", MaxKeys: 10000, ConnTimeout: 10 * time.Second, ShutdownTimeout: 5 * time.Second, SweepInterval: time.Second}
}
func (c Config) Validate() error {
	if c.Addr == "" || c.Name == "" {
		return fmt.Errorf("addr and name are required")
	}
	if c.MaxKeys < 1 {
		return fmt.Errorf("max-keys must be positive")
	}
	return nil
}
func (c Config) Address() string     { return c.Addr }
func (c Config) HTTPAddress() string { return c.HTTPAddr }
func (c *Config) ApplyDefaults() {
	d := Default()
	if c.Addr == "" {
		c.Addr = d.Addr
	}
	if c.Name == "" {
		c.Name = d.Name
	}
	if c.HTTPAddr == "" {
		c.HTTPAddr = d.HTTPAddr
	}
	if c.MaxKeys == 0 {
		c.MaxKeys = d.MaxKeys
	}
	if c.ConnTimeout == 0 {
		c.ConnTimeout = d.ConnTimeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = d.ShutdownTimeout
	}
	if c.SweepInterval == 0 {
		c.SweepInterval = d.SweepInterval
	}
}
func (c Config) IsDevelopment() bool             { return c.LogLevel == "debug" || c.LogLevel == "trace" }
func (c Config) MaxConnections() int             { return c.MaxKeys/10 + 1 }
func (c Config) SweepEnabled() bool              { return c.SweepInterval > 0 }
func (c Config) WithAddr(addr string) Config     { c.Addr = addr; return c }
func (c Config) WithHTTPAddr(addr string) Config { c.HTTPAddr = addr; return c }
func (c Config) WithName(name string) Config     { c.Name = name; return c }
func (c Config) WithMaxKeys(n int) Config        { c.MaxKeys = n; return c }
func (c Config) Summary() string {
	return fmt.Sprintf("%s tcp=%s http=%s max=%d", c.Name, c.Addr, c.HTTPAddr, c.MaxKeys)
}
func (c Config) Timeouts() time.Duration { return c.ConnTimeout + c.ShutdownTimeout }
func (c Config) ValidName() bool         { return len(c.Name) > 0 && len(c.Name) < 128 }
func (c Config) ValidAddr() bool         { return len(c.Addr) > 0 && len(c.Addr) < 256 }
