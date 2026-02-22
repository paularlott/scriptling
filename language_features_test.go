package scriptling

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// ============================================================================
// Implicit String Concatenation Tests
// ============================================================================

func TestImplicitStringConcatenation(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "two adjacent strings",
			code:     `result = "hello" " world"`,
			expected: "hello world",
		},
		{
			name:     "three adjacent strings",
			code:     `result = "a" "b" "c"`,
			expected: "abc",
		},
		{
			name:     "adjacent strings in parentheses across lines",
			code: `result = ("hello"
    " world")`,
			expected: "hello world",
		},
		{
			name:     "multiline concatenation in parens",
			code: `result = (
    "line one"
    " line two"
    " line three"
)`,
			expected: "line one line two line three",
		},
		{
			name:     "adjacent strings in function call",
			code: `
def concat(s):
    return s

result = concat("hello" " world")`,
			expected: "hello world",
		},
		{
			name:     "adjacent strings in list",
			code:     `items = ["hello" " world", "foo" " bar"]; result = items[0]`,
			expected: "hello world",
		},
		{
			name:     "no concatenation across semicolons",
			code:     `x = "hello"; result = x`,
			expected: "hello",
		},
		{
			name:     "no concatenation across newlines outside parens",
			code: `x = "hello"
result = x`,
			expected: "hello",
		},
		{
			name:     "empty string concatenation",
			code:     `result = "hello" "" " world"`,
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			result, objErr := p.GetVarAsString("result")
			if objErr != nil {
				t.Fatalf("Expected result variable, got error: %v", objErr)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestImplicitStringConcatWithFStrings(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "string followed by f-string",
			code:     `name = "Paul"; result = "Hello, " f"{name}!"`,
			expected: "Hello, Paul!",
		},
		{
			name:     "f-string followed by string",
			code:     `name = "Paul"; result = f"Hello, {name}" " the Great"`,
			expected: "Hello, Paul the Great",
		},
		{
			name:     "f-string followed by f-string",
			code:     `first = "Paul"; last = "Smith"; result = f"First: {first}" f" Last: {last}"`,
			expected: "First: Paul Last: Smith",
		},
		{
			name:     "mixed chain",
			code:     `name = "Paul"; result = "ID: " f"{name}" " (active)"`,
			expected: "ID: Paul (active)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			result, objErr := p.GetVarAsString("result")
			if objErr != nil {
				t.Fatalf("Expected result variable, got error: %v", objErr)
			}
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestStringConcatNotAppliedToVariables(t *testing.T) {
	// Ensure variable references followed by strings don't concatenate
	p := New()
	_, err := p.Eval(`
x = "hello"
y = "world"
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	// x and y should be separate
	x, _ := p.GetVarAsString("x")
	y, _ := p.GetVarAsString("y")
	if x != "hello" || y != "world" {
		t.Errorf("Expected separate variables, got x=%q, y=%q", x, y)
	}
}

// ============================================================================
// isinstance Tests
// ============================================================================

func TestIsinstanceWithBareTypes(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		// Bare type names (Python-style)
		{"dict check true", `result = isinstance({}, dict)`, true},
		{"dict check false", `result = isinstance([], dict)`, false},
		{"list check true", `result = isinstance([], list)`, true},
		{"list check false", `result = isinstance({}, list)`, false},
		{"int check true", `result = isinstance(42, int)`, true},
		{"int check false", `result = isinstance("42", int)`, false},
		{"str check true", `result = isinstance("hello", str)`, true},
		{"str check false", `result = isinstance(42, str)`, false},
		{"float check true", `result = isinstance(3.14, float)`, true},
		{"float check false", `result = isinstance(3, float)`, false},
		{"bool check true", `result = isinstance(True, bool)`, true},
		{"bool check false", `result = isinstance(1, bool)`, false},
		{"tuple check true", `result = isinstance((1, 2), tuple)`, true},
		{"tuple check false", `result = isinstance([1, 2], tuple)`, false},

		// String type names (backwards compatible)
		{"string isinstance dict", `result = isinstance({}, "dict")`, true},
		{"string isinstance list", `result = isinstance([], "list")`, true},
		{"string isinstance int", `result = isinstance(42, "int")`, true},
		{"string isinstance str", `result = isinstance("hello", "str")`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			result, objErr := p.GetVar("result")
			if objErr != nil {
				t.Fatalf("Expected result, got error: %v", objErr)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsinstanceWithClasses(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Animal:
    def __init__(self, name):
        self.name = name

class Dog(Animal):
    def __init__(self, name, breed):
        super().__init__(name)
        self.breed = breed

dog = Dog("Rex", "Labrador")
is_dog = isinstance(dog, Dog)
is_animal = isinstance(dog, Animal)
is_str = isinstance(dog, str)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	isDog, _ := p.GetVar("is_dog")
	if isDog != true {
		t.Error("Expected isinstance(dog, Dog) to be True")
	}

	isAnimal, _ := p.GetVar("is_animal")
	if isAnimal != true {
		t.Error("Expected isinstance(dog, Animal) to be True")
	}

	isStr, _ := p.GetVar("is_str")
	if isStr != false {
		t.Error("Expected isinstance(dog, str) to be False")
	}
}

// ============================================================================
// Cross-Type Comparison Tests
// ============================================================================

func TestCrossTypeEquality(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		// Integer vs String
		{"int == string", `result = 5 == "hello"`, false},
		{"int != string", `result = 5 != "hello"`, true},
		{"string == int", `result = "hello" == 5`, false},
		{"string != int", `result = "hello" != 5`, true},

		// Float vs String
		{"float == string", `result = 3.14 == "hello"`, false},
		{"float != string", `result = 3.14 != "hello"`, true},

		// Integer vs None
		{"int == None", `result = 5 == None`, false},
		{"int != None", `result = 5 != None`, true},

		// Bool comparisons
		{"bool == bool true", `result = True == True`, true},
		{"bool != bool", `result = True != False`, true},

		// Integer vs List
		{"int == list", `result = 5 == [5]`, false},
		{"int != list", `result = 5 != [5]`, true},

		// Integer vs Dict
		{"int == dict", `result = 5 == {}`, false},
		{"int != dict", `result = 5 != {}`, true},

		// Same type comparisons (regression)
		{"int == int true", `result = 5 == 5`, true},
		{"int == int false", `result = 5 == 6`, false},
		{"str == str true", `result = "a" == "a"`, true},
		{"float == float true", `result = 3.14 == 3.14`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			result, objErr := p.GetVar("result")
			if objErr != nil {
				t.Fatalf("Expected result, got error: %v", objErr)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCrossTypeComparisonStillErrors(t *testing.T) {
	// Ordering comparisons between incompatible types should still error
	tests := []struct {
		name string
		code string
	}{
		{"int < string", `result = 5 < "hello"`},
		{"int > string", `result = 5 > "hello"`},
		{"int <= string", `result = 5 <= "hello"`},
		{"int >= string", `result = 5 >= "hello"`},
		{"float < string", `result = 3.14 < "hello"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err == nil {
				t.Error("Expected error for ordering comparison between incompatible types")
			}
		})
	}
}

func TestCrossTypeInFilterContext(t *testing.T) {
	// Real-world use case: filtering params where value might be int or string
	p := New()
	_, err := p.Eval(`
params = {"page": 1, "limit": 50, "name": "test", "empty": ""}
result = {}
for key in params.keys():
    value = params[key]
    if str(value) != "":
        result[key] = value
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatalf("Expected result, got error: %v", objErr)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result)
	}

	// "empty" should not be in the result since str("") == ""
	if _, hasEmpty := resultMap["empty"]; hasEmpty {
		t.Error("Expected 'empty' to be filtered out")
	}
}

// ============================================================================
// Error Reporting Tests
// ============================================================================

func TestErrorIncludesLineNumber(t *testing.T) {
	p := New()
	_, err := p.Eval(`
x = 1
y = "hello"
z = x + y
`)
	if err == nil {
		t.Fatal("Expected error")
	}

	// Error message should include line number
	if !strings.Contains(err.Error(), "line") {
		t.Errorf("Expected error to include line info, got: %v", err)
	}
}

func TestErrorIncludesSourceFile(t *testing.T) {
	p := New()
	p.SetSourceFile("test_script.py")

	_, err := p.Eval(`
x = undefined_var
`)
	if err == nil {
		t.Fatal("Expected error")
	}

	// Error message should include the source file
	if !strings.Contains(err.Error(), "test_script.py") {
		t.Errorf("Expected error to include source file 'test_script.py', got: %v", err)
	}
}

func TestErrorIncludesFileAndLine(t *testing.T) {
	p := New()
	p.SetSourceFile("my_script.py")

	_, err := p.Eval(`
a = 10
b = a / 0
`)
	if err == nil {
		t.Fatal("Expected error")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "my_script.py") {
		t.Errorf("Expected error to include file name, got: %v", errMsg)
	}
	// Should have file:line format
	if !strings.Contains(errMsg, "my_script.py:3") {
		t.Errorf("Expected error to include file:line format, got: %v", errMsg)
	}
}

func TestErrorInScriptLibraryIncludesLibName(t *testing.T) {
	p := New()

	// Register a library with an error
	err := p.RegisterScriptLibrary("buggy_lib", `
x = 1
y = "not a number"
z = x + y
`)
	if err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	_, err = p.Eval(`import buggy_lib`)
	if err == nil {
		t.Fatal("Expected error when importing buggy library")
	}

	// Error should mention the library name
	if !strings.Contains(err.Error(), "buggy_lib") {
		t.Errorf("Expected error to include library name 'buggy_lib', got: %v", err)
	}
}

func TestNoSourceFileNoFileInError(t *testing.T) {
	p := New()
	// Don't set source file

	_, err := p.Eval(`
x = undefined_var
`)
	if err == nil {
		t.Fatal("Expected error")
	}

	// Error should still include line info but not a file name
	errMsg := err.Error()
	if !strings.Contains(errMsg, "line") {
		t.Errorf("Expected line info even without source file, got: %v", errMsg)
	}
}

// ============================================================================
// Library Import Refactor Tests (nested imports with on-demand callback)
// ============================================================================

func TestNestedScriptLibraryImport(t *testing.T) {
	p := New()

	// Register a base library
	err := p.RegisterScriptLibrary("base_lib", `
PI = 3.14159

def double(x):
    return x * 2
`)
	if err != nil {
		t.Fatalf("Failed to register base_lib: %v", err)
	}

	// Register a library that imports the base library
	err = p.RegisterScriptLibrary("derived_lib", `
import base_lib

def quadruple(x):
    return base_lib.double(base_lib.double(x))

TWO_PI = base_lib.PI * 2
`)
	if err != nil {
		t.Fatalf("Failed to register derived_lib: %v", err)
	}

	_, err = p.Eval(`
import derived_lib
result = derived_lib.quadruple(5)
two_pi = derived_lib.TWO_PI
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(20) {
		t.Errorf("Expected 20, got %v", result)
	}

	twoPi, objErr := p.GetVar("two_pi")
	if objErr != nil {
		t.Errorf("Expected two_pi, got error: %v", objErr)
	}
	if twoPi.(float64) < 6.28 || twoPi.(float64) > 6.29 {
		t.Errorf("Expected ~6.28, got %v", twoPi)
	}
}

func TestNestedOnDemandImport(t *testing.T) {
	p := New()

	// Set up on-demand callback that registers libraries on first access
	p.SetOnDemandLibraryCallback(func(s *Scriptling, name string) bool {
		switch name {
		case "utils":
			return s.RegisterScriptLibrary("utils", `
def add(a, b):
    return a + b
`) == nil
		case "calculator":
			return s.RegisterScriptLibrary("calculator", `
import utils

def sum_list(items):
    total = 0
    for item in items:
        total = utils.add(total, item)
    return total
`) == nil
		}
		return false
	})

	_, err := p.Eval(`
import calculator
result = calculator.sum_list([1, 2, 3, 4, 5])
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(15) {
		t.Errorf("Expected 15, got %v", result)
	}
}

func TestNestedImportWithRegisteredAndScriptLibraries(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.JSONLibrary)

	// Register a script library that uses a registered (Go) library
	err := p.RegisterScriptLibrary("json_helper", `
import json

def parse_name(json_str):
    data = json.loads(json_str)
    return data["name"]
`)
	if err != nil {
		t.Fatalf("Failed to register: %v", err)
	}

	_, err = p.Eval(`
import json_helper
result = json_helper.parse_name('{"name": "Alice", "age": 30}')
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	result, objErr := p.GetVarAsString("result")
	if objErr != nil || result != "Alice" {
		t.Errorf("Expected 'Alice', got %q", result)
	}
}

// ============================================================================
// Integration Tests - Real-world patterns from fortix dev libraries
// ============================================================================

func TestFortixStyleParamFiltering(t *testing.T) {
	// Pattern from fortix_dev.py: filtering params with mixed types
	p := New()
	_, err := p.Eval(`
params = {"page": 1, "limit": 500, "search": "", "active": True}
filtered = {}
for key in params.keys():
    value = params[key]
    if str(value) != "":
        filtered[key] = value

count = len(filtered)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	count, objErr := p.GetVar("count")
	if objErr != nil || count != int64(3) {
		t.Errorf("Expected 3 filtered params (page, limit, active), got %v", count)
	}
}

func TestFortixStyleIsinstanceChecks(t *testing.T) {
	// Pattern from fortix library: checking response types
	p := New()
	_, err := p.Eval(`
response = {"records": [{"id": 1}, {"id": 2}]}

is_dict = isinstance(response, dict)
records = response["records"]
is_list = isinstance(records, list)
first_record = records[0]
is_also_dict = isinstance(first_record, dict)
is_not_str = isinstance(first_record, str)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	isDict, _ := p.GetVar("is_dict")
	if isDict != true {
		t.Error("Expected response to be dict")
	}

	isList, _ := p.GetVar("is_list")
	if isList != true {
		t.Error("Expected records to be list")
	}

	isAlsoDict, _ := p.GetVar("is_also_dict")
	if isAlsoDict != true {
		t.Error("Expected first_record to be dict")
	}

	isNotStr, _ := p.GetVar("is_not_str")
	if isNotStr != false {
		t.Error("Expected first_record to NOT be str")
	}
}

func TestFortixStyleMultilineStrings(t *testing.T) {
	// Pattern: building URLs with implicit concatenation
	p := New()
	_, err := p.Eval(`
base = "https://api.example.com"
path = "/v1/customers"
url = base + path

# Multi-line string building in parens
message = (
    "Error occurred while "
    "processing the request. "
    "Please try again later."
)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	url, _ := p.GetVarAsString("url")
	if url != "https://api.example.com/v1/customers" {
		t.Errorf("Expected URL, got %q", url)
	}

	message, _ := p.GetVarAsString("message")
	expected := "Error occurred while processing the request. Please try again later."
	if message != expected {
		t.Errorf("Expected multiline message, got %q", message)
	}
}

// ============================================================================
// Regression Tests
// ============================================================================

func TestRegressionSameTypeComparisonsUnaffected(t *testing.T) {
	// Ensure same-type comparisons still work after cross-type fix
	p := New()
	_, err := p.Eval(`
int_eq = 5 == 5
int_ne = 5 != 6
int_lt = 3 < 5
int_gt = 5 > 3
str_eq = "a" == "a"
str_ne = "a" != "b"
float_eq = 3.14 == 3.14
none_eq = None == None
list_eq = [1, 2] == [1, 2]
dict_eq = {"a": 1} == {"a": 1}
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	checks := map[string]bool{
		"int_eq": true, "int_ne": true, "int_lt": true, "int_gt": true,
		"str_eq": true, "str_ne": true, "float_eq": true, "none_eq": true,
		"list_eq": true, "dict_eq": true,
	}

	for name, expected := range checks {
		val, objErr := p.GetVar(name)
		if objErr != nil || val != expected {
			t.Errorf("%s: expected %v, got %v", name, expected, val)
		}
	}
}

func TestRegressionIsinstanceStringStillWorks(t *testing.T) {
	// Ensure string-based isinstance still works (backwards compatibility)
	p := New()
	_, err := p.Eval(`
r1 = isinstance(42, "int")
r2 = isinstance("hello", "str")
r3 = isinstance(3.14, "float")
r4 = isinstance({}, "dict")
r5 = isinstance([], "list")
r6 = isinstance(True, "bool")
r7 = isinstance(None, "None")
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	for _, name := range []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		val, objErr := p.GetVar(name)
		if objErr != nil || val != true {
			t.Errorf("%s: expected true, got %v", name, val)
		}
	}
}

func TestRegressionExistingImportsUnaffected(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.JSONLibrary)

	_, err := p.Eval(`
import json
data = json.loads('{"key": "value"}')
result = data["key"]
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	result, objErr := p.GetVarAsString("result")
	if objErr != nil || result != "value" {
		t.Errorf("Expected 'value', got %q", result)
	}
}

// ============================================================================
// Assert Statement Tests
// ============================================================================

func TestAssertPassing(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"true literal", `assert True`},
		{"equality", `assert 1 == 1`},
		{"comparison", `assert 10 > 5`},
		{"string length", `assert len("hello") == 5`},
		{"truthy list", `assert [1, 2, 3]`},
		{"truthy dict", `assert {"k": "v"}`},
		{"with message", `assert True, "should not appear"`},
		{"with fstring message", `x = 10; assert x == 10, f"expected 10 got {x}"`},
		{"not false", `assert not False`},
		{"complex expr", `assert 2 ** 8 == 256`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err != nil {
				t.Errorf("Expected assert to pass, got error: %v", err)
			}
		})
	}
}

func TestAssertFailing(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		wantMsgPart string
	}{
		{
			name:        "false literal",
			code:        `assert False`,
			wantMsgPart: "AssertionError",
		},
		{
			name:        "failing comparison",
			code:        `assert 1 == 2`,
			wantMsgPart: "AssertionError",
		},
		{
			name:        "custom message string",
			code:        `assert False, "x must be positive"`,
			wantMsgPart: "x must be positive",
		},
		{
			name:        "custom message fstring",
			code:        `x = 5; assert x > 10, f"x={x} is not > 10"`,
			wantMsgPart: "x=5 is not > 10",
		},
		{
			name:        "zero is falsy",
			code:        `assert 0`,
			wantMsgPart: "AssertionError",
		},
		{
			name:        "empty string is falsy",
			code:        `assert ""`,
			wantMsgPart: "AssertionError",
		},
		{
			name:        "empty list is falsy",
			code:        `assert []`,
			wantMsgPart: "AssertionError",
		},
		{
			name:        "none is falsy",
			code:        `assert None`,
			wantMsgPart: "AssertionError",
		},
		{
			name:        "includes line number",
			code:        "x = 1\nassert x == 2",
			wantMsgPart: "line 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.code)
			if err == nil {
				t.Fatal("Expected assert to fail, got no error")
			}
			if !strings.Contains(err.Error(), tt.wantMsgPart) {
				t.Errorf("Expected error to contain %q, got: %v", tt.wantMsgPart, err)
			}
		})
	}
}

func TestAssertInsideFunction(t *testing.T) {
	p := New()
	_, err := p.Eval(`
def validate(x):
    assert x > 0, "must be positive"
    return x * 2

result = validate(5)
`)
	if err != nil {
		t.Fatalf("Expected passing assert in function, got: %v", err)
	}
	result, _ := p.GetVar("result")
	if result != int64(10) {
		t.Errorf("Expected 10, got %v", result)
	}

	// Now test failing assert inside function
	p2 := New()
	_, err = p2.Eval(`
def validate(x):
    assert x > 0, "must be positive"

validate(-1)
`)
	if err == nil {
		t.Fatal("Expected assert failure inside function")
	}
	if !strings.Contains(err.Error(), "must be positive") {
		t.Errorf("Expected custom message, got: %v", err)
	}
}

func TestAssertNotCatchableByTryExcept(t *testing.T) {
	// In Python, AssertionError IS catchable by try/except.
	// Scriptling matches this behaviour — assert raises an Error which
	// try/except converts to an Exception and catches.
	p := New()
	_, err := p.Eval(`
caught = False
try:
    assert False, "caught me"
except:
    caught = True
`)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Error("Expected AssertionError to be catchable by try/except (matches Python behaviour)")
	}
}

