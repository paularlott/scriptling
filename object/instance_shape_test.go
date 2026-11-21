package object

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func fieldNames(i *Instance) string {
	var names []string
	i.RangeFields(func(name string, v Object) bool {
		names = append(names, fmt.Sprintf("%s=%s", name, v.Inspect()))
		return true
	})
	return strings.Join(names, ",")
}

// TestInstanceShapesShared checks that instances assigning the same fields in
// the same order share one shape, and that deletion keeps order.
func TestInstanceShapesShared(t *testing.T) {
	c := &Class{Name: "P"}
	a, b := NewInstance(c), NewInstance(c)
	for _, i := range []*Instance{a, b} {
		i.SetField("x", NewInteger(1))
		i.SetField("y", NewInteger(2))
	}
	if a.shape != b.shape || a.shape.private {
		t.Fatalf("instances with the same layout do not share a shape")
	}
	a.DeleteField("x")
	if a.shape.private || a.FieldCount() != 1 {
		t.Fatalf("delete left a private shape or wrong count")
	}
	a.SetField("x", NewInteger(3))
	if got := fieldNames(a); got != "y=2,x=3" {
		t.Fatalf("order after delete and re-add: %s", got)
	}
	if got := fieldNames(b); got != "x=1,y=2" {
		t.Fatalf("other instance changed: %s", got)
	}
}

// TestInstanceShapesPrivate drives instances past every sharing limit and
// checks the fields still read, write, delete and iterate correctly.
func TestInstanceShapesPrivate(t *testing.T) {
	c := &Class{Name: "Bag"}
	for n := range 200 {
		i := NewInstance(c)
		var want []string
		for k := range n % 50 {
			name := fmt.Sprintf("f%d_%d", n%7, (k*31+n)%60)
			if i.HasField(name) {
				continue
			}
			i.SetField(name, NewInteger(int64(k)))
			want = append(want, fmt.Sprintf("%s=%d", name, k))
		}
		if len(want) > 2 {
			del := len(want) / 2
			name := strings.SplitN(want[del], "=", 2)[0]
			i.DeleteField(name)
			want = append(want[:del], want[del+1:]...)
			i.SetField(name, NewInteger(-1))
			want = append(want, name+"=-1")
		}
		if got := fieldNames(i); got != strings.Join(want, ",") {
			t.Fatalf("instance %d: got %s want %s", n, got, strings.Join(want, ","))
		}
		for _, w := range want {
			kv := strings.SplitN(w, "=", 2)
			if v, ok := i.GetField(kv[0]); !ok || v.Inspect() != kv[1] {
				t.Fatalf("instance %d: GetField(%s) = %v, %v", n, kv[0], v, ok)
			}
		}
	}
	if got := c.rootShape().count.Load(); got > maxClassShapes {
		t.Fatalf("class has %d shared shapes, limit %d", got, maxClassShapes)
	}
	var nilClass Instance
	nilClass.SetField("a", NewInteger(1))
	nilClass.SetField("b", NewInteger(2))
	nilClass.DeleteField("a")
	if got := fieldNames(&nilClass); got != "b=2" {
		t.Fatalf("instance without class: %s", got)
	}
}

// TestInstanceShapesConcurrent creates instances of one class from many
// goroutines, as interpreters sharing a class do; run it with -race.
func TestInstanceShapesConcurrent(t *testing.T) {
	c := &Class{Name: "Shared"}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range 500 {
				i := NewInstance(c)
				var order []string
				vals := map[string]int{}
				for k := range (n + g) % 12 {
					name := fmt.Sprintf("a%d", (k+g+n)%15)
					if _, ok := vals[name]; !ok {
						order = append(order, name)
					}
					vals[name] = k
					i.SetField(name, NewInteger(int64(k)))
				}
				if len(order) > 1 {
					i.DeleteField(order[0])
					delete(vals, order[0])
					order = order[1:]
				}
				var want []string
				for _, name := range order {
					want = append(want, fmt.Sprintf("%s=%d", name, vals[name]))
				}
				if got := fieldNames(i); got != strings.Join(want, ",") {
					t.Errorf("goroutine %d instance %d: got %s want %s", g, n, got, strings.Join(want, ","))
					return
				}
			}
		}()
	}
	wg.Wait()
}
