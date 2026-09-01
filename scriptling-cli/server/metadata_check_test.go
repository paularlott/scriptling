package server

import (
	"strings"
	"testing"
)

// TestNewServerRejectsUnmetSetupScript proves a setup script's metadata
// block is verified at server construction, before anything binds: an
// unmet requirement surfaces as a NewServer error naming the setup script,
// the plugin, and how to load plugins.
func TestNewServerRejectsUnmetSetupScript(t *testing.T) {
	setup := `# /// script
# dependencies = ["knot.space via knot >= 1.2.3"]
# ///
import knot.space
`
	_, err := NewServer(ServerConfig{
		ScriptSource: []byte(setup),
		ScriptName:   "setup.py",
	})
	if err == nil {
		t.Fatal("expected NewServer to refuse an unmet setup script")
	}
	msg := err.Error()
	for _, want := range []string{"setup script", `plugin "knot" is not loaded`, "--plugin-dir"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q does not contain %q", msg, want)
		}
	}
}

// TestNewServerAcceptsMetSetupScript is the counterpart: a satisfiable
// block must not change server startup.
func TestNewServerAcceptsMetSetupScript(t *testing.T) {
	setup := `# /// script
# requires-scriptling = ">=0.1"
# dependencies = ["json"]
# ///
import json
`
	if _, err := NewServer(ServerConfig{
		ScriptSource: []byte(setup),
		ScriptName:   "setup.py",
	}); err != nil {
		t.Fatalf("expected a satisfiable block to boot: %v", err)
	}
}