func TestRegressionCallFunctionWithContextStillWorks(t *testing.T) {
	p := New()
	p.RegisterFunc("add", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		a, _ := args[0].AsInt()
		b, _ := args[1].AsInt()
		return &object.Integer{Value: a + b}
	})

	result, err := p.CallFunction("add", 10, 20)
	if err != nil {
		t.Fatalf("CallFunction failed: %v", err)
	}

	val, _ := result.AsInt()
	if val != 30 {
		t.Errorf("Expected 30, got %d", val)
	}
}

// ============================================================================
// Dunder Methods Tests (§1.3)
// ============================================================================

func TestDunderStr(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __str__(self):
        return f"({self.x}, {self.y})"

pt = Point(3, 4)
s = str(pt)
fs = f"{pt}"
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	s, _ := p.GetVarAsString("s")
	if s != "(3, 4)" {
		t.Errorf("str(pt): expected '(3, 4)', got %q", s)
	}
	fs, _ := p.GetVarAsString("fs")
	if fs != "(3, 4)" {
		t.Errorf("f-string: expected '(3, 4)', got %q", fs)
	}
}

func TestDunderRepr(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Foo:
    def __repr__(self):
        return "Foo()"
    def __str__(self):
        return "foo"

f = Foo()
r = repr(f)
s = str(f)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r, _ := p.GetVarAsString("r")
	if r != "Foo()" {
		t.Errorf("repr: expected 'Foo()', got %q", r)
	}
	s, _ := p.GetVarAsString("s")
	if s != "foo" {
		t.Errorf("str: expected 'foo', got %q", s)
	}
}

