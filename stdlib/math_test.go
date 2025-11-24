package stdlib

import (
	"testing"
	"github.com/paularlott/scriptling/object"
	"math"
)

func TestMathSqrt(t *testing.T) {
	lib := GetMathLibrary()
	sqrt := lib["sqrt"]
	
	result := sqrt.Fn(&object.Integer{Value: 16})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 4.0 {
			t.Errorf("sqrt(16) = %v, want 4.0", f.Value)
		}
	} else {
		t.Errorf("sqrt() returned %T, want Float", result)
	}
}

func TestMathPow(t *testing.T) {
	lib := GetMathLibrary()
	pow := lib["pow"]
	
	result := pow.Fn(&object.Integer{Value: 2}, &object.Integer{Value: 8})
	if f, ok := result.(*object.Float); ok {
		if f.Value != 256.0 {
			t.Errorf("pow(2, 8) = %v, want 256.0", f.Value)
		}
	} else {
		t.Errorf("pow() returned %T, want Float", result)
	}
}

func TestMathAbs(t *testing.T) {
	lib := GetMathLibrary()
	abs := lib["abs"]
	
	result := abs.Fn(&object.Integer{Value: -5})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 5 {
			t.Errorf("abs(-5) = %v, want 5", i.Value)
		}
	} else {
		t.Errorf("abs() returned %T, want Integer", result)
	}
}

func TestMathFloor(t *testing.T) {
	lib := GetMathLibrary()
	floor := lib["floor"]
	
	result := floor.Fn(&object.Float{Value: 3.7})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 3 {
			t.Errorf("floor(3.7) = %v, want 3", i.Value)
		}
	} else {
		t.Errorf("floor() returned %T, want Integer", result)
	}
}

func TestMathCeil(t *testing.T) {
	lib := GetMathLibrary()
	ceil := lib["ceil"]
	
	result := ceil.Fn(&object.Float{Value: 3.2})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 4 {
			t.Errorf("ceil(3.2) = %v, want 4", i.Value)
		}
	} else {
		t.Errorf("ceil() returned %T, want Integer", result)
	}
}

func TestMathRound(t *testing.T) {
	lib := GetMathLibrary()
	round := lib["round"]
	
	result := round.Fn(&object.Float{Value: 3.5})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 4 {
			t.Errorf("round(3.5) = %v, want 4", i.Value)
		}
	} else {
		t.Errorf("round() returned %T, want Integer", result)
	}
}

func TestMathMin(t *testing.T) {
	lib := GetMathLibrary()
	min := lib["min"]
	
	result := min.Fn(&object.Integer{Value: 3}, &object.Integer{Value: 1}, &object.Integer{Value: 2})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 1 {
			t.Errorf("min(3, 1, 2) = %v, want 1", i.Value)
		}
	} else {
		t.Errorf("min() returned %T, want Integer", result)
	}
}

func TestMathMax(t *testing.T) {
	lib := GetMathLibrary()
	max := lib["max"]
	
	result := max.Fn(&object.Integer{Value: 3}, &object.Integer{Value: 1}, &object.Integer{Value: 2})
	if i, ok := result.(*object.Integer); ok {
		if i.Value != 3 {
			t.Errorf("max(3, 1, 2) = %v, want 3", i.Value)
		}
	} else {
		t.Errorf("max() returned %T, want Integer", result)
	}
}

func TestMathConstants(t *testing.T) {
	lib := GetMathLibrary()
	
	pi := lib["pi"].Fn()
	if f, ok := pi.(*object.Float); ok {
		if f.Value != math.Pi {
			t.Errorf("math.pi = %v, want %v", f.Value, math.Pi)
		}
	} else {
		t.Errorf("math.pi is %T, want Float", pi)
	}
	
	e := lib["e"].Fn()
	if f, ok := e.(*object.Float); ok {
		if f.Value != math.E {
			t.Errorf("math.e = %v, want %v", f.Value, math.E)
		}
	} else {
		t.Errorf("math.e is %T, want Float", e)
	}
}
