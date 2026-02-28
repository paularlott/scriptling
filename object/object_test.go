package object

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/ast"
)

func TestObjectTypes(t *testing.T) {
	tests := []struct {
		obj      Object
		expected ObjectType
	}{
		{&Integer{Value: 42}, INTEGER_OBJ},
		{&Float{Value: 3.14}, FLOAT_OBJ},
		{&Boolean{Value: true}, BOOLEAN_OBJ},
		{&String{Value: "hello"}, STRING_OBJ},
		{&Null{}, NULL_OBJ},
		{&ReturnValue{Value: &Integer{Value: 1}}, RETURN_OBJ},
		{&Break{}, BREAK_OBJ},
		{&Continue{}, CONTINUE_OBJ},
		{&Function{}, FUNCTION_OBJ},
		{&Builtin{}, BUILTIN_OBJ},
		{&List{}, LIST_OBJ},
		{&Dict{}, DICT_OBJ},
		{&Error{Message: "test"}, ERROR_OBJ},
		{&Exception{Message: "test"}, EXCEPTION_OBJ},
	}

	for _, tt := range tests {
		if tt.obj.Type() != tt.expected {
			t.Errorf("obj.Type() = %q, want %q", tt.obj.Type(), tt.expected)
		}
	}
}

func TestObjectInspect(t *testing.T) {
	tests := []struct {
		obj      Object
		expected string
	}{
		{&Integer{Value: 42}, "42"},
		{&Float{Value: 3.14}, "3.14"},
		{&Boolean{Value: true}, "true"},
		{&Boolean{Value: false}, "false"},
		{&String{Value: "hello"}, "hello"},
		{&Null{}, "None"},
		{&Break{}, "break"},
		{&Continue{}, "continue"},
		{&Function{}, "<function>"},
		{&Builtin{}, "<builtin function>"},
		{&Error{Message: "test error"}, "ERROR: test error"},
		{&Exception{Message: "test exception"}, "EXCEPTION: test exception"},
	}

	for _, tt := range tests {
		if tt.obj.Inspect() != tt.expected {
			t.Errorf("obj.Inspect() = %q, want %q", tt.obj.Inspect(), tt.expected)
		}
	}
}

func TestListInspect(t *testing.T) {
	list := &List{
		Elements: []Object{
			&Integer{Value: 1},
			&String{Value: "hello"},
			&Boolean{Value: true},
		},
	}
	expected := "[1, hello, true]"
	if list.Inspect() != expected {
		t.Errorf("list.Inspect() = %q, want %q", list.Inspect(), expected)
	}
}

func TestDictInspect(t *testing.T) {
	dict := &Dict{
		Pairs: map[string]DictPair{
			"name": {Key: &String{Value: "name"}, Value: &String{Value: "Alice"}},
			"age":  {Key: &String{Value: "age"}, Value: &Integer{Value: 30}},
		},
	}
	result := dict.Inspect()
	// Dict order is not guaranteed, so check both possibilities
	if result != "{name: Alice, age: 30}" && result != "{age: 30, name: Alice}" {
		t.Errorf("dict.Inspect() = %q, want either order", result)
	}
}

func TestEnvironment(t *testing.T) {
	env := NewEnvironment()

	// Test Set and Get
	val := &Integer{Value: 42}
	env.Set("x", val)

	result, ok := env.Get("x")
	if !ok {
		t.Fatal("expected to find variable x")
	}
	if result != val {
		t.Errorf("got %v, want %v", result, val)
	}
}

func TestEnclosedEnvironment(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("x", &Integer{Value: 10})

	inner := NewEnclosedEnvironment(outer)
	inner.Set("y", &Integer{Value: 20})

	// Inner should see outer variables
	x, ok := inner.Get("x")
	if !ok {
		t.Fatal("expected to find variable x from outer scope")
	}
	if x.(*Integer).Value != 10 {
		t.Errorf("x = %d, want 10", x.(*Integer).Value)
	}

	// Inner should see its own variables
	y, ok := inner.Get("y")
	if !ok {
		t.Fatal("expected to find variable y")
	}
	if y.(*Integer).Value != 20 {
		t.Errorf("y = %d, want 20", y.(*Integer).Value)
	}

	// Outer should not see inner variables
	_, ok = outer.Get("y")
	if ok {
		t.Error("outer scope should not see inner variable y")
	}
}

func TestGlobalVariables(t *testing.T) {
	outer := NewEnvironment()
	inner := NewEnclosedEnvironment(outer)

	// Mark variable as global in inner scope
	inner.MarkGlobal("global_var")

	// Set global variable from inner scope
	inner.Set("global_var", &Integer{Value: 42})

	// Should be set in outer (global) scope
	result, ok := outer.Get("global_var")
	if !ok {
		t.Fatal("expected global variable to be set in outer scope")
	}
	if result.(*Integer).Value != 42 {
		t.Errorf("global_var = %d, want 42", result.(*Integer).Value)
	}

	// Check IsGlobal
	if !inner.IsGlobal("global_var") {
		t.Error("expected global_var to be marked as global")
	}
}

func TestNonlocalVariables(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("nonlocal_var", &Integer{Value: 10})

	inner := NewEnclosedEnvironment(outer)
	inner.MarkNonlocal("nonlocal_var")

	// Modify nonlocal variable from inner scope
	inner.Set("nonlocal_var", &Integer{Value: 20})

	// Should be modified in outer scope
	result, ok := outer.Get("nonlocal_var")
	if !ok {
		t.Fatal("expected nonlocal variable to exist in outer scope")
	}
	if result.(*Integer).Value != 20 {
		t.Errorf("nonlocal_var = %d, want 20", result.(*Integer).Value)
	}

	// Check IsNonlocal
	if !inner.IsNonlocal("nonlocal_var") {
		t.Error("expected nonlocal_var to be marked as nonlocal")
	}
}

func TestReturnValue(t *testing.T) {
	val := &Integer{Value: 42}
	ret := &ReturnValue{Value: val}

	if ret.Type() != RETURN_OBJ {
		t.Errorf("ret.Type() = %q, want %q", ret.Type(), RETURN_OBJ)
	}
	if ret.Inspect() != "42" {
		t.Errorf("ret.Inspect() = %q, want %q", ret.Inspect(), "42")
	}
}

func TestFunction(t *testing.T) {
	// Create a simple function object
	params := []*ast.Identifier{
		{Value: "x"},
		{Value: "y"},
	}
	body := &ast.BlockStatement{}
	env := NewEnvironment()

	fn := &Function{
		Name:       "test_function",
		Parameters: params,
		Body:       body,
		Env:        env,
	}

	if fn.Type() != FUNCTION_OBJ {
		t.Errorf("fn.Type() = %q, want %q", fn.Type(), FUNCTION_OBJ)
	}
	if fn.Inspect() != "<function>" {
		t.Errorf("fn.Inspect() = %q, want %q", fn.Inspect(), "<function>")
	}
}

func TestBuiltinFunction(t *testing.T) {
	builtin := &Builtin{
		Fn: func(ctx context.Context, kwargs Kwargs, args ...Object) Object {
			return &Integer{Value: 42}
		},
	}

	if builtin.Type() != BUILTIN_OBJ {
		t.Errorf("builtin.Type() = %q, want %q", builtin.Type(), BUILTIN_OBJ)
	}
	if builtin.Inspect() != "<builtin function>" {
		t.Errorf("builtin.Inspect() = %q, want %q", builtin.Inspect(), "<builtin function>")
	}

	// Test function call
	result := builtin.Fn(context.Background(), NewKwargs(nil))
	if result.(*Integer).Value != 42 {
		t.Errorf("builtin function result = %d, want 42", result.(*Integer).Value)
	}
}

func TestKwargs(t *testing.T) {
	kwargs := map[string]Object{
		"string": &String{Value: "hello"},
		"int":    &Integer{Value: 42},
		"float":  &Float{Value: 3.14},
		"bool":   &Boolean{Value: true},
		"list":   &List{Elements: []Object{&Integer{Value: 1}}},
	}

	k := NewKwargs(kwargs)

	// Test Has
	if !k.Has("string") {
		t.Error("Kwargs.Has() should return true for existing key")
	}
	if k.Has("nonexistent") {
		t.Error("Kwargs.Has() should return false for non-existent key")
	}

	// Test Get
	if k.Get("string") == nil {
		t.Error("Kwargs.Get() should return value for existing key")
	}
	if k.Get("nonexistent") != nil {
		t.Error("Kwargs.Get() should return nil for non-existent key")
	}

	// Test Keys
	keys := k.Keys()
	if len(keys) != 5 {
		t.Errorf("Kwargs.Keys() length = %d, want 5", len(keys))
	}

	// Test Len
	if k.Len() != 5 {
		t.Errorf("Kwargs.Len() = %d, want 5", k.Len())
	}
}