func TestDunderLen(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Bag:
    def __init__(self):
        self.items = []
    def add(self, x):
        self.items.append(x)
    def __len__(self):
        return len(self.items)

b = Bag()
l0 = len(b)
b.add(1)
b.add(2)
l2 = len(b)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	l0, _ := p.GetVar("l0")
	if l0 != int64(0) {
		t.Errorf("len(empty): expected 0, got %v", l0)
	}
	l2, _ := p.GetVar("l2")
	if l2 != int64(2) {
		t.Errorf("len(2 items): expected 2, got %v", l2)
	}
}

func TestDunderBool(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Flag:
    def __init__(self, v):
        self.v = v
    def __bool__(self):
        return self.v

t_flag = Flag(True)
f_flag = Flag(False)
bt = bool(t_flag)
bf = bool(f_flag)
if_t = "yes" if t_flag else "no"
if_f = "yes" if f_flag else "no"
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	bt, _ := p.GetVar("bt")
	if bt != true {
		t.Errorf("bool(true flag): expected true, got %v", bt)
	}
	bf, _ := p.GetVar("bf")
	if bf != false {
		t.Errorf("bool(false flag): expected false, got %v", bf)
	}
	ifT, _ := p.GetVarAsString("if_t")
	if ifT != "yes" {
		t.Errorf("if truthy: expected 'yes', got %q", ifT)
	}
	ifF, _ := p.GetVarAsString("if_f")
	if ifF != "no" {
		t.Errorf("if falsy: expected 'no', got %q", ifF)
	}
}

