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
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			s, err := args[0].AsString()
			if err != nil {
				return err
			}
			id, ok := parseUUIDString(s)
			if !ok {
				return errors.NewError("badly formed hexadecimal UUID string")
			}
			return uuidInstance(id)
		},
		HelpText: `UUID(hex_string) - UUID object from a canonical string

Accepts hyphenated, bare-hex or URN forms; str() renders the canonical
hyphenated form.`,
	},
	"uuid3": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			nsStr, err := args[0].AsString()
			if err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			ns, ok := parseUUIDString(nsStr)
			if !ok {
				return errors.NewError("uuid3: namespace must be a UUID string")
			}
			return uuidInstance(uuid.NewMD5(ns, []byte(name)))
		},
		HelpText: "uuid3(namespace, name) - MD5 name-based UUID (deterministic)",
	},
	"uuid5": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			nsStr, err := args[0].AsString()
			if err != nil {
				return err
			}
			name, err := args[1].AsString()
			if err != nil {
				return err
			}
			ns, ok := parseUUIDString(nsStr)
			if !ok {
				return errors.NewError("uuid5: namespace must be a UUID string")
			}
			return uuidInstance(uuid.NewSHA1(ns, []byte(name)))
		},
		HelpText: "uuid5(namespace, name) - SHA-1 name-based UUID (deterministic)",
	},
}, uuidNamespaceConstants, "UUID generation library")
