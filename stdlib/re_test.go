package stdlib

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestRegexMatch(t *testing.T) {
	lib := ReLibrary
	match := lib.Functions()["match"]

	result := match.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "123abc"})
	if b, ok := result.(*object.Boolean); ok {
		if !b.Value {
			t.Errorf("match('[0-9]+', '123abc') should return true")
		}
	} else {
		t.Errorf("match() returned %T, want Boolean", result)
	}

	result = match.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123"})
	if b, ok := result.(*object.Boolean); ok {
		if b.Value {
			t.Errorf("match('[0-9]+', 'abc123') should return false")
		}
	} else {
		t.Errorf("match() returned %T, want Boolean", result)
	}
}

func TestRegexMatchWithFlags(t *testing.T) {
	lib := ReLibrary
	match := lib.Functions()["match"]

	// Test case-insensitive matching with flag
	result := match.Fn(context.Background(), &object.String{Value: "hello"}, &object.String{Value: "HELLO world"}, &object.Integer{Value: RE_IGNORECASE})
	if b, ok := result.(*object.Boolean); ok {
		if !b.Value {
			t.Errorf("match('hello', 'HELLO world', re.I) should return true")
		}
	} else {
		t.Errorf("match() returned %T, want Boolean", result)
	}

	// Without flag, should not match
	result = match.Fn(context.Background(), &object.String{Value: "hello"}, &object.String{Value: "HELLO world"})
	if b, ok := result.(*object.Boolean); ok {
		if b.Value {
			t.Errorf("match('hello', 'HELLO world') without flag should return false")
		}
	} else {
		t.Errorf("match() returned %T, want Boolean", result)
	}
}

func TestRegexSearch(t *testing.T) {
	lib := ReLibrary
	search := lib.Functions()["search"]

	result := search.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123def"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "123" {
			t.Errorf("search('[0-9]+', 'abc123def') = %v, want '123'", s.Value)
		}
	} else {
		t.Errorf("search() returned %T, want String", result)
	}
}

func TestRegexSearchWithFlags(t *testing.T) {
	lib := ReLibrary
	search := lib.Functions()["search"]

	// Test case-insensitive search
	result := search.Fn(context.Background(), &object.String{Value: "world"}, &object.String{Value: "Hello WORLD"}, &object.Integer{Value: RE_IGNORECASE})
	if s, ok := result.(*object.String); ok {
		if s.Value != "WORLD" {
			t.Errorf("search('world', 'Hello WORLD', re.I) = %v, want 'WORLD'", s.Value)
		}
	} else {
		t.Errorf("search() returned %T, want String", result)
	}
}

func TestRegexFindall(t *testing.T) {
	lib := ReLibrary
	findall := lib.Functions()["findall"]

	result := findall.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123def456ghi"})
	if l, ok := result.(*object.List); ok {
		if len(l.Elements) != 2 {
			t.Errorf("findall() returned %d matches, want 2", len(l.Elements))
		}
		if s, ok := l.Elements[0].(*object.String); ok {
			if s.Value != "123" {
				t.Errorf("first match = %v, want '123'", s.Value)
			}
		}
		if s, ok := l.Elements[1].(*object.String); ok {
			if s.Value != "456" {
				t.Errorf("second match = %v, want '456'", s.Value)
			}
		}
	} else {
		t.Errorf("findall() returned %T, want List", result)
	}
}

func TestRegexFindallWithFlags(t *testing.T) {
	lib := ReLibrary
	findall := lib.Functions()["findall"]

	// Test case-insensitive findall
	result := findall.Fn(context.Background(), &object.String{Value: "a+"}, &object.String{Value: "aAaAbBAAA"}, &object.Integer{Value: RE_IGNORECASE})
	if l, ok := result.(*object.List); ok {
		if len(l.Elements) != 2 {
			t.Errorf("findall() returned %d matches, want 2", len(l.Elements))
		}
	} else {
		t.Errorf("findall() returned %T, want List", result)
	}
}