func TestDunderEqLt(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Num:
    def __init__(self, n):
        self.n = n
    def __eq__(self, other):
        return self.n == other.n
    def __lt__(self, other):
        return self.n < other.n

a = Num(1)
b = Num(1)
c = Num(2)
eq = a == b
ne = a == c
lt = a < c
gt = c < a
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	eq, _ := p.GetVar("eq")
	if eq != true {
		t.Errorf("__eq__: expected true, got %v", eq)
	}
	ne, _ := p.GetVar("ne")
	if ne != false {
		t.Errorf("__eq__ false: expected false, got %v", ne)
	}
	lt, _ := p.GetVar("lt")
	if lt != true {
		t.Errorf("__lt__: expected true, got %v", lt)
	}
	gt, _ := p.GetVar("gt")
	if gt != false {
		t.Errorf("__lt__ reverse: expected false, got %v", gt)
	}
}

func TestDunderContains(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class WordList:
    def __init__(self, words):
        self.words = words
    def __contains__(self, word):
        return word in self.words

wl = WordList(["hello", "world"])
has_hello = "hello" in wl
has_foo = "foo" in wl
not_foo = "foo" not in wl
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	hasHello, _ := p.GetVar("has_hello")
	if hasHello != true {
		t.Errorf("__contains__ true: expected true, got %v", hasHello)
	}
	hasFoo, _ := p.GetVar("has_foo")
	if hasFoo != false {
		t.Errorf("__contains__ false: expected false, got %v", hasFoo)
	}
	notFoo, _ := p.GetVar("not_foo")
	if notFoo != true {
		t.Errorf("not in: expected true, got %v", notFoo)
	}
}

