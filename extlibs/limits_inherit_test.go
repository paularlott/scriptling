package extlibs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// setupLimitedSandbox returns a parent interpreter configured by opts and a
// sandbox factory that records every instance it creates.
func setupLimitedSandbox(t *testing.T, opts ...scriptling.Option) (*scriptling.Scriptling, *[]*scriptling.Scriptling) {
	t.Helper()
	ResetRuntime()
	SetSandboxFactory(nil)
	t.Cleanup(func() { SetSandboxFactory(nil) })

	p := scriptling.New(opts...)
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)

	created := &[]*scriptling.Scriptling{}
	SetSandboxFactory(func() SandboxInstance {
		child := scriptling.New()
		stdlib.RegisterAll(child)
		RegisterRuntimeLibraryAll(child, nil)
		*created = append(*created, child)
		return child
	})
	return p, created
}

// A sandbox shares its parent's guard: the same limits and the same memory
// budget.
func TestSandboxSharesParentGuard(t *testing.T) {
	p, created := setupLimitedSandbox(t, scriptling.WithMaxValueBytes(1024))
	if _, err := p.Eval(`
import scriptling.runtime as runtime
env = runtime.sandbox.create()
`); err != nil {
		t.Fatal(err)
	}
	if len(*created) != 1 {
		t.Fatalf("factory called %d times", len(*created))
	}
	child := (*created)[0]
	if child.ResourceGuard() != p.ResourceGuard() {
		t.Fatal("sandbox does not share the parent's guard")
	}
	if child.MaxValueBytes() != 1024 {
		t.Fatalf("sandbox value limit = %d", child.MaxValueBytes())
	}
}

// The parent's value limit applies to code the sandbox runs.
func TestSandboxInheritsValueLimit(t *testing.T) {
	p, _ := setupLimitedSandbox(t, scriptling.WithMaxValueBytes(1024))
	result, err := p.Eval(`
import scriptling.runtime as runtime
env = runtime.sandbox.create()
env.exec("""
n = 2000
try:
    s = "x" * n
    r = "no error"
except MemoryError:
    r = "caught"
small = len("y" * 500)
""")
[env.get("r"), env.get("small"), env.exit_code()]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['caught', 500, 0]" {
		t.Fatalf("got %s", got)
	}
}

// Memory held by a sandbox counts against the parent's memory limit, so a
// script cannot escape its limit by allocating in sandboxes.
func TestSandboxMemoryCountsAgainstParent(t *testing.T) {
	p, _ := setupLimitedSandbox(t, scriptling.WithMemoryLimit(16<<20), scriptling.WithMaxValueBytes(0))
	if _, err := p.Eval(`
import scriptling.runtime as runtime
env = runtime.sandbox.create()
env.exec("n = 100000\nheld = [str(i) * n for i in range(30)]")
`); err != nil {
		t.Fatal(err)
	}
	if used := p.ResourceUsage().MemoryBytes; used < 2_000_000 {
		t.Fatalf("parent charged %d bytes for a sandbox holding about 3 MB", used)
	}

	// Going over the shared budget in a sandbox fails the sandbox's exec.
	result, err := p.Eval(`
env2 = runtime.sandbox.create()
env2.exec("n = 100000\nheld = []\nwhile True:\n    held.append(str(len(held)) * n)")
env2.exit_code()
`)
	if err != nil {
		if !strings.Contains(err.Error(), "memory limit exceeded") {
			t.Fatalf("unexpected error: %v", err)
		}
		return // the shared run was cancelled as a whole: also correct
	}
	if n, _ := result.AsInt(); n == 0 {
		t.Fatal("sandbox holding more than the parent's memory limit succeeded")
	}
}

// Instances that cannot share a guard keep their own, and a parent with no
// guard changes nothing.
func TestInheritResourceGuardEdgeCases(t *testing.T) {
	plain := &plainSandboxInstance{}
	inheritResourceGuard(plain, object.NewResourceGuard()) // must not panic

	child := scriptling.New()
	own := child.ResourceGuard()
	inheritResourceGuard(child, nil)
	if child.ResourceGuard() != own {
		t.Fatal("a nil parent guard replaced the instance's own")
	}

	shared := object.NewResourceGuard()
	inheritResourceGuard(child, shared)
	if child.ResourceGuard() != shared {
		t.Fatal("guard not shared")
	}
	child.ShareResourceGuard(nil)
	if child.ResourceGuard() == nil || child.ResourceGuard() == shared {
		t.Fatal("ShareResourceGuard(nil) did not give a fresh guard")
	}
}

// plainSandboxInstance implements SandboxInstance without ShareResourceGuard.
type plainSandboxInstance struct{}

func (*plainSandboxInstance) SetObjectVar(string, object.Object) error { return nil }
func (*plainSandboxInstance) GetVarAsObject(string) (object.Object, error) {
	return &object.Null{}, nil
}
func (*plainSandboxInstance) EvalWithContext(context.Context, string) (object.Object, error) {
	return &object.Null{}, nil
}
func (*plainSandboxInstance) SetSourceFile(string)                                 {}
func (*plainSandboxInstance) LoadLibraryIntoEnv(string, *object.Environment) error { return nil }
func (*plainSandboxInstance) SetOutputWriter(io.Writer)                            {}

// A sandbox's exec_file reads the file under the value limit.
func TestSandboxExecFileValueLimit(t *testing.T) {
	p, _ := setupLimitedSandbox(t, scriptling.WithMaxValueBytes(1024))
	dir := t.TempDir()
	small := filepath.Join(dir, "small.py")
	large := filepath.Join(dir, "large.py")
	os.WriteFile(small, []byte("r = 1\n"), 0o600)
	os.WriteFile(large, []byte("r = 1\n# "+strings.Repeat("x", 2000)+"\n"), 0o600)
	p.SetVar("small", small)
	p.SetVar("large", large)
	result, err := p.Eval(`
import scriptling.runtime as runtime
a = runtime.sandbox.create()
a.exec_file(small)
b = runtime.sandbox.create()
b.exec_file(large)
[a.exit_code(), a.get("r"), b.exit_code()]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[0, 1, 1]" {
		t.Fatalf("got %s", got)
	}
}

