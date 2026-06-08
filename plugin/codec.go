package plugin

import (
	"context"
	"fmt"

	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

const remoteFieldName = "__plugin_remote"

type remoteObject struct {
	Client        *Client
	Library       string
	Class         string
	EnvironmentID string
	ID            string
	Released      bool
}

func objectToValue(obj object.Object) (Value, error) {
	switch v := obj.(type) {
	case nil, *object.Null:
		return Value{Type: valueNull}, nil
	case *object.Boolean:
		b, _ := v.AsBool()
		return Value{Type: valueBool, Value: b}, nil
	case *object.Integer:
		return Value{Type: valueInt, Value: v.IntValue()}, nil
	case *object.Float:
		return Value{Type: valueFloat, Value: v.FloatValue()}, nil
	case *object.String:
		return Value{Type: valueString, Value: v.StringValue()}, nil
	case *object.List:
		items := make([]Value, 0, len(v.Elements))
		for _, item := range v.Elements {
			encoded, err := objectToValue(item)
			if err != nil {
				return Value{}, err
			}
			items = append(items, encoded)
		}
		return Value{Type: valueList, Items: items}, nil
	case *object.Tuple:
		items := make([]Value, 0, len(v.Elements))
		for _, item := range v.Elements {
			encoded, err := objectToValue(item)
			if err != nil {
				return Value{}, err
			}
			items = append(items, encoded)
		}
		return Value{Type: valueList, Items: items}, nil
	case *object.Dict:
		entries := make(map[string]Value, len(v.Pairs))
		for _, pair := range v.Pairs {
			key, ok := pair.Key.(*object.String)
			if !ok {
				return Value{}, fmt.Errorf("plugin transport only supports dicts with string keys")
			}
			encoded, err := objectToValue(pair.Value)
			if err != nil {
				return Value{}, err
			}
			entries[key.StringValue()] = encoded
		}
		return Value{Type: valueDict, Entries: entries}, nil
	case *object.Instance:
		if remote, ok := remoteFromInstance(v); ok {
			return Value{
				Type: valueRemote,
				Remote: &RemoteRef{
					Library:       remote.Library,
					Class:         remote.Class,
					EnvironmentID: remote.EnvironmentID,
					ID:            remote.ID,
				},
			}, nil
		}
		return Value{}, fmt.Errorf("cannot pass non-plugin instance %s to plugin", v.Class.Name)
	default:
		return Value{}, fmt.Errorf("unsupported plugin value type %s", obj.Type())
	}
}

func objectToValueForCall(client *Client, env *object.Environment, obj object.Object) (Value, error) {
	switch obj.(type) {
	case *object.Function, *object.LambdaFunction, *object.Builtin:
		callbackID := client.RegisterCallback(obj, env)
		return Value{Type: valueCallback, Value: callbackID}, nil
	default:
		return objectToValue(obj)
	}
}

func valueToObject(value Value) (object.Object, error) {
	switch value.Type {
	case "", valueNull:
		return &object.Null{}, nil
	case valueBool:
		if b, ok := value.Value.(bool); ok {
			return object.NewBoolean(b), nil
		}
		return nil, fmt.Errorf("invalid bool transport value")
	case valueInt:
		return object.NewInteger(numberToInt64(value.Value)), nil
	case valueFloat:
		return object.NewFloat(numberToFloat64(value.Value)), nil
	case valueString:
		if s, ok := value.Value.(string); ok {
			return object.NewString(s), nil
		}
		return nil, fmt.Errorf("invalid string transport value")
	case valueList:
		items := make([]object.Object, 0, len(value.Items))
		for _, item := range value.Items {
			decoded, err := valueToObject(item)
			if err != nil {
				return nil, err
			}
			items = append(items, decoded)
		}
		return &object.List{Elements: items}, nil
	case valueDict:
		entries := make(map[string]object.Object, len(value.Entries))
		for key, item := range value.Entries {
			decoded, err := valueToObject(item)
			if err != nil {
				return nil, err
			}
			entries[key] = decoded
		}
		return object.NewStringDict(entries), nil
	case valueRemote:
		return nil, fmt.Errorf("remote values require a plugin client")
	default:
		return nil, fmt.Errorf("unsupported plugin transport value type %q", value.Type)
	}
}

func valuesFromObjects(args []object.Object) ([]Value, error) {
	values := make([]Value, 0, len(args))
	for _, arg := range args {
		encoded, err := objectToValue(arg)
		if err != nil {
			return nil, err
		}
		values = append(values, encoded)
	}
	return values, nil
}

func valuesFromObjectsForCall(ctx context.Context, client *Client, args []object.Object) ([]Value, []string, error) {
	env := evaluator.GetEnvFromContext(ctx)
	values := make([]Value, 0, len(args))
	callbacks := make([]string, 0)
	for _, arg := range args {
		encoded, err := objectToValueForCall(client, env, arg)
		if err != nil {
			for _, callbackID := range callbacks {
				client.UnregisterCallback(callbackID)
			}
			return nil, nil, err
		}
		if encoded.Type == valueCallback {
			if id, ok := encoded.Value.(string); ok {
				callbacks = append(callbacks, id)
			}
		}
		values = append(values, encoded)
	}
	return values, callbacks, nil
}

func valuesFromKwargs(kwargs object.Kwargs) (map[string]Value, error) {
	if len(kwargs.Kwargs) == 0 {
		return nil, nil
	}
	values := make(map[string]Value, len(kwargs.Kwargs))
	for key, arg := range kwargs.Kwargs {
		encoded, err := objectToValue(arg)
		if err != nil {
			return nil, err
		}
		values[key] = encoded
	}
	return values, nil
}

func remoteFromInstance(instance *object.Instance) (*remoteObject, bool) {
	if instance == nil || instance.Fields == nil {
		return nil, false
	}
	wrapper, ok := object.GetClientField(instance, remoteFieldName)
	if !ok {
		return nil, false
	}
	remote, ok := wrapper.Client.(*remoteObject)
	return remote, ok
}

func numberToInt64(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case jsonNumber:
		i, _ := v.Int64()
		return i
	default:
		return 0
	}
}

func numberToFloat64(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case jsonNumber:
		f, _ := v.Float64()
		return f
	default:
		return 0
	}
}

type jsonNumber interface {
	Int64() (int64, error)
	Float64() (float64, error)
}
