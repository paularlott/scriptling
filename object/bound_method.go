package object

// BoundMethod represents a method bound to an instance
// When called, it automatically prepends self to the arguments
type BoundMethod struct {
	Instance Object // The instance (self)
	Method   Object // The method function
}

func (bm *BoundMethod) Type() ObjectType { return FUNCTION_OBJ } // Behaves like a function
func (bm *BoundMethod) Inspect() string  { return "<bound method>" }

func (bm *BoundMethod) AsString() (string, Object)          { return "", errMustBeString }
func (bm *BoundMethod) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (bm *BoundMethod) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (bm *BoundMethod) AsBool() (bool, Object)              { return true, nil }
func (bm *BoundMethod) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (bm *BoundMethod) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (bm *BoundMethod) CoerceString() (string, Object) { return bm.Inspect(), nil }
func (bm *BoundMethod) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (bm *BoundMethod) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }
