package stdlib

import (
	"context"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Counter is collections.Counter: a dict of element -> count with Python's
// API. Counts live in a real dict (field counterField) so keys keep their
// type: Counter([3, 3]) has the integer key 3.
const counterField = "_counts"

// counterData returns the dict holding a Counter's counts.
func counterData(inst *object.Instance) *object.Dict {
	if v, ok := inst.GetField(counterField); ok {
		if d, ok := v.(*object.Dict); ok {
			return d
		}
	}
	d := &object.Dict{Pairs: make(map[string]object.DictPair)}
	inst.SetField(counterField, d)
	return d
}

func newCounter() *object.Instance {
	inst := object.NewInstanceWithFields(counterClassRef, make(map[string]object.Object))
	counterData(inst)
	return inst
}

// Counts are numbers: ints normally, floats when a script stores them, as
// Python allows. counterZero is the count of a missing element.
var counterZero = object.NewInteger(0)

func counterCount(d *object.Dict, key object.Object) object.Object {
	if pair, ok := d.Pairs[object.DictKey(key)]; ok {
		return pair.Value
	}
	return counterZero
}

// countValue returns a count as a float for comparisons, and whether it is
// a number at all.
func countValue(n object.Object) (float64, bool) {
	switch v := n.(type) {
	case *object.Integer:
		return float64(v.IntValue()), true
	case *object.Float:
		return v.FloatValue(), true
	case *object.Boolean:
		if v.BoolValue() {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// addCounts returns a + sign*b, staying an int when both are ints.
func addCounts(a, b object.Object, sign int64) (object.Object, object.Object) {
	ai, aInt := a.(*object.Integer)
	bi, bInt := b.(*object.Integer)
	if aInt && bInt {
		return object.NewInteger(ai.IntValue() + sign*bi.IntValue()), nil
	}
	af, aOK := countValue(a)
	bf, bOK := countValue(b)
	if !aOK {
		return nil, errors.NewTypeError("a number count", a.Type().String())
	}
	if !bOK {
		return nil, errors.NewTypeError("a number count", b.Type().String())
	}
	return object.NewFloat(af + float64(sign)*bf), nil
}

func counterAdd(d *object.Dict, key, n object.Object, sign int64) object.Object {
	if !object.IsHashable(key) {
		return &object.Exception{Message: "unhashable type: '" + strings.ToLower(key.Type().String()) + "'", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
	}
	sum, err := addCounts(counterCount(d, key), n, sign)
	if err != nil {
		return err
	}
	d.Pairs[object.DictKey(key)] = object.DictPair{Key: key, Value: sum}
	return nil
}

var counterOne = object.NewInteger(1)

// counterApply adds (sign 1) or subtracts (sign -1) counts from src: another
// Counter or a mapping contributes its counts, None nothing, and any other
// iterable counts each element once.
func counterApply(ctx context.Context, d *object.Dict, src object.Object, sign int64) object.Object {
	var elems []object.Object
	switch s := src.(type) {
	case *object.Null:
		return nil
	case *object.Instance:
		if s.Class == counterClassRef {
			for _, pair := range counterData(s).Pairs {
				if err := counterAdd(d, pair.Key, pair.Value, sign); err != nil {
					return err
				}
			}
			return nil
		}
		var errObj object.Object
		if elems, errObj = instanceElements(ctx, s); errObj != nil {
			return errObj
		}
	case *object.Dict:
		for _, pair := range s.Pairs {
			if err := counterAdd(d, pair.Key, pair.Value, sign); err != nil {
				return err
			}
		}
		return nil
	default:
		var errObj object.Object
		if elems, errObj = collectIterable(ctx, src); errObj != nil {
			return errObj
		}
	}
	for _, e := range elems {
		if err := counterAdd(d, e, counterOne, sign); err != nil {
			return err
		}
	}
	return nil
}

// instanceElements collects the elements of a script object that defines
// __iter__, whose iterator may itself be a script object with __next__.
func instanceElements(ctx context.Context, inst *object.Instance) ([]object.Object, object.Object) {
	iterFn, ok := inst.Class.Methods["__iter__"]
	if !ok {
		return nil, &object.Exception{Message: "'" + inst.Class.Name + "' object is not iterable", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
	}
	it := callCallable(ctx, iterFn, inst)
	if object.IsError(it) || it.Type() == object.EXCEPTION_OBJ {
		return nil, it
	}
	itInst, isInst := it.(*object.Instance)
	if !isInst {
		return collectIterable(ctx, it)
	}
	nextFn, ok := itInst.Class.Methods["__next__"]
	if !ok {
		return nil, &object.Exception{Message: "iter() returned non-iterator of type '" + itInst.Class.Name + "'", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
	}
	var elems []object.Object
	for {
		if err := ctx.Err(); err != nil {
			return nil, errors.NewError("%s", err.Error())
		}
		v := callCallable(ctx, nextFn, itInst)
		if ex, isEx := v.(*object.Exception); isEx && ex.ExceptionType == object.ExceptionTypeStopIteration {
			return elems, nil
		}
		if object.IsError(v) || v.Type() == object.EXCEPTION_OBJ {
			return nil, v
		}
		elems = append(elems, v)
	}
}

// counterApplyKwargs adds keyword counts: Counter(a=1, b=2).
func counterApplyKwargs(d *object.Dict, kwargs object.Kwargs, sign int64) object.Object {
	for _, key := range kwargs.Keys() {
		if err := counterAdd(d, object.NewString(key), kwargs.Get(key), sign); err != nil {
			return err
		}
	}
	return nil
}

// counterKeyRepr renders a key as Python's repr would.
func counterKeyRepr(k object.Object) string {
	if s, ok := k.(*object.String); ok {
		return object.ReprString(s.StringValue())
	}
	r, _ := object.InspectRepr(k, nil)
	return r
}

type counterEntry struct {
	key   object.Object
	count object.Object
	value float64 // count as a number, for ordering
}

// counterSorted lists entries most common first. Python breaks ties by
// insertion order, which Scriptling dicts don't keep, so ties are ordered by
// key for stable output.
func counterSorted(d *object.Dict) []counterEntry {
	entries := make([]counterEntry, 0, len(d.Pairs))
	for _, pair := range d.Pairs {
		v, _ := countValue(pair.Value)
		entries = append(entries, counterEntry{pair.Key, pair.Value, v})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].value != entries[j].value {
			return entries[i].value > entries[j].value
		}
		return counterKeyRepr(entries[i].key) < counterKeyRepr(entries[j].key)
	})
	return entries
}

// counterMostCommon returns the n most common entries, or all of them when
// all is set. A negative n gives none, as in Python.
func counterMostCommon(inst *object.Instance, n int, all bool) object.Object {
	entries := counterSorted(counterData(inst))
	if all || n > len(entries) {
		n = len(entries)
	}
	n = max(n, 0)
	out := make([]object.Object, n)
	for i := 0; i < n; i++ {
		out[i] = &object.Tuple{Elements: []object.Object{entries[i].key, entries[i].count}}
	}
	return &object.List{Elements: out}
}

// mostCommonArg reads most_common's optional n: None or absent means all;
// anything but an int is a TypeError.
func mostCommonArg(args []object.Object, i int) (n int, all bool, errObj object.Object) {
	if len(args) <= i {
		return 0, true, nil
	}
	switch v := args[i].(type) {
	case *object.Null:
		return 0, true, nil
	case *object.Integer:
		return int(v.IntValue()), false, nil
	}
	return 0, false, &object.Exception{Message: "'" + strings.ToLower(args[i].Type().String()) + "' object cannot be interpreted as an integer", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
}

// counterArithmetic implements +, -, | and & like Python: results keep only
// positive counts.
func counterArithmetic(op byte, a, b *object.Instance) object.Object {
	ad, bd := counterData(a), counterData(b)
	out := newCounter()
	od := counterData(out)
	keys := map[string]object.Object{}
	for k, p := range ad.Pairs {
		keys[k] = p.Key
	}
	for k, p := range bd.Pairs {
		keys[k] = p.Key
	}
	for _, key := range keys {
		x, y := counterCount(ad, key), counterCount(bd, key)
		var v object.Object
		switch op {
		case '+', '-':
			sign := int64(1)
			if op == '-' {
				sign = -1
			}
			sum, err := addCounts(x, y, sign)
			if err != nil {
				return err
			}
			v = sum
		case '|', '&':
			xf, xOK := countValue(x)
			yf, yOK := countValue(y)
			if !xOK || !yOK {
				_, err := addCounts(x, y, 1)
				return err
			}
			v = x
			if (op == '|') == (yf > xf) {
				v = y
			}
		}
		if f, _ := countValue(v); f > 0 {
			od.Pairs[object.DictKey(key)] = object.DictPair{Key: key, Value: v}
		}
	}
	return out
}

func counterSelf(args []object.Object) (*object.Instance, *object.Dict) {
	inst := args[0].(*object.Instance)
	return inst, counterData(inst)
}

// counterOperator wraps a binary Counter operator.
func counterOperator(op byte) *object.Builtin {
	return &object.Builtin{Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if err := errors.ExactArgs(args, 2); err != nil {
			return err
		}
		a := args[0].(*object.Instance)
		b, ok := args[1].(*object.Instance)
		if !ok || b.Class != counterClassRef {
			return errors.NewTypeError("Counter", args[1].Type().String())
		}
		return counterArithmetic(op, a, b)
	}}
}

func counterMethod(help string, fn object.BuiltinFunction) *object.Builtin {
	return &object.Builtin{Fn: fn, HelpText: help}
}

var CounterClass = &object.Class{
	Name: "Counter",
	Methods: map[string]object.Object{
		"__init__": counterMethod("__init__([iterable_or_mapping], **kwargs)", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			if len(args) == 2 {
				if err := counterApply(ctx, d, args[1], 1); err != nil {
					return err
				}
			}
			if err := counterApplyKwargs(d, kwargs, 1); err != nil {
				return err
			}
			return &object.Null{}
		}),
		"__getitem__": counterMethod("c[key] - count of key, 0 when missing", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			return counterCount(d, args[1])
		}),
		"__setitem__": counterMethod("c[key] = count", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 3); err != nil {
				return err
			}
			if !object.IsHashable(args[1]) {
				return &object.Exception{Message: "unhashable type: '" + strings.ToLower(args[1].Type().String()) + "'", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
			}
			_, d := counterSelf(args)
			d.Pairs[object.DictKey(args[1])] = object.DictPair{Key: args[1], Value: args[2]}
			return &object.Null{}
		}),
		"__delitem__": counterMethod("del c[key]", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			delete(d.Pairs, object.DictKey(args[1]))
			return &object.Null{}
		}),
		"__len__": counterMethod("len(c)", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			return object.NewInteger(int64(len(d.Pairs)))
		}),
		"__bool__": counterMethod("bool(c)", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			return object.NewBoolean(len(d.Pairs) > 0)
		}),
		"__contains__": counterMethod("key in c", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			_, ok := d.Pairs[object.DictKey(args[1])]
			return object.NewBoolean(ok)
		}),
		"__iter__": counterMethod("iter(c) - the keys", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			return d.CreateIterator()
		}),
		"__eq__": counterMethod("c == other - same counts", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			var other *object.Dict
			switch o := args[1].(type) {
			case *object.Instance:
				if o.Class != counterClassRef {
					return object.NewBoolean(false)
				}
				other = counterData(o)
			case *object.Dict:
				other = o
			default:
				return object.NewBoolean(false)
			}
			// Python 3.10+: a missing element counts as zero, and counts
			// compare as numbers (1 == 1.0).
			same := func(a, b *object.Dict) bool {
				for k, p := range a.Pairs {
					var q object.Object = counterZero
					if pair, ok := b.Pairs[k]; ok {
						q = pair.Value
					}
					x, xOK := countValue(p.Value)
					y, yOK := countValue(q)
					if xOK && yOK {
						if x != y {
							return false
						}
					} else if p.Value.Inspect() != q.Inspect() {
						return false
					}
				}
				return true
			}
			return object.NewBoolean(same(d, other) && same(other, d))
		}),
		"__repr__": counterMethod("repr(c) - Counter({'a': 2, 'b': 1})", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			entries := counterSorted(d)
			if len(entries) == 0 {
				return object.NewString("Counter()")
			}
			parts := make([]string, len(entries))
			for i, e := range entries {
				parts[i] = counterKeyRepr(e.key) + ": " + counterKeyRepr(e.count)
			}
			return object.NewString("Counter({" + strings.Join(parts, ", ") + "})")
		}),
		"__add__": counterOperator('+'),
		"__sub__": counterOperator('-'),
		"__or__":  counterOperator('|'),
		"__and__": counterOperator('&'),
		"keys": counterMethod("keys() - view of the elements", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			return &object.DictKeys{Dict: d}
		}),
		"values": counterMethod("values() - view of the counts", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			return &object.DictValues{Dict: d}
		}),
		"items": counterMethod("items() - view of (element, count) pairs", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			return &object.DictItems{Dict: d}
		}),
		"get": counterMethod("get(key, default=None)", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 2, 3); err != nil {
				return err
			}
			_, d := counterSelf(args)
			if pair, ok := d.Pairs[object.DictKey(args[1])]; ok {
				return pair.Value
			}
			if len(args) == 3 {
				return args[2]
			}
			return &object.Null{}
		}),
		"pop": counterMethod("pop(key[, default])", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 2, 3); err != nil {
				return err
			}
			_, d := counterSelf(args)
			k := object.DictKey(args[1])
			if pair, ok := d.Pairs[k]; ok {
				delete(d.Pairs, k)
				return pair.Value
			}
			if len(args) == 3 {
				return args[2]
			}
			return &object.Exception{Message: counterKeyRepr(args[1]), ExceptionType: object.ExceptionTypeKeyError, Raised: true}
		}),
		"update": counterMethod("update([iterable_or_mapping], **kwargs) - add counts", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			if len(args) == 2 {
				if err := counterApply(ctx, d, args[1], 1); err != nil {
					return err
				}
			}
			if err := counterApplyKwargs(d, kwargs, 1); err != nil {
				return err
			}
			return &object.Null{}
		}),
		"subtract": counterMethod("subtract([iterable_or_mapping], **kwargs) - subtract counts (may go negative)", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			_, d := counterSelf(args)
			if len(args) == 2 {
				if err := counterApply(ctx, d, args[1], -1); err != nil {
					return err
				}
			}
			if err := counterApplyKwargs(d, kwargs, -1); err != nil {
				return err
			}
			return &object.Null{}
		}),
		"total": counterMethod("total() - sum of all counts", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			var sum object.Object = counterZero
			for _, p := range d.Pairs {
				var err object.Object
				if sum, err = addCounts(sum, p.Value, 1); err != nil {
					return err
				}
			}
			return sum
		}),
		"most_common": counterMethod("most_common([n]) - (element, count) pairs, most common first", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			inst, _ := counterSelf(args)
			n, all, errObj := mostCommonArg(args, 1)
			if errObj != nil {
				return errObj
			}
			return counterMostCommon(inst, n, all)
		}),
		"elements": counterMethod("elements() - each element repeated by its count", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			var out []object.Object
			for _, e := range counterSorted(d) {
				n, isInt := e.count.(*object.Integer)
				if !isInt {
					if _, isFloat := e.count.(*object.Float); isFloat {
						return &object.Exception{Message: "'float' object cannot be interpreted as an integer", ExceptionType: object.ExceptionTypeTypeError, Raised: true}
					}
					continue
				}
				for i := int64(0); i < n.IntValue(); i++ {
					out = append(out, e.key)
				}
			}
			return &object.List{Elements: out}
		}),
		"copy": counterMethod("copy() - a shallow copy", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			out := newCounter()
			od := counterData(out)
			for k, p := range d.Pairs {
				od.Pairs[k] = p
			}
			return out
		}),
		"clear": counterMethod("clear() - remove all elements", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			d.Pairs = make(map[string]object.DictPair)
			return &object.Null{}
		}),
	},
}

// counterConstructor is collections.Counter([iterable_or_mapping], **kwargs).
func counterConstructor(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.MaxArgs(args, 1); err != nil {
		return err
	}
	inst := newCounter()
	d := counterData(inst)
	if len(args) == 1 {
		if err := counterApply(ctx, d, args[0], 1); err != nil {
			return err
		}
	}
	if err := counterApplyKwargs(d, kwargs, 1); err != nil {
		return err
	}
	return inst
}

// counterMostCommonFn is the module-level collections.most_common(counter, n).
func counterMostCommonFn(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.RangeArgs(args, 1, 2); err != nil {
		return err
	}
	inst, ok := args[0].(*object.Instance)
	if !ok || inst.Class != counterClassRef {
		return errors.NewTypeError("Counter", args[0].Type().String())
	}
	n, all, errObj := mostCommonArg(args, 1)
	if errObj != nil {
		return errObj
	}
	return counterMostCommon(inst, n, all)
}
