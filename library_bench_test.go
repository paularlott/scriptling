package scriptling

import (
	"testing"
)

func BenchmarkLibraryImport(b *testing.B) {
	p := New()
	script := `import math`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(script)
	}
}

func BenchmarkMathOperations(b *testing.B) {
	p := New()
	script := `
import math
x = math.sqrt(16)
y = math.pow(2, 8)
z = math.abs(-5)
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(script)
	}
}

func BenchmarkJSONOperations(b *testing.B) {
	p := New()
	script := `
import json
data = json.loads('{"key":"value","num":42}')
result = json.dumps(data)
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(script)
	}
}

func BenchmarkMultipleLibraries(b *testing.B) {
	p := New()
	script := `
import math
import json
import time
import base64

x = math.sqrt(25)
data = json.loads('{"test":123}')
encoded = base64.encode("hello")
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(script)
	}
}

func BenchmarkLibraryFunctionCall(b *testing.B) {
	p := New()
	// Pre-import the library
	p.Eval(`import math`)
	script := `result = math.sqrt(16)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(script)
	}
}