// Background tasks run under the caller's limits, with and without a
// factory and for library handlers.
func TestBackgroundTasksInheritLimits(t *testing.T) {
	task := `
import scriptling.runtime as runtime

def task():
    n = 2000
    try:
        s = "x" * n
        return "no error"
    except MemoryError:
        return "caught"
`
	t.Run("without a factory", func(t *testing.T) {
		ResetRuntime()
		SetSandboxFactory(nil)
		p := scriptling.New(scriptling.WithMaxValueBytes(1024))
		RegisterRuntimeLibraryAll(p, nil)
		result, err := p.Eval(task + "\nruntime.background('limited1', 'task').get()")
		if err != nil {
			t.Fatal(err)
		}
		if result.Inspect() != "caught" {
			t.Fatalf("background task ran without the caller's value limit: %s", result.Inspect())
		}
	})

	// useBackgroundFactory makes background tasks use the sandbox factory and
	// start at once, as a server does after its setup script.
	useBackgroundFactory := func(t *testing.T) {
		SetBackgroundFactory(GetSandboxFactory())
		ReleaseBackgroundTasks()
		t.Cleanup(ResetRuntime)
	}

	t.Run("with a factory", func(t *testing.T) {
		p, created := setupLimitedSandbox(t, scriptling.WithMaxValueBytes(1024))
		useBackgroundFactory(t)
		result, err := p.Eval(task + "\nruntime.background('limited2', 'task').get()")
		if err != nil {
			t.Fatal(err)
		}
		if result.Inspect() != "caught" {
			t.Fatalf("background task ran without the caller's value limit: %s", result.Inspect())
		}
		if len(*created) == 0 {
			t.Fatal("the task did not use the factory")
		}
		for _, child := range *created {
			if child.ResourceGuard() != p.ResourceGuard() {
				t.Fatal("factory instance for the task does not share the caller's guard")
			}
		}
	})

	t.Run("library handler", func(t *testing.T) {
		p, _ := setupLimitedSandbox(t, scriptling.WithMaxValueBytes(1024))
		SetSandboxFactory(func() SandboxInstance {
			child := scriptling.New()
			stdlib.RegisterAll(child)
			RegisterRuntimeLibraryAll(child, nil)
			child.RegisterScriptLibrary("limitedlib", `
def work():
    n = 2000
    try:
        s = "x" * n
        return "no error"
    except MemoryError:
        return "caught"
`)
			return child
		})
		useBackgroundFactory(t)
		result, err := p.Eval("import scriptling.runtime as runtime\nruntime.background('limited3', 'limitedlib.work').get()")
		if err != nil {
			t.Fatal(err)
		}
		if result.Inspect() != "caught" {
			t.Fatalf("library task ran without the caller's value limit: %s", result.Inspect())
		}
	})
}
