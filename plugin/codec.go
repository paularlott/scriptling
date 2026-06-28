package plugin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const remoteFieldName = "__plugin_remote"

type remoteObject struct {
	Client   *Client
	Library  string
	Class    string
	ID       string
	Released bool
}

type callbackSet struct {
	callbacks map[string]object.Object
}

var nextCallbackID atomic.Int64

func newCallbackSet() *callbackSet {
	return &callbackSet{callbacks: make(map[string]object.Object)}
}

func (s *callbackSet) add(fn object.Object) string {
	id := "cb-" + strconv.FormatInt(nextCallbackID.Add(1), 10)
	s.callbacks[id] = fn
	return id
}

func (s *callbackSet) get(id string) (object.Object, bool) {
	fn, ok := s.callbacks[id]
	return fn, ok
}

func objectToValue(obj object.Object) (Value, error) {
	return objectToValueWithCallbacks(obj, nil)
}

func objectToValueWithCallbacks(obj object.Object, callbacks *callbackSet) (Value, error) {
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
			encoded, err := objectToValueWithCallbacks(item, callbacks)
			if err != nil {
				return Value{}, err
			}
			items = append(items, encoded)
		}
		return Value{Type: valueList, Items: items}, nil
	case *object.Tuple:
		items := make([]Value, 0, len(v.Elements))
		for _, item := range v.Elements {
			encoded, err := objectToValueWithCallbacks(item, callbacks)
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
			encoded, err := objectToValueWithCallbacks(pair.Value, callbacks)
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
					Library: remote.Library,
					Class:   remote.Class,
					ID:      remote.ID,
				},
			}, nil
		}
		return Value{}, fmt.Errorf("cannot pass non-plugin instance %s to plugin", v.Class.Name)
	case *object.Function, *object.LambdaFunction, *object.Builtin:
		if callbacks == nil {
			return Value{}, fmt.Errorf("callbacks can only be passed during plugin calls")
		}
		return Value{
			Type: valueCallback,
			Callback: &CallbackRef{
				ID: callbacks.add(v),
			},
		}, nil
	case *object.Error:
		return Value{Type: valueString, Value: v.Message}, nil
	default:
		return Value{}, fmt.Errorf("unsupported plugin value type %s", obj.Type())
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
	case valueCallback:
		if value.Callback == nil || value.Callback.ID == "" {
			return nil, fmt.Errorf("invalid callback transport value")
		}
		return &object.ClientWrapper{
			TypeName: "PluginCallback",
			Client:   &callbackHandle{id: value.Callback.ID},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported plugin transport value type %q", value.Type)
	}
}

func transportValueToAny(value Value) any {
	switch value.Type {
	case "", valueNull:
		return nil
	case valueBool, valueString:
		return value.Value
	case valueInt:
		return numberToInt64(value.Value)
	case valueFloat:
		return numberToFloat64(value.Value)
	case valueList:
		items := make([]any, 0, len(value.Items))
		for _, item := range value.Items {
			items = append(items, transportValueToAny(item))
		}
		return items
	case valueDict:
		entries := make(map[string]any, len(value.Entries))
		for key, item := range value.Entries {
			entries[key] = transportValueToAny(item)
		}
		return entries
	default:
		return value.Value
	}
}

func valuesFromObjects(args []object.Object) ([]Value, error) {
	return valuesFromObjectsWithCallbacks(args, nil)
}

func valuesFromObjectsWithCallbacks(args []object.Object, callbacks *callbackSet) ([]Value, error) {
	values := make([]Value, 0, len(args))
	for _, arg := range args {
		encoded, err := objectToValueWithCallbacks(arg, callbacks)
		if err != nil {
			return nil, err
		}
		values = append(values, encoded)
	}
	return values, nil
}

func valuesFromKwargs(kwargs object.Kwargs) (map[string]Value, error) {
	return valuesFromKwargsWithCallbacks(kwargs, nil)
}

func valuesFromKwargsWithCallbacks(kwargs object.Kwargs, callbacks *callbackSet) (map[string]Value, error) {
	if len(kwargs.Kwargs) == 0 {
		return nil, nil
	}
	values := make(map[string]Value, len(kwargs.Kwargs))
	for key, arg := range kwargs.Kwargs {
		encoded, err := objectToValueWithCallbacks(arg, callbacks)
		if err != nil {
			return nil, err
		}
		values[key] = encoded
	}
	return values, nil
}

func remoteFromInstance(instance *object.Instance) (*remoteObject, bool) {
	if instance == nil {
		return nil, false
	}
	wrapper, ok := object.GetClientField(instance, remoteFieldName)
	if !ok {
		return nil, false
	}
	remote, ok := wrapper.Client.(*remoteObject)
	return remote, ok
}

func callHostCallback(ctx context.Context, callbacks *callbackSet, params callbackCallParams) (Value, error) {
	if callbacks == nil {
		return Value{}, fmt.Errorf("callback %s is not active", params.ID)
	}
	fn, ok := callbacks.get(params.ID)
	if !ok {
		return Value{}, fmt.Errorf("unknown callback %s", params.ID)
	}
	args, err := transportValuesToObjects(params.Args)
	if err != nil {
		return Value{}, err
	}
	kwargs, err := transportKwargsToObjects(params.Kwargs)
	if err != nil {
		return Value{}, err
	}
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return Value{}, fmt.Errorf("callback execution requires evaluator in context")
	}
	result := eval.CallObjectFunction(ctx, fn, args, kwargs, nil)
	if errObj, ok := result.(*object.Error); ok {
		return Value{}, errors.New(errObj.Message)
	}
	return objectToValue(result)
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
