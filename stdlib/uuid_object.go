package stdlib

import (
	"context"
	"fmt"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"

	"github.com/google/uuid"
)

// The uuid library returns UUID *objects* (not strings): str() renders the
// canonical form, .int/.hex/.version/.variant expose the fields, and
// uuid3/uuid5 give deterministic name-based UUIDs. Constants provide the
// standard namespace UUIDs.

// uuidNamespaceConstants are real UUID objects (as in Python), so they compare
// equal to uuid.UUID(...) of the same value and expose .hex/.version.
var uuidNamespaceConstants = map[string]object.Object{
	"NAMESPACE_DNS":  uuidInstance(uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")),
	"NAMESPACE_URL":  uuidInstance(uuid.MustParse("6ba7b811-9dad-11d1-80b4-00c04fd430c8")),
	"NAMESPACE_OID":  uuidInstance(uuid.MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430c8")),
	"NAMESPACE_X500": uuidInstance(uuid.MustParse("6ba7b814-9dad-11d1-80b4-00c04fd430c8")),
}

// uuidStringOf returns the canonical string of a UUID object or a plain
// string, for functions accepting either (uuid3/uuid5 namespaces).
func uuidStringOf(obj object.Object) (string, object.Object) {
	if inst, ok := obj.(*object.Instance); ok {
		if repr, ok := inst.Field("__str_repr__").(*object.String); ok {
			return repr.StringValue(), nil
		}
	}
	return stringOrError(obj)
}

func stringOrError(obj object.Object) (string, object.Object) {
	str, err := obj.AsString()
	if err != nil {
		return "", err
	}
	return str, nil
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
		"bytes":        object.NewBytes(id[:]),
		"urn":          object.NewString("urn:uuid:" + id.String()),
	}
	// .int is deliberately absent: a UUID is a 128-bit integer and scriptling
	// ints are 64-bit, so any value here would be silently wrong.
	inst := object.NewInstanceWithFields(UUIDClass, fields)
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
				a, okA := uuidReprOf(args[0])
				b, okB := uuidReprOf(args[1])
				return object.NewBoolean(okA && okB && a == b)
			},
			HelpText: "__eq__(other) - Compare by value",
		},
		// Ordering follows the 128-bit value; the canonical lowercase hex form
		// sorts identically, so uuid7() values order by creation time.
		"__lt__": uuidOrderMethod("<", func(c int) bool { return c < 0 }),
		"__le__": uuidOrderMethod("<=", func(c int) bool { return c <= 0 }),
		"__gt__": uuidOrderMethod(">", func(c int) bool { return c > 0 }),
		"__ge__": uuidOrderMethod(">=", func(c int) bool { return c >= 0 }),
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

// uuidReprOf returns the canonical string of a UUID object.
func uuidReprOf(obj object.Object) (string, bool) {
	inst, ok := obj.(*object.Instance)
	if !ok || inst.Class.Name != "UUID" { // not a pointer compare: UUIDClass's initialiser refers to this
		return "", false
	}
	repr, ok := inst.Field("__str_repr__").(*object.String)
	if !ok {
		return "", false
	}
	return repr.StringValue(), true
}

func uuidOrderMethod(op string, keep func(cmp int) bool) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, okA := uuidReprOf(args[0])
			b, okB := uuidReprOf(args[1])
			if !okA || !okB {
				return errors.NewTypeErrorTagged("'%s' not supported between instances of 'UUID' and '%s'", op, getTypeNameFor(args[1]))
			}
			return object.NewBoolean(keep(strings.Compare(a, b)))
		},
		HelpText: op + "(other) - Compare UUID values",
	}
}

func parseUUIDString(s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}
