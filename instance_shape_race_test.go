package scriptling

import (
	"fmt"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestSharedClassInstancesAcrossInterpreters creates instances of one
// Go-built class, shared by several interpreters running at once, so the
// class's shared field layouts are used from many goroutines; run with -race.
func TestSharedClassInstancesAcrossInterpreters(t *testing.T) {
	cb := object.NewClassBuilder("Shared")
	cb.Method("__init__", func(self *object.Instance, n int) {
		self.SetField("n", object.NewInteger(int64(n)))
		if n%2 == 0 {
			self.SetField("even", object.NewBoolean(true))
		}
	})
	class := cb.Build()
	script := `
total = 0
for i in range(300):
    o = Shared(i)
    o.extra = i
    if i % 3 == 0:
        o.third = 1
        del o.n
        o.n = i
    total += o.n + o.extra
    if list(vars(o))[-1] not in ("extra", "n"):
        raise ValueError("bad order: " + str(list(vars(o))))
`
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := New()
			if err := p.SetObjectVar("Shared", class); err != nil {
				errs <- err
				return
			}
			if _, err := p.Eval(script); err != nil {
				errs <- fmt.Errorf("interpreter %d: %w", g, err)
				return
			}
			total, err := p.GetVar("total")
			if err != nil || fmt.Sprint(total) != "89700" {
				errs <- fmt.Errorf("interpreter %d: total %v, %v", g, total, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
