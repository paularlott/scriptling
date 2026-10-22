package setup

import (
	"strings"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
)

// containerDenyPrivileged reaches the container library: a privileged run is
// refused before the runtime is contacted, and allowed when false.
func TestScriptlingContainerDenyPrivileged(t *testing.T) {
	run := `import scriptling.container as container
container.Client("docker").run("alpine", privileged=True)`
	for _, deny := range []bool{true, false} {
		p := scriptling.New()
		Scriptling(p, nil, false, nil, nil, nil, logslog.New(logslog.Config{Level: "error"}), "/nonexistent/docker.sock", "", deny)
		_, err := p.Eval(run)
		denied := err != nil && strings.Contains(err.Error(), "privileged containers are not allowed")
		if denied != deny {
			t.Errorf("deny=%v: err=%v", deny, err)
		}
	}
}
