package pluginpack

import (
	"context"
	"io/fs"
	"testing"
)

// The fetch gateway's host-side escape defense is cleanPath: every pluginFS
// entry point (Open, ReadFile, Stat, ReadDir) must reject a traversing or
// absolute path with fs.ErrInvalid BEFORE it turns into an RPC to the plugin.
// A malicious source/import string must not be able to walk the host out of
// the plugin's virtual tree, and the plugin must never even see such a path.
//
// These tests were added during review — the guard had no direct coverage.

var traversalPaths = []string{
	"../etc/passwd",
	"a/../../b",
	"/etc/passwd",
	"./../secret",
	"foo/../../bar",
	"", // empty is not a valid fs path
	"a//b",
}

// TestCleanPathRejectsTraversal is the direct unit check on the guard.
func TestCleanPathRejectsTraversal(t *testing.T) {
	for _, p := range traversalPaths {
		if _, err := cleanPath(p); err == nil {
			t.Errorf("cleanPath(%q) = nil error, want rejection", p)
		}
	}
	// Sanity: legitimate clean relative paths pass.
	for _, ok := range []string{".", "lib/hello.py", "a/b/c.txt"} {
		if _, err := cleanPath(ok); err != nil {
			t.Errorf("cleanPath(%q) rejected a valid path: %v", ok, err)
		}
	}
}

// TestPluginFSEntryPointsRejectTraversalBeforeRPC drives each fs entry point
// with traversing paths against a pluginFS whose client is nil: if any entry
// point tried to fetch instead of rejecting the path first, it would panic on
// the nil client. cleanPath must short-circuit before any RPC.
func TestPluginFSEntryPointsRejectTraversalBeforeRPC(t *testing.T) {
	p := newPluginFS(context.Background(), nil, "ppond://libs", 0)

	for _, bad := range traversalPaths {
		if _, err := p.ReadFile(bad); err == nil {
			t.Errorf("ReadFile(%q) = nil error, want rejection", bad)
		}
		if _, err := p.Open(bad); err == nil {
			t.Errorf("Open(%q) = nil error, want rejection", bad)
		}
		if _, err := p.Stat(bad); err == nil {
			t.Errorf("Stat(%q) = nil error, want rejection", bad)
		}
		if _, err := p.ReadDir(bad); err == nil {
			t.Errorf("ReadDir(%q) = nil error, want rejection", bad)
		}
	}
}

// TestCleanPathErrorIsInvalid confirms the rejection carries fs.ErrInvalid, the
// signal fs helpers (WalkDir, Sub) rely on.
func TestCleanPathErrorIsInvalid(t *testing.T) {
	_, err := cleanPath("../escape")
	if err == nil {
		t.Fatal("expected an error")
	}
	pe, ok := err.(*fs.PathError)
	if !ok || pe.Err != fs.ErrInvalid {
		t.Fatalf("expected fs.PathError wrapping fs.ErrInvalid, got %T: %v", err, err)
	}
}