func TestKwargsGetString(t *testing.T) {
	kwargs := map[string]Object{
		"valid":   &String{Value: "hello"},
		"invalid": &Integer{Value: 42},
	}

	k := NewKwargs(kwargs)

	// Test valid string
	val, err := k.GetString("valid", "default")
	if val != "hello" {
		t.Errorf("GetString() = %q, want %q", val, "hello")
	}
	if err != nil {
		t.Error("GetString() should not return error for valid string")
	}

	// Test invalid type
	val, err = k.GetString("invalid", "default")
	if val != "default" {
		t.Errorf("GetString() with invalid type = %q, want %q", val, "default")
	}
	if err == nil {
		t.Error("GetString() should return error for invalid type")
	}

	// Test missing key
	val, err = k.GetString("missing", "default")
	if val != "default" {
		t.Errorf("GetString() with missing key = %q, want %q", val, "default")
	}
	if err != nil {
		t.Error("GetString() should not return error for missing key")
	}
}

func TestKwargsGetInt(t *testing.T) {
	kwargs := map[string]Object{
		"valid":   &Integer{Value: 42},
		"invalid": &String{Value: "hello"},
	}

	k := NewKwargs(kwargs)

	val, err := k.GetInt("valid", 0)
	if val != 42 {
		t.Errorf("GetInt() = %d, want 42", val)
	}
	if err != nil {
		t.Error("GetInt() should not return error for valid int")
	}

	val, err = k.GetInt("invalid", 0)
	if err == nil {
		t.Error("GetInt() should return error for invalid type")
	}
}

func TestKwargsGetFloat(t *testing.T) {
	kwargs := map[string]Object{
		"valid": &Float{Value: 3.14},
	}

	k := NewKwargs(kwargs)

	val, err := k.GetFloat("valid", 0)
	if val != 3.14 {
		t.Errorf("GetFloat() = %f, want 3.14", val)
	}
	if err != nil {
		t.Error("GetFloat() should not return error for valid float")
	}
}

func TestKwargsGetBool(t *testing.T) {
	kwargs := map[string]Object{
		"valid": &Boolean{Value: true},
	}

	k := NewKwargs(kwargs)

	val, err := k.GetBool("valid", false)
	if val != true {
		t.Errorf("GetBool() = %t, want true", val)
	}
	if err != nil {
		t.Error("GetBool() should not return error for valid bool")
	}
}

func TestKwargsGetList(t *testing.T) {
	kwargs := map[string]Object{
		"valid": &List{Elements: []Object{&Integer{Value: 1}}},
	}

	k := NewKwargs(kwargs)

	val, err := k.GetList("valid", nil)
	if len(val) != 1 {
		t.Errorf("GetList() length = %d, want 1", len(val))
	}
	if err != nil {
		t.Error("GetList() should not return error for valid list")
	}
}

func TestKwargsMustMethods(t *testing.T) {
	kwargs := map[string]Object{
		"string": &String{Value: "hello"},
		"int":    &Integer{Value: 42},
	}

	k := NewKwargs(kwargs)

	// MustGetString should ignore errors
	if k.MustGetString("int", "default") != "default" {
		t.Error("MustGetString() should return default for invalid type")
	}

	// MustGetInt should ignore errors
	if k.MustGetInt("string", 0) != 0 {
		t.Error("MustGetInt() should return default for invalid type")
	}

	// Valid cases
	if k.MustGetString("string", "default") != "hello" {
		t.Error("MustGetString() should return value for valid string")
	}

	if k.MustGetInt("int", 0) != 42 {
		t.Error("MustGetInt() should return value for valid int")
	}
}

