package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcplib "github.com/paularlott/mcp"
)

// client.complete() asks a server for argument completions, by prompt name,
// resource template URI or the spec's ref dict, passing context arguments.
func TestClientComplete(t *testing.T) {
	server := mcplib.NewServer("complete-server", "1.0")
	server.RegisterPrompt(
		mcplib.NewPrompt("review", "Review code").
			Argument("language", "Language", true).
			Complete(func(ctx context.Context, req *mcplib.CompletionRequest) ([]string, error) {
				return mcplib.FilterPrefix([]string{"go", "perl", "python"}, req.Value()), nil
			}).
			Argument("framework", "Framework", false).
			Complete(func(ctx context.Context, req *mcplib.CompletionRequest) ([]string, error) {
				lang, _ := req.ContextArg("language")
				return []string{lang + "-web"}, nil
			}),
		func(ctx context.Context, req *mcplib.PromptRequest) (*mcplib.PromptResponse, error) {
			return mcplib.NewPromptResponseText("review"), nil
		})
	server.RegisterResourceTemplate(
		mcplib.NewResourceTemplate("docs://{page}", "Docs", "", "text/plain").
			Complete("page", func(ctx context.Context, req *mcplib.CompletionRequest) ([]string, error) {
				return []string{"intro", "install"}, nil
			}),
		func(ctx context.Context, req *mcplib.ResourceRequest) (*mcplib.ResourceResponse, error) {
			return mcplib.NewResourceResponseText(req.URI(), "doc", "text/plain"), nil
		})
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	p := newMCPScriptling(t)
	result, err := p.Eval(`
import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `")

out = c.complete("review", "language", "p")
assert out["values"] == ["perl", "python"], str(out)
assert out["total"] == 2 and out["hasMore"] == False, str(out)

out = c.complete("review", "framework", context={"language": "go"})
assert out["values"] == ["go-web"], str(out)

out = c.complete({"type": "ref/prompt", "name": "review"}, "language", "g")
assert out["values"] == ["go"], str(out)

out = c.complete("docs://{page}", "page", "")
assert out["values"] == ["intro", "install"], str(out)

raised = ""
try:
    c.complete("review", "nope", "")
except Exception as e:
    raised = str(e)
assert raised != "", "unknown argument should raise"

"OK"
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if got, _ := result.AsString(); got != "OK" {
		t.Fatalf("got %q", got)
	}
}
