package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/pluginpack"
	"path"
)

// srvFetcher serves one package and one setup script for the server-mode
// plugin integration test.
type srvFetcher struct {
	mu      sync.Mutex
	files   map[string]string
	scripts map[string]string
}

func (f *srvFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if path == "" {
		content, ok := f.scripts[source]
		if !ok {
			return nil, fmt.Errorf("%w: %s", plugin.ErrFetchNotFound, source)
		}
		return []byte(content), nil
	}
	content, ok := f.files[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s in %s", plugin.ErrFetchNotFound, path, source)
	}
	return []byte(content), nil
}

func (f *srvFetcher) Glob(ctx context.Context, source, pattern string) ([]plugin.FetchEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	tree := map[string]bool{"lib": true}
	for name := range f.files {
		tree[name] = false
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			tree[dir] = true
		}
	}
	entries := []plugin.FetchEntry{}
	for name, isDir := range tree {
		if plugin.MatchGlob(pattern, name) {
			entries = append(entries, plugin.FetchEntry{Name: name, IsDir: isDir})
		}
	}
	return entries, nil
}

// TestServerSetupScriptAndLibsFromFetcherPlugin proves the server-mode flow
// the CLI wires up: the setup script arrives as a scheme source and is handed
// to the server as source text — nothing is staged to disk — and every import
// resolves through the plugin's declared package, with no --package and no
// local lib dirs.
func TestServerSetupScriptAndLibsFromFetcherPlugin(t *testing.T) {
	fetcher := &srvFetcher{
		files: map[string]string{
			"lib/greet.py": "def greeting(name):\n    return \"hello from ppsrv://libs, \" + name\n",
			"lib/calc.py":  "import greet\n\ndef add(params):\n    return params[\"a\"] + params[\"b\"]\n\ndef hello(params):\n    return greet.greeting(params.get(\"name\", \"World\"))\n",
		},
		scripts: map[string]string{
			"ppsrv://scripts/setup": "import scriptling.runtime as runtime\n\nruntime.jsonrpc.method(\"ppsrv.add\", \"calc.add\")\nruntime.jsonrpc.method(\"ppsrv.hello\", \"calc.hello\")\n",
		},
	}

	pluginSrv := plugin.NewServer("ppsrv-plugin", "1.0.0", "server pluginpack test")
	pluginSrv.RegisterFetcher("ppsrv", fetcher)
	pluginHTTP := httptest.NewServer(pluginSrv)
	defer pluginHTTP.Close()

	manager := plugin.NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadURL(context.Background(), "ppsrv-plugin", pluginHTTP.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	bridge := pluginpack.New(pluginpack.Options{
		Manager: manager,
		Context: context.Background(),
	})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Bridge.Register: %v", err)
	}
	defer bridge.Close()

	// Fetch the setup script exactly like the CLI's setupScript: source text,
	// no temporary file anywhere.
	content, err := bridge.FetchScript(context.Background(), "ppsrv://scripts/setup")
	if err != nil {
		t.Fatalf("FetchScript: %v", err)
	}

	// Open the declared packages exactly like the CLI's declaredLibBundles.
	bundles, err := bridge.Bundles()
	if err != nil {
		t.Fatalf("Bundles: %v", err)
	}
	if len(bundles) != 1 || bundles[0].Manifest.Name != "ppsrv-plugin" {
		t.Fatalf("expected the plugin's library bundle, got %+v", bundles)
	}

	s, err := NewServer(ServerConfig{
		ScriptSource: content,
		ScriptName:   "ppsrv://scripts/setup",
		LibBundles:   bundles,
		JSONRPC:      true,
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

// TestServerFetcherPluginFullStack closes the combination gaps: one app
// bundle served entirely by a fetcher plugin (manifest, setup script, handler
// modules, background task module and an MCP tool), driven through every
// server surface. Each surface proves its own scheme-import path:
//
//   - HTTP route: the handler module is imported per request from the scheme,
//     and it imports a second scheme module (chained fetcher imports)
//   - JSON-RPC: the module-ref method resolves through the scheme
//   - background: a request-time task runs a module whose own imports
//     delegate through the caller's loader to the fetcher, awaited on the
//     promise the handler holds
//   - MCP: a decorated tool's module imports from the scheme inside the
//     tool's environment
func TestServerFetcherPluginFullStack(t *testing.T) {
	fetcher := &srvFetcher{
		files: map[string]string{
			"manifest.toml": `name = "ppsapp"
version = "1.0.0"
main = "setup.py"
libs = ["lib"]
serve = ["http", "mcp"]
`,
			"setup.py": `import scriptling.runtime as runtime

runtime.http.get("/hello", "web.hello")
runtime.jsonrpc.method("ppsrv.add", "calc.add")
runtime.jsonrpc.method("ppsrv.bg", "bgapi.run")
`,
			"lib/greet.py":  "def greeting(name):\n    return \"hello from ppsrv://app, \" + name\n",
			"lib/web.py":    "import greet\n\ndef hello(request):\n    return {\"status\": 200, \"headers\": {}, \"body\": greet.greeting(\"route\")}\n",
			"lib/calc.py":   "def add(params):\n    return params[\"a\"] + params[\"b\"]\n",
			"lib/bgapi.py":  "import scriptling.runtime as runtime\n\ndef run(params):\n    promise = runtime.background(\"ppsrv-bg\", \"bgwork.task\")\n    return promise.get()\n",
			"lib/bgwork.py": "import greet\n\ndef task():\n    return greet.greeting(\"background\")\n",
			"tools/say_hi.py": `import scriptling.runtime.mcp as mcp
import greet

@mcp.tool("Greet via a scheme-served tool", params={"name": {"type": "string", "description": "Who to greet"}})
def say_hi(name):
    return greet.greeting(name)
`,
		},
	}

	pluginSrv := plugin.NewServer("ppsfull-plugin", "1.0.0", "full-stack pluginpack test")
	pluginSrv.RegisterFetcher("ppsfull", fetcher)
	pluginHTTP := httptest.NewServer(pluginSrv)
	defer pluginHTTP.Close()

	manager := plugin.NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadURL(context.Background(), "ppsfull-plugin", pluginHTTP.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	bridge := pluginpack.New(pluginpack.Options{Manager: manager, Context: context.Background()})
	if err := bridge.Register(); err != nil {
		t.Fatalf("Bridge.Register: %v", err)
	}
	defer bridge.Close()

	// The whole app bundle, manifest included, arrives through the fetcher.
	bundle, err := pack.FetchBundle("ppsfull://app", false, t.TempDir())
	if err != nil {
		t.Fatalf("FetchBundle: %v", err)
	}
	s, err := NewServer(ServerConfig{Bundle: bundle, JSONRPC: true})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// HTTP route: handler module from the scheme, importing another one.
	resp, err := http.Get(ts.URL + "/hello")
	if err != nil {
		t.Fatalf("GET /hello: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "hello from ppsrv://app, route") {
		t.Fatalf("GET /hello = %d %s", resp.StatusCode, body)
	}

	post := func(request string) map[string]any {
		t.Helper()
		resp, err := http.Post(ts.URL+"/json-rpc", "application/json", strings.NewReader(request))
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

	// JSON-RPC module ref through the scheme.
	addResp := post(`{"jsonrpc":"2.0","method":"ppsrv.add","params":{"a":40,"b":2},"id":1}`)
	if result, ok := addResp["result"].(float64); !ok || result != 42 {
		t.Errorf("ppsrv.add: response %v, want result 42", addResp)
	}

	// Request-time background task whose module imports through the fetcher,
	// awaited on the promise inside the handler.
	bgResp := post(`{"jsonrpc":"2.0","method":"ppsrv.bg","id":2}`)
	if result, ok := bgResp["result"].(string); !ok || result != "hello from ppsrv://app, background" {
		t.Errorf("ppsrv.bg: response %v, want the background task's scheme-imported greeting", bgResp)
	}

	// MCP tool: listed and callable, its module importing from the scheme.
	mcpServer := s.mcpHandler.server.Load()
	if mcpServer == nil {
		t.Fatal("MCP server not initialized")
	}
	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()
	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	found := false
	for _, tool := range tools {
		if tool.Name == "say_hi" {
			found = true
		}
	}
	if !found {
		t.Fatalf("say_hi tool not listed, got %d tools", len(tools))
	}
	result, err := client.CallTool(ctx, "say_hi", map[string]interface{}{"name": "mcp"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello from ppsrv://app, mcp" {
		t.Fatalf("say_hi(mcp) = %+v", result.Content)
	}
}
