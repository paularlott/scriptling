package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// The parallel loader starts plugins concurrently (capped) but registers them
// in collection order, so behaviour must match sequential loading exactly:
// every plugin registers, the first executable declaring a name wins, and one
// broken plugin warns without blocking the rest. The helpers below re-exec
// the test binary once per plugin, with the declared library name carried in
// an environment variable, so one binary stands in for a whole directory.

func runParallelPluginTestHelper() {
	name := os.Getenv("SCRIPTLING_PARALLEL_PLUGIN_NAME")
	if name == "" {
		name = "parallel"
	}
	server := NewServer(name, "1.0.0", "parallel load test helper")
	server.RegisterLibrary(object.NewLibrary(name, map[string]*object.Builtin{
		"ping": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewString(name)
		}},
	}, nil, "parallel load test helper"))
	if err := server.Run(); err != nil {
		panic(err)
	}
}

func writeParallelPluginHelper(t *testing.T, path, library string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PARALLEL_PLUGIN_HELPER=1\r\nset SCRIPTLING_PARALLEL_PLUGIN_NAME=" + library + "\r\n\"" + exe + "\" -test.run=TestLoadParallelBeyondCap --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PARALLEL_PLUGIN_HELPER=1 SCRIPTLING_PARALLEL_PLUGIN_NAME=" + library + " exec \"" + exe + "\" -test.run=TestLoadParallelBeyondCap --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func parallelPluginNames(t *testing.T, m *Manager) []string {
	t.Helper()
	names := make([]string, 0, len(m.List()))
	for _, meta := range m.List() {
		names = append(names, meta.Name)
	}
	sort.Strings(names)
	return names
}

// TestLoadParallelBeyondCap loads eight plugins from one directory, past the
// default cap of five concurrent starts: all eight must register, with no
// warnings, and the library each declares must answer.
func TestLoadParallelBeyondCap(t *testing.T) {
	if os.Getenv("SCRIPTLING_PARALLEL_PLUGIN_HELPER") == "1" {
		runParallelPluginTestHelper()
		return
	}

	libraries := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	dir := t.TempDir()
	for i, library := range libraries {
		writeParallelPluginHelper(t, filepath.Join(dir, "p"+string(rune('0'+i+1))+"-"+library), library)
	}

	m := NewManager(nil)
	m.AddDir(dir)
	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer m.Close()

	if warnings := m.Warnings(); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	names := parallelPluginNames(t, m)
	if len(names) != len(libraries) {
		t.Fatalf("loaded %d plugins, want %d: %#v", len(names), len(libraries), names)
	}
	for i, library := range libraries {
		want := "plugin." + library
		if names[i] != want {
			t.Fatalf("names[%d] = %s, want %s (%#v)", i, names[i], want, names)
		}
		if _, ok := m.Get(want); !ok {
			t.Fatalf("plugin %s not registered", want)
		}
	}
}

// TestLoadParallelDuplicateKeepsDirectoryOrder proves order survives the
// parallel start: two executables declaring the same name, both started
// concurrently, and the one that sorts first in the directory wins with the
// other ignored as a duplicate.
func TestLoadParallelDuplicateKeepsDirectoryOrder(t *testing.T) {
	if os.Getenv("SCRIPTLING_PARALLEL_PLUGIN_HELPER") == "1" {
		runParallelPluginTestHelper()
		return
	}

	dir := t.TempDir()
	// Fillers first, so the duplicates sit in a later start window than each
	// other and than the fillers.
	for i, library := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
		writeParallelPluginHelper(t, filepath.Join(dir, "p"+string(rune('0'+i+1))+"-"+library), library)
	}
	writeParallelPluginHelper(t, filepath.Join(dir, "q-dup"), "twin")
	writeParallelPluginHelper(t, filepath.Join(dir, "z-dup"), "twin")

	m := NewManager(nil)
	m.AddDir(dir)
	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer m.Close()

	names := parallelPluginNames(t, m)
	if len(names) != 7 {
		t.Fatalf("loaded %d plugins, want 7: %#v", len(names), names)
	}
	warnings := m.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "duplicate library plugin.twin") || !strings.Contains(warnings[0], "z-dup") {
		t.Fatalf("expected one z-dup duplicate warning, got %#v", warnings)
	}
	client, ok := m.Get("plugin.twin")
	if !ok {
		t.Fatal("plugin.twin not registered")
	}
	if !strings.HasSuffix(client.Path(), "q-dup") {
		t.Fatalf("twin resolved to %s, want the q-dup executable", client.Path())
	}
}

// TestLoadPluginsExplicitBatch covers the embedder-facing batch entry:
// explicit specs load in parallel and register in spec order, and a failed
// start surfaces as an error.
func TestLoadPluginsExplicitBatch(t *testing.T) {
	if os.Getenv("SCRIPTLING_PARALLEL_PLUGIN_HELPER") == "1" {
		runParallelPluginTestHelper()
		return
	}

	dir := t.TempDir()
	specs := []PluginSpec{}
	for _, library := range []string{"alpha", "bravo", "charlie"} {
		path := filepath.Join(dir, "x-"+library)
		writeParallelPluginHelper(t, path, library)
		specs = append(specs, PluginSpec{Path: path})
	}

	m := NewManager(nil)
	defer m.Close()
	if err := m.LoadPlugins(context.Background(), specs); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	names := parallelPluginNames(t, m)
	if len(names) != 3 || names[0] != "plugin.alpha" || names[2] != "plugin.charlie" {
		t.Fatalf("unexpected plugins: %#v", names)
	}

	failing := NewManager(nil)
	defer failing.Close()
	err := failing.LoadPlugins(context.Background(), []PluginSpec{
		specs[0],
		{Path: filepath.Join(dir, "does-not-exist")},
	})
	if err == nil || !strings.Contains(err.Error(), "failed to load") {
		t.Fatalf("expected failed-to-load error, got: %v", err)
	}
	if _, ok := failing.Get("plugin.alpha"); !ok {
		t.Fatal("specs before the failure should have registered")
	}
}
