package stdlib

import (
	"context"
	"math"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func randomFn(name string) *object.Builtin {
	return RandomLibrary.Functions()[name]
}

func TestRandomSeed(t *testing.T) {
	fn := randomFn("seed")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, object.NewInteger(42))
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("seed() should return None, got %T", result)
	}

	result = fn.Fn(ctx, kwargs)
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("seed() without args should return None, got %T", result)
	}
}

func TestRandomRandint(t *testing.T) {
	fn := randomFn("seed")
	fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(42))

	fn2 := randomFn("randint")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 100; i++ {
		result := fn2.Fn(ctx, kwargs, object.NewInteger(1), object.NewInteger(10))
		n, ok := result.(*object.Integer)
		if !ok {
			t.Fatalf("randint() returned %T, want Integer", result)
		}
		if n.Value < 1 || n.Value > 10 {
			t.Errorf("randint(1,10) = %d, out of range", n.Value)
		}
	}

	result := fn2.Fn(ctx, kwargs, object.NewInteger(5), object.NewInteger(5))
	if n, ok := result.(*object.Integer); !ok || n.Value != 5 {
		t.Errorf("randint(5,5) = %v, want 5", result)
	}

	result = fn2.Fn(ctx, kwargs, object.NewInteger(10), object.NewInteger(1))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("randint(10,1) should return error, got %T", result)
	}
}

func TestRandomRandom(t *testing.T) {
	fn := randomFn("random")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 100; i++ {
		result := fn.Fn(ctx, kwargs)
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("random() returned %T, want Float", result)
		}
		if f.Value < 0.0 || f.Value >= 1.0 {
			t.Errorf("random() = %v, out of [0,1)", f.Value)
		}
	}
}

func TestRandomChoiceList(t *testing.T) {
	fn := randomFn("choice")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	items := &object.List{Elements: []object.Object{
		&object.String{Value: "a"},
		&object.String{Value: "b"},
		&object.String{Value: "c"},
	}}

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, items)
		s, ok := result.(*object.String)
		if !ok {
			t.Fatalf("choice() returned %T, want String", result)
		}
		if s.Value != "a" && s.Value != "b" && s.Value != "c" {
			t.Errorf("choice() = %q, not in list", s.Value)
		}
	}
}

func TestRandomChoiceString(t *testing.T) {
	fn := randomFn("choice")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.String{Value: "hello"})
	s, ok := result.(*object.String)
	if !ok {
		t.Fatalf("choice() on string returned %T, want String", result)
	}
	if len(s.Value) != 1 {
		t.Errorf("choice() on string returned %q, want 1 char", s.Value)
	}
}

func TestRandomChoiceEmpty(t *testing.T) {
	fn := randomFn("choice")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.List{Elements: []object.Object{}})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("choice() on empty list should return error, got %T", result)
	}
}

func TestRandomShuffle(t *testing.T) {
	fn := randomFn("shuffle")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	list := &object.List{Elements: []object.Object{
		object.NewInteger(1), object.NewInteger(2), object.NewInteger(3),
		object.NewInteger(4), object.NewInteger(5),
	}}
	result := fn.Fn(ctx, kwargs, list)
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("shuffle() should return None, got %T", result)
	}
	if len(list.Elements) != 5 {
		t.Errorf("shuffle() changed list length to %d", len(list.Elements))
	}

	seen := make(map[int64]bool)
	for _, el := range list.Elements {
		seen[el.(*object.Integer).Value] = true
	}
	if len(seen) != 5 {
		t.Errorf("shuffle() lost or duplicated elements")
	}

	result = fn.Fn(ctx, kwargs, &object.Integer{Value: 5})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("shuffle() with non-list should return error, got %T", result)
	}
}

func TestRandomUniform(t *testing.T) {
	fn := randomFn("uniform")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 100; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: -10.0}, &object.Float{Value: 10.0})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("uniform() returned %T, want Float", result)
		}
		if f.Value < -10.0 || f.Value > 10.0 {
			t.Errorf("uniform(-10, 10) = %v, out of range", f.Value)
		}
	}
}

func TestRandomSample(t *testing.T) {
	fn := randomFn("sample")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	list := &object.List{Elements: []object.Object{
		object.NewInteger(1), object.NewInteger(2), object.NewInteger(3),
		object.NewInteger(4), object.NewInteger(5),
	}}

	result := fn.Fn(ctx, kwargs, list, object.NewInteger(3))
	sample, ok := result.(*object.List)
	if !ok {
		t.Fatalf("sample() returned %T, want List", result)
	}
	if len(sample.Elements) != 3 {
		t.Errorf("sample(list, 3) returned %d elements, want 3", len(sample.Elements))
	}

	seen := make(map[int64]bool)
	for _, el := range sample.Elements {
		v := el.(*object.Integer).Value
		if seen[v] {
			t.Errorf("sample() returned duplicate %d", v)
		}
		seen[v] = true
	}

	result = fn.Fn(ctx, kwargs, list, object.NewInteger(6))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("sample() k > len should return error, got %T", result)
	}

	result = fn.Fn(ctx, kwargs, list, object.NewInteger(-1))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("sample() negative k should return error, got %T", result)
	}
}

