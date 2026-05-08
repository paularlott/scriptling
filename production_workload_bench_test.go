package scriptling

import (
	"os"
	"strings"
	"testing"
)

func loadProductionEmailSummaryShape(b testing.TB) string {
	b.Helper()

	data, err := os.ReadFile("phpvs/email_summary_shape.sl")
	if err != nil {
		b.Fatalf("failed to read production-shaped benchmark script: %v", err)
	}

	return string(data)
}

func loadProductionEmailSummaryProgram(b testing.TB) *Scriptling {
	b.Helper()

	p := New()
	script := loadProductionEmailSummaryShape(b)
	if _, err := p.Eval(script); err != nil {
		b.Fatalf("failed to load production-shaped benchmark script: %v", err)
	}

	return p
}

func TestProductionEmailSummaryShape(t *testing.T) {
	p := loadProductionEmailSummaryProgram(t)

	result, err := p.CallFunction("run_summary")
	if err != nil {
		t.Fatalf("CallFunction failed: %v", err)
	}

	text, objErr := result.AsString()
	if objErr != nil {
		t.Fatalf("result.AsString failed: %s", objErr.Inspect())
	}

	if !strings.Contains(text, "## Email Summary") {
		t.Fatalf("summary missing heading: %q", text)
	}
	if !strings.Contains(text, "Images Included: 3") {
		t.Fatalf("summary missing expected image count: %q", text)
	}
	if !strings.Contains(text, "Other Attachments: 3") {
		t.Fatalf("summary missing expected other attachment count: %q", text)
	}
}

func BenchmarkProductionEmailSummaryShape_CallFunction(b *testing.B) {
	p := loadProductionEmailSummaryProgram(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := p.CallFunction("run_summary")
		if err != nil {
			b.Fatal(err)
		}
		if _, objErr := result.AsString(); objErr != nil {
			b.Fatalf("result.AsString failed: %s", objErr.Inspect())
		}
	}
}

func BenchmarkProductionEmailSummaryShape_Eval(b *testing.B) {
	script := loadProductionEmailSummaryShape(b)
	p := New()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := p.Eval(script + "\nresult = run_summary()\n"); err != nil {
			b.Fatal(err)
		}
	}
}
