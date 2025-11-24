package scriptling

import "testing"

func TestMultipleAssignment(t *testing.T) {
	tests := []struct {
		name   string
		script string
		checks map[string]interface{}
	}{
		{
			"two variables",
			"a, b = [1, 2]",
			map[string]interface{}{"a": int64(1), "b": int64(2)},
		},
		{
			"three variables",
			"x, y, z = [10, 20, 30]",
			map[string]interface{}{"x": int64(10), "y": int64(20), "z": int64(30)},
		},
		{
			"mixed types",
			`name, age = ["Alice", 30]`,
			map[string]interface{}{"name": "Alice", "age": int64(30)},
		},
		{
			"from expression",
			"a, b = [1 + 1, 2 * 2]",
			map[string]interface{}{"a": int64(2), "b": int64(4)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			for varName, expected := range tt.checks {
				result, ok := p.GetVar(varName)
				if !ok {
					t.Fatalf("%s variable not found", varName)
				}

				if result != expected {
					t.Errorf("%s = %v, want %v", varName, result, expected)
				}
			}
		})
	}
}

func TestMultipleAssignmentErrors(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{"too few values", "a, b, c = [1, 2]"},
		{"too many values", "a, b = [1, 2, 3]"},
		{"not a list", "a, b = 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tt.script)
			if err == nil {
				t.Fatal("Expected error but got none")
			}
		})
	}
}

func TestMultipleAssignmentSwap(t *testing.T) {
	p := New()
	_, err := p.Eval(`
x = 1
y = 2
x, y = [y, x]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	x, _ := p.GetVar("x")
	y, _ := p.GetVar("y")

	if x != int64(2) {
		t.Errorf("x = %v, want 2", x)
	}
	if y != int64(1) {
		t.Errorf("y = %v, want 1", y)
	}
}
