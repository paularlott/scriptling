package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcplib "github.com/paularlott/mcp"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/ai"
	scriptlingmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/stdlib"
)

// TestAgentMCPServersEndToEnd runs the full mcp_servers bridge against a real
// MCP server over HTTP: the agent registers the server's tools under their
// namespaced names with the server's own schema, lists its skills in the
// system prompt with namespace-qualified URIs, and a scripted LLM walks the
// whole loop (get_skill, then the remote tool) proving both routes work.
func TestAgentMCPServersEndToEnd(t *testing.T) {
	var toolCalls atomic.Int32

	server := mcplib.NewServer("shop-server", "1.0")
	server.RegisterTool(
		mcplib.NewTool("price_lookup", "Look up a price",
			mcplib.String("sku", "Product SKU", mcplib.Required()),
		),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			toolCalls.Add(1)
			sku, err := req.String("sku")
			if err != nil {
				return nil, mcplib.NewToolErrorInvalidParams("sku is required")
			}
			return mcplib.NewToolResponseText("price of "+sku+": 4.99"), nil
		},
	)
	if err := server.RegisterSkill(mcplib.NewSkill("product-copy").
		Description("How to write product copy").
		File("SKILL.md", []byte("---\nname: product-copy\ndescription: How to write product copy\n---\nAlways quote USD prices."))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

# Scripted LLM: turn 1 fetches the skill, turn 2 calls the remote tool,
# turn 3 reports both results.
class MockClient:
    def __init__(self):
        self.turn = 0
        self.seen_system = ""
    def completion(self, model, messages, **kwargs):
        self.turn += 1
        for m in messages:
            if isinstance(m, dict) and m.get("role") == "system":
                self.seen_system = m["content"]
        if self.turn == 1:
            content = ""
            calls = [{"id": "c1", "type": "function", "function": {"name": "get_skill", "arguments": "{\"uri\": \"skill://shop/product-copy/SKILL.md\"}"}}]
        elif self.turn == 2:
            content = ""
            calls = [{"id": "c2", "type": "function", "function": {"name": "shop__price_lookup", "arguments": "{\"sku\": \"mug\"}"}}]
        else:
            content = "done"
            calls = []
        return {"choices": [{"message": {"role": "assistant", "content": content, "tool_calls": calls}}]}

shop = mcp.Client("` + ts.URL + `", namespace="shop")
client = MockClient()
bot = agent.Agent(client, mcp_servers=[shop], system_prompt="You are a shop assistant.")

# Wiring: namespaced tool with the server's schema, skills section in the prompt.
names = [t["function"]["name"] for t in bot.tool_schemas]
assert names == ["shop__price_lookup", "get_skill"], "schemas: " + str(names)
sku = bot.tool_schemas[0]["function"]["parameters"]["properties"]["sku"]
assert sku["description"] == "Product SKU", "server schema must be verbatim: " + str(sku)
assert bot.tool_schemas[0]["function"]["parameters"]["required"] == ["sku"]

assert "- shop/product-copy: How to write product copy (skill://shop/product-copy/SKILL.md)" in bot.system_prompt
assert "## Skills" in bot.system_prompt

# Loop: get_skill routed to the server, remote tool executed by the server.
resp = bot.trigger("price of a mug", max_iterations=5)
assert resp.content == "done"
assert "## Skills" in client.seen_system, "skills section must reach the model"

tool_results = []
for m in bot.messages:
    if isinstance(m, dict) and m.get("role") == "tool":
        tool_results.append(m.get("content", ""))
assert any("Always quote USD prices." in r for r in tool_results), "skill body not fetched: " + str(tool_results)
assert any("price of mug: 4.99" in r for r in tool_results), "remote tool result missing: " + str(tool_results)

# Unhappy path: an unknown skill namespace comes back as a tool-result error
# string, not a crashed loop.
handler = bot.tools.get_handler("get_skill")
bad = handler({"uri": "skill://nope/whatever/SKILL.md"})
assert isinstance(bad, str) and "unknown skill URI" in str(bad), "unknown namespace: " + str(bad)

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
	if got := toolCalls.Load(); got != 1 {
		t.Errorf("server-side tool calls = %d, want 1", got)
	}
}

