package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// CopyLibrary provides Python-like copy functions
var CopyLibrary = object.NewLibrary(map[string]*object.Builtin{
	"copy": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// copy(obj) - Shallow copy
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return shallowCopy(args[0])
		},
		HelpText: `copy(obj) - Create a shallow copy

Creates a shallow copy of the object. Nested objects are not copied,
only references are shared.

Example:
  original = [1, [2, 3]]
  copied = copy.copy(original)
  copied[0] = 99        # original unchanged
  copied[1][0] = 99     # original also changed (shared reference)`,
	},
	"deepcopy": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			// deepcopy(obj) - Deep copy
			if len(args) != 1 {
				return errors.NewArgumentError(len(args), 1)
			}
			return deepCopy(args[0], make(map[uintptr]object.Object))
		},
		HelpText: `deepcopy(obj) - Create a deep copy

Creates a deep copy of the object. All nested objects are also copied.

Example:
  original = [1, [2, 3]]
  copied = copy.deepcopy(original)
  copied[1][0] = 99     # original unchanged`,
	},
}, nil, "Python-compatible copy library for shallow and deep copying")

// shallowCopy creates a shallow copy of an object
func shallowCopy(obj object.Object) object.Object {
	switch v := obj.(type) {
	case *object.List:
		newElements := make([]object.Object, len(v.Elements))
		copy(newElements, v.Elements)
		return &object.List{Elements: newElements}

	case *object.Dict:
		newPairs := make(map[string]object.DictPair, len(v.Pairs))
		for k, pair := range v.Pairs {
			newPairs[k] = pair
		}
		return &object.Dict{Pairs: newPairs}

	case *object.Tuple:
		newElements := make([]object.Object, len(v.Elements))
		copy(newElements, v.Elements)
		return &object.Tuple{Elements: newElements}

	case *object.Integer:
		return object.NewInteger(v.Value)

	case *object.Float:
		return &object.Float{Value: v.Value}

	case *object.String:
		return &object.String{Value: v.Value}

	case *object.Boolean:
		if v.Value {
			return &object.Boolean{Value: true}
		}
		return &object.Boolean{Value: false}

	case *object.Null:
		return &object.Null{}

	default:
		// For other types, return the same object (immutable or unsupported)
		return obj
	}
}

// deepCopy creates a deep copy of an object
// The memo map is used to handle circular references
func deepCopy(obj object.Object, memo map[uintptr]object.Object) object.Object {
	switch v := obj.(type) {
	case *object.List:
		newElements := make([]object.Object, len(v.Elements))
		for i, elem := range v.Elements {
			newElements[i] = deepCopy(elem, memo)
		}
		return &object.List{Elements: newElements}

	case *object.Dict:
		newPairs := make(map[string]object.DictPair, len(v.Pairs))
		for k, pair := range v.Pairs {
			newPairs[k] = object.DictPair{
				Key:   deepCopy(pair.Key, memo),
				Value: deepCopy(pair.Value, memo),
			}
		}
		return &object.Dict{Pairs: newPairs}

	case *object.Tuple:
		newElements := make([]object.Object, len(v.Elements))
		for i, elem := range v.Elements {
			newElements[i] = deepCopy(elem, memo)
		}
		return &object.Tuple{Elements: newElements}

	case *object.Integer:
		return object.NewInteger(v.Value)

	case *object.Float:
		return &object.Float{Value: v.Value}

	case *object.String:
		return &object.String{Value: v.Value}

	case *object.Boolean:
		if v.Value {
			return &object.Boolean{Value: true}
		}
		return &object.Boolean{Value: false}

	case *object.Null:
		return &object.Null{}

	default:
		// For other types, return the same object
		return obj
	}
}
