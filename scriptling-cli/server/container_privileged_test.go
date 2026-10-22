package server

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

// ServerConfig.ContainerDenyPrivileged reaches the evaluators the server
// builds for its setup script and HTTP, JSON-RPC and WebSocket handlers.
func TestServerContainerDenyPrivileged(t *testing.T) {
	run := `import scriptling.container as container
container.Client("docker").run("alpine", privileged=True)`
	for _, deny := range []bool{true, false} {
		s := &Server{config: ServerConfig{DockerSock: "/nonexistent/docker.sock", ContainerDenyPrivileged: deny}}
		p := scriptling.New()
		s.setupScriptling(p)
		_, err := p.Eval(run)
		if denied := err != nil && strings.Contains(err.Error(), "privileged containers are not allowed"); denied != deny {
			t.Errorf("server deny=%v: err=%v", deny, err)
		}
	}
}
