package mcp

import (
	"strings"
	"testing"
)

// Tool, resource and prompt metadata TOML is depth-checked before decoding, so
// a crafted metadata file cannot exhaust the host (vuln-0014).
func TestMetadataParsersRejectDeepNesting(t *testing.T) {
	deep := []byte("x = " + strings.Repeat("{a=", 4000) + "1" + strings.Repeat("}", 4000))
	if _, err := parseToolMetadata(deep); err == nil {
		t.Error("deep tool metadata should be rejected")
	}
	if _, _, _, _, err := parseResourceMetadata(deep); err == nil {
		t.Error("deep resource metadata should be rejected")
	}
	if _, _, err := parsePromptMetadata(deep); err == nil {
		t.Error("deep prompt metadata should be rejected")
	}
	// Normal metadata still parses.
	if _, err := parseToolMetadata([]byte("description = \"a tool\"\n")); err != nil {
		t.Errorf("normal tool metadata rejected: %v", err)
	}
}
