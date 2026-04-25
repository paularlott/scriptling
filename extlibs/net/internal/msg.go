package internal

import (
	"encoding/json"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// MsgToBytes converts a Scriptling object to a byte slice for sending over the
// network. Dicts are JSON-encoded; strings and other types are coerced to string.
func MsgToBytes(msg object.Object) ([]byte, object.Object) {
	if dict, ok := msg.(*object.Dict); ok {
		jsonData, jsonErr := json.Marshal(conversion.ToGo(dict))
		if jsonErr != nil {
			return nil, errors.NewError("failed to encode JSON: %s", jsonErr.Error())
		}
		return jsonData, nil
	}
	if str, ok := msg.(*object.String); ok {
		return []byte(str.Value), nil
	}
	strVal, coerceErr := msg.CoerceString()
	if coerceErr != nil {
		return nil, errors.NewError("message must be string or dict")
	}
	return []byte(strVal), nil
}
