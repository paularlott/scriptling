package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

// === COMPILE TIME (Lexer + Parser) ===
func BenchmarkCompile_Simple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New().Eval("x = 5")
	}
}

func BenchmarkCompile_Function(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New().Eval("def add(a, b):\n    return a + b\nresult = add(5, 3)")
	}
}

func BenchmarkCompile_Loop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New().Eval("for i in [1, 2, 3, 4, 5]:\n    x = i * 2")
	}
}

func BenchmarkCompile_Complex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		New().Eval("def fib(n):\n    if n <= 1:\n        return n\n    return fib(n-1) + fib(n-2)\nresult = fib(10)")
	}
}

// === RUNTIME - Arithmetic ===
func BenchmarkRuntime_IntArithmetic(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = 5 + 3 * 2 - 1")
	}
}

func BenchmarkRuntime_FloatArithmetic(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = 5.5 + 3.2 * 2.1 - 1.0")
	}
}

func BenchmarkRuntime_Division(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = 10 / 3")
	}
}

// === RUNTIME - Variables ===
func BenchmarkRuntime_VarAssign(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = 42")
	}
}

func BenchmarkRuntime_VarRead(b *testing.B) {
	p := New()
	p.Eval("x = 42")
	for i := 0; i < b.N; i++ {
		p.Eval("y = x")
	}
}

func BenchmarkRuntime_AugmentedAssign(b *testing.B) {
	p := New()
	p.Eval("x = 10")
	for i := 0; i < b.N; i++ {
		p.Eval("x += 1")
	}
}

// === RUNTIME - Strings ===
func BenchmarkRuntime_StringConcat(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval(`x = "hello" + " " + "world"`)
	}
}

func BenchmarkRuntime_StringConcatLoop(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval(`result = ""\nfor i in range(10):\n    result = result + str(i)`)
	}
}

func BenchmarkRuntime_StringMethods(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval(`x = upper("hello")`)
	}
}

func BenchmarkRuntime_StringJoin(b *testing.B) {
	p := New()
	p.Eval(`items = ["a", "b", "c", "d", "e"]`)
	for i := 0; i < b.N; i++ {
		p.Eval(`result = join(items, ",")`)
	}
}

func BenchmarkRuntime_StringJoinLarge(b *testing.B) {
	p := New()
	p.Eval(`items = []`)
	p.Eval(`for i in range(100):\n    items = append(items, str(i))`)
	for i := 0; i < b.N; i++ {
		p.Eval(`result = join(items, ",")`)
	}
}

// === RUNTIME - Lists ===
func BenchmarkRuntime_ListCreate(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = [1, 2, 3, 4, 5]")
	}
}

func BenchmarkRuntime_ListAppend(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = []")
		p.Eval("append(x, 1)")
	}
}

func BenchmarkRuntime_ListIndex(b *testing.B) {
	p := New()
	p.Eval("x = [1, 2, 3, 4, 5]")
	for i := 0; i < b.N; i++ {
		p.Eval("y = x[2]")
	}
}

func BenchmarkRuntime_ListSlice(b *testing.B) {
	p := New()
	p.Eval("x = [1, 2, 3, 4, 5]")
	for i := 0; i < b.N; i++ {
		p.Eval("y = x[1:3]")
	}
}

// === RUNTIME - Dictionaries ===
func BenchmarkRuntime_DictCreate(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval(`x = {"a": 1, "b": 2, "c": 3}`)
	}
}

func BenchmarkRuntime_DictAccess(b *testing.B) {
	p := New()
	p.Eval(`x = {"a": 1, "b": 2, "c": 3}`)
	for i := 0; i < b.N; i++ {
		p.Eval(`y = x["b"]`)
	}
}

func BenchmarkRuntime_DictKeys(b *testing.B) {
	p := New()
	p.Eval(`x = {"a": 1, "b": 2, "c": 3}`)
	for i := 0; i < b.N; i++ {
		p.Eval("y = keys(x)")
	}
}

// === RUNTIME - Control Flow ===
func BenchmarkRuntime_IfStatement(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = 10\nif x > 5:\n    y = 1\nelse:\n    y = 0")
	}
}

func BenchmarkRuntime_WhileLoop(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = 0\nwhile x < 10:\n    x = x + 1")
	}
}

func BenchmarkRuntime_ForLoop(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("for i in [1, 2, 3, 4, 5]:\n    x = i * 2")
	}
}

func BenchmarkRuntime_ForRange(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("for i in range(10):\n    x = i * 2")
	}
}

// === RUNTIME - Functions ===
func BenchmarkRuntime_FunctionCall(b *testing.B) {
	p := New()
	p.Eval("def add(a, b):\n    return a + b")
	for i := 0; i < b.N; i++ {
		p.Eval("result = add(5, 3)")
	}
}

