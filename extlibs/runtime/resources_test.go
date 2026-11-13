package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

// A background task closes what it opened and left open when its function
// returns; what the script that started it opened stays open until that
// interpreter closes it.
func TestBackgroundTaskClosesItsResources(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)
	closed := make(chan string, 4)
	lib := object.NewLibraryBuilder("res", "test resources")
	lib.Function("open", func(ctx context.Context, name string) object.Object {
		evaluator.TrackResource(ctx, nil, func() { closed <- name })
		return &object.Null{}
	})
	p.RegisterLibrary(lib.Build())

	if _, err := p.Eval(`
import scriptling.runtime as runtime
import res
res.open("caller")

def task():
    import res
    res.open("task")
    return 1

runtime.background("t1", "task").get()
`); err != nil {
		t.Fatal(err)
	}
	select {
	case name := <-closed:
		if name != "task" {
			t.Fatalf("closed %q when the task returned", name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the task's resource is still open after it returned")
	}
	select {
	case name := <-closed:
		t.Fatalf("closed %q before the interpreter was closed", name)
	default:
	}
	p.Close()
	if name := <-closed; name != "caller" {
		t.Fatalf("closed %q", name)
	}
}
