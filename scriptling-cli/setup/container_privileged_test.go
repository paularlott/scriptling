package setup

import (
	"runtime"
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

// The Apple driver's CLI is the operator's, run from a fixed path, so it is
// available with the subprocess library disabled, as Docker and Podman are.
func TestScriptlingContainerAppleWithoutSubprocess(t *testing.T) {
	p := scriptling.New()
	Scriptling(p, nil, false, nil, []string{"subprocess"}, nil, logslog.New(logslog.Config{Level: "error"}), "/nonexistent/docker.sock", "", false)
	_, err := p.Eval("import scriptling.container as container\ncontainer.Client(\"apple\")")
	if err != nil && runtime.GOOS == "darwin" {
		t.Fatalf("apple driver with subprocess disabled: %v", err)
	}
	if _, err := p.Eval("import subprocess"); err == nil {
		t.Fatal("subprocess is registered")
	}
}
