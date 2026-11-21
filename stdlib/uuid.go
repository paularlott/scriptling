package stdlib

import (
	"context"

	"github.com/google/uuid"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var UUIDLibrary = object.NewLibrary(UUIDLibraryName, map[string]*object.Builtin{
	"uuid1": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("uuid1", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			id, err := uuid.NewUUID()
			if err != nil {
				return errors.NewError("failed to generate UUID v1: %s", err.Error())
			}
			return uuidInstance(id)
		},
		HelpText: `uuid1() - Generate a UUID version 1 (time-based)

Returns a UUID based on current time and MAC address.
Format: xxxxxxxx-xxxx-1xxx-yxxx-xxxxxxxxxxxx

Example:
  import uuid
  id = uuid.uuid1()
  print(id)  # e.g., "f47ac10b-58cc-1e4c-a26f-e3fc32165abc"`,
	},
	"uuid4": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("uuid4", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			return uuidInstance(uuid.New())
		},
		HelpText: `uuid4() - Generate a UUID version 4 (random)

Returns a randomly generated UUID.
Format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx

Example:
  import uuid
  id = uuid.uuid4()
  print(id)  # e.g., "550e8400-e29b-41d4-a716-446655440000"`,
	},
	"uuid7": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("uuid7", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 0); err != nil {
				return err
			}
			id, err := uuid.NewV7()
			if err != nil {
				return errors.NewError("failed to generate UUID v7: %s", err.Error())
			}
			return uuidInstance(id)
		},
		HelpText: `uuid7() - Generate a UUID version 7 (Unix timestamp-based, sortable)

Returns a UUID based on Unix timestamp in milliseconds.
UUIDs generated in sequence will sort in chronological order.
Format: xxxxxxxx-xxxx-7xxx-yxxx-xxxxxxxxxxxx

Example:
  import uuid
  id = uuid.uuid7()
  print(id)  # e.g., "018f6b1c-4e5d-7abc-8def-0123456789ab"`,
	},
	"UUID": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if kwargs.Len() > 0 {
				if err := errors.CheckKwargs("UUID", kwargs, "hex", "bytes", "bytes_le", "fields", "int", "version", "is_safe"); err != nil {
					return err
				}
			}
			if err := errors.MaxArgs(args, 1); err != nil {
				return err
			}
			hexArg := kwargs.Get("hex")
			if len(args) == 1 {
				hexArg = args[0]
			}
			given := 0
			for _, o := range []object.Object{hexArg, kwargs.Get("bytes"), kwargs.Get("bytes_le"), kwargs.Get("fields"), kwargs.Get("int")} {
				if o != nil && o.Type() != object.NULL_OBJ {
					given++
				}
			}
			if given != 1 {
				return errors.NewTypeErrorTagged("one of the hex, bytes, bytes_le, fields, or int arguments must be given")
			}
			var id uuid.UUID
			switch {
			case hexArg != nil && hexArg.Type() != object.NULL_OBJ:
				s, err := hexArg.AsString()
				if err != nil {
					return err
				}
				var ok bool
				if id, ok = parseUUIDString(s); !ok {
					return errors.NewValueError("badly formed hexadecimal UUID string")
				}
			case kwargs.Get("bytes") != nil || kwargs.Get("bytes_le") != nil:
				b, le := kwargs.Get("bytes"), false
				if b == nil {
					b, le = kwargs.Get("bytes_le"), true
				}
				raw, ok := b.(*object.Bytes)
				if !ok || raw.Len() != 16 {
					if le {
						return errors.NewValueError("bytes_le is not a 16-char string")
					}
					return errors.NewValueError("bytes is not a 16-char string")
				}
				copy(id[:], raw.BytesValue())
				if le {
					// The first three fields are little-endian.
					id[0], id[1], id[2], id[3] = id[3], id[2], id[1], id[0]
					id[4], id[5] = id[5], id[4]
					id[6], id[7] = id[7], id[6]
				}
			default:
				// int= and fields= need integers wider than 64 bits.
				return errors.NewValueError("UUID(int=...) and UUID(fields=...) are not supported: integers are 64-bit")
			}
			if v := kwargs.Get("version"); v != nil && v.Type() != object.NULL_OBJ {
				n, err := v.AsInt()
				if err != nil {
					return err
				}
				if n < 1 || n > 8 {
					return errors.NewValueError("illegal version number")
				}
				// Set the RFC 4122 variant and the version number.
				id[8] = id[8]&0x3f | 0x80
				id[6] = id[6]&0x0f | byte(n)<<4
			}
			return uuidInstance(id)
		},
		HelpText: `UUID(hex=None, bytes=None, bytes_le=None, *, version=None) - UUID object

Accepts hyphenated, bare-hex, braced or URN strings, or 16 bytes (bytes_le:
first three fields little-endian); str() renders the canonical hyphenated
form. int= is not supported (UUIDs are 128-bit, integers 64-bit).`,
	},
	"uuid3": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("uuid3", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			nsStr, err := uuidStringOf(args[0])
			if err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			ns, ok := parseUUIDString(nsStr)
			if !ok {
				return errors.NewValueError("uuid3: namespace must be a UUID string")
			}
			return uuidInstance(uuid.NewMD5(ns, []byte(name)))
		},
		HelpText: "uuid3(namespace, name) - MD5 name-based UUID (deterministic)",
	},
	"uuid5": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.NoKwargs("uuid5", kwargs); err != nil {
				return err
			}
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			nsStr, err := uuidStringOf(args[0])
			if err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			ns, ok := parseUUIDString(nsStr)
			if !ok {
				return errors.NewValueError("uuid5: namespace must be a UUID string")
			}
			return uuidInstance(uuid.NewSHA1(ns, []byte(name)))
		},
		HelpText: "uuid5(namespace, name) - SHA-1 name-based UUID (deterministic)",
	},
}, uuidNamespaceConstants, "UUID generation library")
