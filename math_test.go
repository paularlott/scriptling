package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestMathLibrary(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected interface{}
	}{
		{"sqrt", "import math\nresult = math.sqrt(16)", 4.0},
		{"pow", "import math\nresult = math.pow(2, 8)", 256.0},
		{"fabs int", "import math\nresult = math.fabs(-5)", 5.0},
		{"fabs float", "import math\nresult = math.fabs(-5.5)", 5.5},
		{"floor", "import math\nresult = math.floor(3.7)", int64(3)},
		{"ceil", "import math\nresult = math.ceil(3.2)", int64(4)},
		{"pi", "import math\nresult = math.pi", 3.141592653589793},
		{"e", "import math\nresult = math.e", 2.718281828459045},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			p.RegisterLibrary(stdlib.MathLibraryName, stdlib.MathLibrary)
			_, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			result, objErr := p.GetVar("result")
			if objErr != nil {
				t.Fatal("result variable not found")
			}

			switch expected := tt.expected.(type) {
			case int64:
				if result != expected {
					t.Errorf("got %v, want %v", result, expected)
				}
			case float64:
				if fResult, ok := result.(float64); ok {
					if fResult != expected {
						t.Errorf("got %v, want %v", fResult, expected)
					}
				} else {
					t.Errorf("result is %T, want float64", result)
				}
			}
		})
	}
}

func TestMathInExpression(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibraryName, stdlib.MathLibrary)
	_, err := p.Eval(`
import math

# Calculate circle area
radius = 5
area = math.pi * math.pow(radius, 2)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	area, objErr := p.GetVar("area")
	if objErr != nil {
		t.Fatal("area variable not found")
	}

	expected := 78.53981633974483
	if fArea, ok := area.(float64); ok {
		if fArea != expected {
			t.Errorf("area = %v, want %v", fArea, expected)
		}
	} else {
		t.Errorf("area is %T, want float64", area)
	}
}
