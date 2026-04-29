package stdlib

import (
	"context"
	"math"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestMathSqrt(t *testing.T) {
	lib := MathLibrary
	sqrt := lib.Functions()["sqrt"]

	result := sqrt.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 16})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 4.0 {
			t.Errorf("sqrt(16) = %v, want 4.0", f.Value)
		}
	} else {
		t.Errorf("sqrt() returned %T, want Float", result)
	}
}

func TestMathPow(t *testing.T) {
	lib := MathLibrary
	pow := lib.Functions()["pow"]

	result := pow.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 2}, &object.Integer{Value: 8})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 256.0 {
			t.Errorf("pow(2, 8) = %v, want 256.0", f.Value)
		}
	} else {
		t.Errorf("pow() returned %T, want Float", result)
	}
}

func TestMathFabs(t *testing.T) {
	lib := MathLibrary
	fabs := lib.Functions()["fabs"]

	result := fabs.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: -5})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 5.0 {
			t.Errorf("fabs(-5) = %v, want 5.0", f.Value)
		}
	} else {
		t.Errorf("fabs() returned %T, want Float", result)
	}

	result = fabs.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: -3.14})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 3.14 {
			t.Errorf("fabs(-3.14) = %v, want 3.14", f.Value)
		}
	} else {
		t.Errorf("fabs() returned %T, want Float", result)
	}
}

func TestMathFloor(t *testing.T) {
	lib := MathLibrary
	floor := lib.Functions()["floor"]

	result := floor.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 3.7})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 3 {
			t.Errorf("floor(3.7) = %v, want 3", i.Value)
		}
	} else {
		t.Errorf("floor() returned %T, want Integer", result)
	}
}

func TestMathCeil(t *testing.T) {
	lib := MathLibrary
	ceil := lib.Functions()["ceil"]

	result := ceil.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 3.2})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 4 {
			t.Errorf("ceil(3.2) = %v, want 4", i.Value)
		}
	} else {
		t.Errorf("ceil() returned %T, want Integer", result)
	}
}

func TestMathConstants(t *testing.T) {
	lib := MathLibrary

	pi := lib.Constants()["pi"]
	if f, ok := pi.(*object.Float); ok {
		if f.Value != math.Pi {
			t.Errorf("math.pi = %v, want %v", f.Value, math.Pi)
		}
	} else {
		t.Errorf("math.pi is %T, want Float", pi)
	}

	e := lib.Constants()["e"]
	if f, ok := e.(*object.Float); ok {
		if f.Value != math.E {
			t.Errorf("math.e = %v, want %v", f.Value, math.E)
		}
	} else {
		t.Errorf("math.e is %T, want Float", e)
	}
}

func TestMathSin(t *testing.T) {
	lib := MathLibrary
	sin := lib.Functions()["sin"]

	result := sin.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 0.0 {
			t.Errorf("sin(0) = %v, want 0.0", f.Value)
		}
	} else {
		t.Errorf("sin() returned %T, want Float", result)
	}
}

func TestMathCos(t *testing.T) {
	lib := MathLibrary
	cos := lib.Functions()["cos"]

	result := cos.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 1.0 {
			t.Errorf("cos(0) = %v, want 1.0", f.Value)
		}
	} else {
		t.Errorf("cos() returned %T, want Float", result)
	}
}

func TestMathTan(t *testing.T) {
	lib := MathLibrary
	tan := lib.Functions()["tan"]

	result := tan.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 0.0 {
			t.Errorf("tan(0) = %v, want 0.0", f.Value)
		}
	} else {
		t.Errorf("tan() returned %T, want Float", result)
	}
}

func TestMathLog(t *testing.T) {
	lib := MathLibrary
	log := lib.Functions()["log"]

	result := log.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 1})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 0.0 {
			t.Errorf("log(1) = %v, want 0.0", f.Value)
		}
	} else {
		t.Errorf("log() returned %T, want Float", result)
	}

	// Test error case
	result = log.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 0})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("log(0) should return error, got %T", result)
	}
}

func TestMathExp(t *testing.T) {
	lib := MathLibrary
	exp := lib.Functions()["exp"]

	result := exp.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 1.0 {
			t.Errorf("exp(0) = %v, want 1.0", f.Value)
		}
	} else {
		t.Errorf("exp() returned %T, want Float", result)
	}
}

