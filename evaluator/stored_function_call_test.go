package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestStoredFunctionCall(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name: "call stored function via self.field()",
			input: `
class Handler:
    def __init__(self, fn):
        self.callback = fn
    
    def execute(self, x):
        return self.callback(x * 2)

def double(n):
    return n * 2

h = Handler(double)
result = h.execute(5)
result
`,
			expected: int64(20),
		},
		{
			name: "call stored lambda via self.field()",
			input: `
class Processor:
    def __init__(self):
        self.transform = lambda x: x + 10
    
    def process(self, val):
        return self.transform(val)

p = Processor()
result = p.process(5)
result
`,
			expected: int64(15),
		},
		{
			name: "call stored builtin via self.field()",
			input: `
class Container:
    def __init__(self):
        self.converter = str
    
    def convert(self, val):
        return self.converter(val)

c = Container()
result = c.convert(42)
result
`,
			expected: "42",
		},
		{
			name: "multiple stored functions",
			input: `
class Calculator:
    def __init__(self):
        self.add = lambda a, b: a + b
        self.mul = lambda a, b: a * b
    
    def compute(self, x, y):
        return self.add(x, y) + self.mul(x, y)

calc = Calculator()
result = calc.compute(3, 4)
result
`,
			expected: int64(19), // (3+4) + (3*4) = 7 + 12 = 19
		},
		{
			name: "stored function from dict",
			input: `
class CommandBot:
    def __init__(self):
        self.commands = {}
    
    def register(self, name, handler):
        self.commands[name] = {"handler": handler}
    
    def execute(self, name, arg):
        handler = self.commands[name]["handler"]
        return handler(arg)

def greet(name):
    return "Hello " + name

bot = CommandBot()
bot.register("greet", greet)
result = bot.execute("greet", "World")
result
`,
			expected: "Hello World",
		},
		{
			name: "direct call from dict without intermediate variable",
			input: `
class CommandBot:
    def __init__(self):
        self.commands = {}
    
    def register(self, name, handler):
        self.commands[name] = {"handler": handler}
    
    def execute_direct(self, name, arg):
        return self.commands[name]["handler"](arg)

def greet(name):
    return "Hello " + name

bot = CommandBot()
bot.register("greet", greet)
result = bot.execute_direct("greet", "World")
result
`,
			expected: "Hello World",
		},
		{
			name: "stored function with kwargs",
			input: `
class Formatter:
    def __init__(self, fn):
        self.format_fn = fn
    
    def format(self, text):
        return self.format_fn(text, prefix=">>", suffix="<<")

def wrap(text, prefix="[", suffix="]"):
    return prefix + text + suffix

f = Formatter(wrap)
result = f.format("test")
result
`,
			expected: ">>test<<",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()
			
			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			env := object.NewEnvironment()
			result := Eval(program, env)

			if object.IsError(result) {
				t.Fatalf("evaluation error: %s", result.Inspect())
			}

			switch expected := tt.expected.(type) {
			case int64:
				intResult, ok := result.(*object.Integer)
				if !ok {
					t.Fatalf("expected Integer, got %T", result)
				}
				if intResult.Value != expected {
					t.Errorf("expected %d, got %d", expected, intResult.Value)
				}
			case string:
				strResult, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if strResult.Value != expected {
					t.Errorf("expected %s, got %s", expected, strResult.Value)
				}
			}
		})
	}
}