// TestAgentMCPServersRequireNamespace verifies the two namespace rules fail
// loudly at construction: a server without a namespace, and two servers
// sharing one.
func TestAgentMCPServersRequireNamespace(t *testing.T) {
	server := mcplib.NewServer("skillless-server", "1.0")
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

named = mcp.Client("` + ts.URL + `", namespace="dup")
unnamed = mcp.Client("` + ts.URL + `")

err1 = None
try:
    agent.Agent(MockClient(), mcp_servers=[unnamed])
except ValueError as e:
    err1 = str(e)
assert err1 is not None and "namespace" in err1, "unnamed server must error: " + str(err1)

err2 = None
try:
    agent.Agent(MockClient(), mcp_servers=[named, mcp.Client("` + ts.URL + `", namespace="dup")])
except ValueError as e:
    err2 = str(e)
assert err2 is not None and "duplicate" in err2, "duplicate namespace must error: " + str(err2)

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentMCPServersNoSkillsServer keeps the whole bridge working when a
// server has no skills capability at all: tools register, no skills section
// is added, no get_skill tool appears.
func TestAgentMCPServersNoSkillsServer(t *testing.T) {
	server := mcplib.NewServer("tool-only-server", "1.0")
	server.RegisterTool(
		mcplib.NewTool("ping", "Ping"),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			return mcplib.NewToolResponseText("pong"), nil
		},
	)
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

plain = mcp.Client("` + ts.URL + `", namespace="plain")
bot = agent.Agent(MockClient(), mcp_servers=[plain], system_prompt="base")
names = [t["function"]["name"] for t in bot.tool_schemas]
assert names == ["plain__ping"], "schemas: " + str(names)
assert "## Skills" not in bot.system_prompt
assert "get_skill" not in bot.system_prompt

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentMCPServersWithMemory verifies the memory augmentation appends to
// (rather than replaces) a system prompt that already carries the MCP skills
// section: both must coexist.
func TestAgentMCPServersWithMemory(t *testing.T) {
	server := mcplib.NewServer("skills-server", "1.0")
	if err := server.RegisterSkill(mcplib.NewSkill("product-copy").
		Description("How to write product copy").
		File("SKILL.md", []byte("---\nname: product-copy\ndescription: d\n---\nBody"))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

class Mem:
    def remember(self, content, type="note", importance=0.5):
        return "stored"
    def recall(self, query="", limit=10, type=""):
        return []
    def forget(self, id):
        return "forgotten"

shop = mcp.Client("` + ts.URL + `", namespace="shop")
bot = agent.Agent(
    MockClient(),
    mcp_servers=[shop],
    system_prompt="base prompt",
    memory=Mem(),
)

assert "base prompt" in bot.system_prompt
assert "## Skills" in bot.system_prompt, "skills section lost: " + bot.system_prompt
assert "- shop/product-copy:" in bot.system_prompt
assert "## Memory" in bot.system_prompt, "memory section lost"
names = [t["function"]["name"] for t in bot.tool_schemas]
assert "shop__" not in str(None) and "get_skill" in names, "get_skill missing: " + str(names)
assert "memory_remember" in names and "memory_recall" in names and "memory_forget" in names, "memory tools missing: " + str(names)

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentMCPServersMixedWithLocalTools covers the full matrix cell of an
// agent with its own local tools AND an MCP server: local tool, remote tool
// and get_skill all dispatch in the same loop.
func TestAgentMCPServersMixedWithLocalTools(t *testing.T) {
	server := mcplib.NewServer("mixed-server", "1.0")
	server.RegisterTool(
		mcplib.NewTool("remote_echo", "Echo remotely",
			mcplib.String("text", "Text to echo", mcplib.Required()),
		),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			text, _ := req.String("text")
			return mcplib.NewToolResponseText("remote: " + text), nil
		},
	)
	if err := server.RegisterSkill(mcplib.NewSkill("mixed-skill").
		Description("A mixed skill").
		File("SKILL.md", []byte("---\nname: mixed-skill\ndescription: A mixed skill\n---\nMixed skill body."))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

calls = []

class MockClient:
    def __init__(self):
        self.turn = 0
    def completion(self, model, messages, **kwargs):
        self.turn += 1
        if self.turn == 1:
            content, tc = "", [{"id": "c1", "type": "function", "function": {"name": "local_shout", "arguments": "{\"text\": \"hi\"}"}}]
        elif self.turn == 2:
            content, tc = "", [{"id": "c2", "type": "function", "function": {"name": "mixed__remote_echo", "arguments": "{\"text\": \"yo\"}"}}]
        elif self.turn == 3:
            content, tc = "", [{"id": "c3", "type": "function", "function": {"name": "get_skill", "arguments": "{\"uri\": \"skill://mixed/mixed-skill/SKILL.md\"}"}}]
        else:
            content, tc = "all done", []
        return {"choices": [{"message": {"role": "assistant", "content": content, "tool_calls": tc}}]}

def local_shout(args):
    return "local: " + args["text"]

tools = ai.ToolRegistry()
tools.add("local_shout", "Shout locally", {"text": "string"}, local_shout)

mixed = mcp.Client("` + ts.URL + `", namespace="mixed")
bot = agent.Agent(MockClient(), tools=tools, mcp_servers=[mixed])

names = [t["function"]["name"] for t in bot.tool_schemas]
assert names == ["local_shout", "mixed__remote_echo", "get_skill"], "schemas: " + str(names)

resp = bot.trigger("run everything", max_iterations=5)
assert resp.content == "all done"

results = []
for m in bot.messages:
    if isinstance(m, dict) and m.get("role") == "tool":
        results.append(m.get("content", ""))
assert "local: hi" in results, "local tool result missing: " + str(results)
assert "remote: yo" in results, "remote tool result missing: " + str(results)
assert any("Mixed skill body." in r for r in results), "skill body missing: " + str(results)

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentMCPServersToolsEmptySkillsOnly covers an MCP-only agent (no local
// tools) against a server exposing a skill but no tools at all: the registry
// holds just get_skill and the loop still runs.
func TestAgentMCPServersToolsEmptySkillsOnly(t *testing.T) {
	server := mcplib.NewServer("skill-only-server", "1.0")
	if err := server.RegisterSkill(mcplib.NewSkill("lone-skill").
		Description("The only thing here").
		File("SKILL.md", []byte("---\nname: lone-skill\ndescription: The only thing here\n---\nOnly skill body."))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

class MockClient:
    def __init__(self):
        self.turn = 0
    def completion(self, model, messages, **kwargs):
        self.turn += 1
        if self.turn == 1:
            content, tc = "", [{"id": "c1", "type": "function", "function": {"name": "get_skill", "arguments": "{\"uri\": \"skill://only/lone-skill/SKILL.md\"}"}}]
        else:
            content, tc = "fetched", []
        return {"choices": [{"message": {"role": "assistant", "content": content, "tool_calls": tc}}]}

only = mcp.Client("` + ts.URL + `", namespace="only")
bot = agent.Agent(MockClient(), mcp_servers=[only])

names = [t["function"]["name"] for t in bot.tool_schemas]
assert names == ["get_skill"], "schemas must be exactly [get_skill]: " + str(names)
assert bot.tools is not None, "registry must exist for get_skill"

resp = bot.trigger("read the skill", max_iterations=3)
results = [m.get("content", "") for m in bot.messages if isinstance(m, dict) and m.get("role") == "tool"]
assert any("Only skill body." in r for r in results), "skill body missing: " + str(results)

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentMCPServersTwoServersSkills lists skills from two servers in one
// prompt and routes each namespaced URI back to its owning server, including
// a supporting-file read.
func TestAgentMCPServersTwoServersSkills(t *testing.T) {
	mkServer := func(name string) *httptest.Server {
		s := mcplib.NewServer(name, "1.0")
		if err := s.RegisterSkill(mcplib.NewSkill("guide").
			Description("Guide of " + name).
			File("SKILL.md", []byte("---\nname: guide\ndescription: Guide of "+name+"\n---\nBody of "+name+".")).
			File("notes.md", []byte("notes of "+name))); err != nil {
			t.Fatalf("register skill: %v", err)
		}
		return httptest.NewServer(http.HandlerFunc(s.HandleRequest))
	}
	ts1, ts2 := mkServer("alpha"), mkServer("beta")
	defer ts1.Close()
	defer ts2.Close()

	script := `
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

alpha = mcp.Client("` + ts1.URL + `", namespace="alpha")
beta = mcp.Client("` + ts2.URL + `", namespace="beta")
bot = agent.Agent(Keep(), mcp_servers=[alpha, beta])

assert "- alpha/guide: Guide of alpha (skill://alpha/guide/SKILL.md)" in bot.system_prompt, bot.system_prompt
assert "- beta/guide: Guide of beta (skill://beta/guide/SKILL.md)" in bot.system_prompt, bot.system_prompt

handler = bot.tools.get_handler("get_skill")
assert "Body of alpha." in handler({"uri": "skill://alpha/guide/SKILL.md"})
assert "notes of beta" in handler({"uri": "skill://beta/guide/notes.md"})

"OK"
`

	// Keep is a minimal LLM stub; the assertions do not need a loop.
	script = "class Keep:\n    def completion(self, model, messages, **kwargs):\n        return {\"choices\": [{\"message\": {\"role\": \"assistant\", \"content\": \"ok\"}}]}\n" + script

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentGetSkillServerDown verifies a dead server mid-session degrades to
// an error tool-result string rather than crashing the loop.
func TestAgentGetSkillServerDown(t *testing.T) {
	server := mcplib.NewServer("dying-server", "1.0")
	if err := server.RegisterSkill(mcplib.NewSkill("doomed").
		Description("Will die").
		File("SKILL.md", []byte("---\nname: doomed\ndescription: Will die\n---\nBody."))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))

	script := `
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

dying = mcp.Client("` + ts.URL + `", namespace="dying")
bot = agent.Agent(MockClient(), mcp_servers=[dying])
handler = bot.tools.get_handler("get_skill")
` + "TS_CLOSE_MARKER" + `
result = handler({"uri": "skill://dying/doomed/SKILL.md"})
assert isinstance(result, str), "dead server must return a string, got: " + str(type(result))
assert "Error reading skill" in result, "dead server message: " + str(result)

"OK"
`
	// Close the server after the agent is constructed but before the handler
	// runs: split the script at the marker and close in between.
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	before, after, found := strings.Cut(script, "TS_CLOSE_MARKER\n")
	if !found {
		t.Fatal("marker missing")
	}
	if _, err := p.Eval(before + "None\n"); err != nil {
		t.Fatalf("setup eval failed: %v", err)
	}
	ts.Close()
	result, err := p.Eval(after)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestAgentMCPServersKeepsUserGetSkill pins the rule that a caller-supplied
// get_skill tool wins: the agent does not register its own, and the skills
// section points at the caller's tool.
func TestAgentMCPServersKeepsUserGetSkill(t *testing.T) {
	server := mcplib.NewServer("keep-server", "1.0")
	if err := server.RegisterSkill(mcplib.NewSkill("kept-skill").
		Description("Kept").
		File("SKILL.md", []byte("---\nname: kept-skill\ndescription: Kept\n---\nKept body."))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent
import scriptling.mcp as mcp

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

def my_get_skill(args):
    return "custom handler saw " + args["uri"]

tools = ai.ToolRegistry()
tools.add("get_skill", "Custom skill fetcher", {"uri": "string"}, my_get_skill)

keep = mcp.Client("` + ts.URL + `", namespace="keep")
bot = agent.Agent(MockClient(), tools=tools, mcp_servers=[keep])

names = [t["function"]["name"] for t in bot.tool_schemas]
assert names == ["get_skill"], "only the caller's get_skill must exist: " + str(names)
assert "## Skills" in bot.system_prompt, "skills section must still be injected"

result = bot.tools.get_handler("get_skill")({"uri": "skill://keep/kept-skill/SKILL.md"})
assert result == "custom handler saw skill://keep/kept-skill/SKILL.md", "custom handler result: " + str(result)

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	scriptlingmcp.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}
