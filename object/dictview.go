package object

import (
	"fmt"
)

// DictKeys represents a view of a dictionary's keys
type DictKeys struct {
	Dict *Dict
}

func (dk *DictKeys) Type() ObjectType { return DICT_KEYS_OBJ }
func (dk *DictKeys) Inspect() string  { return fmt.Sprintf("dict_keys(%s)", dk.Dict.Inspect()) }

func (dk *DictKeys) AsString() (string, bool)          { return dk.Inspect(), true }
func (dk *DictKeys) AsInt() (int64, bool)              { return 0, false }
func (dk *DictKeys) AsFloat() (float64, bool)          { return 0, false }
func (dk *DictKeys) AsBool() (bool, bool)              { return len(dk.Dict.Pairs) > 0, true }
func (dk *DictKeys) AsList() ([]Object, bool)          { return nil, false }
func (dk *DictKeys) AsDict() (map[string]Object, bool) { return nil, false }

// CreateIterator returns an iterator for the keys
func (dk *DictKeys) CreateIterator() *Iterator {
	// Snapshot keys to avoid concurrent map modification issues during iteration
	// In Python 3, modifying dict during iteration raises RuntimeError, but for simplicity
	// we'll just iterate over a snapshot of keys at the start.
	// To be truly dynamic/lazy, we'd need to iterate the map directly, but Go maps
	// don't support safe concurrent modification/iteration easily without restart.
	// However, "View" objects in Python reflect changes.
	// Let's grab the keys at the moment CreateIterator is called.

	keys := make([]string, 0, len(dk.Dict.Pairs))
	for k := range dk.Dict.Pairs {
		keys = append(keys, k)
	}

	index := 0
	return &Iterator{
		next: func() (Object, bool) {
			for index < len(keys) {
				key := keys[index]
				index++

				// Check if key still exists (view behavior)
				if pair, ok := dk.Dict.Pairs[key]; ok {
					return pair.Key, true
				}
				// If key deleted, loop continues to next key in snapshot
			}
			return nil, false
		},
	}
}

// DictValues represents a view of a dictionary's values
type DictValues struct {
	Dict *Dict
}

func (dv *DictValues) Type() ObjectType { return DICT_VALUES_OBJ }
func (dv *DictValues) Inspect() string  { return fmt.Sprintf("dict_values(%s)", dv.Dict.Inspect()) }

func (dv *DictValues) AsString() (string, bool)          { return dv.Inspect(), true }
func (dv *DictValues) AsInt() (int64, bool)              { return 0, false }
func (dv *DictValues) AsFloat() (float64, bool)          { return 0, false }
func (dv *DictValues) AsBool() (bool, bool)              { return len(dv.Dict.Pairs) > 0, true }
func (dv *DictValues) AsList() ([]Object, bool)          { return nil, false }
func (dv *DictValues) AsDict() (map[string]Object, bool) { return nil, false }

func (dv *DictValues) CreateIterator() *Iterator {
	keys := make([]string, 0, len(dv.Dict.Pairs))
	for k := range dv.Dict.Pairs {
		keys = append(keys, k)
	}

	index := 0
	return &Iterator{
		next: func() (Object, bool) {
			for index < len(keys) {
				key := keys[index]
				index++

				if pair, ok := dv.Dict.Pairs[key]; ok {
					return pair.Value, true
				}
				// If key deleted, loop to next
			}
			return nil, false
		},
	}
}

// DictItems represents a view of a dictionary's items
type DictItems struct {
	Dict *Dict
}

func (di *DictItems) Type() ObjectType { return DICT_ITEMS_OBJ }
func (di *DictItems) Inspect() string  { return fmt.Sprintf("dict_items(%s)", di.Dict.Inspect()) }

func (di *DictItems) AsString() (string, bool)          { return di.Inspect(), true }
func (di *DictItems) AsInt() (int64, bool)              { return 0, false }
func (di *DictItems) AsFloat() (float64, bool)          { return 0, false }
func (di *DictItems) AsBool() (bool, bool)              { return len(di.Dict.Pairs) > 0, true }
func (di *DictItems) AsList() ([]Object, bool)          { return nil, false }
func (di *DictItems) AsDict() (map[string]Object, bool) { return nil, false }

func (di *DictItems) CreateIterator() *Iterator {
	keys := make([]string, 0, len(di.Dict.Pairs))
	for k := range di.Dict.Pairs {
		keys = append(keys, k)
	}

	index := 0
	return &Iterator{
		next: func() (Object, bool) {
			for index < len(keys) {
				key := keys[index]
				index++

				if pair, ok := di.Dict.Pairs[key]; ok {
					return &Tuple{Elements: []Object{pair.Key, pair.Value}}, true
				}
			}
			return nil, false
		},
	}
}
