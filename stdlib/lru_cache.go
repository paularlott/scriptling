package stdlib

import (
	"context"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// lruCache is the mutable state behind one lru_cache wrapper.
type lruCache struct {
	maxsize int64 // <= 0 means unbounded (or no caching for 0 handled below)
	entries map[string]object.Object
	order   []string // least-recently-used first
	hits    int64
	misses  int64
}

// lruEntrySize approximates what one cached call costs beyond its key text: the
// map slot, the entry in the eviction order and the key's string header.
const lruEntrySize = 96

// lruWrapperSize approximates the memory of a wrapper: its callable, the cache
// state, the entry map and the two method builtins it exposes.
const lruWrapperSize = 640

// account reports the memory a newly cached call holds. The cache is a Go map
// with no object of its own to carry a credit, so growth is reported each time
// the entry count doubles, for half the new count: the sum is the map's size,
// and the credit rides on the map header until the cache is collected.
func (c *lruCache) account(key string) {
	n := len(c.entries) + 1
	if n >= 64 && n&(n-1) == 0 {
		object.TrackMap(c.entries, int64(n/2)*(lruEntrySize+int64(len(key))))
	}
}

// newLRUCacheWrapper wraps fn in a memoizing callable. maxsize <= 0 with
// maxsize==0 meaning no caching at all (always call through), and -1
// unbounded, mirroring functools.lru_cache.
func newLRUCacheWrapper(fn object.Object, maxsize int64) object.Object {
	c := &lruCache{
		maxsize: maxsize,
		entries: make(map[string]object.Object),
	}
	var wrapper *object.Builtin
	wrapper = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Copy the args: the evaluator's fast paths pass a reused buffer.
			callArgs := append([]object.Object{}, args...)
			callKwargs := make(map[string]object.Object, len(kwargs.Kwargs))
			for k, v := range kwargs.Kwargs {
				callKwargs[k] = v
			}

			key, keyErr := c.key(callArgs, callKwargs)
			if keyErr != nil {
				return keyErr
			}
			if c.maxsize != 0 {
				if v, ok := c.entries[key]; ok {
					c.touch(key)
					c.hits++
					return v
				}
			}
			c.misses++

			eval := evaliface.FromContext(ctx)
			if eval == nil {
				return errors.NewError("evaluator not available in context")
			}
			result := eval.CallObjectFunction(ctx, fn, callArgs, callKwargs, nil)
			if result == nil || object.IsError(result) || result.Type() == object.EXCEPTION_OBJ {
				return result
			}
			if c.maxsize != 0 {
				c.store(key, result)
			}
			return result
		},
		HelpText: "lru_cache wrapper - memoized calls to the wrapped function",
	}
	// A wrapper holds the function it wraps, so wrappers can be chained out of
	// reach of any counted container: account for each one.
	object.TrackAlloc(wrapper, lruWrapperSize)
	wrapper.Attributes = map[string]object.Object{
		"cache_clear": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				c.entries = make(map[string]object.Object)
				c.order = nil
				c.hits = 0
				c.misses = 0
				return &object.Null{}
			},
			HelpText: "cache_clear() - Drop all cached results",
		},
		"cache_info": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				eval := evaliface.FromContext(ctx)
				if eval == nil {
					return errors.NewError("evaluator not available in context")
				}
				ntFn := CollectionsLibrary.Functions()["namedtuple"]
				cls := ntFn.Fn(ctx, object.NewKwargs(nil), object.NewString("CacheInfo"), object.NewString("hits misses maxsize currsize"))
				if object.IsError(cls) {
					return cls
				}
				return eval.CallObjectFunction(ctx, cls, []object.Object{
					object.NewInteger(c.hits),
					object.NewInteger(c.misses),
					object.NewInteger(c.maxsize),
					object.NewInteger(int64(len(c.entries))),
				}, nil, nil)
			},
			HelpText: "cache_info() - Cache statistics (hits, misses, maxsize, currsize)",
		},
	}
	return wrapper
}

// key builds the cache key from the call arguments. Unhashable arguments
// produce the TypeError Python's lru_cache raises.
func (c *lruCache) key(args []object.Object, kwargs map[string]object.Object) (string, object.Object) {
	var b strings.Builder
	for _, a := range args {
		if !object.IsHashable(a) {
			return "", &object.Exception{
				Message:       "unhashable type: '" + strings.ToLower(a.Type().String()) + "'",
				ExceptionType: object.ExceptionTypeTypeError,
				Raised:        true,
			}
		}
		b.WriteString(object.DictKey(a))
		b.WriteByte(0x1f)
	}
	if len(kwargs) > 0 {
		names := make([]string, 0, len(kwargs))
		for k := range kwargs {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			if !object.IsHashable(kwargs[k]) {
				return "", &object.Exception{
					Message:       "unhashable type: '" + strings.ToLower(kwargs[k].Type().String()) + "'",
					ExceptionType: object.ExceptionTypeTypeError,
					Raised:        true,
				}
			}
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(object.DictKey(kwargs[k]))
			b.WriteByte(0x1f)
		}
	}
	return b.String(), nil
}

func (c *lruCache) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			return
		}
	}
}

func (c *lruCache) store(key string, value object.Object) {
	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
		c.account(key)
	}
	c.entries[key] = value
	if c.maxsize > 0 && int64(len(c.order)) > c.maxsize {
		evict := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, evict)
	}
}
