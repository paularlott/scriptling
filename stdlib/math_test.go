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

	result := sqrt.Fn(context.Background(), nil, &object.Integer{Value: 16})
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

	result := pow.Fn(context.Background(), nil, &object.Integer{Value: 2}, &object.Integer{Value: 8})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 256.0 {
			t.Errorf("pow(2, 8) = %v, want 256.0", f.Value)
		}
	} else {
		t.Errorf("pow() returned %T, want Float", result)
	}
}

func TestMathAbs(t *testing.T) {
	lib := MathLibrary
	abs := lib.Functions()["abs"]

	result := abs.Fn(context.Background(), nil, &object.Integer{Value: -5})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 5 {
			t.Errorf("abs(-5) = %v, want 5", i.Value)
		}
	} else {
		t.Errorf("abs() returned %T, want Integer", result)
	}
}

func TestMathFloor(t *testing.T) {
	lib := MathLibrary
	floor := lib.Functions()["floor"]

	result := floor.Fn(context.Background(), nil, &object.Float{Value: 3.7})
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

	result := ceil.Fn(context.Background(), nil, &object.Float{Value: 3.2})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 4 {
			t.Errorf("ceil(3.2) = %v, want 4", i.Value)
		}
	} else {
		t.Errorf("ceil() returned %T, want Integer", result)
	}
}

func TestMathRound(t *testing.T) {
	lib := MathLibrary
	round := lib.Functions()["round"]

	result := round.Fn(context.Background(), nil, &object.Float{Value: 3.5})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 4 {
			t.Errorf("round(3.5) = %v, want 4", i.Value)
		}
	} else {
		t.Errorf("round() returned %T, want Integer", result)
	}
}

func TestMathMin(t *testing.T) {
	lib := MathLibrary
	min := lib.Functions()["min"]

	result := min.Fn(context.Background(), nil, &object.Integer{Value: 3}, &object.Integer{Value: 1})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 1 {
			t.Errorf("min(3, 1, 2) = %v, want 1", i.Value)
		}
	} else {
		t.Errorf("min() returned %T, want Integer", result)
	}
}

func TestMathMax(t *testing.T) {
	lib := MathLibrary
	max := lib.Functions()["max"]

	result := max.Fn(context.Background(), nil, &object.Integer{Value: 3}, &object.Integer{Value: 1})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 3 {
			t.Errorf("max(3, 1, 2) = %v, want 3", i.Value)
		}
	} else {
		t.Errorf("max() returned %T, want Integer", result)
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

	result := sin.Fn(context.Background(), nil, &object.Integer{Value: 0})
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

	result := cos.Fn(context.Background(), nil, &object.Integer{Value: 0})
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

	result := tan.Fn(context.Background(), nil, &object.Integer{Value: 0})
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

	result := log.Fn(context.Background(), nil, &object.Integer{Value: 1})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 0.0 {
			t.Errorf("log(1) = %v, want 0.0", f.Value)
		}
	} else {
		t.Errorf("log() returned %T, want Float", result)
	}

	// Test error case
	result = log.Fn(context.Background(), nil, &object.Integer{Value: 0})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("log(0) should return error, got %T", result)
	}
}

func TestMathExp(t *testing.T) {
	lib := MathLibrary
	exp := lib.Functions()["exp"]

	result := exp.Fn(context.Background(), nil, &object.Integer{Value: 0})
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

	result := degrees.Fn(context.Background(), nil, &object.Float{Value: math.Pi})
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

	result := radians.Fn(context.Background(), nil, &object.Integer{Value: 180})
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

	result := fmod.Fn(context.Background(), nil, &object.Float{Value: 5.5}, &object.Float{Value: 2.0})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 1.5 {
			t.Errorf("fmod(5.5, 2.0) = %v, want 1.5", f.Value)
		}
	} else {
		t.Errorf("fmod() returned %T, want Float", result)
	}

	// Test error case
	result = fmod.Fn(context.Background(), nil, &object.Float{Value: 5.0}, &object.Float{Value: 0.0})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("fmod(5.0, 0.0) should return error, got %T", result)
	}
}

func TestMathGcd(t *testing.T) {
	lib := MathLibrary
	gcd := lib.Functions()["gcd"]

	result := gcd.Fn(context.Background(), nil, &object.Integer{Value: 48}, &object.Integer{Value: 18})
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

	result := factorial.Fn(context.Background(), nil, &object.Integer{Value: 5})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 120 {
			t.Errorf("factorial(5) = %v, want 120", i.Value)
		}
	} else {
		t.Errorf("factorial() returned %T, want Integer", result)
	}

	// Test error cases
	result = factorial.Fn(context.Background(), nil, &object.Integer{Value: -1})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("factorial(-1) should return error, got %T", result)
	}

	result = factorial.Fn(context.Background(), nil, &object.Integer{Value: 21})
	if _, ok := result.(*object.Error); !ok {
		t.Errorf("factorial(21) should return error, got %T", result)
	}
}
