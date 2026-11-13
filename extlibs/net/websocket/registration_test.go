package websocket_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/net/websocket"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if websocket.LibraryName != "scriptling.net.websocket" {
		t.Errorf("websocket.LibraryName = %q, want %q (scripts import it by this name)", websocket.LibraryName, "scriptling.net.websocket")
	}
}

func TestWebsocketRegisterAndImport(t *testing.T) {
	p := scriptling.New()
	websocket.Register(p)
	scripttest.Imports(t, p, "scriptling.net.websocket")
	if got := scripttest.Eval(t, p, "import scriptling.net.websocket as ws\ncallable(ws.connect)").Inspect(); got != "True" {
		t.Errorf("websocket.connect is not callable after Register: %s", got)
	}
}

func TestWebsocketConnectFailureIsCatchable(t *testing.T) {
	p := scriptling.New()
	websocket.Register(p)
	// Port 1 on loopback refuses the connection.
	_, msg := scripttest.Raises(t, p, "import scriptling.net.websocket as ws\nws.connect('ws://127.0.0.1:1/', timeout=1)")
	if !strings.Contains(msg, "connect") {
		t.Errorf("failed connect raised %q, want a connect error", msg)
	}
}

func TestWebsocketPolicyRefusalIsCatchable(t *testing.T) {
	p := scriptling.New()
	websocket.Register(p, &netsecurity.Config{})
	_, msg := scripttest.Raises(t, p, "import scriptling.net.websocket as ws\nws.connect('ws://10.0.0.1:8080/', timeout=1)")
	if !strings.Contains(msg, "network policy") {
		t.Errorf("blocked dial raised %q, want a network policy error", msg)
	}
}

func TestWebsocketNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "scriptling.net.websocket", "it is only available after websocket.Register(p, policy)")
}
