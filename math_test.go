package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func TestMathLibrary(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected interface{}
	}{
		{"sqrt", "import math\nresult = math.sqrt(16)", 4.0},
		{"pow", "import math\nresult = math.pow(2, 8)", 256.0},
		{"fabs int", "import math\nresult = math.fabs(-5)", 5.0},
		{"fabs float", "import math\nresult = math.fabs(-5.5)", 5.5},
		{"floor", "import math\nresult = math.floor(3.7)", int64(3)},
		{"ceil", "import math\nresult = math.ceil(3.2)", int64(4)},
		{"pi", "import math\nresult = math.pi", 3.141592653589793},
		{"e", "import math\nresult = math.e", 2.718281828459045},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			p.RegisterLibrary(stdlib.MathLibrary)
			_, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}

			result, objErr := p.GetVar("result")
			if objErr != nil {
				t.Fatal("result variable not found")
			}

			switch expected := tt.expected.(type) {
			case int64:
				if result != expected {
					t.Errorf("got %v, want %v", result, expected)
				}
			case float64:
				if fResult, ok := result.(float64); ok {
					if fResult != expected {
						t.Errorf("got %v, want %v", fResult, expected)
					}
				} else {
					t.Errorf("result is %T, want float64", result)
				}
			}
		})
	}
}

func TestMathInExpression(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math

# Calculate circle area
radius = 5
area = math.pi * math.pow(radius, 2)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	area, objErr := p.GetVar("area")
	if objErr != nil {
		t.Fatal("area variable not found")
	}

	expected := 78.53981633974483
	if fArea, ok := area.(float64); ok {
		if fArea != expected {
			t.Errorf("area = %v, want %v", fArea, expected)
		}
	} else {
		t.Errorf("area is %T, want float64", area)
	}
}

func TestFloatArrayMatmul(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = [[1.0, 2.0], [3.0, 4.0]]
b = [[5.0, 6.0], [7.0, 8.0]]
result = math.matmul(a, b)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	result, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatal("result variable not found:", objErr)
	}
	list, ok := result.([]interface{})
	if !ok {
		t.Fatalf("result is %T, want []interface{} (list)", result)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(list))
	}
	row0 := list[0].([]interface{})
	if row0[0].(float64) != 19.0 || row0[1].(float64) != 22.0 {
		t.Errorf("row0 = %v, want [19 22]", row0)
	}
}

func TestFloatArrayIndexing2D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]])
row0 = m[0]
row1 = m[-1]
val = m[1][2]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	row0, _ := p.GetVarAsObject("row0")
	fa, ok := row0.(*object.FloatArray)
	if !ok {
		t.Fatalf("row0 is %T, want FloatArray", row0)
	}
	if len(fa.Data) != 3 || fa.Data[0] != 1.0 || fa.Data[1] != 2.0 || fa.Data[2] != 3.0 {
		t.Errorf("row0 = %v, want [1 2 3]", fa.Data)
	}

	row1, _ := p.GetVarAsObject("row1")
	fa1, ok := row1.(*object.FloatArray)
	if !ok {
		t.Fatalf("row1 is %T, want FloatArray", row1)
	}
	if len(fa1.Data) != 3 || fa1.Data[0] != 4.0 {
		t.Errorf("row1 = %v, want [4 5 6]", fa1.Data)
	}

	val, _ := p.GetVar("val")
	if val != 6.0 {
		t.Errorf("val = %v, want 6.0", val)
	}
}

func TestFloatArrayIndexAssignment(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0]])
m[0] = [10.0, 20.0]
val = m[0][0]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	val, _ := p.GetVar("val")
	if val != 10.0 {
		t.Errorf("val = %v, want 10.0", val)
	}
}

func TestFloatArrayNestedIndexAssignment(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0]])
m[0][1] = 9.0
val = m[0][1]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	val, _ := p.GetVar("val")
	if val != 9.0 {
		t.Errorf("val = %v, want 9.0", val)
	}
}

