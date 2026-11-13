package mercure

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// A subscription keeps its client alive. The client here is never bound to a
// name, so without that the collector may finalise it, which closes the
// client and ends the subscription before the event arrives. collect() forces
// the collection that otherwise happens only by timing.
func TestSubscriptionKeepsUnnamedClientAlive(t *testing.T) {
	h := newHub(t)
	p := scriptling.New()
	Register(p)
	p.RegisterLibrary(object.NewLibrary("gctest", map[string]*object.Builtin{
		"collect": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			for range 3 {
				runtime.GC()
				time.Sleep(20 * time.Millisecond) // let finalizers and their close goroutines run
			}
			return &object.Null{}
		}},
	}, nil, "forces garbage collection"))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := p.EvalWithContext(ctx, `import scriptling.mercure as mercure
import gctest
KEY = "`+testKey+`"
URL = "`+h.url+`"
def token(**grants):
    return mercure.create_token(KEY, URL, **grants)
sub = mercure.Client(URL, token=token(subscribe=["*"])).subscribe("https://example.com/t")
gctest.collect()
mercure.Client(URL, token=token(publish=["*"])).publish("https://example.com/t", "kept")
ev = sub.receive(timeout=10)
sub.close()
ev.data if ev is not None else "subscription ended"
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "kept" {
		t.Fatalf("got %s, want kept", got)
	}
}
