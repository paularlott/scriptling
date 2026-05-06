package scriptling

import "testing"

func BenchmarkStringMethodUpper(b *testing.B) {
	p := New()
	script := `"hello world".upper()`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStringMethodLower(b *testing.B) {
	p := New()
	script := `"HELLO WORLD".lower()`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStringMethodUpperLowerLoop(b *testing.B) {
	p := New()
	script := `
result = ""
for i in range(1000):
    result = result + "hello"
    result = result.upper()
    result = result.lower()
`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := p.Eval(script)
		if err != nil {
			b.Fatal(err)
		}
	}
}
