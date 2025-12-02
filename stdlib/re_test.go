package stdlib

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestRegexMatch(t *testing.T) {
	lib := ReLibrary
	match := lib.Functions()["match"]

	// Test matching at start - should return Match object
	result := match.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "123abc"})
	if m, ok := result.(*object.Instance); ok && m.Class == MatchClass {
		groups := m.Fields["groups"].(*object.List).Elements
		if len(groups) == 0 || groups[0].(*object.String).Value != "123" {
			t.Errorf("match('[0-9]+', '123abc').group(0) = %v, want '123'", groups[0])
		}
		start := m.Fields["start"].(*object.Integer).Value
		end := m.Fields["end"].(*object.Integer).Value
		if start != 0 || end != 3 {
			t.Errorf("match span = (%d, %d), want (0, 3)", start, end)
		}
	} else {
		t.Errorf("match() returned %T, want Match instance", result)
	}

	// Test non-matching at start - should return Null
	result = match.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123"})
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("match('[0-9]+', 'abc123') returned %T, want Null", result)
	}
}

func TestRegexMatchWithFlags(t *testing.T) {
	lib := ReLibrary
	match := lib.Functions()["match"]

	// Test case-insensitive matching with flag
	result := match.Fn(context.Background(), nil, &object.String{Value: "hello"}, &object.String{Value: "HELLO world"}, &object.Integer{Value: RE_IGNORECASE})
	if m, ok := result.(*object.Instance); ok && m.Class == MatchClass {
		groups := m.Fields["groups"].(*object.List).Elements
		if groups[0].(*object.String).Value != "HELLO" {
			t.Errorf("match('hello', 'HELLO world', re.I).group(0) = %v, want 'HELLO'", groups[0])
		}
	} else {
		t.Errorf("match() returned %T, want Match instance", result)
	}

	// Without flag, should not match
	result = match.Fn(context.Background(), nil, &object.String{Value: "hello"}, &object.String{Value: "HELLO world"})
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("match('hello', 'HELLO world') without flag returned %T, want Null", result)
	}
}

func TestRegexSearch(t *testing.T) {
	lib := ReLibrary
	search := lib.Functions()["search"]

	result := search.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123def"})
	if m, ok := result.(*object.Instance); ok && m.Class == MatchClass {
		groups := m.Fields["groups"].(*object.List).Elements
		if groups[0].(*object.String).Value != "123" {
			t.Errorf("search('[0-9]+', 'abc123def').group(0) = %v, want '123'", groups[0])
		}
		start := m.Fields["start"].(*object.Integer).Value
		end := m.Fields["end"].(*object.Integer).Value
		if start != 3 || end != 6 {
			t.Errorf("search span = (%d, %d), want (3, 6)", start, end)
		}
	} else {
		t.Errorf("search() returned %T, want Match instance", result)
	}

	// Test no match
	result = search.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "abcdef"})
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("search('[0-9]+', 'abcdef') returned %T, want Null", result)
	}
}

func TestRegexSearchWithFlags(t *testing.T) {
	lib := ReLibrary
	search := lib.Functions()["search"]

	// Test case-insensitive search
	result := search.Fn(context.Background(), nil, &object.String{Value: "world"}, &object.String{Value: "Hello WORLD"}, &object.Integer{Value: RE_IGNORECASE})
	if m, ok := result.(*object.Instance); ok && m.Class == MatchClass {
		groups := m.Fields["groups"].(*object.List).Elements
		if groups[0].(*object.String).Value != "WORLD" {
			t.Errorf("search('world', 'Hello WORLD', re.I).group(0) = %v, want 'WORLD'", groups[0])
		}
	} else {
		t.Errorf("search() returned %T, want Match instance", result)
	}
}

func TestRegexSearchWithGroups(t *testing.T) {
	lib := ReLibrary
	search := lib.Functions()["search"]

	// Test capturing groups
	result := search.Fn(context.Background(), nil, &object.String{Value: `(\w+)@(\w+)\.(\w+)`}, &object.String{Value: "Email: user@example.com"})
	if m, ok := result.(*object.Instance); ok && m.Class == MatchClass {
		groups := m.Fields["groups"].(*object.List).Elements
		if groups[0].(*object.String).Value != "user@example.com" {
			t.Errorf("search().group(0) = %v, want 'user@example.com'", groups[0])
		}
		if len(groups) != 4 {
			t.Errorf("search() returned %d groups, want 4", len(groups))
		}
		if groups[1].(*object.String).Value != "user" {
			t.Errorf("search().group(1) = %v, want 'user'", groups[1])
		}
		if groups[2].(*object.String).Value != "example" {
			t.Errorf("search().group(2) = %v, want 'example'", groups[2])
		}
		if groups[3].(*object.String).Value != "com" {
			t.Errorf("search().group(3) = %v, want 'com'", groups[3])
		}
	} else {
		t.Errorf("search() returned %T, want Match instance", result)
	}
}

