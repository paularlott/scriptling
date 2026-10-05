package stdlib

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/object"

	"github.com/google/uuid"
)

// The uuid library returns UUID *objects* (not strings): str() renders the
// canonical form, .int/.hex/.version/.variant expose the fields, and
// uuid3/uuid5 give deterministic name-based UUIDs. Constants provide the
// standard namespace UUIDs.

var uuidNamespaceConstants = map[string]object.Object{
	"NAMESPACE_DNS":  object.NewString("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	"NAMESPACE_URL":  object.NewString("6ba7b811-9dad-11d1-80b4-00c04fd430c8"),
	"NAMESPACE_OID":  object.NewString("6ba7b812-9dad-11d1-80b4-00c04fd430c8"),
	"NAMESPACE_X500": object.NewString("6ba7b814-9dad-11d1-80b4-00c04fd430c8"),
}

func uuidInstance(id uuid.UUID) *object.Instance {
	hexStr := ""
	for _, b := range id {
		hexStr += fmt.Sprintf("%02x", b)
	}
	variant := "reserved for NCS backward compatibility"
	v := id[8] >> 6
	switch v {
	case 2:
		variant = "specified in RFC 4122"
	case 3:
		variant = "reserved, Microsoft"
	}
	fields := map[string]object.Object{
		"__str_repr__": object.NewString(id.String()),
		"hex":          object.NewString(hexStr),
		"int":          nil, // placeholder replaced below
	}
	delete(fields, "int")
	inst := object.NewInstanceWithFields(UUIDClass, fields)
	inst.SetField("int", object.NewInteger(int64(int8(id.ID()))))
	inst.SetField("version", object.NewInteger(int64(id.Version())))
	inst.SetField("variant", object.NewString(variant))
	return inst
}

// UUIDClass backs uuid values: attribute access and comparisons.
var UUIDClass = &object.Class{
	Name: "UUID",
	Methods: map[string]object.Object{
		"__str__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				inst := args[0].(*object.Instance)
				if repr, ok := inst.Field("__str_repr__").(*object.String); ok {
					return repr
				}
				return object.NewString("")
			},
			HelpText: "__str__() - Canonical hyphenated form",
		},
		"__eq__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				inst := args[0].(*object.Instance)
				other, ok := args[1].(*object.Instance)
				if !ok {
					return object.NewBoolean(false)
				}
				a, _ := inst.Field("__str_repr__").(*object.String)
				b, _ := other.Field("__str_repr__").(*object.String)
				return object.NewBoolean(a.StringValue() == b.StringValue())
			},
			HelpText: "__eq__(other) - Compare by value",
		},
		// UUIDs hash by value (Python UUIDs hash their int), so they work as
		// dict keys and set members.
		"__hash__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				inst := args[0].(*object.Instance)
				repr, _ := inst.Field("__str_repr__").(*object.String)
				h := uint64(14695981039346656037)
				for _, c := range repr.StringValue() {
					h ^= uint64(c)
					h *= 1099511628211
				}
				return object.NewInteger(int64(h))
			},
			HelpText: "__hash__() - Hash by UUID value",
		},
		"__repr__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				inst := args[0].(*object.Instance)
				if repr, ok := inst.Field("__str_repr__").(*object.String); ok {
					return object.NewString("UUID('" + repr.StringValue() + "')")
				}
				return object.NewString("UUID('')")
			},
			HelpText: "__repr__() - Constructor-style form",
		},
	},
}

func parseUUIDString(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}
