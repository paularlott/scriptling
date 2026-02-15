package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestBoundMethodReferences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "basic method reference",
			input: `
class Test:
    def method(self, x):
        return x * 2

t = Test()
m = t.method
result = m(5)
result
`,
			expected: int64(10),
		},
		{
			name: "method reference as callback",
			input: `
def caller(fn, arg):
    return fn(arg)

class Adder:
    def __init__(self, base):
        self.base = base

    def add(self, x):
        return self.base + x

a = Adder(10)
result = caller(a.add, 5)
result
`,
			expected: int64(15),
		},
		{
			name: "method reference in list",
			input: `
class Counter:
    def __init__(self):
        self.count = 0

    def increment(self):
        self.count = self.count + 1
        return self.count

c = Counter()
methods = [c.increment, c.increment, c.increment]
results = [m() for m in methods]
results
`,
			expected: []int64{1, 2, 3},
		},
		{
			name: "method reference stored in variable",
			input: `
class Multiplier:
    def __init__(self, factor):
        self.factor = factor
    
    def multiply(self, x):
        return x * self.factor

m = Multiplier(3)
func = m.multiply
result = func(7)
result
`,
			expected: int64(21),
		},
		{
			name: "multiple method references from same instance",
			input: `
class Calculator:
    def __init__(self, value):
        self.value = value
    
    def add(self, x):
        return self.value + x
    
    def multiply(self, x):
        return self.value * x

calc = Calculator(10)
add_func = calc.add
mul_func = calc.multiply
result = add_func(5) + mul_func(3)
result
`,
			expected: int64(45), // (10 + 5) + (10 * 3) = 15 + 30 = 45
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			env := object.NewEnvironment()
			ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
			result := EvalWithContext(ctx, program, env)

			if object.IsError(result) {
				t.Fatalf("evaluation error: %s", result.Inspect())
			}

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, result, expected)
			case []int64:
				list, ok := result.(*object.List)
				if !ok {
					t.Fatalf("result is not a list. got=%T", result)
				}
				if len(list.Elements) != len(expected) {
					t.Fatalf("wrong number of elements. got=%d, want=%d", len(list.Elements), len(expected))
				}
				for i, exp := range expected {
					testIntegerObject(t, list.Elements[i], exp)
				}
			}
		})
	}
}

func TestInstanceAttributesWithFunctions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "store function in instance attribute",
			input: `
class Container:
    def __init__(self):
        self.func = None

    def set_func(self, f):
        self.func = f

    def call_func(self, x):
        if self.func is not None:
            return self.func(x)
        return None

def double(x):
    return x * 2

c = Container()
c.set_func(double)
result = c.call_func(5)
result
`,
			expected: int64(10),
		},
		{
			name: "store lambda in instance attribute",
			input: `
class Config:
    def __init__(self):
        self.transformer = None

    def set_transform(self, fn):
        self.transformer = fn

    def transform(self, x):
        if self.transformer:
            return self.transformer(x)
        return x

cfg = Config()
cfg.set_transform(lambda x: x.upper())
result = cfg.transform("hello")
result
`,
			expected: "HELLO",
		},
		{
			name: "check truthiness of stored attribute",
			input: `
class Handler:
    def __init__(self):
        self.callback = None

    def has_callback(self):
        return self.callback is not None

def cb():
    pass

h = Handler()
result1 = h.has_callback()
h.callback = cb
result2 = h.has_callback()
results = [result1, result2]
results
`,
			expected: []bool{false, true},
		},
		{
			name: "store and call method reference",
			input: `
class Processor:
    def __init__(self):
        self.handler = None
    
    def set_handler(self, h):
        self.handler = h
    
    def process(self, x):
        if self.handler is not None:
            return self.handler(x)
        return x

class Transformer:
    def __init__(self, multiplier):
        self.multiplier = multiplier
    
    def transform(self, x):
        return x * self.multiplier

p = Processor()
t = Transformer(3)
p.set_handler(t.transform)
result = p.process(7)
result
`,
			expected: int64(21),
		},
		{
			name: "multiple function attributes",
			input: `
class EventHandler:
    def __init__(self):
        self.on_start = None
        self.on_end = None
    
    def trigger_start(self):
        if self.on_start:
            return self.on_start()
        return "no start handler"
    
    def trigger_end(self):
        if self.on_end:
            return self.on_end()
        return "no end handler"

eh = EventHandler()
eh.on_start = lambda: "started"
eh.on_end = lambda: "ended"
result = [eh.trigger_start(), eh.trigger_end()]
result
`,
			expected: []string{"started", "ended"},
		},
		{
			name: "attribute access in conditional",
			input: `
class Router:
    def __init__(self):
        self.route_handler = None
    
    def handle(self, path):
        if self.route_handler is not None:
            return self.route_handler(path)
        return "no handler"

r = Router()
result1 = r.handle("/test")
r.route_handler = lambda p: "handled: " + p
result2 = r.handle("/test")
results = [result1, result2]
results
`,
			expected: []string{"no handler", "handled: /test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			env := object.NewEnvironment()
			ctx := ContextWithCallDepth(context.Background(), DefaultMaxCallDepth)
			result := EvalWithContext(ctx, program, env)

			if object.IsError(result) {
				t.Fatalf("evaluation error: %s", result.Inspect())
			}

			switch expected := tt.expected.(type) {
			case int64:
				testIntegerObject(t, result, expected)
			case string:
				testStringObject(t, result, expected)
			case []bool:
				list, ok := result.(*object.List)
				if !ok {
					t.Fatalf("result is not a list. got=%T", result)
				}
				if len(list.Elements) != len(expected) {
					t.Fatalf("wrong number of elements. got=%d, want=%d", len(list.Elements), len(expected))
				}
				for i, exp := range expected {
					testBooleanObject(t, list.Elements[i], exp)
				}
			case []string:
				list, ok := result.(*object.List)
				if !ok {
					t.Fatalf("result is not a list. got=%T", result)
				}
				if len(list.Elements) != len(expected) {
					t.Fatalf("wrong number of elements. got=%d, want=%d", len(list.Elements), len(expected))
				}
				for i, exp := range expected {
					testStringObject(t, list.Elements[i], exp)
				}
			}
		})
	}
}

func checkParserErrors(t *testing.T, p *parser.Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}