func TestSet(t *testing.T) {
	// Test NewSet
	s := NewSet()
	if s.Type() != SET_OBJ {
		t.Errorf("Set.Type() = %q, want %q", s.Type(), SET_OBJ)
	}

	// Test Add and Contains
	s.Add(&String{Value: "hello"})
	s.Add(&Integer{Value: 42})

	if !s.Contains(&String{Value: "hello"}) {
		t.Error("Set should contain added element")
	}

	// Test Remove
	if !s.Remove(&String{Value: "hello"}) {
		t.Error("Remove() should return true for existing element")
	}
	if s.Remove(&String{Value: "nonexistent"}) {
		t.Error("Remove() should return false for non-existent element")
	}

	// Test Inspect
	s = NewSet()
	s.Add(&Integer{Value: 3})
	s.Add(&Integer{Value: 1})
	s.Add(&Integer{Value: 2})

	inspect := s.Inspect()
	if inspect[0] != '{' || inspect[len(inspect)-1] != '}' {
		t.Errorf("Set.Inspect() = %q, should be wrapped in {}", inspect)
	}
}

func TestSetUnion(t *testing.T) {
	s1 := NewSetFromElements([]Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
	})
	s2 := NewSetFromElements([]Object{
		&Integer{Value: 2},
		&Integer{Value: 3},
	})

	result := s1.Union(s2)
	if result.Contains(&Integer{Value: 1}) == false ||
		result.Contains(&Integer{Value: 2}) == false ||
		result.Contains(&Integer{Value: 3}) == false {
		t.Error("Union() should contain all elements from both sets")
	}
}

func TestSetIntersection(t *testing.T) {
	s1 := NewSetFromElements([]Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
	})
	s2 := NewSetFromElements([]Object{
		&Integer{Value: 2},
		&Integer{Value: 3},
	})

	result := s1.Intersection(s2)
	if !result.Contains(&Integer{Value: 2}) {
		t.Error("Intersection() should contain common element")
	}
	if result.Contains(&Integer{Value: 1}) || result.Contains(&Integer{Value: 3}) {
		t.Error("Intersection() should not contain unique elements")
	}
}

func TestSetDifference(t *testing.T) {
	s1 := NewSetFromElements([]Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
	})
	s2 := NewSetFromElements([]Object{
		&Integer{Value: 2},
		&Integer{Value: 3},
	})

	result := s1.Difference(s2)
	if !result.Contains(&Integer{Value: 1}) {
		t.Error("Difference() should contain element only in s1")
	}
	if result.Contains(&Integer{Value: 2}) {
		t.Error("Difference() should not contain common element")
	}
}

func TestSetIsSubset(t *testing.T) {
	s1 := NewSetFromElements([]Object{&Integer{Value: 1}})
	s2 := NewSetFromElements([]Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
	})

	if !s1.IsSubset(s2) {
		t.Error("s1 should be a subset of s2")
	}
	if s2.IsSubset(s1) {
		t.Error("s2 should not be a subset of s1")
	}
}

func TestSetCopy(t *testing.T) {
	s1 := NewSetFromElements([]Object{&Integer{Value: 1}})
	s2 := s1.Copy()

	if !s2.Contains(&Integer{Value: 1}) {
		t.Error("Copy() should contain all elements")
	}

	// Modify original
	s1.Add(&Integer{Value: 2})
	if s2.Contains(&Integer{Value: 2}) {
		t.Error("Copy() should be independent")
	}
}

func TestSetAsBool(t *testing.T) {
	s1 := NewSet()
	boolVal, _ := s1.AsBool()
	if boolVal != false {
		t.Errorf("Empty set AsBool() = %t, want false", boolVal)
	}

	s1.Add(&Integer{Value: 1})
	boolVal, _ = s1.AsBool()
	if boolVal != true {
		t.Errorf("Non-empty set AsBool() = %t, want true", boolVal)
	}
}

func TestIterator(t *testing.T) {
	// Test basic iterator
	count := 0
	it := NewIterator(func() (Object, bool) {
		if count >= 3 {
			return nil, false
		}
		val := &Integer{Value: int64(count)}
		count++
		return val, true
	})

	for i := 0; i < 3; i++ {
		val, hasNext := it.Next()
		if !hasNext {
			t.Errorf("Iterator should have more elements at iteration %d", i)
		}
		if val == nil {
			t.Error("Iterator should return value")
		}
	}

	// Test exhausted
	val, hasNext := it.Next()
	if hasNext {
		t.Error("Iterator should be exhausted")
	}
	if val != nil {
		t.Error("Exhausted iterator should return nil value")
	}
}

