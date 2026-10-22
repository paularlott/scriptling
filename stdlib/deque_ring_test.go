package stdlib

import (
	"math/rand"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// The ring buffer matches a plain slice model through random operations,
// with and without maxlen, never keeps a popped element alive, and shrinks
// as it empties.
func TestDequeRingMatchesModel(t *testing.T) {
	rng := rand.New(rand.NewSource(17))
	for _, maxlen := range []int64{-1, 0, 1, 5, 33} {
		d := &dequeBuf{maxlen: maxlen}
		var model []object.Object
		clamp := func(right bool) {
			for maxlen >= 0 && int64(len(model)) > maxlen {
				if right {
					model = model[1:]
				} else {
					model = model[:len(model)-1]
				}
			}
		}
		for step := 0; step < 20000; step++ {
			v := object.NewInteger(int64(step))
			switch rng.Intn(9) {
			case 0, 1:
				d.pushRight(v)
				model = append(model, v)
				clamp(true)
			case 2, 3:
				d.pushLeft(v)
				model = append([]object.Object{v}, model...)
				clamp(false)
			case 4:
				if d.n > 0 {
					if got := d.popRight(); got != model[len(model)-1] {
						t.Fatalf("step %d: popRight", step)
					}
					model = model[:len(model)-1]
				}
			case 5:
				if d.n > 0 {
					if got := d.popLeft(); got != model[0] {
						t.Fatalf("step %d: popLeft", step)
					}
					model = model[1:]
				}
			case 6:
				vals := make([]object.Object, rng.Intn(40))
				for i := range vals {
					vals[i] = object.NewInteger(int64(-i))
				}
				if rng.Intn(2) == 0 {
					d.extendRight(vals)
					model = append(model, vals...)
					clamp(true)
				} else {
					d.extendLeft(vals)
					for _, x := range vals {
						model = append([]object.Object{x}, model...)
					}
					clamp(false)
				}
			case 7:
				k := int64(rng.Intn(200) - 100)
				d.rotate(k)
				if n := int64(len(model)); n > 0 {
					k = ((k % n) + n) % n
					model = append(append([]object.Object{}, model[n-k:]...), model[:n-k]...)
				}
			case 8:
				for i := 0; i < 50 && d.n > 0; i++ {
					d.popLeft()
					model = model[1:]
				}
			}
			if d.n != len(model) {
				t.Fatalf("maxlen %d step %d: len %d, model %d", maxlen, step, d.n, len(model))
			}
			for i := range model {
				if d.at(i) != model[i] {
					t.Fatalf("maxlen %d step %d: element %d differs", maxlen, step, i)
				}
			}
			// Slots outside the live range hold nothing.
			live := 0
			for _, x := range d.buf {
				if x != nil {
					live++
				}
			}
			if live != d.n {
				t.Fatalf("maxlen %d step %d: %d occupied slots for %d elements", maxlen, step, live, d.n)
			}
		}
	}

	d := &dequeBuf{maxlen: -1}
	for i := 0; i < 100000; i++ {
		d.pushRight(object.NewInteger(int64(i)))
	}
	big := len(d.buf)
	for d.n > 10 {
		d.popLeft()
	}
	if len(d.buf) >= big/100 {
		t.Fatalf("ring of %d slots kept for %d elements", len(d.buf), d.n)
	}
}
