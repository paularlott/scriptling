package mcp_test

import (
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