func TestRangeIterator(t *testing.T) {
	it := NewRangeIterator(0, 3, 1)

	expected := []int64{0, 1, 2}
	for _, exp := range expected {
		val, hasNext := it.Next()
		if !hasNext {
			t.Error("RangeIterator should have more elements")
		}
		if val.(*Integer).Value != exp {
			t.Errorf("RangeIterator value = %d, want %d", val.(*Integer).Value, exp)
		}
	}

	_, hasNext := it.Next()
	if hasNext {
		t.Error("RangeIterator should be exhausted")
	}
}

func TestRangeIteratorDescending(t *testing.T) {
	it := NewRangeIterator(3, 0, -1)

	expected := []int64{3, 2, 1}
	for _, exp := range expected {
		val, hasNext := it.Next()
		if !hasNext {
			t.Error("RangeIterator should have more elements")
		}
		if val.(*Integer).Value != exp {
			t.Errorf("RangeIterator value = %d, want %d", val.(*Integer).Value, exp)
		}
	}
}

func TestZipIterator(t *testing.T) {
	list1 := &List{Elements: []Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
	}}
	list2 := &List{Elements: []Object{
		&String{Value: "a"},
		&String{Value: "b"},
	}}

	it := NewZipIterator([]Object{list1, list2})

	val, hasNext := it.Next()
	if !hasNext {
		t.Error("ZipIterator should have elements")
	}

	tuple, ok := val.(*Tuple)
	if !ok || len(tuple.Elements) != 2 {
		t.Error("ZipIterator should return tuples")
	}
}

func TestReversedIterator(t *testing.T) {
	list := &List{Elements: []Object{
		&Integer{Value: 1},
		&Integer{Value: 2},
		&Integer{Value: 3},
	}}

	it := NewReversedIterator(list)

	expected := []int64{3, 2, 1}
	for _, exp := range expected {
		val, hasNext := it.Next()
		if !hasNext {
			t.Error("ReversedIterator should have more elements")
		}
		if val.(*Integer).Value != exp {
			t.Errorf("ReversedIterator value = %d, want %d", val.(*Integer).Value, exp)
		}
	}
}

func TestEnumerateIterator(t *testing.T) {
	list := &List{Elements: []Object{
		&String{Value: "a"},
		&String{Value: "b"},
	}}

	it := NewEnumerateIterator(list, 0)

	for i := 0; i < 2; i++ {
		val, hasNext := it.Next()
		if !hasNext {
			t.Error("EnumerateIterator should have more elements")
		}

		tuple, ok := val.(*Tuple)
		if !ok || len(tuple.Elements) != 2 {
			t.Error("EnumerateIterator should return (index, value) tuples")
		}

		index := tuple.Elements[0].(*Integer).Value
		if index != int64(i) {
			t.Errorf("EnumerateIterator index = %d, want %d", index, i)
		}
	}
}

func TestIterableToSlice(t *testing.T) {
	// Test List
	list := &List{Elements: []Object{&Integer{Value: 1}, &Integer{Value: 2}}}
	slice, ok := IterableToSlice(list)
	if !ok || len(slice) != 2 {
		t.Error("IterableToSlice(List) should work")
	}

	// Test String
	str := &String{Value: "ab"}
	slice, ok = IterableToSlice(str)
	if !ok || len(slice) != 2 {
		t.Error("IterableToSlice(String) should work")
	}

	// Test Dict
	dict := &Dict{Pairs: map[string]DictPair{
		"a": {Key: &String{Value: "a"}, Value: &Integer{Value: 1}},
	}}
	slice, ok = IterableToSlice(dict)
	if !ok || len(slice) != 1 {
		t.Error("IterableToSlice(Dict) should return keys")
	}

	// Test invalid type
	invalid := &Integer{Value: 42}
	_, ok = IterableToSlice(invalid)
	if ok {
		t.Error("IterableToSlice(Integer) should fail")
	}
}

