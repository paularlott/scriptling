package stdlib_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// stdlibAcceptingAnyKeyword are the standard library callables that take
// arbitrary keyword arguments, keyed by dotted path.
var stdlibAcceptingAnyKeyword = map[string]string{
	"functools.partial":               "stores the keywords for the wrapped function",
	"operator.methodcaller":           "stores the keywords for the method",
	"collections.Counter":             "keywords are initial counts",
	"collections.Counter.__init__":    "keywords are initial counts",
	"collections.Counter.update":      "keywords are counts to add",
	"collections.Counter.subtract":    "keywords are counts to subtract",
	"collections.OrderedDict":         "keywords are initial items",
	"collections.defaultdict":         "keywords are initial items",
	"collections.DefaultDict":         "keywords are initial items",
	"string.Template.substitute":      "keywords are the substitution values",
	"string.Template.safe_substitute": "keywords are the substitution values",
}

type libCollector struct{ libs []*object.Library }

func (c *libCollector) RegisterLibrary(lib *object.Library) { c.libs = append(c.libs, lib) }

// stdlibCallable is one Go-implemented callable reachable from a library.
type stdlibCallable struct {
	path string
	fn   *object.Builtin
}

// collectStdlibCallables walks every registered library's functions, and
// the Go methods of the classes and sub-libraries among its constants.
func collectStdlibCallables() []stdlibCallable {
	var c libCollector
	stdlib.RegisterAll(&c)
	var out []stdlibCallable
	seen := map[any]bool{}
	var walkClass func(path string, cls *object.Class)
	var walkLib func(path string, lib *object.Library)
	walkValue := func(path string, v object.Object) {
		switch x := v.(type) {
		case *object.Builtin:
			out = append(out, stdlibCallable{path, x})
		case *object.Class:
			walkClass(path, x)
		case *object.Library:
			walkLib(path, x)
		}
	}
	walkClass = func(path string, cls *object.Class) {
		if seen[cls] {
			return
		}
		seen[cls] = true
		for name, m := range cls.Methods {
			if b, ok := m.(*object.Builtin); ok {
				out = append(out, stdlibCallable{path + "." + name, b})
			}
		}
	}
	walkLib = func(path string, lib *object.Library) {
		if seen[lib] {
			return
		}
		seen[lib] = true
		for name, fn := range lib.Functions() {
			out = append(out, stdlibCallable{path + "." + name, fn})
		}
		for name, v := range lib.Constants() {
			walkValue(path+"."+name, v)
		}
	}
	for _, lib := range c.libs {
		walkLib(lib.Name(), lib)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

func rejectsKeyword(result object.Object, name string) (string, bool) {
	var msg, typ string
	switch r := result.(type) {
	case *object.Error:
		msg, typ = r.Message, r.ExceptionType
	case *object.Exception:
		msg, typ = r.Message, r.ExceptionType
	default:
		if result == nil {
			return "nil", false
		}
		return fmt.Sprintf("%s %s", result.Type(), result.Inspect()), false
	}
	if typ != object.ExceptionTypeTypeError {
		return fmt.Sprintf("%s: %s", typ, msg), false
	}
	if strings.Contains(msg, "'"+name+"'") || strings.Contains(msg, "takes no keyword arguments") {
		return msg, true
	}
	return "TypeError not about the keyword: " + msg, false
}

// TestEveryStdlibCallableRejectsUnknownKeyword calls every Go callable of
// the standard library with a bogus keyword, with no positional arguments and
// with one, and expects a TypeError naming it.
func TestEveryStdlibCallableRejectsUnknownKeyword(t *testing.T) {
	const bogus = "__no_such_keyword__"
	kw := object.NewKwargs(map[string]object.Object{bogus: object.NewInteger(1)})
	callables := collectStdlibCallables()
	if len(callables) < 300 {
		t.Fatalf("found only %d stdlib callables; the walk is broken", len(callables))
	}
	t.Logf("checking %d stdlib callables", len(callables))
	paths := map[string]bool{}
	for _, c := range callables {
		paths[c.path] = true
	}
	for path := range stdlibAcceptingAnyKeyword {
		if !paths[path] {
			t.Errorf("allow-listed %s is not a stdlib callable; remove it", path)
		}
	}
	for _, c := range callables {
		if _, ok := stdlibAcceptingAnyKeyword[c.path]; ok {
			continue
		}
		for _, args := range [][]object.Object{nil, {object.NewString("abc")}} {
			res, panicked := func() (res object.Object, p string) {
				defer func() {
					if r := recover(); r != nil {
						p = fmt.Sprint(r)
					}
				}()
				return c.fn.Fn(context.Background(), kw, args...), ""
			}()
			if panicked != "" {
				t.Errorf("%s(%d args, %s=1) panicked: %s", c.path, len(args), bogus, panicked)
				continue
			}
			if got, ok := rejectsKeyword(res, bogus); !ok {
				t.Errorf("%s(%d args, %s=1) = %s; want a TypeError rejecting the keyword", c.path, len(args), bogus, got)
			}
		}
	}
}
