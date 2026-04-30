package object

import (
	"fmt"
	"strings"
)

type FloatArray struct {
	Data  []float64
	Shape []int
}

func NewFloatArray1D(data []float64) *FloatArray {
	if data == nil {
		data = []float64{}
	}
	return &FloatArray{Data: data, Shape: []int{len(data)}}
}

func NewFloatArray2D(data []float64, rows, cols int) *FloatArray {
	return &FloatArray{Data: data, Shape: []int{rows, cols}}
}

func (fa *FloatArray) Type() ObjectType { return FLOAT_ARRAY_OBJ }
func (fa *FloatArray) Inspect() string {
	return fa.PrettyPrint()
}

func (fa *FloatArray) AsString() (string, Object)          { return fa.Inspect(), nil }
func (fa *FloatArray) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (fa *FloatArray) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (fa *FloatArray) AsBool() (bool, Object)              { return len(fa.Data) > 0, nil }
func (fa *FloatArray) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (fa *FloatArray) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (fa *FloatArray) CoerceString() (string, Object) { return fa.Inspect(), nil }
func (fa *FloatArray) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (fa *FloatArray) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

func (fa *FloatArray) FloatArrayData() ([]float64, []int, bool) {
	return fa.Data, fa.Shape, true
}

func (fa *FloatArray) Rows() int {
	if len(fa.Shape) >= 2 {
		return fa.Shape[0]
	}
	return 1
}

func (fa *FloatArray) Cols() int {
	if len(fa.Shape) >= 2 {
		return fa.Shape[1]
	}
	return fa.Shape[0]
}

func (fa *FloatArray) Row(i int) []float64 {
	if len(fa.Shape) < 2 {
		return fa.Data
	}
	cols := fa.Shape[1]
	start := i * cols
	return fa.Data[start : start+cols]
}

func (fa *FloatArray) Is2D() bool {
	return len(fa.Shape) >= 2
}

func (fa *FloatArray) ToList() *List {
	if len(fa.Shape) < 2 {
		elems := make([]Object, len(fa.Data))
		for i, v := range fa.Data {
			elems[i] = &Float{Value: v}
		}
		return &List{Elements: elems}
	}
	rows := fa.Shape[0]
	cols := fa.Shape[1]
	rowList := make([]Object, rows)
	for i := 0; i < rows; i++ {
		elems := make([]Object, cols)
		for j := 0; j < cols; j++ {
			elems[j] = &Float{Value: fa.Data[i*cols+j]}
		}
		rowList[i] = &List{Elements: elems}
	}
	return &List{Elements: rowList}
}

func (fa *FloatArray) PrettyPrint() string {
	if !fa.Is2D() {
		var b strings.Builder
		b.WriteString("[")
		for i, v := range fa.Data {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%g", v))
		}
		b.WriteString("]")
		return b.String()
	}
	rows := fa.Shape[0]
	cols := fa.Shape[1]
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < rows; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("[")
		for j := 0; j < cols; j++ {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(fmt.Sprintf("%g", fa.Data[i*cols+j]))
		}
		b.WriteString("]")
	}
	b.WriteString("]")
	return b.String()
}

func IsFloatArray(obj Object) bool {
	_, ok := obj.(*FloatArray)
	return ok
}

func GetFloatArrayData(obj Object) ([]float64, []int, bool) {
	if fa, ok := obj.(*FloatArray); ok {
		return fa.Data, fa.Shape, true
	}
	return nil, nil, false
}
