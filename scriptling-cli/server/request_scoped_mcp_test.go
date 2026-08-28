package server

// Tests for request-scoped MCP entries: middleware calls
// scriptling.runtime.mcp.register_request_tool / _resource / _prompt to expose
// tools, resources and prompts for the life of the request being served — so
// per-user entries are possible, with authorization re-evaluated on every MCP
// message. Also covers mcp.transport() / runtime.jsonrpc.transport().

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"encoding/json"
	"github.com/paularlott/scriptling"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

// writeRequestMCPServer builds a serving test server: a static greet tool,
// token middleware that authenticates alice (registering her request-scoped
// tool, resources and prompt) and bob (registering only a tool named like the
// static one, for the collision test).
func writeRequestMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	libDir := t.TempDir()
	toolsDir := t.TempDir()

	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    token = request.header("authorization", "")
    if token == "Bearer alice-key":
        request.context["user"] = "alice"
        mcp.register_request_tool("secret_tool", handler="secretmod.secret",
            description="Alice only",
            params={"x": {"type": "string", "description": "an argument", "required": True}})
        mcp.register_request_resource("user://alice/profile", handler="restools.profile",
            name="Alice profile", mime_type="application/json")
        mcp.register_request_resource("user://alice/docs/{path}", handler="restools.doc",
            name="Alice docs", template=True)
        mcp.register_request_prompt("summarise", handler="promptmod.summarise",
            description="Summarise something",
            arguments=[{"name": "topic", "description": "What to summarise", "required": True}])
        return None
    if token == "Bearer bob-key":
        request.context["user"] = "bob"
        mcp.register_request_tool("greet", handler="secretmod.not_greet",
            description="Shadows the static greet")
        return None
    return {"status": 401, "body": "unauthorized"}
`))
	writeFile(t, libDir+"/secretmod.py", []byte(`
import scriptling.mcp.tool as tool

def secret(x):
    user = tool.request_context().get("user", "anon")
    return {"user": user, "x": x}

def not_greet(name):
    return "shadowed"
`))
	writeFile(t, libDir+"/restools.py", []byte(`
import scriptling.mcp.tool as tool

def profile(__uri):
    user = tool.request_context().get("user", "anon")
    return {"uri": __uri, "user": user}

def doc(__uri, path):
    return "doc:" + path
`))
	writeFile(t, libDir+"/promptmod.py", []byte(`
def summarise(topic):
    return {"messages": [
        {"role": "user", "content": "summarise " + topic},
        {"role": "assistant", "content": "working on it"},
    ]}
`))

	writeFile(t, toolsDir+"/greet.toml", []byte("description = \"Greet\"\n\n[[parameters]]\nname=\"name\"\ntype=\"string\"\ndescription=\"Name\"\nrequired=true\n"))
	writeFile(t, toolsDir+"/greet.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi ' + tool.get_string('name'))\n"))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

// listToolNames returns the tool names from tools/list for the given token.
func listToolNames(t *testing.T, ts *httptest.Server, auth string) map[string]bool {
	t.Helper()
	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, auth)
	if status != http.StatusOK {
		t.Fatalf("tools/list: %d %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	names := map[string]bool{}
	for _, tl := range tools {
		if tool, ok := tl.(map[string]any); ok {
			names[tool["name"].(string)] = true
		}
	}
	return names
}

func TestRequestToolPerUserVisibilityAndCall(t *testing.T) {
	ts := writeRequestMCPServer(t)

	alice := listToolNames(t, ts, "Bearer alice-key")
	if !alice["greet"] || !alice["secret_tool"] {
		t.Fatalf("alice's tools = %v, want greet + secret_tool", alice)
	}
	bob := listToolNames(t, ts, "Bearer bob-key")
	if !bob["greet"] || bob["secret_tool"] {
		t.Fatalf("bob's tools = %v, want greet only", bob)
	}

	// Alice can call her tool; the handler sees the middleware context and
	// its arguments.
	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"secret_tool","arguments":{"x":"hello"}}}`,
		"Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("tools/call secret_tool: %d %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) < 1 {
		t.Fatalf("secret_tool produced no content: %#v", body)
	}
	first, _ := content[0].(map[string]any)
	if !strings.Contains(first["text"].(string), `"user":"alice"`) || !strings.Contains(first["text"].(string), `"x":"hello"`) {
		t.Fatalf("secret_tool result = %#v, want user=alice x=hello", first["text"])
	}

	// Bob cannot call it: authorization is re-evaluated on the call.
	status, body = mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"secret_tool","arguments":{"x":"hi"}}}`,
		"Bearer bob-key")
	if status != http.StatusOK {
		t.Fatalf("tools/call as bob: %d", status)
	}
	if obj, ok := body["error"].(map[string]any); !ok || !strings.Contains(obj["message"].(string), "unknown tool") {
		t.Fatalf("bob calling secret_tool = %#v, want unknown tool", body)
	}
}

// A request tool named like a static tool never shadows it.
func TestRequestToolDoesNotShadowStaticTool(t *testing.T) {
	ts := writeRequestMCPServer(t)

	names := listToolNames(t, ts, "Bearer bob-key")
	if count := len(names); count != 1 || !names["greet"] {
		t.Fatalf("bob's tools = %v, want exactly greet", names)
	}

	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"greet","arguments":{"name":"world"}}}`,
		"Bearer bob-key")
	if status != http.StatusOK {
		t.Fatalf("tools/call greet: %d", status)
	}
	result, _ := body["result"].(map[string]any)
	content, _ := result["content"].([]any)
	first, _ := content[0].(map[string]any)
	if first["text"] != "hi world" {
		t.Fatalf("greet = %#v, want hi world (static tool must win)", first["text"])
	}
}

