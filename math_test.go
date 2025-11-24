package scriptling

import "testing"

func TestMathLibrary(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected interface{}
	}{
		{"sqrt", "import math\nresult = math.sqrt(16)", 4.0},
		{"pow", "import math\nresult = math.pow(2, 8)", 256.0},
		{"abs int", "import math\nresult = math.abs(-5)", int64(5)},
		{"abs float", "import math\nresult = math.abs(-5.5)", 5.5},
		{"floor", "import math\nresult = math.floor(3.7)", int64(3)},
		{"ceil", "import math\nresult = math.ceil(3.2)", int64(4)},
		{"round", "import math\nresult = math.round(3.5)", int64(4)},
		{"min", "import math\nresult = math.min(3, 1, 2)", int64(1)},
		{"max", "import math\nresult = math.max(3, 1, 2)", int64(3)},
		{"pi", "import math\nresult = math.pi()", 3.141592653589793},
		{"e", "import math\nresult = math.e()", 2.718281828459045},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			result, ok := p.GetVar("result")
			if !ok {
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
	_, err := p.Eval(`
import math

# Calculate circle area
radius = 5
area = math.pi() * math.pow(radius, 2)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	area, ok := p.GetVar("area")
	if !ok {
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
