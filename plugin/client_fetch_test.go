package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
)

// fetchHelper runs a plugin server with a registered fetcher. It backs the
// --plugin / LoadPlugin tests.
func fetchHelper() {
	fetcher := &countingFetcher{content: "print('served')", onlyPath: "manifest.toml"}
	server := NewServer("fetchhelper", "1.0.0", "fetch helper plugin")
	server.RegisterFetcher("fh", fetcher)
	_ = server.Run()
	os.Exit(0)
}

func writeFetchHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_FETCH_HELPER=1\r\n\"" + exe + "\" -test.run=TestFetchHelper --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_FETCH_HELPER=1 exec \"" + exe + "\" -test.run=TestFetchHelper --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

// TestFetchHelper is the re-exec entry point used by writeFetchHelper.
func TestFetchHelper(t *testing.T) {
	if os.Getenv("SCRIPTLING_FETCH_HELPER") == "1" {
		fetchHelper()
	}
}

func TestLoadPluginRegistersUnderDeclaredName(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fetch-plugin")
	writeFetchHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()
	client, err := manager.LoadPlugin(context.Background(), helper, nil)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}
	if got := client.Metadata().Name; got != "plugin.fetchhelper" {
		t.Fatalf("expected declared name plugin.fetchhelper, got %s", got)
	}
	if !client.SupportsFetch() {
		t.Fatal("expected SupportsFetch after handshake")
	}
	if scheme := client.Scheme(); scheme != "fh" {
		t.Fatalf("expected scheme fh, got %q", scheme)
	}
	if _, ok := manager.Get("plugin.fetchhelper"); !ok {
		t.Fatal("expected client registered under declared name")
	}
}

func TestLoadPluginIsIdempotentByPath(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fetch-plugin")
	writeFetchHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()
	first, err := manager.LoadPlugin(context.Background(), helper, nil)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}
	second, err := manager.LoadPlugin(context.Background(), helper, nil)
	if err != nil {
		t.Fatalf("LoadPlugin again: %v", err)
	}
	if first != second {
		t.Fatal("expected the same client for the same executable")
	}
}

func TestClientFetchFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fetch-plugin")
	writeFetchHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()
	client, err := manager.LoadPlugin(context.Background(), helper, nil)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	res, err := client.FetchFile(context.Background(), "fh://libs", "manifest.toml")
	if err != nil {
		t.Fatalf("FetchFile: %v", err)
	}
	if string(res.Data) != "print('served')" {
		t.Fatalf("unexpected result: %+v", res)
	}

	// Miss maps to ErrFetchNotFound.
	_, err = client.FetchFile(context.Background(), "fh://libs", "missing.py")
	if !errors.Is(err, ErrFetchNotFound) {
		t.Fatalf("expected ErrFetchNotFound, got %v", err)
	}

	entries, err := client.FetchList(context.Background(), "fh://libs", ".")
	if err != nil {
		t.Fatalf("FetchList: %v", err)
	}
	if len(entries) != 2 || entries[0].Name != "lib" || !entries[0].IsDir {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestClientFetchRefusedWithoutCapability(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "plain-plugin")
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()
	client, err := manager.LoadPlugin(context.Background(), helper, nil)
	if err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}
	if client.SupportsFetch() {
		t.Fatal("plain plugin must not report fetch support")
	}
	if _, err := client.FetchFile(context.Background(), "fh://libs", "x"); err == nil {
		t.Fatal("expected an error fetching from a plugin without the fetch capability")
	}
	if _, err := client.FetchList(context.Background(), "fh://libs", "."); err == nil {
		t.Fatal("expected an error listing from a plugin without the fetch capability")
	}
}

func TestDescribeExposesFetchCapabilityAndSchemes(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "fetch-plugin")
	writeFetchHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadPlugin(context.Background(), helper, nil); err != nil {
		t.Fatalf("LoadPlugin: %v", err)
	}

	p := scriptling.New()
	RegisterLibraries(p, manager)
	result, err := p.Eval(`import scriptling.plugin
meta = scriptling.plugin.describe("plugin.fetchhelper")
"fetch" in meta["capabilities"] and meta["scheme"] == "fh"`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "True" {
		t.Fatalf("expected capabilities/schemes in describe(), got %s", result.Inspect())
	}
}

func TestSpawnedPeerSeesPluginEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell peer helper is unix-only")
	}
	outFile := filepath.Join(t.TempDir(), "peer-env.txt")
	helper := filepath.Join(t.TempDir(), "env-peer")
	script := "#!/bin/sh\nprintf '%s' \"${SCRIPTLING_PLUGIN_PEER:-unset}\" > '" + outFile + "'\nexit 0\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	client, err := SpawnClient(context.Background(), helper, nil)
	if err != nil {
		t.Fatalf("SpawnClient: %v", err)
	}
	defer client.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(outFile); err == nil {
			if got := strings.TrimSpace(string(data)); got != "1" {
				t.Fatalf("peer saw SCRIPTLING_PLUGIN_PEER=%q, want 1", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("peer never reported its environment")
}