func TestDictKeys(t *testing.T) {
	dict := &Dict{Pairs: map[string]DictPair{
		"a": {Key: &String{Value: "a"}, Value: &Integer{Value: 1}},
		"b": {Key: &String{Value: "b"}, Value: &Integer{Value: 2}},
	}}

	keys := &DictKeys{Dict: dict}

	if keys.Type() != DICT_KEYS_OBJ {
		t.Errorf("DictKeys.Type() = %q, want %q", keys.Type(), DICT_KEYS_OBJ)
	}

	boolVal, _ := keys.AsBool()
	if !boolVal {
		t.Error("Non-empty DictKeys AsBool() should be true")
	}

	// Test iterator
	it := keys.CreateIterator()
	val, hasNext := it.Next()
	if !hasNext {
		t.Error("DictKeys iterator should have elements")
	}
	if _, ok := val.(*String); !ok {
		t.Error("DictKeys iterator should return String keys")
	}
}

func TestDictValues(t *testing.T) {
	dict := &Dict{Pairs: map[string]DictPair{
		"a": {Key: &String{Value: "a"}, Value: &Integer{Value: 1}},
	}}

	values := &DictValues{Dict: dict}

	if values.Type() != DICT_VALUES_OBJ {
		t.Errorf("DictValues.Type() = %q, want %q", values.Type(), DICT_VALUES_OBJ)
	}

	// Test iterator
	it := values.CreateIterator()
	_, hasNext := it.Next()
	if !hasNext {
		t.Error("DictValues iterator should have elements")
	}
}

func TestDictItems(t *testing.T) {
	dict := &Dict{Pairs: map[string]DictPair{
		"a": {Key: &String{Value: "a"}, Value: &Integer{Value: 1}},
	}}

	items := &DictItems{Dict: dict}

	if items.Type() != DICT_ITEMS_OBJ {
		t.Errorf("DictItems.Type() = %q, want %q", items.Type(), DICT_ITEMS_OBJ)
	}

	// Test iterator
	it := items.CreateIterator()
	val, hasNext := it.Next()
	if !hasNext {
		t.Error("DictItems iterator should have elements")
	}

	tuple, ok := val.(*Tuple)
	if !ok || len(tuple.Elements) != 2 {
		t.Error("DictItems iterator should return (key, value) tuples")
	}
}

func TestIsError(t *testing.T) {
	err := &Error{Message: "test error"}
	if !IsError(err) {
		t.Error("IsError(Error) should return true")
	}

	if IsError(&Integer{Value: 42}) {
		t.Error("IsError(Integer) should return false")
	}

	if IsError(nil) {
		t.Error("IsError(nil) should return false")
	}
}

// Test LibraryBuilder methods
func TestLibraryBuilderAlias(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")
	builder.Function("add", func(a, b int) int { return a + b })
	builder.Alias("sum", "add")

	lib := builder.Build()
	if _, ok := lib.Functions()["sum"]; !ok {
		t.Error("Alias should create sum function")
	}
}

func TestLibraryBuilderDescription(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")

	if builder.GetDescription() != "Test library" {
		t.Errorf("GetDescription = %q, want 'Test library'", builder.GetDescription())
	}

	builder.Description("New description")
	if builder.GetDescription() != "New description" {
		t.Errorf("GetDescription = %q, want 'New description'", builder.GetDescription())
	}
}

func TestLibraryBuilderHasFunction(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")

	if builder.HasFunction("foo") {
		t.Error("HasFunction should return false for non-existent function")
	}

	builder.Function("foo", func() int { return 42 })

	if !builder.HasFunction("foo") {
		t.Error("HasFunction should return true for existing function")
	}
}

func TestLibraryBuilderHasConstant(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")

	if builder.HasConstant("PI") {
		t.Error("HasConstant should return false for non-existent constant")
	}

	builder.Constant("PI", &Float{Value: 3.14})

	if !builder.HasConstant("PI") {
		t.Error("HasConstant should return true for existing constant")
	}
}

func TestLibraryBuilderRemoveFunction(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")
	builder.Function("foo", func() int { return 42 })

	if !builder.HasFunction("foo") {
		t.Error("Function should exist")
	}

	builder.RemoveFunction("foo")

	if builder.HasFunction("foo") {
		t.Error("RemoveFunction should remove function")
	}
}

func TestLibraryBuilderRemoveConstant(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")
	builder.Constant("PI", &Float{Value: 3.14})

	if !builder.HasConstant("PI") {
		t.Error("Constant should exist")
	}

	builder.RemoveConstant("PI")

	if builder.HasConstant("PI") {
		t.Error("RemoveConstant should remove constant")
	}
}