func TestRegexFindall(t *testing.T) {
	lib := ReLibrary
	findall := lib.Functions()["findall"]

	result := findall.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123def456ghi"})
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
	result := findall.Fn(context.Background(), nil, &object.String{Value: "a+"}, &object.String{Value: "aAaAbBAAA"}, &object.Integer{Value: RE_IGNORECASE})
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
	result := sub.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "XXX"}, &object.String{Value: "abc123def"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "abcXXXdef" {
			t.Errorf("sub() = %v, want 'abcXXXdef'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}

	// Test multiple replacements
	result = sub.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "#"}, &object.String{Value: "a1b2c3"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "a#b#c#" {
			t.Errorf("sub() = %v, want 'a#b#c#'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}

	// Test no match
	result = sub.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "XXX"}, &object.String{Value: "abc"})
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
	result := sub.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "X"}, &object.String{Value: "a1b2c3"}, &object.Integer{Value: 1})
	if s, ok := result.(*object.String); ok {
		if s.Value != "aXb2c3" {
			t.Errorf("sub() with count=1 = %v, want 'aXb2c3'", s.Value)
		}
	} else {
		t.Errorf("sub() returned %T, want String", result)
	}

	// Test count=2
	result = sub.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "X"}, &object.String{Value: "a1b2c3"}, &object.Integer{Value: 2})
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

	result := split.Fn(context.Background(), nil, &object.String{Value: "[,;]"}, &object.String{Value: "one,two;three"})
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
	result := split.Fn(context.Background(), nil, &object.String{Value: "[,;]"}, &object.String{Value: "one,two;three;four"}, &object.Integer{Value: 2})
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

	result := compile.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"})
	if r, ok := result.(*object.Instance); ok && r.Class == RegexClass {
		pattern := r.Fields["pattern"].(*object.String).Value
		if pattern != "[0-9]+" {
			t.Errorf("compile() = %v, want '[0-9]+'", pattern)
		}
	} else {
		t.Errorf("compile() returned %T, want Regex instance", result)
	}

	// Test invalid pattern
	result = compile.Fn(context.Background(), nil, &object.String{Value: "[0-9"})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("compile() with invalid pattern should return error, got %T", result)
	}
}

func TestRegexCompileWithFlags(t *testing.T) {
	lib := ReLibrary
	compile := lib.Functions()["compile"]

	// Compile with IGNORECASE flag
	result := compile.Fn(context.Background(), nil, &object.String{Value: "hello"}, &object.Integer{Value: RE_IGNORECASE})
	if r, ok := result.(*object.Instance); ok && r.Class == RegexClass {
		pattern := r.Fields["pattern"].(*object.String).Value
		if pattern != "(?i)hello" {
			t.Errorf("compile() with flag = %v, want '(?i)hello'", pattern)
		}
	} else {
		t.Errorf("compile() returned %T, want Regex instance", result)
	}
}

func TestRegexEscape(t *testing.T) {
	lib := ReLibrary
	escape := lib.Functions()["escape"]

	result := escape.Fn(context.Background(), nil, &object.String{Value: "a.b+c"})
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

	// Test full match - should return Match object
	result := fullmatch.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "123"})
	if _, ok := result.(*object.Instance); !ok {
		t.Errorf("fullmatch('[0-9]+', '123') should return Match object, got %T", result)
	}

	// Test partial match - should return Null
	result = fullmatch.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "123abc"})
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("fullmatch('[0-9]+', '123abc') should return Null, got %T", result)
	}

	// Test no match - should return Null
	result = fullmatch.Fn(context.Background(), nil, &object.String{Value: "[0-9]+"}, &object.String{Value: "abc"})
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("fullmatch('[0-9]+', 'abc') should return Null, got %T", result)
	}

	// Test with groups - should return Match with groups
	result = fullmatch.Fn(context.Background(), nil, &object.String{Value: "(\\d+)-(\\d+)"}, &object.String{Value: "123-456"})
	if match, ok := result.(*object.Instance); ok {
		groups := match.Fields["groups"].(*object.List)
		if len(groups.Elements) != 3 {
			t.Errorf("fullmatch groups should have 3 elements (full match + 2 groups), got %d", len(groups.Elements))
		}
	} else {
		t.Errorf("fullmatch('(\\\\d+)-(\\\\d+)', '123-456') should return Match, got %T", result)
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
