package plugin

import (
	"encoding/json"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestObjectToValue(t *testing.T) {
	tests := []struct {
		name    string
		obj     object.Object
		wantTyp string
		wantVal any
	}{
		{"null", &object.Null{}, valueNull, nil},
		{"nil", nil, valueNull, nil},
		{"bool true", object.NewBoolean(true), valueBool, true},
		{"bool false", object.NewBoolean(false), valueBool, false},
		{"int", object.NewInteger(42), valueInt, int64(42)},
		{"float", object.NewFloat(3.14), valueFloat, 3.14},
		{"string", object.NewString("hello"), valueString, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := objectToValue(tt.obj)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v.Type != tt.wantTyp {
				t.Errorf("type = %q, want %q", v.Type, tt.wantTyp)
			}
			if tt.wantVal != nil && v.Value != tt.wantVal {
				t.Errorf("value = %v, want %v", v.Value, tt.wantVal)
			}
		})
	}
}

func TestObjectToValueList(t *testing.T) {
	list := &object.List{Elements: []object.Object{
		object.NewInteger(1),
		object.NewString("two"),
		object.NewBoolean(true),
	}}
	v, err := objectToValue(list)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != valueList {
		t.Fatalf("type = %q, want %q", v.Type, valueList)
	}
	if len(v.Items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(v.Items))
	}
	if v.Items[0].Type != valueInt || v.Items[1].Type != valueString || v.Items[2].Type != valueBool {
		t.Errorf("list item types: %s, %s, %s", v.Items[0].Type, v.Items[1].Type, v.Items[2].Type)
	}
}

func TestObjectToValueTuple(t *testing.T) {
	tuple := &object.Tuple{Elements: []object.Object{
		object.NewInteger(1),
		object.NewString("two"),
	}}
	v, err := objectToValue(tuple)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != valueList {
		t.Fatalf("type = %q, want %q", v.Type, valueList)
	}
	if len(v.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(v.Items))
	}
}

func TestObjectToValueDict(t *testing.T) {
	dict := object.NewStringDict(map[string]object.Object{
		"name": object.NewString("Ada"),
		"age":  object.NewInteger(30),
	})
	v, err := objectToValue(dict)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != valueDict {
		t.Fatalf("type = %q, want %q", v.Type, valueDict)
	}
	if len(v.Entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(v.Entries))
	}
	if v.Entries["name"].Type != valueString || v.Entries["age"].Type != valueInt {
		t.Errorf("dict entry types incorrect")
	}
}

func TestObjectToValueNested(t *testing.T) {
	inner := object.NewStringDict(map[string]object.Object{
		"key": object.NewString("val"),
	})
	outer := &object.List{Elements: []object.Object{
		object.NewInteger(1),
		inner,
		&object.List{Elements: []object.Object{object.NewBoolean(true)}},
	}}
	v, err := objectToValue(outer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != valueList {
		t.Fatalf("type = %q, want %q", v.Type, valueList)
	}
	if len(v.Items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(v.Items))
	}
	if v.Items[1].Type != valueDict {
		t.Errorf("nested dict type = %q, want %q", v.Items[1].Type, valueDict)
	}
	if v.Items[2].Type != valueList {
		t.Errorf("nested list type = %q, want %q", v.Items[2].Type, valueList)
	}
}