// When the middleware registers the same tool name twice, the first
// registration wins for both listing and dispatch — mirroring how static
// tools beat provider tools.
func TestRequestToolDuplicateRegistrationFirstWins(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    mcp.register_request_tool("dup", handler="dupmod.first", description="first")
    mcp.register_request_tool("dup", handler="dupmod.second", description="second")
    return None
`))
	writeFile(t, libDir+"/dupmod.py", []byte(`
def first():
    return "from first"

def second():
    return "from second"
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}, MCPToolsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	names := listToolNames(t, ts, "Bearer any")
	if len(names) != 1 || !names["dup"] {
		t.Fatalf("tools = %v, want exactly one dup", names)
	}

	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"dup","arguments":{}}}`,
		"Bearer any")
	if status != http.StatusOK {
		t.Fatalf("tools/call dup: %d %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	content, _ := result["content"].([]any)
	first, _ := content[0].(map[string]any)
	if first["text"] != "from first" {
		t.Fatalf("dup = %#v, want from first (first registration must win)", first["text"])
	}
}

// A request tool whose handler raises surfaces as a tool error to the
// client, not a silent success or a transport failure.
func TestRequestToolHandlerError(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    mcp.register_request_tool("boom", handler="boommod.boom", description="Explodes")
    return None
`))
	writeFile(t, libDir+"/boommod.py", []byte(`
def boom():
    raise ValueError("exploded on purpose")
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}, MCPToolsDir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"boom","arguments":{}}}`,
		"Bearer any")
	if status != http.StatusOK {
		t.Fatalf("tools/call boom: %d %#v", status, body)
	}
	obj, ok := body["error"].(map[string]any)
	if !ok || !strings.Contains(obj["message"].(string), "exploded on purpose") {
		t.Fatalf("boom result = %#v, want an error carrying the raise message", body)
	}
}

func TestRequestResources(t *testing.T) {
	ts := writeRequestMCPServer(t)
	auth := "Bearer alice-key"

	// resources/list shows the static resource; templates/list shows the template.
	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, auth)
	if status != http.StatusOK {
		t.Fatalf("resources/list: %d", status)
	}
	result, _ := body["result"].(map[string]any)
	resources, _ := result["resources"].([]any)
	found := false
	for _, rl := range resources {
		if res, ok := rl.(map[string]any); ok && res["uri"] == "user://alice/profile" {
			found = true
		}
	}
	if !found {
		t.Fatalf("resources/list = %#v, want user://alice/profile", result)
	}

	status, body = mcpPost(t, ts, `{"jsonrpc":"2.0","id":2,"method":"resources/templates/list"}`, auth)
	if status != http.StatusOK {
		t.Fatalf("resources/templates/list: %d", status)
	}
	result, _ = body["result"].(map[string]any)
	templates, _ := result["resourceTemplates"].([]any)
	found = false
	for _, tl := range templates {
		if tmpl, ok := tl.(map[string]any); ok && tmpl["uriTemplate"] == "user://alice/docs/{path}" {
			found = true
		}
	}
	if !found {
		t.Fatalf("resources/templates/list = %#v, want the docs template", result)
	}

	// Reading the static resource runs the handler with the context.
	status, body = mcpPost(t, ts, `{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"user://alice/profile"}}`, auth)
	if status != http.StatusOK {
		t.Fatalf("resources/read profile: %d %#v", status, body)
	}
	result, _ = body["result"].(map[string]any)
	contents, _ := result["contents"].([]any)
	first, _ := contents[0].(map[string]any)
	if !strings.Contains(first["text"].(string), `"user":"alice"`) {
		t.Fatalf("profile content = %#v, want user alice", first["text"])
	}

	// Reading through the template extracts the {path} variable.
	status, body = mcpPost(t, ts, `{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"user://alice/docs/intro"}}`, auth)
	if status != http.StatusOK {
		t.Fatalf("resources/read doc: %d %#v", status, body)
	}
	result, _ = body["result"].(map[string]any)
	contents, _ = result["contents"].([]any)
	first, _ = contents[0].(map[string]any)
	if first["text"] != "doc:intro" {
		t.Fatalf("doc content = %#v, want doc:intro", first["text"])
	}

	// Bob has neither resource.
	status, body = mcpPost(t, ts, `{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"user://alice/profile"}}`, "Bearer bob-key")
	if status != http.StatusOK {
		t.Fatalf("resources/read as bob: %d", status)
	}
	if obj, ok := body["error"].(map[string]any); !ok || !strings.Contains(obj["message"].(string), "not found") {
		t.Fatalf("bob reading alice's resource = %#v, want not found", body)
	}
}