func TestRandomRandrange(t *testing.T) {
	fn := randomFn("randrange")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 100; i++ {
		result := fn.Fn(ctx, kwargs, object.NewInteger(10))
		n, ok := result.(*object.Integer)
		if !ok {
			t.Fatalf("randrange(10) returned %T", result)
		}
		if n.Value < 0 || n.Value >= 10 {
			t.Errorf("randrange(10) = %d, out of [0,10)", n.Value)
		}
	}

	for i := 0; i < 100; i++ {
		result := fn.Fn(ctx, kwargs, object.NewInteger(5), object.NewInteger(15))
		n, ok := result.(*object.Integer)
		if !ok {
			t.Fatalf("randrange(5,15) returned %T", result)
		}
		if n.Value < 5 || n.Value >= 15 {
			t.Errorf("randrange(5,15) = %d, out of [5,15)", n.Value)
		}
	}

	for i := 0; i < 100; i++ {
		result := fn.Fn(ctx, kwargs, object.NewInteger(0), object.NewInteger(10), object.NewInteger(2))
		n, ok := result.(*object.Integer)
		if !ok {
			t.Fatalf("randrange(0,10,2) returned %T", result)
		}
		if n.Value < 0 || n.Value >= 10 || n.Value%2 != 0 {
			t.Errorf("randrange(0,10,2) = %d, invalid", n.Value)
		}
	}
}

func TestRandomGauss(t *testing.T) {
	fn := randomFn("gauss")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := fn.Fn(ctx, kwargs, &object.Float{Value: 0.0}, &object.Float{Value: 1.0})
	if _, ok := result.(*object.Float); !ok {
		t.Errorf("gauss(0,1) returned %T, want Float", result)
	}
}

func TestRandomSeedReproducibility(t *testing.T) {
	seed := randomFn("seed")
	randFn := randomFn("random")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	seed.Fn(ctx, kwargs, object.NewInteger(12345))
	a := randFn.Fn(ctx, kwargs).(*object.Float).Value

	seed.Fn(ctx, kwargs, object.NewInteger(12345))
	b := randFn.Fn(ctx, kwargs).(*object.Float).Value

	if a != b {
		t.Errorf("seeded random not reproducible: %v != %v", a, b)
	}
}

func TestRandomChoices(t *testing.T) {
	fn := randomFn("choices")
	ctx := context.Background()

	pop := &object.List{Elements: []object.Object{
		&object.String{Value: "a"},
		&object.String{Value: "b"},
		&object.String{Value: "c"},
	}}

	result := fn.Fn(ctx, object.NewKwargs(map[string]object.Object{
		"weights": &object.List{Elements: []object.Object{
			&object.Float{Value: 5.0}, &object.Float{Value: 3.0}, &object.Float{Value: 1.0},
		}},
		"k": object.NewInteger(10),
	}), pop)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("choices() returned %T, want List", result)
	}
	if len(list.Elements) != 10 {
		t.Errorf("choices(k=10) returned %d elements", len(list.Elements))
	}
	for _, el := range list.Elements {
		s := el.(*object.String).Value
		if s != "a" && s != "b" && s != "c" {
			t.Errorf("choices() returned %q, not in population", s)
		}
	}

	result = fn.Fn(ctx, object.NewKwargs(nil), pop)
	list = result.(*object.List)
	if len(list.Elements) != 1 {
		t.Errorf("choices() without k should return 1 element, got %d", len(list.Elements))
	}
}

func TestRandomChoicesEmpty(t *testing.T) {
	fn := randomFn("choices")
	ctx := context.Background()

	result := fn.Fn(ctx, object.NewKwargs(nil), &object.List{Elements: []object.Object{}})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("choices() with empty population should return error, got %T", result)
	}
}

func TestRandomChoicesPositionalWeightsAndK(t *testing.T) {
	fn := randomFn("choices")
	ctx := context.Background()

	pop := &object.List{Elements: []object.Object{
		&object.String{Value: "a"},
		&object.String{Value: "b"},
		&object.String{Value: "c"},
	}}
	weights := &object.List{Elements: []object.Object{
		object.NewInteger(0),
		object.NewInteger(0),
		object.NewInteger(1),
	}}

	result := fn.Fn(ctx, object.NewKwargs(nil), pop, weights, object.NewInteger(10))
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("choices() returned %T, want List", result)
	}
	if len(list.Elements) != 10 {
		t.Fatalf("choices() returned %d elements, want 10", len(list.Elements))
	}
	for _, el := range list.Elements {
		if s := el.(*object.String).Value; s != "c" {
			t.Errorf("choices() with zero weights returned %q, want c", s)
		}
	}
}

