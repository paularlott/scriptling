package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/paularlott/mcp"
)

// End to end per SEP-2640: a script lists skills, gets an entry by URI,
// and reads content through resources/read.
func TestClientSkillsRoundTrip(t *testing.T) {
	server := mcplib.NewServer("skills-server", "1.0")
	server.RegisterSkill(mcplib.NewSkill("code-review").
		Description("How to review").
		File("SKILL.md", []byte("Read the diff twice.")))
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	p := newMCPScriptling(t)
	result, err := p.Eval(`import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `")
skills = c.skills()
entry = c.get_skill(skills[0].uri)
content = c.read_resource("skill://code-review/SKILL.md")
skills[0].frontmatter["name"] + "|" + str(len(entry.resources)) + "|" + content.text
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	got, _ := result.AsString()
	if !strings.Contains(got, "code-review|") || !strings.Contains(got, "Read the diff twice.") {
		t.Fatalf("got %q", got)
	}
}

// TestClientConsumesNamespacedParallelAndSkillResources covers two
// consumption paths scripts rely on when a server is namespaced: parallel
// tool calls with namespaced names (the prefix must be stripped per call),
// and skills surfacing through the plain resources API alongside
// resources/list.
func TestClientConsumesNamespacedParallelAndSkillResources(t *testing.T) {
	server := mcplib.NewServer("consume-server", "1.0")
	server.RegisterTool(
		mcplib.NewTool("echo", "Echo", mcplib.String("text", "Text", mcplib.Required())),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			text, _ := req.String("text")
			return mcplib.NewToolResponseText("echo: "+text), nil
		},
	)
	if err := server.RegisterSkill(mcplib.NewSkill("consume-skill").
		Description("Consumption skill").
		File("SKILL.md", []byte("---\nname: consume-skill\ndescription: Consumption skill\n---\nConsume body.")).
		File("extra.md", []byte("consume extra"))); err != nil {
		t.Fatalf("register skill: %v", err)
	}
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	p := newMCPScriptling(t)
	result, err := p.Eval(`
import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `", namespace="srv")

# Parallel calls with namespaced names must reach the server's bare tools.
results = c.call_tools_parallel([
    {"name": "srv__echo", "arguments": {"text": "one"}},
    {"name": "srv__echo", "arguments": {"text": "two"}},
])
texts = [str(r.result) for r in results]
assert texts == ["echo: one", "echo: two"], "parallel results: " + str(texts)
assert all(r.error == "" for r in results), "parallel errors"

# Skills surface through the resources API too: both files are listed and
# readable like any other resource.
uris = sorted(r["uri"] for r in c.list_resources())
assert "skill://consume-skill/SKILL.md" in uris, "resources/list missing SKILL.md: " + str(uris)
assert "skill://consume-skill/extra.md" in uris, "resources/list missing extra.md: " + str(uris)

extra = c.read_resource("skill://consume-skill/extra.md")
assert extra.text == "consume extra", "extra content: " + str(extra.text)

"OK"
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if got, _ := result.AsString(); got != "OK" {
		t.Fatalf("got %q", got)
	}
}
