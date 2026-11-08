package server

// Tests for the MCP server features exposed to scripts: arguments converted
// to their declared types, completion/complete for prompt arguments and
// resource template variables (decorated, TOML and request-scoped), progress
// notifications from a tool, and notifications/resources/updated to a
// subscribed client. Each runs through a real server built by NewServer.

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mcp_lib "github.com/paularlott/mcp"
)

func writeFeaturesServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	toolsDir := t.TempDir()
	libDir := t.TempDir()
	promptsDir := t.TempDir()
	resourcesDir := t.TempDir()

	writeFile(t, toolsDir+"/features.py", []byte(`
import scriptling.runtime.mcp as mcp
import scriptling.mcp.tool as tool

def complete_language(argument, value, context):
    return [l for l in ["go", "perl", "python", "rust"] if l.startswith(value)]

def complete_framework(argument, value, context):
    if context.get("language") == "python":
        return ["django", "flask"]
    return [argument + ":" + value]

def broken(argument, value, context):
    raise ValueError("no suggestions today")

def not_a_list(argument, value, context):
    return "go"

@mcp.prompt(description="Review code", complete={
    "language": complete_language,
    "framework": "complete_framework",
    "style": broken,
    "level": not_a_list,
})
def review(language, framework="", style="", level="", notes=""):
    return "Review " + language

@mcp.resource("docs://{section}/{page}", template=True, complete={"page": "complib.pages"})
def doc(section, page):
    return section + "/" + page

@mcp.tool("Typed arguments", params={
    "n": {"type": "int", "description": "An integer"},
    "ratio": {"type": "float", "description": "A number"},
    "ids": {"type": "array:int", "description": "Integers"},
    "weights": {"type": "array:float", "description": "Numbers"},
})
def typed(n, ratio=0.5, ids=None, weights=None):
    return repr([n, ratio, ids, weights])

@mcp.tool("A long job")
def long_job():
    tool.progress(1, total=2, message="half way")
    tool.progress(2, total=2)
    return "done"

@mcp.tool("Save a note", params={"id": "Note ID"})
def save_note(id):
    return str(mcp.notify_resource_updated("notes://" + id))
`))

	// Legacy .toml + .py tool with typed parameters.
	writeFile(t, toolsDir+"/legacy_typed.toml", []byte(`description = "Legacy typed"
[[parameters]]
name = "n"
type = "int"
description = "n"
required = true
[[parameters]]
name = "weights"
type = "array:float"
description = "w"
`))
	writeFile(t, toolsDir+"/legacy_typed.py", []byte(`import scriptling.mcp.tool as tool
tool.return_string(repr([tool.get_int("n"), tool.get_list("weights")]))
`))

	writeFile(t, libDir+"/complib.py", []byte(`
def pages(argument, value, context):
    return [context.get("section", "?") + "-" + p for p in ["intro", "install"] if p.startswith(value)]

def langs(argument, value, context):
    return [l for l in ["en", "fr", "de"] if l.startswith(value)]

def topics(argument, value, context):
    return ["alpha", "beta"]
`))
	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    mcp.register_request_tool("req_typed", handler="reqmod.typed", description="Typed request tool",
        params={
            "n": {"type": "int", "description": "n", "required": True},
            "ratio": {"type": "float", "description": "r"},
            "ids": {"type": "array:int", "description": "ids"},
        })
    mcp.register_request_prompt("req_prompt", handler="reqmod.prompt", description="Request prompt",
        arguments=[{"name": "lang"}, {"name": "other"}], complete={"lang": "complib.langs"})
    mcp.register_request_resource("req://{topic}", handler="reqmod.res", name="Request resource",
        template=True, complete={"topic": "complib.topics"})
    return None
`))
	writeFile(t, libDir+"/reqmod.py", []byte(`
def typed(n, ratio=0.5, ids=None):
    return repr([n, ratio, ids])

def prompt(lang="", other=""):
    return "lang " + lang

def res(topic, __uri=""):
    return topic
