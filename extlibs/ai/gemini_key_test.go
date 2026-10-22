package ai

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpai "github.com/paularlott/mcp/ai"
	"github.com/paularlott/mcp/ai/openai"
	"github.com/paularlott/scriptling"
)

const geminiSecretKey = "GEMINI-SECRET-KEY-123"

// A host hands a script a Gemini client built with its own key (WrapClient);
// the script cannot read the key from the client, but the Gemini provider
// put it in the models and embedding URLs, so any failed request quoted it in
// the error the script received (CWE-209, strix 4250 vuln-0005). mcp v0.33.1
// sends it in the x-goog-api-key header instead.
func TestGeminiKeyNotLeakedToScript(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":"Echo: %s"}`, r.URL.RequestURI())
	}))
	defer echo.Close()

	for name, base := range map[string]string{
		"connection refused": "http://127.0.0.1:1",
		"rejected with echo": echo.URL,
	} {
		client, err := mcpai.NewClient(mcpai.Config{Provider: mcpai.ProviderGemini, Config: openai.Config{
			APIKey: geminiSecretKey, BaseURL: base, MaxRetries: -1,
		}})
		if err != nil {
			t.Fatal(err)
		}
		p := scriptling.New()
		Register(p)
		if err := p.SetObjectVar("client", WrapClient(client)); err != nil {
			t.Fatal(err)
		}
		result, err := p.Eval(`
errs = []
try:
    client.models()
except Exception as e:
    errs.append(str(e))
try:
    client.embedding("bad model name", "hello")
except Exception as e:
    errs.append(str(e))
"\n".join(errs)`)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := result.Inspect()
		if strings.Count(got, "failed to") != 2 {
			t.Fatalf("%s: expected both calls to fail, got %s", name, got)
		}
		if strings.Contains(got, geminiSecretKey) {
			t.Fatalf("%s: the script recovered the operator's API key:\n%s", name, got)
		}
	}
}