func TestFloatArrayRowAssignmentRejects2DArray(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0]])
m[0] = math.array([[9.0, 8.0]])
`)
	if err == nil {
		t.Fatal("expected row assignment with 2D FloatArray to fail")
	}
}

func TestFloatArraySlicing(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0], [5.0, 6.0]])
sub = m[0:2]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	sub, _ := p.GetVarAsObject("sub")
	fa, ok := sub.(*object.FloatArray)
	if !ok {
		t.Fatalf("sub is %T, want FloatArray", sub)
	}
	if !fa.Is2D() || fa.Rows() != 2 || fa.Cols() != 2 {
		t.Fatalf("shape = %v, want [2,2]", fa.Shape)
	}
	if fa.Data[0] != 1.0 || fa.Data[2] != 3.0 {
		t.Errorf("sub data = %v, want [1 2 3 4]", fa.Data)
	}
}

func TestFloatArrayLen(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0]])
n = len(m)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	n, _ := p.GetVar("n")
	if n != int64(2) {
		t.Errorf("len(m) = %v, want 2", n)
	}
}

func TestFloatArrayForIteration(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0]])
total = 0.0
for row in m:
    total = total + row[0]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	total, _ := p.GetVar("total")
	if total != 4.0 {
		t.Errorf("total = %v, want 4.0", total)
	}
}

func TestFloatArrayEquality(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
b = math.array([1.0, 2.0, 3.0])
c = math.array([1.0, 2.0, 4.0])
eq = a == b
neq = a == c
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	eq, _ := p.GetVar("eq")
	if eq != true {
		t.Errorf("a == b = %v, want true", eq)
	}
	neq, _ := p.GetVar("neq")
	if neq != false {
		t.Errorf("a == c = %v, want false", neq)
	}
}

func TestFloatArrayInOperator(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
found = 2.0 in a
missing = 5.0 in a
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	found, _ := p.GetVar("found")
	if found != true {
		t.Errorf("2.0 in a = %v, want true", found)
	}
	missing, _ := p.GetVar("missing")
	if missing != false {
		t.Errorf("5.0 in a = %v, want false", missing)
	}
}

func TestFloatArrayListConversion(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
l = list(a)
n = len(l)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	n, _ := p.GetVar("n")
	if n != int64(3) {
		t.Errorf("len(list(a)) = %v, want 3", n)
	}
}

func TestFloatArrayReversed(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
r = list(reversed(a))
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	r, _ := p.GetVar("r")
	list, ok := r.([]interface{})
	if !ok {
		t.Fatalf("r is %T, want []interface{}", r)
	}
	if len(list) != 3 || list[0] != 3.0 || list[1] != 2.0 || list[2] != 1.0 {
		t.Errorf("r = %v, want [3 2 1]", r)
	}
}

func TestFloatArraySoftmaxSum(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
s = math.softmax([1.0, 2.0, 3.0])
total = 0.0
for v in s:
    total = total + v
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	total, _ := p.GetVar("total")
	f, ok := total.(float64)
	if !ok {
		t.Fatalf("total is %T, want float64", total)
	}
	if f < 0.999 || f > 1.001 {
		t.Errorf("softmax sum = %v, want ~1.0", f)
	}
}

func TestFloatArraySum(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
total = sum(a)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	total, _ := p.GetVar("total")
	if total != 6.0 {
		t.Errorf("sum(a) = %v, want 6.0", total)
	}
}

func TestFloatArrayFloatConversionRejected(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0])
v = float(a)
`)
	if err == nil {
		t.Fatal("expected float(FloatArray) to fail")
	}
}

