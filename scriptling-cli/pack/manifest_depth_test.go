package pack

import (
	"strings"
	"testing"
)

// A manifest.toml nested past the cap is rejected before the decoder, so a
// crafted package cannot exhaust the host that loads it (vuln-0014).
func TestParseManifestRejectsDeepNesting(t *testing.T) {
	data := []byte("x = " + strings.Repeat("{a=", 4000) + "1" + strings.Repeat("}", 4000))
	if _, err := parseManifest(data); err == nil {
		t.Fatal("deep manifest should be rejected")
	}
	// A normal manifest still parses.
	if _, err := parseManifest([]byte("name = \"demo\"\nversion = \"1.0\"\n")); err != nil {
		t.Fatalf("a normal manifest was rejected: %v", err)
	}
}
