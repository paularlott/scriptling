package ai

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

func serveCounting(t *testing.T, body string, hits *atomic.Int32) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, body)
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln.Addr().String()
}

// The Ollama provider fetched remote image_url values itself with Go's
// default HTTP client, so under a network policy a script could make the host
// read any internal URL and receive it back through the chat endpoint (CWE-918,
// strix 4250 vuln-0002). Ollama only accepts inline images, so mcp v0.33.0
// refuses image URLs instead of fetching them: nothing is requested from the
// image URL or the Ollama server.
func TestOllamaImageURLNotFetched(t *testing.T) {
	var internalHits, chatHits atomic.Int32
	internal := serveCounting(t, "INTERNAL-SECRET", &internalHits)
	ollama := serveCounting(t, `{"model":"m","message":{"role":"assistant","content":"ok"},"done":true}`, &chatHits)
	_, port, _ := net.SplitHostPort(ollama)

	for name, register := range map[string]func(*scriptling.Scriptling){
		"policy": func(p *scriptling.Scriptling) {
			Register(p, &netsecurity.Config{AllowHosts: []string{"localhost"}, AllowLoopback: true})
		},
		"no policy": func(p *scriptling.Scriptling) { Register(p) },
	} {
		p := scriptling.New()
		register(p)
		p.SetVar("ollama_url", "http://localhost:"+port)
		p.SetVar("image_url", "http://"+internal+"/latest/meta-data/token")
		_, err := p.Eval(`
import scriptling.ai as ai
c = ai.Client(ollama_url, provider=ai.OLLAMA)
c.completion("m", [{"role": "user", "content": [
    {"type": "text", "text": "describe"},
    {"type": "image_url", "image_url": {"url": image_url}}]}])
`)
		if err == nil || !strings.Contains(err.Error(), "image_url must be a data: URI") {
			t.Errorf("%s: expected the image URL to be refused, got %v", name, err)
		}
	}
	if internalHits.Load() != 0 || chatHits.Load() != 0 {
		t.Fatalf("requests made: image=%d chat=%d, want none", internalHits.Load(), chatHits.Load())
	}

	// A data: URI image is sent inline as before.
	p := scriptling.New()
	Register(p, &netsecurity.Config{AllowHosts: []string{"localhost"}, AllowLoopback: true})
	p.SetVar("ollama_url", "http://localhost:"+port)
	result, err := p.Eval(`
import scriptling.ai as ai
c = ai.Client(ollama_url, provider=ai.OLLAMA)
r = c.completion("m", [{"role": "user", "content": [
    {"type": "text", "text": "describe"},
    {"type": "image_url", "image_url": {"url": "data:image/png;base64,QUJD"}}]}])
r.choices[0].message.content`)
	if err != nil || result.Inspect() != "ok" || chatHits.Load() != 1 {
		t.Fatalf("data URI image: %v %v (chat requests %d)", result, err, chatHits.Load())
	}
}