func TestMathDegrees(t *testing.T) {
	lib := MathLibrary
	degrees := lib.Functions()["degrees"]

	result := degrees.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: math.Pi})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 180.0 {
			t.Errorf("degrees(π) = %v, want 180.0", f.Value)
		}
	} else {
		t.Errorf("degrees() returned %T, want Float", result)
	}
}

func TestMathRadians(t *testing.T) {
	lib := MathLibrary
	radians := lib.Functions()["radians"]

	result := radians.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 180})
	if f, ok := result.(*object.Float); ok {
		if f.Value != math.Pi {
			t.Errorf("radians(180) = %v, want π", f.Value)
		}
	} else {
		t.Errorf("radians() returned %T, want Float", result)
	}
}

func TestMathFmod(t *testing.T) {
	lib := MathLibrary
	fmod := lib.Functions()["fmod"]

	result := fmod.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 5.5}, &object.Float{Value: 2.0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 1.5 {
			t.Errorf("fmod(5.5, 2.0) = %v, want 1.5", f.Value)
		}
	} else {
		t.Errorf("fmod() returned %T, want Float", result)
	}

	// Test error case
	result = fmod.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 5.0}, &object.Float{Value: 0.0})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("fmod(5.0, 0.0) should return error, got %T", result)
	}
}

func TestMathGcd(t *testing.T) {
	lib := MathLibrary
	gcd := lib.Functions()["gcd"]

	result := gcd.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 48}, &object.Integer{Value: 18})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 6 {
			t.Errorf("gcd(48, 18) = %v, want 6", i.Value)
		}
	} else {
		t.Errorf("gcd() returned %T, want Integer", result)
	}
}

func TestMathFactorial(t *testing.T) {
	lib := MathLibrary
	factorial := lib.Functions()["factorial"]

	result := factorial.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 5})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 120 {
			t.Errorf("factorial(5) = %v, want 120", i.Value)
		}
	} else {
		t.Errorf("factorial() returned %T, want Integer", result)
	}

	// Test error cases
	result = factorial.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: -1})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("factorial(-1) should return error, got %T", result)
	}

	result = factorial.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 21})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("factorial(21) should return error, got %T", result)
	}
}

func TestMathTanh(t *testing.T) {
	lib := MathLibrary
	tanh := lib.Functions()["tanh"]

	result := tanh.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 0.0 {
			t.Errorf("tanh(0) = %v, want 0.0", f.Value)
		}
	} else {
		t.Errorf("tanh() returned %T, want Float", result)
	}

	result = tanh.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 0.0 {
			t.Errorf("tanh(0) = %v, want 0.0", f.Value)
		}
	} else {
		t.Errorf("tanh() returned %T, want Float", result)
	}

	result = tanh.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 1.0})
	if f, ok := result.(*object.Float); ok {
		expected := math.Tanh(1.0)
		if math.Abs(f.Value-expected) > 1e-10 {
			t.Errorf("tanh(1.0) = %v, want %v", f.Value, expected)
		}
	} else {
		t.Errorf("tanh() returned %T, want Float", result)
	}
}

func TestMathSoftmax(t *testing.T) {
	lib := MathLibrary
	softmax := lib.Functions()["softmax"]

	result := softmax.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{
		&object.Float{Value: 1.0},
		&object.Float{Value: 2.0},
		&object.Float{Value: 3.0},
	}})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("softmax() returned %T, want List", result)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("softmax() returned %d elements, want 3", len(list.Elements))
	}
	var sum float64
	vals := make([]float64, 3)
	for i, el := range list.Elements {
		f, ok := el.(*object.Float)
		if !ok {
			t.Fatalf("softmax()[%d] is %T, want Float", i, el)
		}
		vals[i] = f.Value
		sum += f.Value
	}
	if math.Abs(sum-1.0) > 1e-10 {
		t.Errorf("softmax values sum to %v, want 1.0", sum)
	}
	if vals[2] <= vals[1] || vals[1] <= vals[0] {
		t.Errorf("softmax should be monotonically increasing for [1,2,3], got %v", vals)
	}
}

func TestMathSoftmaxNumericalStability(t *testing.T) {
	lib := MathLibrary
	softmax := lib.Functions()["softmax"]

	result := softmax.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{
		&object.Float{Value: 1000.0},
		&object.Float{Value: 1001.0},
		&object.Float{Value: 1002.0},
	}})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("softmax() returned %T, want List", result)
	}
	for _, el := range list.Elements {
		f, ok := el.(*object.Float)
		if !ok {
			t.Fatalf("softmax element is %T, want Float", el)
		}
		if math.IsNaN(f.Value) || math.IsInf(f.Value, 0) {
			t.Errorf("softmax produced NaN or Inf for large inputs: %v", f.Value)
		}
	}
}

