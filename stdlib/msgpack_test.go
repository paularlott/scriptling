package stdlib

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/paularlott/gossip/codec/hashicorp"
	"github.com/paularlott/gossip/codec/shamaton"
	"github.com/paularlott/gossip/codec/vmihailenco"
	"github.com/paularlott/scriptling/object"
)

// fakeCodec is a minimal MsgpackCodec implementation used to verify the
// configurability plumbing. Its wire format is intentionally NOT real msgpack
// (it just round-trips through a sentinel byte) — the goal is to prove the
// codec passed to NewMsgpackLibrary is the one actually used.
type fakeCodec struct {
	name       string
	marshalCnt int
	reader     func([]byte) (interface{}, error)
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

// TestMsgpackDefaultCodec verifies MsgpackLibrary uses ShamatonMsgpackCodec
// by default (matching gossip's DefaultConfig) and produces real msgpack bytes.
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

	// Verify the codec is plumbed through by packing with it.
	lib := NewMsgpackLibrary(c)
	res := lib.Functions()["packb"].Fn(context.Background(), object.NewKwargs(nil), object.NewString("x"))
	if _, ok := res.(*object.Bytes); !ok {
		t.Fatalf("packb via gossip-shaped codec returned %T, want *Bytes", res)
	}
}

// TestMsgpackCodecAcceptsRealGossipCodecs is the headline test for the user's
// request: prove that the *actual* gossip codec types satisfy scriptling's
// MsgpackCodec interface via structural typing — no adapter, single shared
// driver object usable by both gossip and scriptling's msgpack library.
// TestMsgpackCodecMarshalErrorPropagates verifies that a codec whose Marshal
// returns an error surfaces a Scriptling error (not a panic, not silent
// corruption). This is the unhappy path that real codecs hit when given
// unencodable values (channels, funcs, circular refs, etc).
func TestMsgpackCodecMarshalErrorPropagates(t *testing.T) {
	errCodec := &fakeCodec{
		name: "always-errors-marshal",
		reader: func([]byte) (interface{}, error) {
			t.Fatalf("Unmarshal should not be called when Marshal fails")
			return nil, nil
		},
	}
	// Override Marshal on a per-test basis via a wrapper rather than expanding
	// fakeCodec's API. Inline type keeps the test self-contained.
	codec := errorOnMarshalCodec{MsgpackCodec: errCodec}

	lib := NewMsgpackLibrary(codec)
	res := lib.Functions()["packb"].Fn(
		context.Background(), object.NewKwargs(nil), object.NewString("anything"))

	if obj, ok := res.(*object.Error); !ok {
		t.Fatalf("packb with failing codec returned %T (%v), want *object.Error", res, res)
	} else if !strings.Contains(obj.Message, "induced marshal failure") {
		t.Fatalf("error message lost; got %q", obj.Message)
	}
}

// errorOnMarshalCodec wraps a MsgpackCodec and forces Marshal to fail. Used
// only by TestMsgpackCodecMarshalErrorPropagates to verify error plumbing.
type errorOnMarshalCodec struct{ MsgpackCodec }

func (errorOnMarshalCodec) Name() string { return "always-errors-marshal" }
func (errorOnMarshalCodec) Marshal(interface{}) ([]byte, error) {
	return nil, errors.New("induced marshal failure")
}
func (c errorOnMarshalCodec) Unmarshal(data []byte, v interface{}) error {
	return c.MsgpackCodec.Unmarshal(data, v)
}

// TestMsgpackCodecUnmarshalErrorPropagates verifies that a codec whose
// Unmarshal returns an error surfaces a Scriptling error.
func TestMsgpackCodecUnmarshalErrorPropagates(t *testing.T) {
	codec := &fakeCodec{
		name: "always-errors-unmarshal",
		// leave reader nil so fakeCodec.Unmarshal returns its default error
	}

	lib := NewMsgpackLibrary(codec)
	res := lib.Functions()["unpackb"].Fn(
		context.Background(), object.NewKwargs(nil), object.NewBytes([]byte{0x01}))

	if obj, ok := res.(*object.Error); !ok {
		t.Fatalf("unpackb with failing codec returned %T (%v), want *object.Error", res, res)
	} else if !strings.Contains(obj.Message, "no reader configured") {
		t.Fatalf("error message lost; got %q", obj.Message)
	}
}

// TestNewMsgpackLibraryNilCodec verifies the documented "pass nil → default"
// behaviour of NewMsgpackLibrary, so embedders can rely on it.
func TestNewMsgpackLibraryNilCodec(t *testing.T) {
	lib := NewMsgpackLibrary(nil)
	res := lib.Functions()["packb"].Fn(context.Background(), object.NewKwargs(nil), object.NewString("hi"))
	b, ok := res.(*object.Bytes)
	if !ok || b.Len() == 0 {
		t.Fatalf("NewMsgpackLibrary(nil).packb returned %T, want non-empty *Bytes", res)
	}
}

func TestMsgpackCodecAcceptsRealGossipCodecs(t *testing.T) {
	// All three gossip msgpack codecs should satisfy our interface without
	// any adapter — this is what lets an embedder pass one instance to both
	// gossip.Config.MsgCodec and stdlib.NewMsgpackLibrary.
	var c MsgpackCodec = vmihailenco.New()
	if c.Name() != "vmihailenco-msgpack" {
		t.Fatalf("gossip vmihailenco codec incompatible: got Name()=%q", c.Name())
	}

	c = shamaton.New()
	if c.Name() != "shamaton-msgpack" {
		t.Fatalf("gossip shamaton codec incompatible: got Name()=%q", c.Name())
	}

	c = hashicorp.New()
	if c.Name() != "hashicorp-msgpack" {
		t.Fatalf("gossip hashicorp codec incompatible: got Name()=%q", c.Name())
	}

	// End-to-end: feed the real gossip vmihailenco codec through the library
	// and verify it produces the same wire bytes as the default path.
	lib := NewMsgpackLibrary(vmihailenco.New())
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
}
