package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// Response frames mirror request frames and deliberately retain the sequence
// number supplied by the caller. This makes the protocol safe to use with a
// future multiplexed client even though the reference client currently uses
// one request per connection. Code values are unsigned to match the on-wire
// eight-byte field. A non-zero code is an error; the optional Value payload
// contains a human-readable diagnostic or a command result. Key is reserved
// for redirects and future cursor-based commands. Extra carries TTL values,
// counters, or other command-specific numeric data. Decoders enforce the
// magic byte before allocating payload buffers, which protects the server
// from malformed length fields. Encoders write the fixed header first and
// then each payload independently so short writes are handled by net.Conn.

type Response struct {
	Seq        uint32
	Code       Code
	Key, Value []byte
	Extra      uint64
}

func NewResponse(seq uint32, code Code, value []byte) Response {
	return Response{Seq: seq, Code: code, Value: append([]byte(nil), value...)}
}
func (r Response) OK() bool { return r.Code == OK }
func (r Response) Error() error {
	if r.Code == OK {
		return nil
	}
	return fmt.Errorf("protocol error %d", r.Code)
}
func (r Response) ValueString() string                 { return string(r.Value) }
func (r Response) KeyString() string                   { return string(r.Key) }
func (r *Response) SetError(code Code, message string) { r.Code = code; r.Value = []byte(message) }
func (r Response) PayloadSize() int                    { return len(r.Key) + len(r.Value) }
func (r Response) Clone() Response {
	return Response{Seq: r.Seq, Code: r.Code, Key: append([]byte(nil), r.Key...), Value: append([]byte(nil), r.Value...), Extra: r.Extra}
}
func (r Response) IsRedirect() bool { return r.Code == ErrMoved }
func (r Response) IsNotFound() bool { return r.Code == ErrNotFound }
func (r Response) StatusText() string {
	if r.Code == OK {
		return "OK"
	}
	return fmt.Sprintf("ERR_%d", r.Code)
}
func (r Response) HasError() bool         { return r.Code != OK }
func (r Response) IsSuccess() bool        { return r.Code == OK }
func (r Response) HasKey() bool           { return len(r.Key) > 0 }
func (r Response) HasValue() bool         { return len(r.Value) > 0 }
func (r Response) TTLSeconds() int64      { return int64(r.Extra) }
func (r Response) CodeValue() Code        { return r.Code }
func (r Response) SequenceNumber() uint32 { return r.Seq }
func (r Response) IsEmpty() bool          { return !r.HasPayload() && r.Code == OK }
func (r Response) HasPayload() bool       { return len(r.Key) > 0 || len(r.Value) > 0 }
func (r Response) Numeric() int64         { return int64(r.Extra) }
func (r *Response) SetValue(v string)     { r.Value = []byte(v) }
func (r *Response) SetKey(v string)       { r.Key = []byte(v) }
func (r *Response) SetExtra(v uint64)     { r.Extra = v }
func (r Response) Sequence() uint32       { return r.Seq }

func (r Response) Encode(w io.Writer) error {
	h := make([]byte, 28)
	h[0] = Magic
	h[1] = 1
	h[2] = 0x80
	binary.BigEndian.PutUint32(h[4:8], r.Seq)
	binary.BigEndian.PutUint32(h[8:12], uint32(len(r.Key)))
	binary.BigEndian.PutUint32(h[12:16], uint32(len(r.Value)))
	binary.BigEndian.PutUint64(h[16:24], uint64(r.Code))
	if _, e := w.Write(h); e != nil {
		return e
	}
	if _, e := w.Write(r.Key); e != nil {
		return e
	}
	_, e := w.Write(r.Value)
	return e
}
func DecodeResponse(rd *bufio.Reader) (Response, error) {
	var h [28]byte
	if _, e := io.ReadFull(rd, h[:]); e != nil {
		return Response{}, e
	}
	if h[0] != Magic {
		return Response{}, fmt.Errorf("bad magic")
	}
	k, v := binary.BigEndian.Uint32(h[8:12]), binary.BigEndian.Uint32(h[12:16])
	if k > 1<<20 || v > 1<<24 {
		return Response{}, fmt.Errorf("payload too large")
	}
	r := Response{Seq: binary.BigEndian.Uint32(h[4:8]), Code: Code(binary.BigEndian.Uint64(h[16:24])), Key: make([]byte, k), Value: make([]byte, v)}
	_, e := io.ReadFull(rd, r.Key)
	if e != nil {
		return r, e
	}
	_, e = io.ReadFull(rd, r.Value)
	return r, e
}