`))

	// TOML prompt with a completer per argument.
	writeFile(t, promptsDir+"/greet.toml", []byte(`description = "Greet"
[[arguments]]
name = "lang"
description = "Language"
complete = "complib.langs"
`))
	writeFile(t, promptsDir+"/greet.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('hello')\n"))

	// Resource template with a [complete] table in its sidecar.
	writeFile(t, resourcesDir+"/kb/{topic}.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('kb')\n"))
	writeFile(t, resourcesDir+"/kb/_{topic}.toml", []byte("name = \"Knowledge base\"\n[complete]\ntopic = \"complib.topics\"\n"))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile:      script,
		LibDirs:         []string{libDir},
		MCPToolsDir:     toolsDir,
		MCPPromptsDir:   promptsDir,
		MCPResourcesDir: resourcesDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return s, ts
}

func callToolText(t *testing.T, ts *httptest.Server, name, args string) string {
	t.Helper()
	result := mcpCall(t, ts, "tools/call", `{"name":"`+name+`","arguments":`+args+`}`)
	if result["isError"] == true {
		t.Fatalf("%s(%s): isError result %#v", name, args, result)
	}
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("%s(%s): content %#v", name, args, result)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	return text
}

func TestMCPToolArgumentsReachHandlersAsDeclaredTypes(t *testing.T) {
	_, ts := writeFeaturesServer(t)

	cases := []struct{ tool, args, want string }{
		// Decorated: integers are ints, numbers are floats, whatever the JSON looks like.
		{"typed", `{"n": 7, "ratio": 2, "ids": [1, 2], "weights": [1, 2.5]}`, `[7, 2.0, [1, 2], [1.0, 2.5]]`},
		{"typed", `{"n": "7", "ratio": "2"}`, `[7, 2.0, None, None]`},
		// Request-scoped tools used to get 7.0 for an integer.
		{"req_typed", `{"n": 7, "ratio": 3, "ids": [4, 5]}`, `[7, 3.0, [4, 5]]`},
		{"req_typed", `{"n": "7"}`, `[7, 0.5, None]`},
		// Legacy .toml + .py tools.
		{"legacy_typed", `{"n": 7, "weights": [1, 2]}`, `[7, [1.0, 2.0]]`},
	}
	for _, c := range cases {
		if got := callToolText(t, ts, c.tool, c.args); got != c.want {
			t.Errorf("%s(%s) = %s, want %s", c.tool, c.args, got, c.want)
		}
	}
}

func completion(t *testing.T, ts *httptest.Server, ref, argument, value, ctxArgs string) (map[string]any, map[string]any) {
	t.Helper()
	params := `{"ref":` + ref + `,"argument":{"name":"` + argument + `","value":"` + value + `"}`
	if ctxArgs != "" {
		params += `,"context":{"arguments":` + ctxArgs + `}`
	}
	params += `}`
	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"completion/complete","params":`+params+`}`, "")
	if status != http.StatusOK {
		t.Fatalf("completion/complete %s: status %d %#v", params, status, body)
	}
	result, _ := body["result"].(map[string]any)
	errObj, _ := body["error"].(map[string]any)
	if result == nil && errObj == nil {
		t.Fatalf("completion/complete %s: %#v", params, body)
	}
	if result != nil {
		c, _ := result["completion"].(map[string]any)
		return c, nil
	}
	return nil, errObj
}