func TestMathSoftmaxEmptyError(t *testing.T) {
	lib := MathLibrary
	softmax := lib.Functions()["softmax"]

	result := softmax.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{}})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("softmax([]) should return error, got %T", result)
	}
}

func TestMathDot(t *testing.T) {
	lib := MathLibrary
	dot := lib.Functions()["dot"]

	result := dot.Fn(context.Background(), object.NewKwargs(nil),
		&object.List{Elements: []object.Object{
			&object.Float{Value: 1.0}, &object.Float{Value: 2.0}, &object.Float{Value: 3.0},
		}},
		&object.List{Elements: []object.Object{
			&object.Float{Value: 4.0}, &object.Float{Value: 5.0}, &object.Float{Value: 6.0},
		}},
	)
	f, ok := result.(*object.Float)
	if !ok {
		t.Fatalf("dot() returned %T, want Float", result)
	}
	if f.Value != 32.0 {
		t.Errorf("dot([1,2,3],[4,5,6]) = %v, want 32.0", f.Value)
	}
}

func TestMathDotMismatchedLength(t *testing.T) {
	lib := MathLibrary
	dot := lib.Functions()["dot"]

	result := dot.Fn(context.Background(), object.NewKwargs(nil),
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}}},
	)
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("dot() with mismatched lengths should return error, got %T", result)
	}
}

func TestMathDotEmpty(t *testing.T) {
	lib := MathLibrary
	dot := lib.Functions()["dot"]

	result := dot.Fn(context.Background(), object.NewKwargs(nil),
		&object.List{Elements: []object.Object{}},
		&object.List{Elements: []object.Object{}},
	)
	f, ok := result.(*object.Float)
	if !ok {
		t.Fatalf("dot() returned %T, want Float", result)
	}
	if f.Value != 0.0 {
		t.Errorf("dot([],[]) = %v, want 0.0", f.Value)
	}
}

func TestMathMatmul(t *testing.T) {
	lib := MathLibrary
	matmul := lib.Functions()["matmul"]

	a := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 3.0}, &object.Float{Value: 4.0}}},
	}}
	b := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 5.0}, &object.Float{Value: 6.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 7.0}, &object.Float{Value: 8.0}}},
	}}

	result := matmul.Fn(context.Background(), object.NewKwargs(nil), a, b)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("matmul() returned %T, want List", result)
	}
	if len(list.Elements) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(list.Elements))
	}

	row0 := list.Elements[0].(*object.List)
	row1 := list.Elements[1].(*object.List)

	if row0.Elements[0].(*object.Float).Value != 19.0 {
		t.Errorf("matmul[0][0] = %v, want 19.0", row0.Elements[0].(*object.Float).Value)
	}
	if row0.Elements[1].(*object.Float).Value != 22.0 {
		t.Errorf("matmul[0][1] = %v, want 22.0", row0.Elements[1].(*object.Float).Value)
	}
	if row1.Elements[0].(*object.Float).Value != 43.0 {
		t.Errorf("matmul[1][0] = %v, want 43.0", row1.Elements[0].(*object.Float).Value)
	}
	if row1.Elements[1].(*object.Float).Value != 50.0 {
		t.Errorf("matmul[1][1] = %v, want 50.0", row1.Elements[1].(*object.Float).Value)
	}
}

func TestMathMatmulDimensionMismatch(t *testing.T) {
	lib := MathLibrary
	matmul := lib.Functions()["matmul"]

	a := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}}},
	}}
	b := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 2.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 3.0}}},
	}}

	result := matmul.Fn(context.Background(), object.NewKwargs(nil), a, b)
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("matmul() with dimension mismatch should return error, got %T", result)
	}
}

func TestMathTranspose(t *testing.T) {
	lib := MathLibrary
	transpose := lib.Functions()["transpose"]

	m := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}, &object.Float{Value: 3.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 4.0}, &object.Float{Value: 5.0}, &object.Float{Value: 6.0}}},
	}}

	result := transpose.Fn(context.Background(), object.NewKwargs(nil), m)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("transpose() returned %T, want List", result)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("expected 3 rows after transpose, got %d", len(list.Elements))
	}

	expected := [][]float64{{1.0, 4.0}, {2.0, 5.0}, {3.0, 6.0}}
	for i, row := range list.Elements {
		r := row.(*object.List)
		for j, el := range r.Elements {
			v := el.(*object.Float).Value
			if v != expected[i][j] {
				t.Errorf("transpose()[%d][%d] = %v, want %v", i, j, v, expected[i][j])
			}
		}
	}
}

