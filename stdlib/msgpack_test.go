package stdlib

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/paularlott/gossip/codec"
	"github.com/paularlott/scriptling/object"
)

// fakeCodec is a minimal MsgpackCodec implementation used to verify the
// configurability plumbing. Its wire format is intentionally NOT real msgpack
// (it just round-trips through a sentinel byte) — the goal is to prove the
// codec passed to NewMsgpackLibrary is the one actually used.
type fakeCodec struct {
	name        string
	marshalCnt  int
	reader      func([]byte) (interface{}, error)
}

func (f *fakeCodec) Name() string { return f.name }
func (f *fakeCodec) Marshal(v interface{}) ([]byte, error) {
	f.marshalCnt++
	return []byte("FAKE:" + stringify(v)), nil
}
func (f *fakeCodec) Unmarshal(data []byte, v interface{}) error {
	if f.reader != nil {
		out, err := f.reader(data)
		if err != nil {
			return err
		}
		ptr, ok := v.(*interface{})
		if !ok {
			return errors.New("target must be *interface{}")
		}
		*ptr = out
		return nil
	}
	return errors.New("no reader configured")
}

func stringify(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	}
	return "non-string-input"
}

// TestMsgpackDefaultCodec verifies MsgpackLibrary uses VmihailencoMsgpackCodec
// by default and produces real msgpack bytes.
func TestMsgpackDefaultCodec(t *testing.T) {
	lib := MsgpackLibrary
	fn := lib.Functions()["packb"].Fn

	result := fn(context.Background(), object.NewKwargs(nil), object.NewString("hi"))
	b, ok := result.(*object.Bytes)
	if !ok {
		t.Fatalf("packb returned %T, want *Bytes", result)
	}
	if b.Len() == 0 {
		t.Fatalf("packb returned empty bytes")
	}

	// Real msgpack for "hi" as a str is the prefix byte 0xa2 followed by "hi".
	got := b.BytesValue()
	if len(got) != 3 || got[0] != 0xa2 || got[1] != 'h' || got[2] != 'i' {
		t.Fatalf("unexpected msgpack encoding %x", got)
	}

	// codec_name reports vmihailenco.
	cn := lib.Functions()["codec_name"].Fn(context.Background(), object.NewKwargs(nil))
	if s, _ := cn.AsString(); s != "vmihailenco-msgpack" {
		t.Fatalf("codec_name = %q, want vmihailenco-msgpack", s)
	}
}

// TestMsgpackCustomCodec verifies NewMsgpackLibrary wires the supplied codec
// into the library's pack/unpack builtins.
func TestMsgpackCustomCodec(t *testing.T) {
	codec := &fakeCodec{name: "fake-codec"}

	// Wire the unmarshal reader to round-trip our fake marshalled bytes.
	// (Marshal produces "FAKE:<input>"; Unmarshal reads them back as a string.)
	codec.reader = func(data []byte) (interface{}, error) {
		if !strings.HasPrefix(string(data), "FAKE:") {
			return nil, errors.New("unexpected payload")
		}
		return string(data[len("FAKE:"):]), nil
	}

	lib := NewMsgpackLibrary(codec)

	// packb routes through codec.Marshal.
	packFn := lib.Functions()["packb"].Fn
	result := packFn(context.Background(), object.NewKwargs(nil), object.NewString("hello"))
	b, ok := result.(*object.Bytes)
	if !ok {
		t.Fatalf("packb returned %T, want *Bytes", result)
	}
	if got := string(b.BytesValue()); got != "FAKE:hello" {
		t.Fatalf("custom codec not used: got %q", got)
	}
	if codec.marshalCnt != 1 {
		t.Fatalf("codec.Marshal called %d times, want 1", codec.marshalCnt)
	}

	// codec_name reports the custom name.
	cn := lib.Functions()["codec_name"].Fn(context.Background(), object.NewKwargs(nil))
	if s, _ := cn.AsString(); s != "fake-codec" {
		t.Fatalf("codec_name = %q, want fake-codec", s)
	}
}

// gossipShapedCodec is a minimal type whose method set matches
// gossip/codec.Serializer. It's used to prove structural compat without
// importing the gossip package in this specific test.
type gossipShapedCodec struct{}

func (gossipShapedCodec) Name() string                          { return "gossip-shaped" }
func (gossipShapedCodec) Marshal(v interface{}) ([]byte, error) { return []byte{0x00}, nil }
func (gossipShapedCodec) Unmarshal(data []byte, v interface{}) error {
	ptr, ok := v.(*interface{})
	if !ok {
		return errors.New("target must be *interface{}")
	}
	*ptr = "decoded"
	return nil
}

func TestMsgpackCodecInterfaceGossipCompatible(t *testing.T) {
	// Assignment must succeed with no adapter — proves structural compat.
	var c MsgpackCodec = gossipShapedCodec{}
	if c.Name() != "gossip-shaped" {
		t.Fatalf("structural-typing assignment failed: %v", c)
	}

	lib := NewMsgpackLibrary(c)
	cn := lib.Functions()["codec_name"].Fn(context.Background(), object.NewKwargs(nil))
	if s, _ := cn.AsString(); s != "gossip-shaped" {
		t.Fatalf("gossip-shaped codec not plumbed through: got %q", s)
	}
}

// TestMsgpackCodecAcceptsRealGossipCodecs is the headline test for the user's
// request: prove that the *actual* gossip codec types satisfy scriptling's
// MsgpackCodec interface via structural typing — no adapter, single shared
// driver object usable by both gossip and scriptling's msgpack library.
func TestMsgpackCodecAcceptsRealGossipCodecs(t *testing.T) {
	// All three gossip msgpack codecs should satisfy our interface without
	// any adapter — this is what lets an embedder pass one instance to both
	// gossip.Config.MsgCodec and stdlib.NewMsgpackLibrary / SetDefaultMsgpackCodec.
	var c MsgpackCodec = codec.NewVmihailencoMsgpackCodec()
	if c.Name() != "vmihailenco-msgpack" {
		t.Fatalf("gossip vmihailenco codec incompatible: got Name()=%q", c.Name())
	}

	c = codec.NewShamatonMsgpackCodec()
	if c.Name() != "shamaton-msgpack" {
		t.Fatalf("gossip shamaton codec incompatible: got Name()=%q", c.Name())
	}

	// End-to-end: feed the real gossip vmihailenco codec through the library
	// and verify it produces the same wire bytes as the default path.
	lib := NewMsgpackLibrary(codec.NewVmihailencoMsgpackCodec())
	packFn := lib.Functions()["packb"].Fn
	got := packFn(context.Background(), object.NewKwargs(nil), object.NewString("hi"))
	b, ok := got.(*object.Bytes)
	if !ok {
		t.Fatalf("packb returned %T, want *Bytes", got)
	}
	// Real msgpack encoding of "hi" as a str: 0xa2, 'h', 'i'
	if g := b.BytesValue(); len(g) != 3 || g[0] != 0xa2 || g[1] != 'h' || g[2] != 'i' {
		t.Fatalf("gossip codec via scriptling produced unexpected bytes %x", g)
	}

	// codec_name reports the gossip codec's name through the scriptling library.
	cn := lib.Functions()["codec_name"].Fn(context.Background(), object.NewKwargs(nil))
	if s, _ := cn.AsString(); s != "vmihailenco-msgpack" {
		t.Fatalf("codec_name via gossip codec = %q, want vmihailenco-msgpack", s)
	}
}
