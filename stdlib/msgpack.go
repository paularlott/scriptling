package stdlib

import (
	"context"

	"github.com/paularlott/gossip/codec/shamaton"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// MsgpackLibraryName is the import name for the MessagePack library.
const MsgpackLibraryName = "msgpack"

// MsgpackCodec is the interface a MessagePack (or binary-compatible) codec
// must satisfy. It is structurally identical to
// github.com/paularlott/gossip/codec.Serializer, so any gossip codec
// (ShamatonMsgpackCodec, VmihailencoMsgpackCodec, HashicorpMsgpackCodec) can
// be passed directly to NewMsgpackLibrary — letting a single driver instance
// be shared between Scriptling's msgpack library and a gossip cluster so
// both sides agree on the wire format.
//
// Example — share one codec between gossip and Scriptling:
//
//	import gossipcodec "github.com/paularlott/gossip/codec"
//
//	codec := gossipshamaton.New() // or Vmihailenco, etc.
//
//	// gossip side
//	gossipCfg := gossip.DefaultConfig() // already defaults to shamaton
//	gossipCfg.MsgCodec = codec
//
//	// scriptling side
//	p.RegisterLibrary(stdlib.NewMsgpackLibrary(codec))
type MsgpackCodec interface {
	Name() string
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

// newMsgpackLibraryBuiltin returns a builtin set bound to the given codec.
// Each builtin closes over the codec so a library and its codec are
// inseparable after construction.
func newMsgpackLibraryBuiltin(c MsgpackCodec) map[string]*object.Builtin {
	if c == nil {
		c = shamaton.New()
	}

	pack := func(obj object.Object) object.Object {
		goVal := conversion.ToGo(obj)
		out, err := c.Marshal(goVal)
		if err != nil {
			return errors.NewError("msgpack serialize error: %s", err.Error())
		}
		return object.NewBytes(out)
	}

	unpack := func(b *object.Bytes) object.Object {
		var target interface{}
		if err := c.Unmarshal(b.BytesValue(), &target); err != nil {
			return errors.NewError("msgpack parse error: %s", err.Error())
		}
		return conversion.FromGo(target)
	}

	packFn := func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		return pack(args[0])
	}

	unpackFn := func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 1); err != nil {
			return err
		}
		b, ok := args[0].(*object.Bytes)
		if !ok {
			return errors.NewTypeError("BYTES", args[0].Type().String())
		}
		return unpack(b)
	}

	return map[string]*object.Builtin{
		"packb":   {Fn: packFn, HelpText: packHelp},
		"unpackb": {Fn: unpackFn, HelpText: unpackHelp},
		// pack/unpack aliases — older Python msgpack naming.
		"pack":   {Fn: packFn, HelpText: packHelp},
		"unpack": {Fn: unpackFn, HelpText: unpackHelp},
	}
}

// NewMsgpackLibrary builds a msgpack library backed by the given codec.
// Pass nil to use the default (gossip's ShamatonMsgpackCodec — matching the
// gossip DefaultConfig). The returned library can be registered with a
// Scriptling instance via RegisterLibrary.
//
// Embedders wanting a different codec can build their own library directly:
//
//	p.RegisterLibrary(stdlib.NewMsgpackLibrary(gossipvmihailenco.New()))
//
// or, to swap the default that stdlib.RegisterAll picks up, reassign before
// calling RegisterAll:
//
//	stdlib.MsgpackLibrary = stdlib.NewMsgpackLibrary(myCodec)
func NewMsgpackLibrary(c MsgpackCodec) *object.Library {
	return object.NewLibrary(MsgpackLibraryName, newMsgpackLibraryBuiltin(c), nil,
		"MessagePack binary serialization library")
}

// MsgpackLibrary is the default library instance, backed by the same codec
// as gossip's DefaultConfig (shamaton), so a fresh Scriptling instance and a
// fresh gossip cluster agree on the wire format without any explicit wiring.
var MsgpackLibrary = NewMsgpackLibrary(shamaton.New())

const packHelp = `packb(obj) - Serialize a Scriptling value to MessagePack bytes

Converts a Scriptling value (dict, list, str, int, float, bool, None, Bytes)
to its MessagePack binary representation and returns it as a Bytes value.

Returns:
  bytes: the MessagePack-encoded payload.

Example:
  import msgpack
  payload = msgpack.packb({"user": "alice", "id": 42})
  # payload is a Bytes value holding the packed binary form`

const unpackHelp = `unpackb(packed) - Parse MessagePack bytes into a Scriptling value

Accepts a Bytes value produced by packb (or any MessagePack payload) and
returns the corresponding Scriptling object. msgpack bin decodes to Bytes,
str decodes to String, and integers are clamped to Scriptling's int64.

Parameters:
  packed (bytes): a Bytes value containing a MessagePack payload.

Returns:
  dict, list, str, int, float, bool, None, or bytes: the decoded value.

Raises:
  Error: if packed is not a Bytes value or the payload is malformed.

Example:
  import msgpack
  payload = msgpack.packb({"name": "alice"})
  data = msgpack.unpackb(payload)
  print(data["name"])  # "alice"`