func TestLibraryBuilderFunctionCount(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")

	if builder.FunctionCount() != 0 {
		t.Errorf("FunctionCount = %d, want 0", builder.FunctionCount())
	}

	builder.Function("foo", func() int { return 42 })
	builder.Function("bar", func() int { return 43 })

	if builder.FunctionCount() != 2 {
		t.Errorf("FunctionCount = %d, want 2", builder.FunctionCount())
	}
}

func TestLibraryBuilderConstantCount(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")

	if builder.ConstantCount() != 0 {
		t.Errorf("ConstantCount = %d, want 0", builder.ConstantCount())
	}

	builder.Constant("PI", &Float{Value: 3.14})
	builder.Constant("E", &Float{Value: 2.71})

	if builder.ConstantCount() != 2 {
		t.Errorf("ConstantCount = %d, want 2", builder.ConstantCount())
	}
}

func TestLibraryBuilderClear(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")
	builder.Function("foo", func() int { return 42 })
	builder.Constant("PI", &Float{Value: 3.14})

	if builder.FunctionCount() != 1 {
		t.Error("Function should exist")
	}
	if builder.ConstantCount() != 1 {
		t.Error("Constant should exist")
	}

	builder.Clear()

	if builder.FunctionCount() != 0 {
		t.Errorf("FunctionCount = %d, want 0 after Clear", builder.FunctionCount())
	}
	if builder.ConstantCount() != 0 {
		t.Errorf("ConstantCount = %d, want 0 after Clear", builder.ConstantCount())
	}
}

func TestLibraryBuilderMerge(t *testing.T) {
	builder1 := NewLibraryBuilder("lib1", "Library 1")
	builder1.Function("foo", func() int { return 42 })
	builder1.Constant("PI", &Float{Value: 3.14})

	builder2 := NewLibraryBuilder("lib2", "Library 2")
	builder2.Function("bar", func() int { return 43 })
	builder2.Constant("E", &Float{Value: 2.71})

	builder1.Merge(builder2)

	if builder1.FunctionCount() != 2 {
		t.Errorf("FunctionCount = %d, want 2 after Merge", builder1.FunctionCount())
	}
	if builder1.ConstantCount() != 2 {
		t.Errorf("ConstantCount = %d, want 2 after Merge", builder1.ConstantCount())
	}

	if !builder1.HasFunction("bar") {
		t.Error("Merge should include bar function")
	}
	if !builder1.HasConstant("E") {
		t.Error("Merge should include E constant")
	}
}

func TestLibraryBuilderSubLibrary(t *testing.T) {
	parentBuilder := NewLibraryBuilder("parent", "Parent library")
	parentBuilder.Function("parent_func", func() int { return 1 })

	childBuilder := NewLibraryBuilder("child", "Child library")
	childBuilder.Function("child_func", func() int { return 2 })
	childLib := childBuilder.Build()

	parentBuilder.SubLibrary("child", childLib)

	lib := parentBuilder.Build()

	// Child library should be accessible as a constant
	if _, ok := lib.Constants()["child"]; !ok {
		t.Error("SubLibrary should be registered as constant")
	}
}

func TestLibraryBuilderConstant(t *testing.T) {
	builder := NewLibraryBuilder("test", "Test library")
	builder.Constant("PI", &Float{Value: 3.14})
	builder.Constant("E", &Float{Value: 2.71})

	lib := builder.Build()
	constants := lib.Constants()

	if len(constants) != 2 {
		t.Errorf("len(constants) = %d, want 2", len(constants))
	}

	pi, ok := constants["PI"].(*Float)
	if !ok {
		t.Error("PI should be a Float")
	}
	if pi.Value != 3.14 {
		t.Errorf("PI = %f, want 3.14", pi.Value)
	}
}

// Note: FunctionFromVariadic has a reflection bug when calling variadic functions via reflection.
// Skipping these tests until the implementation is fixed.

