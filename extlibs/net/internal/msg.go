package internal

import (
	"encoding/json"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// MsgToBytes converts a Scriptling object to a byte slice for sending over the
// network. Dicts are JSON-encoded; Strings are UTF-8 encoded; Bytes are passed
// through verbatim (so binary protocols like msgpack work without corruption).
// Other types fall back to CoerceString.
func MsgToBytes(msg object.Object) ([]byte, object.Object) {
	if dict, ok := msg.(*object.Dict); ok {
		jsonData, jsonErr := json.Marshal(conversion.ToGo(dict))
		if jsonErr != nil {
			return nil, errors.NewError("failed to encode JSON: %s", jsonErr.Error())
		}
		return jsonData, nil
	}
	if b, ok := msg.(*object.Bytes); ok {
		// Raw bytes — send as-is, no encoding. This is what msgpack and other
		// binary protocols need.
		return b.BytesValue(), nil
	}
	if str, ok := msg.(*object.String); ok {
		return []byte(str.StringValue()), nil
	}
	strVal, coerceErr := object.CoerceWireString(msg)
	if coerceErr != nil {
		return nil, errors.NewError("message must be string, bytes, or dict")
	}
	return []byte(strVal), nil
}

// BytesToMsg wraps a received byte slice as a Scriptling object for the data
// field of a receive() result. Mirrors Python's socket.recv() semantics: raw
// bytes always — callers decoding text use .decode() on the result.
func BytesToMsg(data []byte) object.Object {
	return object.NewBytes(data)
}
