package server

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
	rthttp "github.com/paularlott/scriptling/extlibs/runtime/http"
)

// End to end: a setup script registers static() through http.Register with
// allowed paths, the server builds the route from the registration, and it
// serves files from the allowed directory while a symlink out stays hidden.
// A directory outside the allowed paths is never registered at all.
func TestStaticRegisteredThroughScriptServesAllowedDir(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()
	allowed, served := staticFixture(t)

	p := scriptling.New()
	rthttp.Register(p, []string{allowed})
	if _, err := p.Eval("import scriptling.runtime.http as http\nhttp.static('/files', " + strconv.Quote(served) + ")"); err != nil {
		t.Fatalf("static() inside the allowed paths: %v", err)
	}
	if _, err := p.Eval("import scriptling.runtime.http as http\nhttp.static('/root', '/')"); err == nil {
		t.Fatal("static('/') outside the allowed paths was accepted")
	}

	s := &Server{staticRoutes: make(map[string]staticRoute), handlers: make(map[string]string), wsHandlers: make(map[string]string)}
	runtime.RuntimeState.Lock()
	s.collectRoutes()
	runtime.RuntimeState.Unlock()

	if _, ok := s.staticRoutes["/root"]; ok {
		t.Fatal("refused static route reached the server")
	}
	route, ok := s.staticRoutes["/files"]
	if !ok {
		t.Fatalf("static route not collected: %+v", s.staticRoutes)
	}
	h := route.handler()
	if code, body := get(t, h, "/hello.txt"); code != http.StatusOK || body != "hello" {
		t.Errorf("GET /hello.txt = %d %q, want 200 hello", code, body)
	}
	if code, _ := get(t, h, "/leak.txt"); code != http.StatusNotFound {
		t.Errorf("GET /leak.txt (symlink out of the allowed paths) = %d, want 404", code)
	}
}
