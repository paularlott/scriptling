package object

import "unsafe"

// CreateIterator returns an iterator over the dict's keys.
// In Python, iterating a dict yields its keys (same as dict.keys()), in
// insertion order.
func (d *Dict) CreateIterator() *Iterator {
	return d.iterate(func(p DictPair) Object { return p.Key })
}

// iterate returns an iterator over the dict's pairs in insertion order,
// yielding pick(pair) for each. It walks the entries as they were when
// iteration began: a pair deleted before the walk reaches it is skipped, a
// pair added during the walk is not visited, and a value changed during the
// walk is seen as changed.
func (d *Dict) iterate(pick func(DictPair) Object) *Iterator {
	entries := d.entries
	pos := int(d.head)
	return trackIterator(&Iterator{
		next: func() (Object, bool) {
			for pos < len(entries) {
				e := &entries[pos]
				pos++
				if e.Key == nil {
					continue
				}
				if unsafe.SliceData(entries) != unsafe.SliceData(d.entries) {
					// The dict has moved its entries since the walk began
					// (growth, compaction or clear): read the pair afresh.
					pair, ok := d.Lookup(e.canonical)
					if !ok {
						continue
					}
					return pick(pair), true
				}
				return pick(e.DictPair), true
			}
			return nil, false
		},
	})
}

// DictKeys represents a view of a dictionary's keys
type DictKeys struct {
	Dict *Dict
}

func (dk *DictKeys) Type() ObjectType { return DICT_KEYS_OBJ }
func (dk *DictKeys) Inspect() string  { return inspectContainer(dk) }

func (dk *DictKeys) AsString() (string, Object)          { return dk.Inspect(), nil }
func (dk *DictKeys) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (dk *DictKeys) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (dk *DictKeys) AsBool() (bool, Object)              { return dk.Dict.Len() > 0, nil }
func (dk *DictKeys) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (dk *DictKeys) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (dk *DictKeys) CoerceString() (string, Object) { return dk.Inspect(), nil }
func (dk *DictKeys) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (dk *DictKeys) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

// CreateIterator returns an iterator for the keys
func (dk *DictKeys) CreateIterator() *Iterator {
	return dk.Dict.iterate(func(p DictPair) Object { return p.Key })
}

// DictValues represents a view of a dictionary's values
type DictValues struct {
	Dict *Dict
}

func (dv *DictValues) Type() ObjectType { return DICT_VALUES_OBJ }
func (dv *DictValues) Inspect() string  { return inspectContainer(dv) }

func (dv *DictValues) AsString() (string, Object)          { return dv.Inspect(), nil }
func (dv *DictValues) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (dv *DictValues) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (dv *DictValues) AsBool() (bool, Object)              { return dv.Dict.Len() > 0, nil }
func (dv *DictValues) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (dv *DictValues) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (dv *DictValues) CoerceString() (string, Object) { return dv.Inspect(), nil }
func (dv *DictValues) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (dv *DictValues) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

func (dv *DictValues) CreateIterator() *Iterator {
	return dv.Dict.iterate(func(p DictPair) Object { return p.Value })
}

// DictItems represents a view of a dictionary's items
type DictItems struct {
	Dict *Dict
}

func (di *DictItems) Type() ObjectType { return DICT_ITEMS_OBJ }
func (di *DictItems) Inspect() string  { return inspectContainer(di) }

func (di *DictItems) AsString() (string, Object)          { return di.Inspect(), nil }
func (di *DictItems) AsInt() (int64, Object)              { return 0, errMustBeInteger }
func (di *DictItems) AsFloat() (float64, Object)          { return 0, errMustBeNumber }
func (di *DictItems) AsBool() (bool, Object)              { return di.Dict.Len() > 0, nil }
func (di *DictItems) AsList() ([]Object, Object)          { return nil, errMustBeList }
func (di *DictItems) AsDict() (map[string]Object, Object) { return nil, errMustBeDict }

func (di *DictItems) CoerceString() (string, Object) { return di.Inspect(), nil }
func (di *DictItems) CoerceInt() (int64, Object)     { return 0, errMustBeInteger }
func (di *DictItems) CoerceFloat() (float64, Object) { return 0, errMustBeNumber }

func (di *DictItems) CreateIterator() *Iterator {
	return di.Dict.iterate(func(p DictPair) Object { return NewPair(p.Key, p.Value) })
}
