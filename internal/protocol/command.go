package protocol

import "fmt"

// Command semantics are intentionally kept in one place. The wire protocol
// uses stable numeric values so clients written in other languages can rely
// on the same request and response framing. Read-only commands never mutate
// the cache and may be retried by a connection pool. Mutation commands are
// processed in order on each TCP session. TTL commands use the Extra field as
// seconds, while GET and SET carry their data in the variable payload. The
// command table is also used by the Web console for validation and display.
// Keeping these notes beside the constants prevents accidental renumbering.

type Command byte

const (
	CmdSet     Command = 1
	CmdGet     Command = 2
	CmdDel     Command = 3
	CmdExists  Command = 4
	CmdExpire  Command = 5
	CmdTTL     Command = 6
	CmdPersist Command = 7
	CmdKeys    Command = 8
	CmdFlush   Command = 9
	CmdInfo    Command = 10
	CmdStats   Command = 11
	CmdMembers Command = 12
	CmdPing    Command = 13
)

type Code uint64

const (
	OK Code = iota
	ErrGeneral
	ErrMoved
	ErrNotFound
	ErrBadCmd
	ErrBadArg
	ErrFull
	ErrTimeout
)

func (c Command) String() string {
	names := map[Command]string{CmdSet: "SET", CmdGet: "GET", CmdDel: "DEL", CmdExists: "EXISTS", CmdExpire: "EXPIRE", CmdTTL: "TTL", CmdPersist: "PERSIST", CmdKeys: "KEYS", CmdFlush: "FLUSH", CmdInfo: "INFO", CmdStats: "STATS", CmdMembers: "MEMBERS", CmdPing: "PING"}
	if s, ok := names[c]; ok {
		return s
	}
	return "UNKNOWN"
}
func ParseCommand(s string) (Command, error) {
	for c := CmdSet; c <= CmdPing; c++ {
		if c.String() == s {
			return c, nil
		}
	}
	return 0, fmt.Errorf("unknown command %q", s)
}
func AllCommands() []Command {
	return []Command{CmdSet, CmdGet, CmdDel, CmdExists, CmdExpire, CmdTTL, CmdPersist, CmdKeys, CmdFlush, CmdInfo, CmdStats, CmdMembers, CmdPing}
}
func (c Command) ReadOnly() bool {
	switch c {
	case CmdGet, CmdExists, CmdTTL, CmdKeys, CmdInfo, CmdStats, CmdMembers, CmdPing:
		return true
	}
	return false
}
func (c Command) RequiresKey() bool {
	switch c {
	case CmdSet, CmdGet, CmdDel, CmdExists, CmdExpire, CmdTTL, CmdPersist, CmdKeys:
		return true
	}
	return false
}
// Validate is the single entry point for request-level argument checking.
// Failures carry a ValidationError so dispatch can map them to the precise
// wire code instead of masking an unexecuted operation as success. Unknown
// commands map to ErrBadCmd; every other failure (oversized payload, missing
// key, out-of-range TTL) is an argument error and maps to ErrBadArg.
func Validate(c Command, key, value []byte, extra uint64) error {
	if c == 0 || c > CmdPing {
		return ValidationError{Code: ErrBadCmd, Msg: "invalid command"}
	}
	if len(key) > 1<<20 || len(value) > 1<<24 {
		return ValidationError{Code: ErrBadArg, Msg: "payload too large"}
	}
	if c.RequiresKey() && c != CmdKeys && len(key) == 0 {
		return ValidationError{Code: ErrBadArg, Msg: "empty key"}
	}
	if (c == CmdSet || c == CmdExpire) && extra > uint64(^uint64(0)>>1) {
		return ValidationError{Code: ErrBadArg, Msg: "ttl is too large"}
	}
	return nil
}

// ValidationError pairs a wire code with a human-readable reason. Validate is
// the only producer; dispatch is the only consumer that reads Code.
type ValidationError struct {
	Code Code
	Msg  string
}

func (e ValidationError) Error() string { return e.Msg }

// ErrCode unwraps a validation error into its wire code. Errors produced by
// Validate always carry a code; any other error defaults to ErrBadArg so that
// a failed precheck is never reported as success.
func ErrCode(err error) Code {
	if err == nil {
		return OK
	}
	if ve, ok := err.(ValidationError); ok {
		return ve.Code
	}
	return ErrBadArg
}
func (c Command) IsAdmin() bool {
	switch c {
	case CmdInfo, CmdStats, CmdMembers, CmdFlush:
		return true
	}
	return false
}
func (c Command) IsTTL() bool     { return c == CmdExpire || c == CmdTTL || c == CmdPersist }
func (c Command) IsCluster() bool { return c == CmdMembers || c == CmdPing }
func (c Command) Code() byte      { return byte(c) }
func CommandFromCode(v byte) (Command, bool) {
	c := Command(v)
	if c < CmdSet || c > CmdPing {
		return 0, false
	}
	return c, true
}