func TestDunderIter(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Counter:
    def __init__(self, n):
        self.n = n
    def __iter__(self):
        return CounterIter(self.n)

class CounterIter:
    def __init__(self, n):
        self.i = 0
        self.n = n
    def __next__(self):
        if self.i >= self.n:
            raise StopIteration()
        v = self.i
        self.i = self.i + 1
        return v

result = []
for x in Counter(4):
    result.append(x)

comp = [x * 2 for x in Counter(3)]
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVar("result")
	elems, ok := result.([]interface{})
	if !ok || len(elems) != 4 {
		t.Errorf("for loop: expected [0,1,2,3], got %v", result)
	} else {
		for i, v := range elems {
			if v != int64(i) {
				t.Errorf("for loop[%d]: expected %d, got %v", i, i, v)
			}
		}
	}
	comp, _ := p.GetVar("comp")
	compElems, ok := comp.([]interface{})
	if !ok || len(compElems) != 3 {
		t.Errorf("comprehension: expected [0,2,4], got %v", comp)
	} else {
		expected := []int64{0, 2, 4}
		for i, v := range compElems {
			if v != expected[i] {
				t.Errorf("comp[%d]: expected %d, got %v", i, expected[i], v)
			}
		}
	}
}

func TestDunderSorted(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Item:
    def __init__(self, val):
        self.val = val
    def __lt__(self, other):
        return self.val < other.val
    def __eq__(self, other):
        return self.val == other.val

items = [Item(3), Item(1), Item(2)]
s = sorted(items)
vals = [x.val for x in s]
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	vals, _ := p.GetVar("vals")
	elems, ok := vals.([]interface{})
	if !ok || len(elems) != 3 {
		t.Errorf("sorted: expected [1,2,3], got %v", vals)
	} else {
		expected := []int64{1, 2, 3}
		for i, v := range elems {
			if v != expected[i] {
				t.Errorf("sorted[%d]: expected %d, got %v", i, expected[i], v)
			}
		}
	}
}