func TestMathTransposeEmpty(t *testing.T) {
	lib := MathLibrary
	transpose := lib.Functions()["transpose"]

	result := transpose.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{}})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("transpose() returned %T, want List", result)
	}
	if len(list.Elements) != 0 {
		t.Errorf("transpose of empty should be empty, got %d elements", len(list.Elements))
	}
}

func TestMathMatAdd(t *testing.T) {
	lib := MathLibrary
	matAdd := lib.Functions()["mat_add"]

	a := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 3.0}, &object.Float{Value: 4.0}}},
	}}
	b := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 5.0}, &object.Float{Value: 6.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 7.0}, &object.Float{Value: 8.0}}},
	}}

	result := matAdd.Fn(context.Background(), object.NewKwargs(nil), a, b)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("mat_add() returned %T, want List", result)
	}

	expected := [][]float64{{6.0, 8.0}, {10.0, 12.0}}
	for i, row := range list.Elements {
		r := row.(*object.List)
		for j, el := range r.Elements {
			v := el.(*object.Float).Value
			if v != expected[i][j] {
				t.Errorf("mat_add()[%d][%d] = %v, want %v", i, j, v, expected[i][j])
			}
		}
	}
}

func TestMathMatAddShapeMismatch(t *testing.T) {
	lib := MathLibrary
	matAdd := lib.Functions()["mat_add"]

	a := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}}},
	}}
	b := &object.List{Elements: []object.Object{
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}, &object.Float{Value: 3.0}}},
	}}

	result := matAdd.Fn(context.Background(), object.NewKwargs(nil), a, b)
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("mat_add() with shape mismatch should return error, got %T", result)
	}
}

func TestMathDotWithIntegers(t *testing.T) {
	lib := MathLibrary
	dot := lib.Functions()["dot"]

	result := dot.Fn(context.Background(), object.NewKwargs(nil),
		&object.List{Elements: []object.Object{
			object.NewInteger(1), object.NewInteger(2), object.NewInteger(3),
		}},
		&object.List{Elements: []object.Object{
			object.NewInteger(4), object.NewInteger(5), object.NewInteger(6),
		}},
	)
	f, ok := result.(*object.Float)
	if !ok {
		t.Fatalf("dot() returned %T, want Float", result)
	}
	if f.Value != 32.0 {
		t.Errorf("dot([1,2,3],[4,5,6]) with ints = %v, want 32.0", f.Value)
	}
}

func TestMathErf(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["erf"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.0})
	if f, ok := result.(*object.Float); !ok || f.Value != 0.0 {
		t.Errorf("erf(0) = %v, want 0.0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 1})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-math.Erf(1.0)) > 1e-10 {
		t.Errorf("erf(1) = %v, want %v", result, math.Erf(1.0))
	}
}

func TestMathErfc(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["erfc"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.0})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-1.0) > 1e-10 {
		t.Errorf("erfc(0) = %v, want 1.0", result)
	}
}

func TestMathGamma(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["gamma"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 5})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-24.0) > 1e-10 {
		t.Errorf("gamma(5) = %v, want 24.0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.5})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-math.Sqrt(math.Pi)) > 1e-10 {
		t.Errorf("gamma(0.5) = %v, want %v", result, math.Sqrt(math.Pi))
	}
}

func TestMathLgamma(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["lgamma"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Integer{Value: 5})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("lgamma() returned %T, want List", result)
	}
	val := list.Elements[0].(*object.Float).Value
	if math.Abs(val-math.Log(24.0)) > 1e-10 {
		t.Errorf("lgamma(5)[0] = %v, want %v", val, math.Log(24.0))
	}
	sign := list.Elements[1].(*object.Integer).Value
	if sign != 1 {
		t.Errorf("lgamma(5)[1] = %d, want 1", sign)
	}
}

func TestMathNextafter(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["nextafter"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.0}, &object.Float{Value: 1.0})
	if f, ok := result.(*object.Float); !ok || f.Value <= 0.0 {
		t.Errorf("nextafter(0, 1) = %v, want > 0", result)
	}
}

