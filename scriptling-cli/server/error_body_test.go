package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMiddlewareErrorBodyIsValidJSON pins the failure-path contract for
// protocol endpoints: when a middleware handler raises, the 500 body the host
// synthesizes must be valid JSON even though the error message contains
// quotes, backslashes and newlines. Before ErrorJSONBody this was an Sprintf
// splice and clients received malformed JSON.
func TestMiddlewareErrorBodyIsValidJSON(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/boom.py", []byte(`
def check(request):
    raise ValueError('bad "token" in\\nheader C:\\temp')
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("boom.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile: script,
		LibDirs:    []string{libDir},
		JSONRPC:    true,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	// Any protocol endpoint runs the middleware first; its failure produces
	// the synthesized 500.
	resp, err := http.Post(ts.URL+"/json-rpc", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping","params":{}}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}

	var decoded struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("500 body is not valid JSON: %v", err)
	}
	if !strings.Contains(decoded.Error, `bad "token"`) {
		t.Fatalf("error message lost its quotes: %q", decoded.Error)
	}
}