func BenchmarkRuntime_RecursiveFib(b *testing.B) {
	p := New()
	p.Eval("def fib(n):\n    if n <= 1:\n        return n\n    return fib(n-1) + fib(n-2)")
	for i := 0; i < b.N; i++ {
		p.Eval("result = fib(10)")
	}
}

// === RUNTIME - Builtins ===
func BenchmarkRuntime_Len(b *testing.B) {
	p := New()
	p.Eval("x = [1, 2, 3, 4, 5]")
	for i := 0; i < b.N; i++ {
		p.Eval("y = len(x)")
	}
}

func BenchmarkRuntime_TypeConversion(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = str(42)")
	}
}

// === LIBRARIES ===
func BenchmarkRuntime_JSONParse(b *testing.B) {
	p := New()
	p.Eval("import json")
	for i := 0; i < b.N; i++ {
		p.Eval(`data = json.loads('{"name":"Alice","age":30}')`)
	}
}

func BenchmarkRuntime_JSONStringify(b *testing.B) {
	p := New()
	p.Eval("import json")
	p.Eval(`data = {"name": "Alice", "age": 30}`)
	for i := 0; i < b.N; i++ {
		p.Eval("result = json.dumps(data)")
	}
}

func BenchmarkRuntime_RegexMatch(b *testing.B) {
	p := New()
	p.Eval("import re")
	for i := 0; i < b.N; i++ {
		p.Eval(`result = re.match("[0-9]+", "abc123")`)
	}
}

func BenchmarkRuntime_RegexFindAll(b *testing.B) {
	p := New()
	p.Eval("import re")
	for i := 0; i < b.N; i++ {
		p.Eval(`result = re.findall("[0-9]+", "abc123def456ghi789")`)
	}
}

func BenchmarkRuntime_RegexCompileAndMethods(b *testing.B) {
	p := New()
	p.Eval("import re")
	for i := 0; i < b.N; i++ {
		p.Eval(`
pattern = re.compile(r"(\w+) (\w+)")
match = pattern.match("hello world")
if match:
    groups = match.groups()
    first = match.group(1)
    span = match.span()
`)
	}
}

func BenchmarkRuntime_RegexComplexOperations(b *testing.B) {
	p := New()
	p.Eval("import re")
	for i := 0; i < b.N; i++ {
		p.Eval(`
# Complex regex operations
text = "The quick brown fox jumps over the lazy dog 123 456"
numbers = re.findall(r"\d+", text)
words = re.findall(r"\w+", text)
pattern = re.compile(r"(\w{4,})")
matches = pattern.findall(text)
`)
	}
}

// === CACHE ===
func BenchmarkCache_Hit(b *testing.B) {
	script := "x = 5 + 3"
	p := New()
	p.Eval(script)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(script)
	}
}

func BenchmarkCache_Miss(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval("x = " + string(rune(i%1000)))
	}
}

// === SCENARIOS ===
func BenchmarkScenario_DataProcessing(b *testing.B) {
	p := New()
	p.Eval("import json")
	for i := 0; i < b.N; i++ {
		p.Eval(`data = json.loads('{"items":[1,2,3,4,5]}')\ntotal = 0\nfor item in data["items"]:\n    total = total + item\nresult = total`)
	}
}

func BenchmarkScenario_ConfigLogic(b *testing.B) {
	p := New()
	for i := 0; i < b.N; i++ {
		p.Eval(`config = {"env": "prod", "timeout": 30}\nif config["env"] == "prod":\n    timeout = config["timeout"] * 2\nelse:\n    timeout = config["timeout"]`)
	}
}

func BenchmarkAccessorOverhead(b *testing.B) {
	// Create test objects
	str := &object.String{Value: "test"}
	intObj := &object.Integer{Value: 42}

	b.Run("DirectAccess", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = str.Value
			_ = intObj.Value
		}
	})

	b.Run("AccessorMethods", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			s, _ := str.AsString()
			n, _ := intObj.AsInt()
			_ = s
			_ = n
		}
	})
}

// === STRING PERFORMANCE ===
func BenchmarkStringConcatenation(b *testing.B) {
	p := New()
	p.Eval("import json")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(`
result = ""
for i in range(100):
    result = result + str(i)
`)
	}
}

func BenchmarkStringConcatenationSmall(b *testing.B) {
	p := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(`
result = "hello" + " " + "world"
`)
	}
}

func BenchmarkStringConcatenationLarge(b *testing.B) {
	p := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(`
result = ""
for i in range(1000):
    result = result + "test string " + str(i) + " "
`)
	}
}

func BenchmarkStringBuilderPoolTest(b *testing.B) {
	p := New()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.Eval(`
# Simulate many small string operations
parts = []
for i in range(100):
    parts.append("part" + str(i))
result = ""
for part in parts:
    result = result + part
`)
	}
}
