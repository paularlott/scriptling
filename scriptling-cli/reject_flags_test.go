package main

import (
	"strings"
	"testing"
)

func TestRejectBundleFlags(t *testing.T) {
	// No conflicts — clean.
	if err := rejectBundleFlags(bundleFlagConflicts{}); err != nil {
		t.Fatalf("empty conflicts should pass, got %v", err)
	}

	// Each flag individually triggers rejection naming that flag.
	conflicts := []struct {
		name string
		c    bundleFlagConflicts
	}{
		{"file", bundleFlagConflicts{File: "setup.py"}},
		{"libpath", bundleFlagConflicts{LibPath: []string{"lib"}}},
		{"mcp-tools", bundleFlagConflicts{MCPTools: "./tools"}},
		{"mcp-resources", bundleFlagConflicts{MCPResources: "./res"}},
		{"mcp-prompts", bundleFlagConflicts{MCPPrompts: "./prompts"}},
		{"web-root", bundleFlagConflicts{WebRoot: "./assets"}},
		{"code", bundleFlagConflicts{Code: "print(1)"}},
		{"interactive", bundleFlagConflicts{Interactive: true}},
	}
	for _, tc := range conflicts {
		err := rejectBundleFlags(tc.c)
		if err == nil {
			t.Errorf("%s set but no error returned", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.name) {
			t.Errorf("error %q does not name the flag %q", err.Error(), tc.name)
		}
		if !strings.Contains(err.Error(), "manifest.toml") {
			t.Errorf("error %q should explain the flag is manifest-owned", err.Error())
		}
	}

	// The first conflict is reported (deterministic order).
	err := rejectBundleFlags(bundleFlagConflicts{File: "x", MCPTools: "y"})
	if err == nil || !strings.Contains(err.Error(), "file") {
		t.Errorf("expected 'file' reported first, got %v", err)
	}
}