func TestFloatArrayMixedRowTypesRejected(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[1.0, 2.0], math.array([3.0, 4.0])])
`)
	if err == nil {
		t.Fatal("expected mixed row types in math.array() to fail")
	}
}

func TestFloatArrayPipeline(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[1.0, 2.0], [3.0, 4.0]])
b = math.transpose(a)
c = math.matmul(a, b)
s = math.shape(c)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	s, _ := p.GetVar("s")
	list, ok := s.([]interface{})
	if !ok {
		t.Fatalf("shape is %T, want []interface{}", s)
	}
	if list[0].(int64) != 2 || list[1].(int64) != 2 {
		t.Errorf("shape = %v, want [2,2]", s)
	}
}

func TestFloatArrayMatmulWithArrayInput(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[1.0, 2.0], [3.0, 4.0]])
b = math.array([[5.0, 6.0], [7.0, 8.0]])
result = math.matmul(a, b)
s = math.shape(result)
val = result[0][0]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	s, _ := p.GetVar("s")
	shape, ok := s.([]interface{})
	if !ok {
		t.Fatalf("shape is %T, want []interface{}", s)
	}
	if shape[0].(int64) != 2 || shape[1].(int64) != 2 {
		t.Errorf("shape = %v, want [2,2]", s)
	}
	val, _ := p.GetVar("val")
	if val != 19.0 {
		t.Errorf("result[0][0] = %v, want 19.0", val)
	}
}

func TestFloatArrayTransposeWithArrayInput(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]])
t = math.transpose(a)
s = math.shape(t)
val = t[0][1]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	s, _ := p.GetVar("s")
	shape, ok := s.([]interface{})
	if !ok {
		t.Fatalf("shape is %T, want []interface{}", s)
	}
	if shape[0].(int64) != 3 || shape[1].(int64) != 2 {
		t.Errorf("shape = %v, want [3,2]", s)
	}
	val, _ := p.GetVar("val")
	if val != 4.0 {
		t.Errorf("t[0][1] = %v, want 4.0", val)
	}
}

func TestFloatArrayTransposeZeroWidthPreservesType(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[]])
t = math.transpose(a)
is_fa = type(t) == "FLOAT_ARRAY"
s = math.shape(t)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	isFA, _ := p.GetVar("is_fa")
	if isFA != true {
		t.Fatalf("transpose result type = %v, want FLOAT_ARRAY", isFA)
	}
	s, _ := p.GetVar("s")
	shape, ok := s.([]interface{})
	if !ok {
		t.Fatalf("shape is %T, want []interface{}", s)
	}
	if len(shape) != 2 || shape[0].(int64) != 0 || shape[1].(int64) != 1 {
		t.Errorf("shape = %v, want [0,1]", s)
	}
}

func TestFloatArraySoftmaxWithArrayInput(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
s = math.softmax(a)
total = sum(s)
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	total, _ := p.GetVar("total")
	f, ok := total.(float64)
	if !ok {
		t.Fatalf("total is %T, want float64", total)
	}
	if f < 0.999 || f > 1.001 {
		t.Errorf("softmax sum = %v, want ~1.0", f)
	}
}

func TestListInputReturnsList(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = [[1.0, 2.0], [3.0, 4.0]]
r = math.transpose(a)
is_list = type(r) == "LIST"
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	isList, _ := p.GetVar("is_list")
	if isList != true {
		t.Errorf("type(transpose(list)) should be LIST, got %v", isList)
	}
}

func TestFloatArrayInputReturnsFloatArray(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[1.0, 2.0], [3.0, 4.0]])
r = math.transpose(a)
is_fa = type(r) == "FLOAT_ARRAY"
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	isFA, _ := p.GetVar("is_fa")
	if isFA != true {
		t.Errorf("type(transpose(FloatArray)) should be FLOAT_ARRAY, got %v", isFA)
	}
}

func TestFloatArrayListComprehension2D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]])
firsts = [row[0] for row in m]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	firsts, _ := p.GetVar("firsts")
	list, ok := firsts.([]interface{})
	if !ok {
		t.Fatalf("firsts is %T, want []interface{}", firsts)
	}
	if len(list) != 2 {
		t.Fatalf("len(firsts) = %d, want 2", len(list))
	}
	if list[0] != 1.0 || list[1] != 4.0 {
		t.Errorf("firsts = %v, want [1.0, 4.0]", list)
	}
}

func TestFloatArrayListComprehension1D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0, 4.0])
doubled = [v * 2 for v in a]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	doubled, _ := p.GetVar("doubled")
	list, ok := doubled.([]interface{})
	if !ok {
		t.Fatalf("doubled is %T, want []interface{}", doubled)
	}
	if len(list) != 4 {
		t.Fatalf("len(doubled) = %d, want 4", len(list))
	}
	if list[0] != 2.0 || list[3] != 8.0 {
		t.Errorf("doubled = %v, want [2.0, 4.0, 6.0, 8.0]", list)
	}
}

func TestFloatArrayListComprehensionWithFilter(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0, 4.0, 5.0])
big = [v for v in a if v > 2.5]
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	big, _ := p.GetVar("big")
	list, ok := big.([]interface{})
	if !ok {
		t.Fatalf("big is %T, want []interface{}", big)
	}
	if len(list) != 3 {
		t.Fatalf("len(big) = %d, want 3", len(list))
	}
	if list[0] != 3.0 || list[1] != 4.0 || list[2] != 5.0 {
		t.Errorf("big = %v, want [3.0, 4.0, 5.0]", list)
	}
}

func TestFloatArrayConcatenation2D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([[1.0, 2.0], [3.0, 4.0]])
b = math.array([[5.0, 6.0]])
c = a + b
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	c, _ := p.GetVarAsObject("c")
	fa, ok := c.(*object.FloatArray)
	if !ok {
		t.Fatalf("c is %T, want FloatArray", c)
	}
	if fa.Rows() != 3 || fa.Cols() != 2 {
		t.Fatalf("shape = %v, want [3,2]", fa.Shape)
	}
	if fa.Data[0] != 1.0 || fa.Data[4] != 5.0 || fa.Data[5] != 6.0 {
		t.Errorf("data = %v", fa.Data)
	}
}

func TestFloatArrayConcatenation1D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0])
b = math.array([3.0, 4.0])
c = a + b
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	c, _ := p.GetVarAsObject("c")
	fa, ok := c.(*object.FloatArray)
	if !ok {
		t.Fatalf("c is %T, want FloatArray", c)
	}
	if len(fa.Data) != 4 {
		t.Fatalf("len(data) = %d, want 4", len(fa.Data))
	}
	if fa.Data[0] != 1.0 || fa.Data[3] != 4.0 {
		t.Errorf("data = %v, want [1 2 3 4]", fa.Data)
	}
}

func TestFloatArrayTolist1D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
l = a.tolist()
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	l, _ := p.GetVar("l")
	list, ok := l.([]interface{})
	if !ok {
		t.Fatalf("l is %T, want []interface{}", l)
	}
	if len(list) != 3 || list[0] != 1.0 || list[2] != 3.0 {
		t.Errorf("l = %v, want [1.0, 2.0, 3.0]", list)
	}
}

func TestFloatArrayTolist2D(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0], [3.0, 4.0]])
l = m.tolist()
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	l, _ := p.GetVar("l")
	list, ok := l.([]interface{})
	if !ok {
		t.Fatalf("l is %T, want []interface{}", l)
	}
	if len(list) != 2 {
		t.Fatalf("len(l) = %d, want 2", len(list))
	}
	row0 := list[0].([]interface{})
	if row0[0] != 1.0 || row0[1] != 2.0 {
		t.Errorf("row0 = %v, want [1.0, 2.0]", row0)
	}
}

func TestFloatArrayShapeMethod(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval(`
import math
m = math.array([[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]])
s = m.shape()
`)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	s, _ := p.GetVar("s")
	list, ok := s.([]interface{})
	if !ok {
		t.Fatalf("s is %T, want []interface{}", s)
	}
	if list[0].(int64) != 2 || list[1].(int64) != 3 {
		t.Errorf("shape = %v, want [2, 3]", list)
	}
}