func completionValues(t *testing.T, c map[string]any) []string {
	t.Helper()
	raw, _ := c["values"].([]any)
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

func TestMCPCompletion(t *testing.T) {
	_, ts := writeFeaturesServer(t)

	// The completions capability is declared.
	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`, "")
	if status != http.StatusOK {
		t.Fatalf("initialize: %d %#v", status, body)
	}
	caps, _ := body["result"].(map[string]any)["capabilities"].(map[string]any)
	if _, ok := caps["completions"]; !ok {
		t.Errorf("capabilities = %#v, want completions", caps)
	}

	prompt := func(name string) string { return `{"type":"ref/prompt","name":"` + name + `"}` }
	resource := func(uri string) string { return `{"type":"ref/resource","uri":"` + uri + `"}` }

	cases := []struct {
		label, ref, argument, value, ctxArgs string
		want                                 string
	}{
		{"decorated prompt, function", prompt("review"), "language", "p", "", "perl,python"},
		{"decorated prompt, function name, context", prompt("review"), "framework", "", `{"language":"python"}`, "django,flask"},
		{"decorated prompt, argument and value reach completer", prompt("review"), "framework", "x", "", "framework:x"},
		{"decorated prompt, no completer", prompt("review"), "notes", "a", "", ""},
		{"decorated template, module ref, context", resource("docs://{section}/{page}"), "page", "in", `{"section":"guide"}`, "guide-intro,guide-install"},
		{"decorated template, no completer", resource("docs://{section}/{page}"), "section", "", "", ""},
		{"TOML prompt", prompt("greet"), "lang", "f", "", "fr"},
		{"TOML resource template", resource("kb://{topic}"), "topic", "", "", "alpha,beta"},
		{"request prompt", prompt("req_prompt"), "lang", "d", "", "de"},
		{"request prompt, no completer", prompt("req_prompt"), "other", "", "", ""},
		{"request resource template", resource("req://{topic}"), "topic", "", "", "alpha,beta"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			comp, errObj := completion(t, ts, c.ref, c.argument, c.value, c.ctxArgs)
			if errObj != nil {
				t.Fatalf("error %#v", errObj)
			}
			if got := strings.Join(completionValues(t, comp), ","); got != c.want {
				t.Errorf("values = %q, want %q", got, c.want)
			}
		})
	}

	// A completer that raises, or returns something that isn't a list of
	// strings, is a JSON-RPC error for that request; an unknown argument is
	// invalid params.
	for _, c := range []struct{ label, ref, argument string }{
		{"raises", prompt("review"), "style"},
		{"not a list", prompt("review"), "level"},
		{"unknown argument", prompt("review"), "nope"},
		{"unknown request argument", prompt("req_prompt"), "nope"},
	} {
		t.Run("error: "+c.label, func(t *testing.T) {
			if comp, errObj := completion(t, ts, c.ref, c.argument, "", ""); errObj == nil {
				t.Fatalf("want a JSON-RPC error, got %#v", comp)
			}
		})
	}

	// The server keeps serving afterwards.
	if comp, errObj := completion(t, ts, prompt("review"), "language", "r", ""); errObj != nil || strings.Join(completionValues(t, comp), ",") != "rust" {
		t.Fatalf("after errors: %#v %#v", comp, errObj)
	}
}

// stdioSession drives the server's MCP stdio transport with raw JSON-RPC lines.
type stdioSession struct {
	t     *testing.T
	in    io.WriteCloser
	lines chan map[string]any
}

func newStdioSession(t *testing.T, server *mcp_lib.Server) *stdioSession {
	t.Helper()
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.ServeStream(ctx, serverReader, serverWriter)
	}()
	lines := make(chan map[string]any, 64)
	go func() {
		sc := bufio.NewScanner(clientReader)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var m map[string]any
			if json.Unmarshal(sc.Bytes(), &m) == nil {
				lines <- m
			}
		}
		close(lines)
	}()
	t.Cleanup(func() {
		cancel()
		clientWriter.Close()
		serverReader.Close()
		<-done
		serverWriter.Close()
		clientReader.Close()
	})
	s := &stdioSession{t: t, in: clientWriter, lines: lines}
	s.request(1, "initialize", `{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}`)
	s.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	return s
}

func (s *stdioSession) send(line string) {
	s.t.Helper()
	if _, err := io.WriteString(s.in, line+"\n"); err != nil {
		s.t.Fatalf("write: %v", err)
	}
}

// request sends a request and returns every message received up to and
// including its response.
func (s *stdioSession) request(id int, method, params string) []map[string]any {
	s.t.Helper()
	s.send(`{"jsonrpc":"2.0","id":` + itoa(id) + `,"method":"` + method + `","params":` + params + `}`)
	var got []map[string]any
	timeout := time.After(10 * time.Second)
	for {
		select {
		case m, ok := <-s.lines:
			if !ok {
				s.t.Fatalf("%s: stream closed; got %#v", method, got)
			}
			got = append(got, m)
			if idv, ok := m["id"].(float64); ok && int(idv) == id {
				if errObj, ok := m["error"]; ok {
					s.t.Fatalf("%s: error %#v", method, errObj)
				}
				return got
			}
		case <-timeout:
			s.t.Fatalf("%s: no response; got %#v", method, got)
		}
	}
}

// next returns the next message, or nil after wait.
func (s *stdioSession) next(wait time.Duration) map[string]any {
	select {
	case m := <-s.lines:
		return m
	case <-time.After(wait):
		return nil
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func notificationsOf(msgs []map[string]any, method string) []map[string]any {
	var out []map[string]any
	for _, m := range msgs {
		if m["method"] == method {
			params, _ := m["params"].(map[string]any)
			out = append(out, params)
		}
	}
	return out
}

func TestMCPToolProgressNotifications(t *testing.T) {
	s, _ := writeFeaturesServer(t)
	session := newStdioSession(t, s.mcpHandler.server.Load())

	msgs := session.request(2, "tools/call", `{"name":"long_job","arguments":{},"_meta":{"progressToken":"job-1"}}`)
	progress := notificationsOf(msgs, "notifications/progress")
	if len(progress) != 2 {
		t.Fatalf("progress notifications = %#v, want 2", progress)
	}
	first, last := progress[0], progress[1]
	if first["progressToken"] != "job-1" || first["progress"] != 1.0 || first["total"] != 2.0 || first["message"] != "half way" {
		t.Errorf("first = %#v", first)
	}
	if last["progress"] != 2.0 || last["total"] != 2.0 {
		t.Errorf("last = %#v", last)
	}
	resp := msgs[len(msgs)-1]
	content, _ := resp["result"].(map[string]any)["content"].([]any)
	if len(content) != 1 || content[0].(map[string]any)["text"] != "done" {
		t.Errorf("response = %#v", resp)
	}

	// Without a progressToken, progress() sends nothing and the tool still works.
	msgs = session.request(3, "tools/call", `{"name":"long_job","arguments":{}}`)
	if p := notificationsOf(msgs, "notifications/progress"); len(p) != 0 {
		t.Errorf("progress without a token = %#v", p)
	}
}

func TestMCPNotifyResourceUpdated(t *testing.T) {
	s, _ := writeFeaturesServer(t)
	session := newStdioSession(t, s.mcpHandler.server.Load())

	session.request(2, "resources/subscribe", `{"uri":"notes://1"}`)

	// A change to a resource nobody subscribed to sends nothing; the tool
	// reports that a server was there to notify.
	msgs := session.request(3, "tools/call", `{"name":"save_note","arguments":{"id":"2"}}`)
	if u := notificationsOf(msgs, "notifications/resources/updated"); len(u) != 0 {
		t.Errorf("unsubscribed update = %#v", u)
	}
	resp := msgs[len(msgs)-1]
	content, _ := resp["result"].(map[string]any)["content"].([]any)
	if len(content) != 1 || content[0].(map[string]any)["text"] != "True" {
		t.Errorf("save_note response = %#v", resp)
	}

	// A change to the subscribed one reaches the client.
	msgs = session.request(4, "tools/call", `{"name":"save_note","arguments":{"id":"1"}}`)
	updates := notificationsOf(msgs, "notifications/resources/updated")
	for len(updates) == 0 {
		m := session.next(2 * time.Second)
		if m == nil {
			t.Fatal("no notifications/resources/updated for notes://1")
		}
		updates = notificationsOf([]map[string]any{m}, "notifications/resources/updated")
	}
	if updates[0]["uri"] != "notes://1" {
		t.Errorf("update = %#v", updates[0])
	}
}
