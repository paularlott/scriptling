package stdlib

import (
	"context"

	"github.com/google/uuid"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

var UUIDLibrary = object.NewLibrary(map[string]*object.Builtin{
	"uuid1": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			id, err := uuid.NewUUID()
			if err != nil {
				return errors.NewError("failed to generate UUID v1: %s", err.Error())
			}
			return &object.String{Value: id.String()}
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
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			return &object.String{Value: uuid.New().String()}
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
			if len(args) != 0 {
				return errors.NewArgumentError(len(args), 0)
			}
			id, err := uuid.NewV7()
			if err != nil {
				return errors.NewError("failed to generate UUID v7: %s", err.Error())
			}
			return &object.String{Value: id.String()}
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
}, nil, "UUID generation library")
