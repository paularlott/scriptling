package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pluginpack"
)

// srvFetcher serves one package and one setup script for the server-mode
// plugin integration test.
type srvFetcher struct {
	mu      sync.Mutex
	files   map[string]string
	scripts map[string]string
}

func (f *srvFetcher) Read(ctx context.Context, source, path, etag, lastModified string) (plugin.FetchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if path == "" {
		content, ok := f.scripts[source]
		if !ok {
			return plugin.FetchResult{}, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
		}
		return plugin.FetchResult{Data: []byte(content), ETag: "ppsrv-v1"}, nil
	}
	content, ok := f.files[path]
	if !ok {
		return plugin.FetchResult{}, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	if etag == "ppsrv-v1" {
		return plugin.FetchResult{NotModified: true, ETag: "ppsrv-v1"}, nil
	}
	return plugin.FetchResult{Data: []byte(content), ETag: "ppsrv-v1"}, nil
}

func (f *srvFetcher) List(ctx context.Context, source, path string) ([]plugin.FetchEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if path == "" || path == "." {
		return []plugin.FetchEntry{{Name: "lib", IsDir: true}, {Name: "manifest.toml"}}, nil
	}
	if path == "lib" {
		return []plugin.FetchEntry{{Name: "calc.py"}, {Name: "greet.py"}}, nil
	}
	return nil, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
}

// TestServerSetupScriptAndLibsFromFetcherPlugin proves the server-mode flow
// the CLI wires up: the setup script arrives as a scheme source (staged to a
// local file like resolveScriptFile does) and every import resolves through
// the plugin's declared package, with no --package and no local lib dirs.
func TestServerSetupScriptAndLibsFromFetcherPlugin(t *testing.T) {
	fetcher := &srvFetcher{
		files: map[string]string{
			"manifest.toml": "name = \"ppsrv-libs\"\nversion = \"1.0.0\"\nlibs = [\"lib\"]\n",
			"lib/greet.py":  "def greeting(name):\n    return \"hello from ppsrv://libs, \" + name\n",
			"lib/calc.py":   "import greet\n\ndef add(params):\n    return params[\"a\"] + params[\"b\"]\n\ndef hello(params):\n    return greet.greeting(params.get(\"name\", \"World\"))\n",
		},
		scripts: map[string]string{
			"ppsrv://scripts/setup": "import scriptling.runtime as runtime\n\nruntime.jsonrpc.method(\"ppsrv.add\", \"calc.add\")\nruntime.jsonrpc.method(\"ppsrv.hello\", \"calc.hello\")\n",
		},
	}

	pluginSrv := plugin.NewServer("ppsrv-plugin", "1.0.0", "server pluginpack test")
	pluginSrv.RegisterFetcher("ppsrv", fetcher)
	pluginSrv.DeclarePackage("ppsrv://libs")
	pluginHTTP := httptest.NewServer(pluginSrv)
	defer pluginHTTP.Close()

	manager := plugin.NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadURL(context.Background(), "ppsrvplugin", pluginHTTP.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	if err := pluginpack.Register(manager); err != nil {
		t.Fatalf("pluginpack.Register: %v", err)
	}

	// Stage the setup script exactly like the CLI's resolveScriptFile.
	content, err := pluginpack.FetchScript("ppsrv://scripts/setup")
	if err != nil {
		t.Fatalf("FetchScript: %v", err)
	}
	staged := filepath.Join(t.TempDir(), "setup.py")
	if err := os.WriteFile(staged, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Open the declared packages exactly like the CLI's declaredLibBundles.
	bundles, err := pluginpack.DeclaredBundles(manager, false, t.TempDir(), nil)
	if err != nil {
		t.Fatalf("DeclaredBundles: %v", err)
	}
	if len(bundles) != 1 || bundles[0].Manifest.Name != "ppsrv-libs" {
		t.Fatalf("expected one declared bundle, got %+v", bundles)
	}

	s, err := NewServer(ServerConfig{
		ScriptFile: staged,
		LibBundles: bundles,
		JSONRPC:    true,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer ts.Close()

	post := func(request string) map[string]any {
		t.Helper()
		resp, err := http.Post(ts.URL, "application/json", strings.NewReader(request))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("response %s: %v", body, err)
		}
		return decoded
	}

	resp := post(`{"jsonrpc":"2.0","method":"ppsrv.add","params":{"a":40,"b":2},"id":1}`)
	if result, ok := resp["result"].(float64); !ok || result != 42 {
		t.Errorf("ppsrv.add: response %v, want result 42", resp)
	}
	resp = post(`{"jsonrpc":"2.0","method":"ppsrv.hello","params":{"name":"Server"},"id":2}`)
	if result, ok := resp["result"].(string); !ok || result != "hello from ppsrv://libs, Server" {
		t.Errorf("ppsrv.hello: response %v, want the plugin-served greeting", resp)
	}
}