func TestDunderInheritance(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Base:
    def __str__(self):
        return "Base"
    def __len__(self):
        return 42

class Child(Base):
    pass

c = Child()
s = str(c)
l = len(c)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	s, _ := p.GetVarAsString("s")
	if s != "Base" {
		t.Errorf("inherited __str__: expected 'Base', got %q", s)
	}
	l, _ := p.GetVar("l")
	if l != int64(42) {
		t.Errorf("inherited __len__: expected 42, got %v", l)
	}
}

func TestDunderStrFallback(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Plain:
    def __init__(self):
        self.x = 1

obj = Plain()
s = str(obj)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	s, _ := p.GetVarAsString("s")
	if s == "" {
		t.Error("str(plain obj): expected non-empty fallback string")
	}
	if !strings.Contains(s, "Plain") {
		t.Errorf("str(plain obj): expected class name in fallback, got %q", s)
	}
}

// ============================================================================
// With Statement Tests (§1.4)
// ============================================================================

func TestWithBasic(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class CM:
    def __init__(self):
        self.entered = False
        self.exited = False
    def __enter__(self):
        self.entered = True
        return self
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.exited = True
        return False

cm = CM()
with cm:
    pass
entered = cm.entered
exited = cm.exited
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	entered, _ := p.GetVar("entered")
	if entered != true {
		t.Error("expected __enter__ to be called")
	}
	exited, _ := p.GetVar("exited")
	if exited != true {
		t.Error("expected __exit__ to be called")
	}
}

func TestWithAsBinding(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class CM:
    def __enter__(self):
        return 99
    def __exit__(self, exc_type, exc_val, exc_tb):
        return False

with CM() as val:
    result = val
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVar("result")
	if result != int64(99) {
		t.Errorf("expected 99, got %v", result)
	}
}

func TestWithExitCalledOnException(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class CM:
    def __init__(self):
        self.exited = False
    def __enter__(self):
        return self
    def __exit__(self, exc_type, exc_val, exc_tb):
        self.exited = True
        return False

cm = CM()
caught = False
try:
    with cm:
        raise ValueError("boom")
except:
    caught = True
exited = cm.exited
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	exited, _ := p.GetVar("exited")
	if exited != true {
		t.Error("expected __exit__ to be called on exception")
	}
	caught, _ := p.GetVar("caught")
	if caught != true {
		t.Error("expected exception to propagate when __exit__ returns False")
	}
}

func TestWithSuppressException(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class CM:
    def __enter__(self):
        return self
    def __exit__(self, exc_type, exc_val, exc_tb):
        return True  # suppress

reached = False
with CM():
    raise ValueError("suppressed")
reached = True
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	reached, _ := p.GetVar("reached")
	if reached != true {
		t.Error("expected exception to be suppressed when __exit__ returns True")
	}
}

func TestWithInheritedDunders(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Base:
    def __enter__(self):
        return "from_base"
    def __exit__(self, exc_type, exc_val, exc_tb):
        return False

class Child(Base):
    pass

with Child() as v:
    result = v
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVarAsString("result")
	if result != "from_base" {
		t.Errorf("expected 'from_base', got %q", result)
	}
}