func TestRequestPrompt(t *testing.T) {
	ts := writeRequestMCPServer(t)
	auth := "Bearer alice-key"

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`, auth)
	if status != http.StatusOK {
		t.Fatalf("prompts/list: %d", status)
	}
	result, _ := body["result"].(map[string]any)
	prompts, _ := result["prompts"].([]any)
	found := false
	for _, pl := range prompts {
		if pr, ok := pl.(map[string]any); ok && pr["name"] == "summarise" {
			found = true
		}
	}
	if !found {
		t.Fatalf("prompts/list = %#v, want summarise", result)
	}

	status, body = mcpPost(t, ts, `{"jsonrpc":"2.0","id":2,"method":"prompts/get","params":{"name":"summarise","arguments":{"topic":"kv stores"}}}`, auth)
	if status != http.StatusOK {
		t.Fatalf("prompts/get: %d %#v", status, body)
	}
	result, _ = body["result"].(map[string]any)
	messages, _ := result["messages"].([]any)
	if len(messages) != 2 {
		t.Fatalf("prompt messages = %#v, want 2", result)
	}
	first, _ := messages[0].(map[string]any)
	if first["role"] != "user" || !strings.Contains(first["content"].(map[string]any)["text"].(string), "kv stores") {
		t.Fatalf("first message = %#v, want user message mentioning the topic", first)
	}

	// Bob does not have the prompt.
	status, body = mcpPost(t, ts, `{"jsonrpc":"2.0","id":3,"method":"prompts/get","params":{"name":"summarise","arguments":{"topic":"x"}}}`, "Bearer bob-key")
	if status != http.StatusOK {
		t.Fatalf("prompts/get as bob: %d", status)
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Fatalf("bob getting alice's prompt = %#v, want an error", body)
	}
}

// A malformed registration is a build error: the request fails with a 500
// rather than serving a different tool set than the script asked for.
func TestRequestRegistrationErrorReturns500(t *testing.T) {
	libDir := t.TempDir()
	toolsDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    mcp.register_request_tool("broken", handler="m.f",
        params={"x": {"type": "not-a-type"}})
    return None
`))
	writeFile(t, toolsDir+"/greet.toml", []byte("description = \"Greet\"\n"))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	status, _ := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "Bearer anything")
	if status != http.StatusInternalServerError {
		t.Fatalf("expected 500 from bad registration, got %d", status)
	}
}

// register_request_* outside a served request is an error, pointing at the
// stdio alternative.
func TestRequestRegistrationOutsideRequestErrors(t *testing.T) {
	p := scriptling.New()
	extlibs.RegisterRuntimeLibraryAll(p, nil)

	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp
mcp.register_request_tool("x", handler="m.f")
`)
	if err == nil || !strings.Contains(err.Error(), "only callable while serving a request over HTTP") {
		t.Fatalf("err = %v, want the outside-a-request error", err)
	}
}

// ── transport() ──────────────────────────────────────────────────────────────

// Over HTTP every handler answers "http", whatever the process-wide mode.
func TestTransportReportsHTTPOverServer(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/rpcmod.py", []byte(`
import scriptling.runtime as runtime

def mode(params):
    return {"transport": runtime.jsonrpc.transport()}
`))
	writeFile(t, libDir+"/toolmod.py", []byte("def transport():\n    return 1\n"))

	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.jsonrpc.method("mode", "rpcmod.mode")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}, JSONRPC: true})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"mode","params":{},"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Result struct {
			Transport string `json:"transport"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Result.Transport != "http" {
		t.Fatalf("transport() = %q, want http", body.Result.Transport)
	}
}

// Outside a request the process-wide mode answers: "stdio" in the stdio
// serving modes, None when not being served at all.
func TestTransportReportsProcessMode(t *testing.T) {
	p := scriptling.New()
	extlibs.RegisterRuntimeLibraryAll(p, nil)

	eval := func(code string) string {
		res, err := p.Eval(code)
		if err != nil {
			t.Fatalf("eval %q: %v", code, err)
		}
		s, _ := res.(*object.String)
		if s == nil {
			return "<null>"
		}
		return s.StringValue()
	}

	extlibs.RuntimeState.Lock()
	extlibs.RuntimeState.Transport = "stdio"
	extlibs.RuntimeState.Unlock()
	if got := eval(`
import scriptling.runtime.mcp as mcp
import scriptling.runtime as runtime
mcp.transport() + "/" + runtime.jsonrpc.transport()
`); got != "stdio/stdio" {
		t.Fatalf("stdio mode: got %q", got)
	}

	extlibs.RuntimeState.Lock()
	extlibs.RuntimeState.Transport = ""
	extlibs.RuntimeState.Unlock()
	res, err := p.Eval(`
import scriptling.runtime.mcp as mcp
mcp.transport() == None
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if b, ok := res.(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("unserved: transport() should be None, got %#v", res)
	}
}