func TestNewKwargs(t *testing.T) {
	// Test with nil map
	kwargs := NewKwargs(nil)
	if len(kwargs.Kwargs) != 0 {
		t.Errorf("len(kwargs.Kwargs) = %d, want 0", len(kwargs.Kwargs))
	}

	// Test with empty map
	kwargs = NewKwargs(map[string]Object{})
	if len(kwargs.Kwargs) != 0 {
		t.Errorf("len(kwargs.Kwargs) = %d, want 0", len(kwargs.Kwargs))
	}

	// Test with values
	kwargs = NewKwargs(map[string]Object{
		"foo": &String{Value: "bar"},
	})
	if len(kwargs.Kwargs) != 1 {
		t.Errorf("len(kwargs.Kwargs) = %d, want 1", len(kwargs.Kwargs))
	}
}

func TestKwargsMustGetString(t *testing.T) {
	tests := []struct {
		name     string
		kwargs   Kwargs
		key      string
		default_ string
		want     string
	}{
		{
			name: "key exists",
			kwargs: Kwargs{Kwargs: map[string]Object{
				"foo": &String{Value: "bar"},
			}},
			key:      "foo",
			default_: "",
			want:     "bar",
		},
		{
			name:     "key missing with default",
			kwargs:   Kwargs{},
			key:      "foo",
			default_: "default",
			want:     "default",
		},
		{
			name:     "key missing empty default",
			kwargs:   Kwargs{},
			key:      "foo",
			default_: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.kwargs.MustGetString(tt.key, tt.default_)
			if got != tt.want {
				t.Errorf("MustGetString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResetStore(t *testing.T) {
	t.Run("removes_all_keys_when_keep_is_empty", func(t *testing.T) {
		env := NewEnvironment()
		env.Set("a", &Integer{Value: 1})
		env.Set("b", &Integer{Value: 2})

		env.ResetStore(map[string]bool{})

		if _, ok := env.Get("a"); ok {
			t.Error("expected 'a' to be removed")
		}
		if _, ok := env.Get("b"); ok {
			t.Error("expected 'b' to be removed")
		}
	})

	t.Run("keeps_specified_keys", func(t *testing.T) {
		env := NewEnvironment()
		env.Set("keep", &Integer{Value: 1})
		env.Set("remove", &Integer{Value: 2})

		env.ResetStore(map[string]bool{"keep": true})

		if _, ok := env.Get("keep"); !ok {
			t.Error("expected 'keep' to remain")
		}
		if _, ok := env.Get("remove"); ok {
			t.Error("expected 'remove' to be removed")
		}
	})

	t.Run("nil_keep_removes_all", func(t *testing.T) {
		env := NewEnvironment()
		env.Set("x", &Integer{Value: 42})

		env.ResetStore(nil)

		if _, ok := env.Get("x"); ok {
			t.Error("expected 'x' to be removed when keep is nil")
		}
	})

	t.Run("does_not_affect_outer_scope", func(t *testing.T) {
		outer := NewEnvironment()
		outer.Set("outer_var", &Integer{Value: 99})
		inner := NewEnclosedEnvironment(outer)
		inner.Set("inner_var", &Integer{Value: 1})

		inner.ResetStore(map[string]bool{})

		// outer_var still visible via scope chain
		if _, ok := inner.Get("outer_var"); !ok {
			t.Error("outer scope variable should still be accessible")
		}
		// inner_var removed from local store
		store := inner.GetStore()
		if _, ok := store["inner_var"]; ok {
			t.Error("inner_var should be removed from local store")
		}
	})
}

func TestGetClientField(t *testing.T) {
	instance := &Instance{
		Fields: map[string]Object{
			"_client": &ClientWrapper{
				TypeName: "TestClient",
				Client:   &String{Value: "test"},
			},
		},
	}

	wrapper, ok := GetClientField(instance, "_client")
	if !ok {
		t.Error("GetClientField should find _client field")
	}
	if wrapper.TypeName != "TestClient" {
		t.Errorf("TypeName = %q, want 'TestClient'", wrapper.TypeName)
	}

	// Test missing field
	_, ok = GetClientField(instance, "_missing")
	if ok {
		t.Error("GetClientField should return false for missing field")
	}

	// Test non-ClientWrapper field
	instance.Fields["foo"] = &String{Value: "bar"}
	_, ok = GetClientField(instance, "foo")
	if ok {
		t.Error("GetClientField should return false for non-ClientWrapper field")
	}
}
