package stdlib

import (
	"context"
	"sort"
	"strconv"
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

func counterCount(d *object.Dict, key object.Object) int64 {
	if pair, ok := d.Pairs[object.DictKey(key)]; ok {
		if n, ok := pair.Value.(*object.Integer); ok {
			return n.IntValue()
		}
	}
	return 0
}

func counterAdd(d *object.Dict, key object.Object, n int64) {
	d.Pairs[object.DictKey(key)] = object.DictPair{Key: key, Value: object.NewInteger(counterCount(d, key) + n)}
}

// counterApply adds (sign 1) or subtracts (sign -1) counts from src: another
// Counter or a mapping contributes its counts, any other iterable counts
// each element once.
func counterApply(d *object.Dict, src object.Object, sign int64) object.Object {
	switch s := src.(type) {
	case *object.Instance:
		if s.Class == counterClassRef {
			for _, pair := range counterData(s).Pairs {
				if n, ok := pair.Value.(*object.Integer); ok {
					counterAdd(d, pair.Key, sign*n.IntValue())
				}
			}
			return nil
		}
	case *object.Dict:
		for _, pair := range s.Pairs {
			n, ok := pair.Value.(*object.Integer)
			if !ok {
				return errors.NewTypeError("INTEGER count", pair.Value.Type().String())
			}
			counterAdd(d, pair.Key, sign*n.IntValue())
		}
		return nil
	}
	elems, ok := object.IterableToSlice(src)
	if !ok {
		return errors.NewTypeError("iterable or mapping", src.Type().String())
	}
	for _, e := range elems {
		counterAdd(d, e, sign)
	}
	return nil
}

// counterApplyKwargs adds keyword counts: Counter(a=1, b=2).
func counterApplyKwargs(d *object.Dict, kwargs object.Kwargs, sign int64) object.Object {
	for _, key := range kwargs.Keys() {
		n, ok := kwargs.Get(key).(*object.Integer)
		if !ok {
			return errors.NewTypeError("INTEGER count", kwargs.Get(key).Type().String())
		}
		counterAdd(d, object.NewString(key), sign*n.IntValue())
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
	count int64
}

// counterSorted lists entries most common first. Python breaks ties by
// insertion order, which Scriptling dicts don't keep, so ties are ordered by
// key for stable output.
func counterSorted(d *object.Dict) []counterEntry {
	entries := make([]counterEntry, 0, len(d.Pairs))
	for _, pair := range d.Pairs {
		if n, ok := pair.Value.(*object.Integer); ok {
			entries = append(entries, counterEntry{pair.Key, n.IntValue()})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return counterKeyRepr(entries[i].key) < counterKeyRepr(entries[j].key)
	})
	return entries
}

func counterMostCommon(inst *object.Instance, n int) object.Object {
	entries := counterSorted(counterData(inst))
	if n < 0 || n > len(entries) {
		n = len(entries)
	}
	out := make([]object.Object, n)
	for i := 0; i < n; i++ {
		out[i] = &object.Tuple{Elements: []object.Object{entries[i].key, object.NewInteger(entries[i].count)}}
	}
	return &object.List{Elements: out}
}

// counterArithmetic implements +, -, | and & like Python: results keep only
// positive counts.
func counterArithmetic(op byte, a, b *object.Instance) *object.Instance {
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
		var v int64
		switch op {
		case '+':
			v = x + y
		case '-':
			v = x - y
		case '|':
			v = max(x, y)
		case '&':
			v = min(x, y)
		}
		if v > 0 {
			counterAdd(od, key, v)
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
				if err := counterApply(d, args[1], 1); err != nil {
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
			return object.NewInteger(counterCount(d, args[1]))
		}),
		"__setitem__": counterMethod("c[key] = count", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 3); err != nil {
				return err
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
			if len(d.Pairs) != len(other.Pairs) {
				return object.NewBoolean(false)
			}
			for k, p := range d.Pairs {
				q, ok := other.Pairs[k]
				if !ok || q.Value.Inspect() != p.Value.Inspect() {
					return object.NewBoolean(false)
				}
			}
			return object.NewBoolean(true)
		}),
		"__repr__": counterMethod("repr(c) - Counter({'a': 2, 'b': 1})", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			entries := counterSorted(d)
			if len(entries) == 0 {
				return object.NewString("Counter()")
			}
			parts := make([]string, len(entries))
			for i, e := range entries {
				parts[i] = counterKeyRepr(e.key) + ": " + strconv.FormatInt(e.count, 10)
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
				if err := counterApply(d, args[1], 1); err != nil {
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
				if err := counterApply(d, args[1], -1); err != nil {
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
			var sum int64
			for _, p := range d.Pairs {
				if n, ok := p.Value.(*object.Integer); ok {
					sum += n.IntValue()
				}
			}
			return object.NewInteger(sum)
		}),
		"most_common": counterMethod("most_common([n]) - (element, count) pairs, most common first", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.RangeArgs(args, 1, 2); err != nil {
				return err
			}
			inst, _ := counterSelf(args)
			n := -1
			if len(args) == 2 {
				if _, isNone := args[1].(*object.Null); !isNone {
					v, err := args[1].AsInt()
					if err != nil {
						return err
					}
					n = int(v)
				}
			}
			return counterMostCommon(inst, n)
		}),
		"elements": counterMethod("elements() - each element repeated by its count", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			_, d := counterSelf(args)
			var out []object.Object
			for _, e := range counterSorted(d) {
				for i := int64(0); i < e.count; i++ {
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
		if err := counterApply(d, args[0], 1); err != nil {
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
	n := -1
	if len(args) == 2 {
		v, err := args[1].AsInt()
		if err != nil {
			return err
		}
		n = int(v)
	}
	return counterMostCommon(inst, n)
}
