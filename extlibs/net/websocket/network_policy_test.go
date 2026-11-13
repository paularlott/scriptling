package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// TestNetworkWebSocketPolicy: the websocket library dials through the guard.
func TestNetworkWebSocketPolicy(t *testing.T) {
	// A plain HTTP server suffices: the denial must happen at policy check,
	// before any protocol negotiation.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	// IP-literal URL, no opt-in → denied despite the server existing.
	p := scriptling.New()
	Register(p, &netsecurity.Config{AllowLoopback: true})
	out, ok := pEvalErr(t, p, `import scriptling.net.websocket as websocket
websocket.connect("`+strings.Replace(srv.URL, "http", "ws", 1)+`")`)
	if ok || !strings.Contains(out, "IP literals") {
		t.Errorf("websocket should be policy-guarded, got ok=%v err=%s", ok, out)
	}
}

func pEvalErr(t *testing.T, p *scriptling.Scriptling, script string) (string, bool) {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		return err.Error(), false
	}
	return result.Inspect(), true
}
