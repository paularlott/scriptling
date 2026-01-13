package errors

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestNewError(t *testing.T) {
	err := NewError("test error: %s", "something")
	if err.Message != "test error: something" {
		t.Errorf("NewError() message = %q, want %q", err.Message, "test error: something")
	}

	err2 := NewError("simple error")
	if err2.Message != "simple error" {
		t.Errorf("NewError() message = %q, want %q", err2.Message, "simple error")
	}
}

func TestNewTimeoutError(t *testing.T) {
	err := NewTimeoutError()
	if err.Message != ErrTimeout {
		t.Errorf("NewTimeoutError() message = %q, want %q", err.Message, ErrTimeout)
	}
}

func TestNewCancelledError(t *testing.T) {
	err := NewCancelledError()
	if err.Message != ErrCancelled {
		t.Errorf("NewCancelledError() message = %q, want %q", err.Message, ErrCancelled)
	}
}

func TestNewTypeError(t *testing.T) {
	err := NewTypeError("string", "int")
	expected := "type error: expected string, got int"
	if err.Message != expected {
		t.Errorf("NewTypeError() message = %q, want %q", err.Message, expected)
	}
}

func TestNewArgumentError(t *testing.T) {
	err := NewArgumentError(3, 2)
	expected := "argument error: got 3 arguments, want 2"
	if err.Message != expected {
		t.Errorf("NewArgumentError() message = %q, want %q", err.Message, expected)
	}

	err2 := NewArgumentError(0, 1)
	expected2 := "argument error: got 0 arguments, want 1"
	if err2.Message != expected2 {
		t.Errorf("NewArgumentError() message = %q, want %q", err2.Message, expected2)
	}
}

func TestNewIdentifierError(t *testing.T) {
	err := NewIdentifierError("x")
	expected := "identifier not found: x"
	if err.Message != expected {
		t.Errorf("NewIdentifierError() message = %q, want %q", err.Message, expected)
	}

	err2 := NewIdentifierError("myFunction")
	expected2 := "identifier not found: myFunction"
	if err2.Message != expected2 {
		t.Errorf("NewIdentifierError() message = %q, want %q", err2.Message, expected2)
	}
}

func TestParameterError(t *testing.T) {
	baseErr := NewError("base error")
	wrappedErr := ParameterError("param1", baseErr)

	expected := "param1: base error"
	if wrappedErr.(*object.Error).Message != expected {
		t.Errorf("ParameterError() message = %q, want %q", wrappedErr.(*object.Error).Message, expected)
	}
}

func TestNoArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []object.Object
		wantError bool
	}{
		{"no args", []object.Object{}, false},
		{"one arg", []object.Object{&object.Integer{}}, true},
		{"multiple args", []object.Object{&object.Integer{}, &object.String{}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NoArgs(tt.args)
			if tt.wantError && err == nil {
				t.Error("NoArgs() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("NoArgs() expected no error, got %v", err)
			}
		})
	}
}

func TestExactArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []object.Object
		n         int
		wantError bool
	}{
		{"exact match", []object.Object{&object.Integer{}}, 1, false},
		{"too few", []object.Object{}, 1, true},
		{"too many", []object.Object{&object.Integer{}, &object.String{}}, 1, true},
		{"zero args exact", []object.Object{}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExactArgs(tt.args, tt.n)
			if tt.wantError && err == nil {
				t.Error("ExactArgs() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("ExactArgs() expected no error, got %v", err)
			}
		})
	}
}

func TestMinArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []object.Object
		n         int
		wantError bool
	}{
		{"exact match", []object.Object{&object.Integer{}}, 1, false},
		{"more than min", []object.Object{&object.Integer{}, &object.String{}}, 1, false},
		{"too few", []object.Object{}, 1, true},
		{"zero min", []object.Object{}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MinArgs(tt.args, tt.n)
			if tt.wantError && err == nil {
				t.Error("MinArgs() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("MinArgs() expected no error, got %v", err)
			}
		})
	}
}

func TestMaxArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []object.Object
		n         int
		wantError bool
	}{
		{"exact match", []object.Object{&object.Integer{}}, 1, false},
		{"less than max", []object.Object{}, 1, false},
		{"too many", []object.Object{&object.Integer{}, &object.String{}}, 1, true},
		{"zero max", []object.Object{}, 0, false},
		{"zero max with args", []object.Object{&object.Integer{}}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MaxArgs(tt.args, tt.n)
			if tt.wantError && err == nil {
				t.Error("MaxArgs() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("MaxArgs() expected no error, got %v", err)
			}
		})
	}
}

func TestRangeArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []object.Object
		min       int
		max       int
		wantError bool
	}{
		{"within range lower", []object.Object{&object.Integer{}}, 1, 3, false},
		{"within range middle", []object.Object{&object.Integer{}, &object.String{}}, 1, 3, false},
		{"within range upper", []object.Object{&object.Integer{}, &object.String{}, &object.Integer{}}, 1, 3, false},
		{"too few", []object.Object{}, 1, 3, true},
		{"too many", []object.Object{&object.Integer{}, &object.String{}, &object.Integer{}, &object.String{}}, 1, 3, true},
		{"same min max", []object.Object{&object.Integer{}}, 1, 1, false},
		{"same min max fail", []object.Object{&object.Integer{}, &object.String{}}, 1, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RangeArgs(tt.args, tt.min, tt.max)
			if tt.wantError && err == nil {
				t.Error("RangeArgs() expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("RangeArgs() expected no error, got %v", err)
			}
		})
	}
}

func TestErrorConstants(t *testing.T) {
	constants := map[string]string{
		ErrTimeout:            "execution timeout",
		ErrCancelled:          "execution cancelled",
		ErrDivisionByZero:     "division by zero",
		ErrIndexOutOfRange:    "index out of range",
		ErrKeyNotFound:        "key not found",
		ErrTypeError:          "type error",
		ErrArgumentError:      "argument error",
		ErrIdentifierNotFound: "identifier not found",
		ErrUnknownOperator:    "unknown operator",
		ErrImportError:        "import error",
	}

	for name, expected := range constants {
		if name != expected {
			t.Errorf("Constant %q = %q, want %q", name, name, expected)
		}
	}
}
