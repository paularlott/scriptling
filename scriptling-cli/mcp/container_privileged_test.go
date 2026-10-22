package mcp

import (
	"strings"
	"testing"
)

// WithContainerDenyPrivileged reaches the interpreter every MCP tool,
// resource and prompt handler runs in.
func TestHandlerContainerDenyPrivileged(t *testing.T) {
	run := `import scriptling.container as container
container.Client("docker").run("alpine", privileged=True)`
	for _, deny := range []bool{true, false} {
		cfg := NewHandlerConfig(nil, WithDockerSock("/nonexistent/docker.sock"), WithContainerDenyPrivileged(deny))
		_, err := prepareScriptling(cfg, nil).Eval(run)
		if denied := err != nil && strings.Contains(err.Error(), "privileged containers are not allowed"); denied != deny {
			t.Errorf("deny=%v: err=%v", deny, err)
		}
	}
}
