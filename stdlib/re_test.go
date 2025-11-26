package stdlib

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestRegexMatch(t *testing.T) {
	lib := ReLibrary()
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

func TestRegexFind(t *testing.T) {
	lib := ReLibrary()
	find := lib.Functions()["find"]

	result := find.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123def"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "123" {
			t.Errorf("find('[0-9]+', 'abc123def') = %v, want '123'", s.Value)
		}
	} else {
		t.Errorf("find() returned %T, want String", result)
	}
}

func TestRegexSearch(t *testing.T) {
	lib := ReLibrary()
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

func TestRegexFindall(t *testing.T) {
	lib := ReLibrary()
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

func TestRegexReplace(t *testing.T) {
	lib := ReLibrary()
	replace := lib.Functions()["replace"]

	result := replace.Fn(context.Background(), &object.String{Value: "[0-9]+"}, &object.String{Value: "abc123def"}, &object.String{Value: "XXX"})
	if s, ok := result.(*object.String); ok {
		if s.Value != "abcXXXdef" {
			t.Errorf("replace() = %v, want 'abcXXXdef'", s.Value)
		}
	} else {
		t.Errorf("replace() returned %T, want String", result)
	}
}

func TestRegexSplit(t *testing.T) {
	lib := ReLibrary()
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

func TestRegexCompile(t *testing.T) {
	lib := ReLibrary()
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

func TestRegexEscape(t *testing.T) {
	lib := ReLibrary()
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
	lib := ReLibrary()
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
