package scriptling

import (
	"testing"
)

func TestLists(t *testing.T) {
	p := New()
	result, err := p.Eval(`
numbers = [1, 2, 3, 4, 5]
numbers[0]
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "1" {
		t.Errorf("expected 1, got %v", result.Inspect())
	}

	result, err = p.Eval(`numbers[4]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "5" {
		t.Errorf("expected 5, got %v", result.Inspect())
	}
}

func TestDictionaries(t *testing.T) {
	p := New()
	result, err := p.Eval(`
person = {"name": "Alice", "age": "30"}
person["name"]
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "Alice" {
		t.Errorf("expected Alice, got %v", result.Inspect())
	}

	result, err = p.Eval(`person["age"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "30" {
		t.Errorf("expected 30, got %v", result.Inspect())
	}
}

func TestForLoop(t *testing.T) {
	p := New()
	_, err := p.Eval(`
sum = 0
numbers = [1, 2, 3, 4, 5]
for num in numbers:
    sum = sum + num
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sum, _ := p.GetVar("sum")
	if sum != int64(15) {
		t.Errorf("expected 15, got %v", sum)
	}
}

func TestStringFunctions(t *testing.T) {
	p := New()
	_, err := p.Eval(`
text = "hello world"
upper_text = upper(text)
lower_text = lower("HELLO")
replaced = replace(text, "world", "scriptling")
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	upper, _ := p.GetVar("upper_text")
	if upper != "HELLO WORLD" {
		t.Errorf("expected HELLO WORLD, got %v", upper)
	}

	lower, _ := p.GetVar("lower_text")
	if lower != "hello" {
		t.Errorf("expected hello, got %v", lower)
	}

	replaced, _ := p.GetVar("replaced")
	if replaced != "hello scriptling" {
		t.Errorf("expected hello scriptling, got %v", replaced)
	}
}

func TestSplitJoin(t *testing.T) {
	p := New()
	_, err := p.Eval(`
words = split("one,two,three", ",")
joined = join(words, "-")
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined, _ := p.GetVar("joined")
	if joined != "one-two-three" {
		t.Errorf("expected one-two-three, got %v", joined)
	}
}

func TestTypeConversions(t *testing.T) {
	p := New()
	_, err := p.Eval(`
num = int("42")
flt = float("3.14")
text = str(100)
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	num, _ := p.GetVar("num")
	if num != int64(42) {
		t.Errorf("expected 42, got %v", num)
	}

	flt, _ := p.GetVar("flt")
	if flt != 3.14 {
		t.Errorf("expected 3.14, got %v", flt)
	}

	text, _ := p.GetVar("text")
	if text != "100" {
		t.Errorf("expected 100, got %v", text)
	}
}

func TestAppend(t *testing.T) {
	p := New()
	_, err := p.Eval(`
numbers = [1, 2, 3]
append(numbers, 4)
append(numbers, 5)
length = len(numbers)
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	length, _ := p.GetVar("length")
	if length != int64(5) {
		t.Errorf("expected 5, got %v", length)
	}
}

func TestJSON(t *testing.T) {
	p := New("json")
	p.SetVar("json_str", `{"name":"Alice","age":30}`)
	result, err := p.Eval(`
data = json["parse"](json_str)
data["name"]
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "Alice" {
		t.Errorf("expected Alice, got %v", result.Inspect())
	}

	result, err = p.Eval(`
obj = {"key": "value"}
json["stringify"](obj)
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != `{"key":"value"}` {
		t.Errorf("expected {\"key\":\"value\"}, got %v", result.Inspect())
	}
}

func TestNestedStructures(t *testing.T) {
	p := New()
	result, err := p.Eval(`
data = [1, 2, [3, 4, 5]]
data[2]
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "[3, 4, 5]" {
		t.Errorf("expected [3, 4, 5], got %v", result.Inspect())
	}
}

func TestStringIndexing(t *testing.T) {
	p := New()
	result, err := p.Eval(`
word = "hello"
word[0]
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "h" {
		t.Errorf("expected h, got %v", result.Inspect())
	}

	result, err = p.Eval(`word[4]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Inspect() != "o" {
		t.Errorf("expected o, got %v", result.Inspect())
	}
}