func TestRandomChoicesInvalidArgsAndWeights(t *testing.T) {
	fn := randomFn("choices")
	ctx := context.Background()

	pop := &object.List{Elements: []object.Object{
		&object.String{Value: "a"},
		&object.String{Value: "b"},
	}}

	tests := []struct {
		name   string
		kwargs object.Kwargs
		args   []object.Object
	}{
		{
			name:   "too many positional args",
			kwargs: object.NewKwargs(nil),
			args: []object.Object{
				pop,
				&object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(1)}},
				object.NewInteger(1),
				object.NewInteger(2),
			},
		},
		{
			name: "duplicate weights",
			kwargs: object.NewKwargs(map[string]object.Object{
				"weights": &object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(1)}},
			}),
			args: []object.Object{
				pop,
				&object.List{Elements: []object.Object{object.NewInteger(1), object.NewInteger(1)}},
			},
		},
		{
			name:   "negative weight",
			kwargs: object.NewKwargs(nil),
			args: []object.Object{
				pop,
				&object.List{Elements: []object.Object{object.NewInteger(-1), object.NewInteger(1)}},
			},
		},
		{
			name:   "nan weight",
			kwargs: object.NewKwargs(nil),
			args: []object.Object{
				pop,
				&object.List{Elements: []object.Object{&object.Float{Value: math.NaN()}, object.NewInteger(1)}},
			},
		},
		{
			name:   "zero total",
			kwargs: object.NewKwargs(nil),
			args: []object.Object{
				pop,
				&object.List{Elements: []object.Object{object.NewInteger(0), object.NewInteger(0)}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fn.Fn(ctx, tt.kwargs, tt.args...)
			if _, ok := result.(*object.Error); !ok {
				t.Errorf("choices() should return error, got %T", result)
			}
		})
	}
}

func TestRandomExpovariate(t *testing.T) {
	fn := randomFn("expovariate")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 1.0})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("expovariate() returned %T, want Float", result)
		}
		if f.Value < 0 {
			t.Errorf("expovariate(1) = %v, should be >= 0", f.Value)
		}
	}

	result := fn.Fn(ctx, kwargs, &object.Float{Value: 0.0})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("expovariate(0) should return error, got %T", result)
	}
}

func TestRandomBetavariate(t *testing.T) {
	fn := randomFn("betavariate")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 2.0}, &object.Float{Value: 5.0})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("betavariate() returned %T, want Float", result)
		}
		if f.Value < 0 || f.Value > 1 {
			t.Errorf("betavariate(2,5) = %v, should be in [0,1]", f.Value)
		}
	}

	result := fn.Fn(ctx, kwargs, &object.Float{Value: 0}, &object.Float{Value: 1})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("betavariate(0,1) should return error, got %T", result)
	}
}

func TestRandomGammavariate(t *testing.T) {
	fn := randomFn("gammavariate")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 2.0}, &object.Float{Value: 1.0})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("gammavariate() returned %T, want Float", result)
		}
		if f.Value < 0 {
			t.Errorf("gammavariate(2,1) = %v, should be >= 0", f.Value)
		}
	}

	result := fn.Fn(ctx, kwargs, &object.Float{Value: -1}, &object.Float{Value: 1})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("gammavariate(-1,1) should return error, got %T", result)
	}
}

func TestRandomTriangular(t *testing.T) {
	fn := randomFn("triangular")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 0.0}, &object.Float{Value: 10.0})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("triangular() returned %T, want Float", result)
		}
		if f.Value < 0 || f.Value > 10 {
			t.Errorf("triangular(0,10) = %v, out of range", f.Value)
		}
	}

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 0.0}, &object.Float{Value: 10.0}, &object.Float{Value: 5.0})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("triangular() with mode returned %T", result)
		}
		if f.Value < 0 || f.Value > 10 {
			t.Errorf("triangular(0,10,5) = %v, out of range", f.Value)
		}
	}

	result := fn.Fn(ctx, kwargs, &object.Float{Value: 5.0}, &object.Float{Value: 5.0})
	f, _ := result.(*object.Float)
	if math.Abs(f.Value-5.0) > 1e-10 {
		t.Errorf("triangular(5,5) = %v, want 5.0", f.Value)
	}
}

func TestRandomParetovariate(t *testing.T) {
	fn := randomFn("paretovariate")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 1.5})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("paretovariate() returned %T, want Float", result)
		}
		if f.Value < 1.0 {
			t.Errorf("paretovariate(1.5) = %v, should be >= 1.0", f.Value)
		}
	}

	result := fn.Fn(ctx, kwargs, &object.Float{Value: 0})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("paretovariate(0) should return error, got %T", result)
	}
}

func TestRandomWeibullvariate(t *testing.T) {
	fn := randomFn("weibullvariate")
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	for i := 0; i < 50; i++ {
		result := fn.Fn(ctx, kwargs, &object.Float{Value: 1.0}, &object.Float{Value: 1.5})
		f, ok := result.(*object.Float)
		if !ok {
			t.Fatalf("weibullvariate() returned %T, want Float", result)
		}
		if f.Value < 0 {
			t.Errorf("weibullvariate(1,1.5) = %v, should be >= 0", f.Value)
		}
	}

	result := fn.Fn(ctx, kwargs, &object.Float{Value: 0}, &object.Float{Value: 1})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("weibullvariate(0,1) should return error, got %T", result)
	}
}