// ============================================================================
// Decorator Tests (§1.5)
// ============================================================================

func TestDecoratorBasic(t *testing.T) {
	p := New()
	_, err := p.Eval(`
def double(fn):
    def wrapper(*args):
        return fn(*args) * 2
    return wrapper

@double
def add(a, b):
    return a + b

result = add(3, 4)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVar("result")
	if result != int64(14) {
		t.Errorf("expected 14, got %v", result)
	}
}

func TestDecoratorStacked(t *testing.T) {
	p := New()
	_, err := p.Eval(`
def add_one(fn):
    def wrapper(*args):
        return fn(*args) + 1
    return wrapper

def double(fn):
    def wrapper(*args):
        return fn(*args) * 2
    return wrapper

@add_one
@double
def val():
    return 5

result = val()
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	// val() = 5, double wraps -> 10, add_one wraps -> 11
	result, _ := p.GetVar("result")
	if result != int64(11) {
		t.Errorf("expected 11, got %v", result)
	}
}

func TestDecoratorProperty(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Circle:
    def __init__(self, r):
        self._r = r

    @property
    def radius(self):
        return self._r

    @property
    def diameter(self):
        return self._r * 2

c = Circle(5)
r = c.radius
d = c.diameter
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r, _ := p.GetVar("r")
	if r != int64(5) {
		t.Errorf("radius: expected 5, got %v", r)
	}
	d, _ := p.GetVar("d")
	if d != int64(10) {
		t.Errorf("diameter: expected 10, got %v", d)
	}
}

func TestDecoratorPropertyInheritance(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Base:
    def __init__(self, name):
        self._name = name

    @property
    def name(self):
        return self._name

class Child(Base):
    pass

c = Child("test")
result = c.name
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVarAsString("result")
	if result != "test" {
		t.Errorf("inherited property: expected 'test', got %q", result)
	}
}

func TestDecoratorStaticMethod(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Math:
    @staticmethod
    def square(x):
        return x * x

r1 = Math.square(4)
m = Math()
r2 = m.square(3)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r1, _ := p.GetVar("r1")
	if r1 != int64(16) {
		t.Errorf("class staticmethod: expected 16, got %v", r1)
	}
	r2, _ := p.GetVar("r2")
	if r2 != int64(9) {
		t.Errorf("instance staticmethod: expected 9, got %v", r2)
	}
}

func TestDecoratorClassDecorator(t *testing.T) {
	p := New()
	_, err := p.Eval(`
def add_version(cls):
    cls.version = "1.0"
    return cls

@add_version
class App:
    pass

a = App()
result = a.version
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVarAsString("result")
	if result != "1.0" {
		t.Errorf("class decorator: expected '1.0', got %q", result)
	}
}

// ============================================================================
// ClassBuilder Property / StaticMethod Tests
// ============================================================================

func TestClassBuilderProperty(t *testing.T) {
	p := New()

	cb := object.NewClassBuilder("Circle")
	cb.MethodWithHelp("__init__", func(self *object.Instance, r float64) {
		self.Fields["radius"] = &object.Float{Value: r}
	}, "")
	cb.Property("radius", func(self *object.Instance) float64 {
		v, _ := self.Fields["radius"].AsFloat()
		return v
	})
	cb.Property("diameter", func(self *object.Instance) float64 {
		v, _ := self.Fields["radius"].AsFloat()
		return v * 2
	})
	p.SetObjectVar("Circle", cb.Build())

	_, err := p.Eval(`
c = Circle(5.0)
r = c.radius
d = c.diameter
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r, _ := p.GetVar("r")
	if r != float64(5) {
		t.Errorf("radius: expected 5.0, got %v", r)
	}
	d, _ := p.GetVar("d")
	if d != float64(10) {
		t.Errorf("diameter: expected 10.0, got %v", d)
	}
}

func TestClassBuilderStaticMethod(t *testing.T) {
	p := New()

	cb := object.NewClassBuilder("Math")
	cb.StaticMethod("square", func(x float64) float64 {
		return x * x
	})
	cb.StaticMethod("add", func(a, b int) int {
		return a + b
	})
	p.SetObjectVar("Math", cb.Build())

	_, err := p.Eval(`
r1 = Math.square(4.0)
r2 = Math.add(3, 7)
m = Math()
r3 = m.square(3.0)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r1, _ := p.GetVar("r1")
	if r1 != float64(16) {
		t.Errorf("class static square: expected 16.0, got %v", r1)
	}
	r2, _ := p.GetVar("r2")
	if r2 != int64(10) {
		t.Errorf("class static add: expected 10, got %v", r2)
	}
	r3, _ := p.GetVar("r3")
	if r3 != float64(9) {
		t.Errorf("instance static square: expected 9.0, got %v", r3)
	}
}

func TestClassBuilderPropertyInheritance(t *testing.T) {
	p := New()

	base := object.NewClassBuilder("Base")
	base.MethodWithHelp("__init__", func(self *object.Instance, name string) {
		self.Fields["name"] = &object.String{Value: name}
	}, "")
	base.Property("name", func(self *object.Instance) string {
		v, _ := self.Fields["name"].AsString()
		return v
	})
	baseClass := base.Build()

	child := object.NewClassBuilder("Child")
	child.BaseClass(baseClass)
	// Child needs its own __init__ to set fields (inherited __init__ is not auto-called)
	child.Method("__init__", func(self *object.Instance, name string) {
		self.Fields["name"] = &object.String{Value: name}
	})
	p.SetObjectVar("Base", baseClass)
	p.SetObjectVar("Child", child.Build())

	_, err := p.Eval(`
c = Child("hello")
result = c.name
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVarAsString("result")
	if result != "hello" {
		t.Errorf("inherited property: expected 'hello', got %q", result)
	}
}

func TestClassBuilderNativePropertyAndStaticMethod(t *testing.T) {
	// Verify object.Property and object.StaticMethod work when set directly
	// in the Methods map (native API path).
	p := New()

	getterFn := func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		inst := args[0].(*object.Instance)
		v, _ := inst.Fields["val"].AsInt()
		return object.NewInteger(v * 2)
	}
	staticFn := func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		v, _ := args[0].AsInt()
		return object.NewInteger(v + 100)
	}

	cls := &object.Class{
		Name: "Box",
		Methods: map[string]object.Object{
			"__init__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					inst := args[0].(*object.Instance)
					n, _ := args[1].AsInt()
					inst.Fields["val"] = object.NewInteger(n)
					return &object.Null{}
				},
			},
			"doubled": &object.Property{Getter: &object.Builtin{Fn: getterFn}},
			"offset":  &object.StaticMethod{Fn: &object.Builtin{Fn: staticFn}},
		},
	}
	p.SetObjectVar("Box", cls)

	_, err := p.Eval(`
b = Box(7)
r1 = b.doubled
r2 = Box.offset(5)
r3 = b.offset(3)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r1, _ := p.GetVar("r1")
	if r1 != int64(14) {
		t.Errorf("property: expected 14, got %v", r1)
	}
	r2, _ := p.GetVar("r2")
	if r2 != int64(105) {
		t.Errorf("class staticmethod: expected 105, got %v", r2)
	}
	r3, _ := p.GetVar("r3")
	if r3 != int64(103) {
		t.Errorf("instance staticmethod: expected 103, got %v", r3)
	}
}

// ============================================================================
// Property Setter Tests
// ============================================================================

func TestDecoratorPropertySetter(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Temperature:
    def __init__(self, c):
        self._c = c

    @property
    def celsius(self):
        return self._c

    @celsius.setter
    def celsius(self, v):
        self._c = v

t = Temperature(100)
r1 = t.celsius
t.celsius = 0
r2 = t.celsius
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r1, _ := p.GetVar("r1")
	if r1 != int64(100) {
		t.Errorf("getter: expected 100, got %v", r1)
	}
	r2, _ := p.GetVar("r2")
	if r2 != int64(0) {
		t.Errorf("setter: expected 0, got %v", r2)
	}
}

func TestDecoratorPropertyReadOnly(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Circle:
    def __init__(self, r):
        self._r = r

    @property
    def radius(self):
        return self._r

c = Circle(5)
try:
    c.radius = 10
    result = "no error"
except Exception as e:
    result = str(e)
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	result, _ := p.GetVarAsString("result")
	if result == "no error" {
		t.Error("expected error when assigning to read-only property")
	}
}

func TestDecoratorPropertySetterInheritance(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class Base:
    def __init__(self, v):
        self._v = v

    @property
    def value(self):
        return self._v

    @value.setter
    def value(self, v):
        self._v = v

class Child(Base):
    pass

c = Child(10)
r1 = c.value
c.value = 99
r2 = c.value
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r1, _ := p.GetVar("r1")
	if r1 != int64(10) {
		t.Errorf("inherited getter: expected 10, got %v", r1)
	}
	r2, _ := p.GetVar("r2")
	if r2 != int64(99) {
		t.Errorf("inherited setter: expected 99, got %v", r2)
	}
}

func TestClassBuilderPropertySetter(t *testing.T) {
	p := New()

	cb := object.NewClassBuilder("Box")
	cb.MethodWithHelp("__init__", func(self *object.Instance, v int) {
		self.Fields["_v"] = object.NewInteger(int64(v))
	}, "")
	cb.PropertyWithSetter("value",
		func(self *object.Instance) int {
			v, _ := self.Fields["_v"].AsInt()
			return int(v)
		},
		func(self *object.Instance, v int) {
			self.Fields["_v"] = object.NewInteger(int64(v))
		},
	)
	p.SetObjectVar("Box", cb.Build())

	_, err := p.Eval(`
b = Box(5)
r1 = b.value
b.value = 42
r2 = b.value
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	r1, _ := p.GetVar("r1")
	if r1 != int64(5) {
		t.Errorf("getter: expected 5, got %v", r1)
	}
	r2, _ := p.GetVar("r2")
	if r2 != int64(42) {
		t.Errorf("setter: expected 42, got %v", r2)
	}
}
