package scriptling

import "testing"

func BenchmarkArithmetic(b *testing.B) {
	p := New()
	code := "result = 10 + 20 * 3 - 5"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}

func BenchmarkFunctionCall(b *testing.B) {
	p := New()
	code := `
def add(a, b):
    return a + b
result = add(5, 3)
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}

func BenchmarkListOperations(b *testing.B) {
	p := New()
	code := `
items = [1, 2, 3]
items = append(items, 4)
length = len(items)
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}

func BenchmarkDictOperations(b *testing.B) {
	p := New()
	code := `
data = {"name": "Alice", "age": 30}
name = data["name"]
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}

func BenchmarkStringOperations(b *testing.B) {
	p := New()
	code := `
text = "hello world"
upper_text = upper(text)
parts = split(text, " ")
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}

func BenchmarkJSONParse(b *testing.B) {
	p := New("json")
	code := `data = json.parse('{"name":"Alice","age":30}')`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}

func BenchmarkJSONStringify(b *testing.B) {
	p := New("json")
	code := `result = json.stringify({"name": "Alice", "age": 30})`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(code)
	}
}
