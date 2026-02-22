package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestFStringFormatSpecs(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		// Float precision
		{"float_precision", `f"{3.14159:.2f}"`, "3.14"},
		{"float_zero_decimal", `f"{3.7:.0f}"`, "4"},
		{"float_width_precision", `f"{3.14:10.2f}"`, "      3.14"},
		// Alignment - strings
		{"str_left_align", `f"{'hello':<20}"`, "hello               "},
		{"str_right_align", `f"{'hello':>20}"`, "               hello"},
		{"str_center_align", `f"{'hello':^20}"`, "       hello        "},
		{"str_fill_left", `f"{'hello':*<20}"`, "hello***************"},
		{"str_fill_right", `f"{'hello':*>20}"`, "***************hello"},
		{"str_fill_center", `f"{'hello':*^20}"`, "*******hello********"},
		// Alignment - integers
		{"int_right_align", `f"{42:10d}"`, "        42"},
		{"int_left_align", `f"{42:<10d}"`, "42        "},
		{"int_center_align", `f"{42:^10d}"`, "    42    "},
		// Zero padding
		{"zero_pad_int", `f"{42:010d}"`, "0000000042"},
		{"zero_pad_hex", `f"{255:08x}"`, "000000ff"},
		// Sign
		{"sign_float_pos", `f"{3.14:+.2f}"`, "+3.14"},
		{"sign_float_neg", `f"{-3.14:+.2f}"`, "-3.14"},
		{"sign_int_pos", `f"{42:+d}"`, "+42"},
		{"sign_int_neg", `f"{-42:+d}"`, "-42"},
		{"space_sign_pos", `f"{42: d}"`, " 42"},
		{"space_sign_neg", `f"{-42: d}"`, "-42"},
		// Thousands grouping
		{"comma_int", `f"{1234567:,}"`, "1,234,567"},
		{"comma_float", `f"{1234567.89:,.2f}"`, "1,234,567.89"},
		// String truncation
		{"str_truncate", `f"{'hello world':.5}"`, "hello"},
		{"str_truncate_s", `f"{'hello world':.5s}"`, "hello"},
		// Percentage
		{"percent_default", `f"{0.75:%}"`, "75.000000%"},
		{"percent_prec", `f"{0.75:.1%}"`, "75.0%"},
		// Hex/oct/bin
		{"hex_lower", `f"{255:x}"`, "ff"},
		{"hex_upper", `f"{255:X}"`, "FF"},
		{"octal", `f"{8:o}"`, "10"},
		{"binary", `f"{10:b}"`, "1010"},
		// Scientific notation
		{"sci_default", `f"{12345.6789:e}"`, "1.234568e+04"},
		{"sci_prec", `f"{12345.6789:.2e}"`, "1.23e+04"},
		{"sci_upper", `f"{12345.6789:.2E}"`, "1.23E+04"},
		// g/G
		{"g_format", `f"{0.00012345:.3g}"`, "0.000123"},
		// Negative numbers with alignment
		{"neg_zero_pad", `f"{-42:010d}"`, "-000000042"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			result, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got := result.Inspect()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