func TestObjectToValueErrors(t *testing.T) {
	t.Run("non-string dict key", func(t *testing.T) {
		dict := &object.Dict{Pairs: map[string]object.DictPair{
			"i": {Key: object.NewInteger(1), Value: object.NewString("v")},
		}}
		_, err := objectToValue(dict)
		if err == nil {
			t.Fatal("expected error for non-string dict key")
		}
	})

	t.Run("non-plugin instance", func(t *testing.T) {
		inst := &object.Instance{
			Class:  &object.Class{Name: "Local"},
			Fields: map[string]object.Object{},
		}
		_, err := objectToValue(inst)
		if err == nil {
			t.Fatal("expected error for non-plugin instance")
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		_, err := objectToValue(&object.Builtin{})
		if err == nil {
			t.Fatal("expected error for unsupported type")
		}
	})

	t.Run("nested error in list", func(t *testing.T) {
		list := &object.List{Elements: []object.Object{
			object.NewInteger(1),
			&object.Builtin{},
		}}
		_, err := objectToValue(list)
		if err == nil {
			t.Fatal("expected error from nested unsupported type in list")
		}
	})
}

func TestObjectToValueRemoteInstance(t *testing.T) {
	inst := &object.Instance{
		Class:  &object.Class{Name: "Config"},
		Fields: map[string]object.Object{},
	}
	inst.Fields[remoteFieldName] = &object.ClientWrapper{
		TypeName: "Config",
		Client:   &remoteObject{Library: "plugin.test", Class: "Config", ID: "42"},
	}

	v, err := objectToValue(inst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Type != valueRemote {
		t.Fatalf("type = %q, want %q", v.Type, valueRemote)
	}
	if v.Remote == nil {
		t.Fatal("expected remote ref")
	}
	if v.Remote.Library != "plugin.test" || v.Remote.Class != "Config" || v.Remote.ID != "42" {
		t.Errorf("remote = %+v", v.Remote)
	}
}

func TestValueToObject(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		check func(t *testing.T, obj object.Object)
	}{
		{"null", Value{Type: valueNull}, func(t *testing.T, obj object.Object) {
			if _, ok := obj.(*object.Null); !ok {
				t.Errorf("expected Null, got %T", obj)
			}
		}},
		{"empty type", Value{Type: ""}, func(t *testing.T, obj object.Object) {
			if _, ok := obj.(*object.Null); !ok {
				t.Errorf("expected Null for empty type, got %T", obj)
			}
		}},
		{"bool", Value{Type: valueBool, Value: true}, func(t *testing.T, obj object.Object) {
			b, ok := obj.(*object.Boolean)
			if !ok || !b.BoolValue() {
				t.Errorf("expected true, got %v", obj)
			}
		}},
		{"int", Value{Type: valueInt, Value: int64(42)}, func(t *testing.T, obj object.Object) {
			i, ok := obj.(*object.Integer)
			if !ok || i.IntValue() != 42 {
				t.Errorf("expected 42, got %v", obj)
			}
		}},
		{"float", Value{Type: valueFloat, Value: 3.14}, func(t *testing.T, obj object.Object) {
			f, ok := obj.(*object.Float)
			if !ok || f.FloatValue() != 3.14 {
				t.Errorf("expected 3.14, got %v", obj)
			}
		}},
		{"string", Value{Type: valueString, Value: "hello"}, func(t *testing.T, obj object.Object) {
			s, ok := obj.(*object.String)
			if !ok || s.StringValue() != "hello" {
				t.Errorf("expected 'hello', got %v", obj)
			}
		}},
		{"list", Value{Type: valueList, Items: []Value{
			{Type: valueInt, Value: int64(1)},
			{Type: valueString, Value: "two"},
		}}, func(t *testing.T, obj object.Object) {
			l, ok := obj.(*object.List)
			if !ok {
				t.Fatalf("expected List, got %T", obj)
			}
			if len(l.Elements) != 2 {
				t.Errorf("len = %d, want 2", len(l.Elements))
			}
		}},
		{"dict", Value{Type: valueDict, Entries: map[string]Value{
			"name": {Type: valueString, Value: "Ada"},
		}}, func(t *testing.T, obj object.Object) {
			d, ok := obj.(*object.Dict)
			if !ok {
				t.Fatalf("expected Dict, got %T", obj)
			}
			if len(d.Pairs) != 1 {
				t.Errorf("len = %d, want 1", len(d.Pairs))
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := valueToObject(tt.value)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, obj)
		})
	}
}

func TestValueToObjectErrors(t *testing.T) {
	t.Run("invalid bool", func(t *testing.T) {
		_, err := valueToObject(Value{Type: valueBool, Value: "not a bool"})
		if err == nil {
			t.Fatal("expected error for invalid bool")
		}
	})
	t.Run("invalid string", func(t *testing.T) {
		_, err := valueToObject(Value{Type: valueString, Value: 42})
		if err == nil {
			t.Fatal("expected error for invalid string")
		}
	})
	t.Run("remote without client", func(t *testing.T) {
		_, err := valueToObject(Value{Type: valueRemote})
		if err == nil {
			t.Fatal("expected error for remote without client")
		}
	})
	t.Run("unknown type", func(t *testing.T) {
		_, err := valueToObject(Value{Type: "bogus"})
		if err == nil {
			t.Fatal("expected error for unknown type")
		}
	})
	t.Run("nested error in list", func(t *testing.T) {
		_, err := valueToObject(Value{Type: valueList, Items: []Value{
			{Type: "bogus"},
		}})
		if err == nil {
			t.Fatal("expected error from nested unknown type in list")
		}
	})
	t.Run("nested error in dict", func(t *testing.T) {
		_, err := valueToObject(Value{Type: valueDict, Entries: map[string]Value{
			"k": {Type: "bogus"},
		}})
		if err == nil {
			t.Fatal("expected error from nested unknown type in dict")
		}
	})
}

func TestValuesFromObjects(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		vals, err := valuesFromObjects(nil)
		if err != nil || len(vals) != 0 {
			t.Fatalf("expected empty, got %v %v", vals, err)
		}
	})
	t.Run("multiple", func(t *testing.T) {
		vals, err := valuesFromObjects([]object.Object{
			object.NewInteger(1),
			object.NewString("two"),
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(vals) != 2 {
			t.Fatalf("len = %d, want 2", len(vals))
		}
	})
	t.Run("error", func(t *testing.T) {
		_, err := valuesFromObjects([]object.Object{&object.Builtin{}})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestValuesFromKwargs(t *testing.T) {
	t.Run("nil kwargs", func(t *testing.T) {
		m, err := valuesFromKwargs(object.NewKwargs(nil))
		if err != nil || m != nil {
			t.Fatalf("expected nil, got %v %v", m, err)
		}
	})
	t.Run("empty kwargs", func(t *testing.T) {
		m, err := valuesFromKwargs(object.NewKwargs(map[string]object.Object{}))
		if err != nil || m != nil {
			t.Fatalf("expected nil, got %v %v", m, err)
		}
	})
	t.Run("with values", func(t *testing.T) {
		m, err := valuesFromKwargs(object.NewKwargs(map[string]object.Object{
			"key": object.NewString("val"),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["key"].Type != valueString {
			t.Errorf("expected string type, got %q", m["key"].Type)
		}
	})
	t.Run("error", func(t *testing.T) {
		_, err := valuesFromKwargs(object.NewKwargs(map[string]object.Object{
			"bad": &object.Builtin{},
		}))
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRemoteFromInstance(t *testing.T) {
	t.Run("nil instance", func(t *testing.T) {
		_, ok := remoteFromInstance(nil)
		if ok {
			t.Error("expected false for nil")
		}
	})
	t.Run("nil fields", func(t *testing.T) {
		_, ok := remoteFromInstance(&object.Instance{})
		if ok {
			t.Error("expected false for nil fields")
		}
	})
	t.Run("no remote field", func(t *testing.T) {
		inst := &object.Instance{Fields: map[string]object.Object{}}
		_, ok := remoteFromInstance(inst)
		if ok {
			t.Error("expected false for missing field")
		}
	})
	t.Run("wrong wrapper type", func(t *testing.T) {
		inst := &object.Instance{Fields: map[string]object.Object{
			remoteFieldName: &object.ClientWrapper{TypeName: "X", Client: "not a remoteObject"},
		}}
		_, ok := remoteFromInstance(inst)
		if ok {
			t.Error("expected false for wrong wrapper client type")
		}
	})
	t.Run("success", func(t *testing.T) {
		remote := &remoteObject{Library: "test", Class: "C", ID: "1"}
		inst := &object.Instance{Fields: map[string]object.Object{
			remoteFieldName: &object.ClientWrapper{TypeName: "C", Client: remote},
		}}
		got, ok := remoteFromInstance(inst)
		if !ok {
			t.Fatal("expected true")
		}
		if got.Library != "test" || got.ID != "1" {
			t.Errorf("got %+v", got)
		}
	})
}

func TestNumberToInt64(t *testing.T) {
	tests := []struct {
		input any
		want  int64
	}{
		{int(42), 42},
		{int64(42), 42},
		{float64(42.5), 42},
		{json.Number("42"), 42},
		{"bad", 0},
	}
	for _, tt := range tests {
		got := numberToInt64(tt.input)
		if got != tt.want {
			t.Errorf("numberToInt64(%v) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestNumberToFloat64(t *testing.T) {
	tests := []struct {
		input any
		want  float64
	}{
		{float64(3.14), 3.14},
		{int(42), 42.0},
		{int64(42), 42.0},
		{json.Number("3.14"), 3.14},
		{"bad", 0.0},
	}
	for _, tt := range tests {
		got := numberToFloat64(tt.input)
		if got != tt.want {
			t.Errorf("numberToFloat64(%v) = %f, want %f", tt.input, got, tt.want)
		}
	}
}