func TestRegexSub(t *testing.T) {
	lib := ReLibrary
	sub := lib.Functions()["sub"]

	// Test Python-compatible signature: sub(pattern, repl, string)
	result := sub.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "XXX"}, &object.String{Value: "abc123def"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "abcXXXdef" {
			t.Errorf("sub() = %v, want 'abcXXXdef'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}

	// Test multiple replacements
	result = sub.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "#"}, &object.String{Value: "a1b2c3"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "a#b#c#" {
			t.Errorf("sub() = %v, want 'a#b#c#'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}

	// Test no match
	result = sub.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "XXX"}, &object.String{Value: "abc"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "abc" {
			t.Errorf("sub() = %v, want 'abc'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}
}

func TestRegexSubWithCount(t *testing.T) {
	lib := ReLibrary
	sub := lib.Functions()["sub"]

	// Test count parameter - replace only first occurrence
	result := sub.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "X"}, &object.String{Value: "a1b2c3"}, &object.Integer{Value: 1})
	if s, ok := result.(*object.String); ok {
		if s.Value != "aXb2c3" {
			t.Errorf("sub() with count=1 = %v, want 'aXb2c3'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}

	// Test count=2
	result = sub.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "X"}, &object.String{Value: "a1b2c3"}, &object.Integer{Value: 2})
	if s, ok := result.(*object.String); ok {
		if s.Value != "aXbXc3" {
			t.Errorf("sub() with count=2 = %v, want 'aXbXc3'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}
}

func TestRegexSplit(t *testing.T) {
	lib := ReLibrary
	split := lib.Functions()["split"]

	result := split.Fn(context.Background(), &object.String{Value: "[,;]"}, &object.String{Value: "one,two;three"})
	if l, ok := result.(*object.List); ok {
		if len(l.Elements) != 3 {
			t.Errorf("split() returned %d parts, want 3", len(l.Elements))
		}
		expected := []string{"one", "two", "three"}
		for i, elem := range l.Elements {
			if s, ok := elem.(*object.String); ok {
				if s.Value != expected[i] {
					t.Errorf("part %d = %v, want %v", i, s.Value, expected[i])
				}
			}
		}
	} else {
		t.Errorf("split() returned %T, want List", result)
	}
}

func TestRegexSplitWithMaxsplit(t *testing.T) {
	lib := ReLibrary
	split := lib.Functions()["split"]

	// Test maxsplit parameter
	result := split.Fn(context.Background(), &object.String{Value: "[,;]"}, &object.String{Value: "one,two;three;four"}, &object.Integer{Value: 2})
	if l, ok := result.(*object.List); ok {
		if len(l.Elements) != 2 {
			t.Errorf("split() with maxsplit=2 returned %d parts, want 2", len(l.Elements))
		}
	} else {
		t.Errorf("split() returned %T, want List", result)
	}
}

func TestRegexCompile(t *testing.T) {
	lib := ReLibrary
	compile := lib.Functions()["compile"]

	result := compile.Fn(context.Background(), &object.String{Value: "[0-9]+"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "[0-9]+" {
			t.Errorf("compile() = %v, want '[0-9]+'", s.Value)
		}
	} else {
		t.Errorf("compile() returned %T, want String", result)
	}

	// Test invalid pattern
	result = compile.Fn(context.Background(), &object.String{Value: "[0-9"})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("compile() with invalid pattern should return error, got %T", result)
	}
}

func TestRegexCompileWithFlags(t *testing.T) {
	lib := ReLibrary
	compile := lib.Functions()["compile"]

	// Compile with IGNORECASE flag
	result := compile.Fn(context.Background(), &object.String{Value: "hello"}, &object.Integer{Value: RE_IGNORECASE})
	if s, ok := result.(*object.String); ok {
		if s.Value != "(?i)hello" {
			t.Errorf("compile() with flag = %v, want '(?i)hello'", s.Value)
		}
	} else {
		t.Errorf("compile() returned %T, want String", result)
	}
}

func TestRegexEscape(t *testing.T) {
	lib := ReLibrary
	escape := lib.Functions()["escape"]

	result := escape.Fn(context.Background(), &object.String{Value: "a.b+c"})
	if s, ok := result.(*object.String); ok {
		if s.Value != `a\.b\+c` {
			t.Errorf("escape() = %v, want 'a\\.b\\+c'", s.Value)
		}
	} else {
		t.Errorf("escape() returned %T, want String", result)
	}
}

func TestRegexFullmatch(t *testing.T) {
	lib := ReLibrary
	fullmatch := lib.Functions()["fullmatch"]

	// Test full match
	result := fullmatch.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "123"})
	if b, ok := result.(*object.Boolean); ok {
		if !b.Value {
			t.Errorf("fullmatch('[0-9]+', '123') should return true")
		}
	} else {
		t.Errorf("fullmatch() returned %T, want Boolean", result)
	}

	// Test partial match
	result = fullmatch.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "123abc"})
	if b, ok := result.(*object.Boolean); ok {
		if b.Value {
			t.Errorf("fullmatch('[0-9]+', '123abc') should return false")
		}
	} else {
		t.Errorf("fullmatch() returned %T, want Boolean", result)
	}

	// Test no match
	result = fullmatch.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "abc"})
	if b, ok := result.(*object.Boolean); ok {
		if b.Value {
			t.Errorf("fullmatch('[0-9]+', 'abc') should return false")
		}
	} else {
		t.Errorf("fullmatch() returned %T, want Boolean", result)
	}
}

func TestRegexConstants(t *testing.T) {
	lib := ReLibrary
	constants := lib.Constants()

	// Check that constants are defined
	tests := []struct {
		name  string
		value int64
	}{
		{"IGNORECASE", RE_IGNORECASE},
		{"I", RE_IGNORECASE},
		{"MULTILINE", RE_MULTILINE},
		{"M", RE_MULTILINE},
		{"DOTALL", RE_DOTALL},
		{"S", RE_DOTALL},
	}

	for _, tt := range tests {
		if val, ok := constants[tt.name]; ok {
			if intVal, ok := val.(*object.Integer); ok {
				if intVal.Value != tt.value {
					t.Errorf("re.%s = %d, want %d", tt.name, intVal.Value, tt.value)
				}
			} else {
				t.Errorf("re.%s is %T, want Integer", tt.name, val)
			}
		} else {
			t.Errorf("re.%s not defined", tt.name)
		}
	}
}