func TestMathCbrt(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["cbrt"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 27.0})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-3.0) > 1e-10 {
		t.Errorf("cbrt(27) = %v, want 3.0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: -8.0})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-(-2.0)) > 1e-10 {
		t.Errorf("cbrt(-8) = %v, want -2.0", result)
	}
}

func TestMathRemainder(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["remainder"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 7.5}, &object.Float{Value: 2.5})
	if f, ok := result.(*object.Float); !ok || f.Value != 0.0 {
		t.Errorf("remainder(7.5, 2.5) = %v, want 0.0", result)
	}
}

func TestMathLog1p(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["log1p"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.0})
	if f, ok := result.(*object.Float); !ok || f.Value != 0.0 {
		t.Errorf("log1p(0) = %v, want 0.0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: math.E - 1})
	if f, ok := result.(*object.Float); !ok || math.Abs(f.Value-1.0) > 1e-10 {
		t.Errorf("log1p(e-1) = %v, want 1.0", result)
	}
}

func TestMathExpm1(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["expm1"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Float{Value: 0.0})
	if f, ok := result.(*object.Float); !ok || f.Value != 0.0 {
		t.Errorf("expm1(0) = %v, want 0.0", result)
	}
}

func TestMathComb(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["comb"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(5), object.NewInteger(2))
	if i, ok := result.(*object.Integer); !ok || i.Value != 10 {
		t.Errorf("comb(5, 2) = %v, want 10", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(10), object.NewInteger(0))
	if i, ok := result.(*object.Integer); !ok || i.Value != 1 {
		t.Errorf("comb(10, 0) = %v, want 1", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(5), object.NewInteger(6))
	if i, ok := result.(*object.Integer); !ok || i.Value != 0 {
		t.Errorf("comb(5, 6) = %v, want 0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(-1), object.NewInteger(0))
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("comb(-1, 0) should return error, got %T", result)
	}
}

func TestMathPerm(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["perm"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(5), object.NewInteger(2))
	if i, ok := result.(*object.Integer); !ok || i.Value != 20 {
		t.Errorf("perm(5, 2) = %v, want 20", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), object.NewInteger(5))
	if i, ok := result.(*object.Integer); !ok || i.Value != 120 {
		t.Errorf("perm(5) = %v, want 120", result)
	}
}

func TestMathProd(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["prod"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{
		object.NewInteger(2), object.NewInteger(3), object.NewInteger(4),
	}})
	if i, ok := result.(*object.Integer); !ok || i.Value != 24 {
		t.Errorf("prod([2,3,4]) = %v, want 24", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{}})
	if i, ok := result.(*object.Integer); !ok || i.Value != 1 {
		t.Errorf("prod([]) = %v, want 1", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil), &object.List{Elements: []object.Object{
		&object.Float{Value: 1.5}, &object.Float{Value: 2.0},
	}})
	if f, ok := result.(*object.Float); !ok || f.Value != 3.0 {
		t.Errorf("prod([1.5, 2.0]) = %v, want 3.0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"start": &object.Float{Value: 0.5},
	}), &object.List{Elements: []object.Object{
		object.NewInteger(2), object.NewInteger(3),
	}})
	if f, ok := result.(*object.Float); !ok || f.Value != 3.0 {
		t.Errorf("prod([2,3], start=0.5) = %v, want 3.0", result)
	}
}

func TestMathDist(t *testing.T) {
	lib := MathLibrary
	fn := lib.Functions()["dist"]

	result := fn.Fn(context.Background(), object.NewKwargs(nil),
		&object.List{Elements: []object.Object{&object.Float{Value: 0.0}, &object.Float{Value: 0.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 3.0}, &object.Float{Value: 4.0}}},
	)
	if f, ok := result.(*object.Float); !ok || f.Value != 5.0 {
		t.Errorf("dist([0,0], [3,4]) = %v, want 5.0", result)
	}

	result = fn.Fn(context.Background(), object.NewKwargs(nil),
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}}},
		&object.List{Elements: []object.Object{&object.Float{Value: 1.0}, &object.Float{Value: 2.0}}},
	)
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("dist with different dimensions should return error")
	}
}

func TestMathTau(t *testing.T) {
	lib := MathLibrary
	tau := lib.Constants()["tau"]
	if f, ok := tau.(*object.Float); !ok || math.Abs(f.Value-2*math.Pi) > 1e-10 {
		t.Errorf("math.tau = %v, want 2*pi", tau)
	}
}
