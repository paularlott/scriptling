package plugin

import "testing"

func TestDeclaredLibraryName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"plugin.hello", "hello"},
		{"plugin.", "plugin."},
		{"pl", "pl"},
		{"", ""},
	}
	for _, tt := range tests {
		got := declaredLibraryName(tt.input)
		if got != tt.want {
			t.Errorf("declaredLibraryName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeLibraryName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "plugin.hello"},
		{"plugin.hello", "plugin.hello"},
		{"", "plugin."},
	}
	for _, tt := range tests {
		got := NormalizeLibraryName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeLibraryName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRPCErrorError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var e *RPCError
		if e.Error() != "" {
			t.Errorf("expected empty string, got %q", e.Error())
		}
	})
	t.Run("non-nil", func(t *testing.T) {
		e := &RPCError{Code: -32000, Message: "test error"}
		if e.Error() != "test error" {
			t.Errorf("expected 'test error', got %q", e.Error())
		}
	})
}
