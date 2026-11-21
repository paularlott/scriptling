package evaluator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// bogusKeyword is a keyword no callable reads.
const bogusKeyword = "__no_such_keyword__"

// builtinsAcceptingAnyKeyword are the builtins that take arbitrary keyword
// arguments, so a bogus keyword is not an error.
var builtinsAcceptingAnyKeyword = map[string]string{
	"dict": "dict(a=1) stores its keywords as items",
}

// keywordErrorText reports whether result is a TypeError that rejects the
// keyword name, returning a description of what it is otherwise.
func keywordErrorText(result object.Object, name string) (string, bool) {
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
	if strings.Contains(msg, "'"+name+"'") || strings.Contains(msg, "takes no keyword arguments") ||
		// object() rejects keywords with Python's own wording.
		msg == "object() takes no arguments" {
		return msg, true
	}
	return "TypeError not about the keyword: " + msg, false
}

// callCatchingPanic calls fn, turning a panic into a failure description.
func callCatchingPanic(fn func() object.Object) (res object.Object, panicked string) {
	defer func() {
		if r := recover(); r != nil {
			panicked = fmt.Sprint(r)
		}
	}()
	return fn(), ""
}

// TestEveryBuiltinRejectsUnknownKeyword guards every builtin: one added
// without a keyword check, or one that forgets it, fails here.
func TestEveryBuiltinRejectsUnknownKeyword(t *testing.T) {
	kw := object.NewKwargs(map[string]object.Object{bogusKeyword: object.NewInteger(1)})
	names := make([]string, 0, len(builtins))
	for name := range builtins {
		names = append(names, name)
	}
	sort.Strings(names)
	t.Logf("checking %d builtins", len(names))
	for _, name := range names {
		if _, ok := builtinsAcceptingAnyKeyword[name]; ok {
			continue
		}
		b := builtins[name]
		for _, args := range [][]object.Object{nil, {object.NewString("abc")}} {
			ctx := SetEnvInContext(context.Background(), object.NewEnvironment())
			res, p := callCatchingPanic(func() object.Object { return b.Fn(ctx, kw, args...) })
			if p != "" {
				t.Errorf("%s(%d args, %s=1) panicked: %s", name, len(args), bogusKeyword, p)
				continue
			}
			if got, ok := keywordErrorText(res, bogusKeyword); !ok {
				t.Errorf("%s(%d args, %s=1) = %s; want a TypeError rejecting the keyword", name, len(args), bogusKeyword, got)
			}
		}
	}
}

// methodsAcceptingAnyKeyword are the type methods that take arbitrary keyword
// arguments.
var methodsAcceptingAnyKeyword = map[string]string{
	"str.format":  "the keywords fill named fields",
	"dict.update": "the keywords are stored as items",
}

// TestEveryTypeMethodRejectsUnknownKeyword calls every method of every
// built-in type with a bogus keyword through the method dispatcher.
func TestEveryTypeMethodRejectsUnknownKeyword(t *testing.T) {
	receivers := map[object.ObjectType]func() object.Object{
		object.STRING_OBJ:  func() object.Object { return object.NewString("a,b") },
		object.LIST_OBJ:    func() object.Object { return &object.List{Elements: []object.Object{object.NewInteger(1)}} },
		object.DICT_OBJ:    func() object.Object { return object.NewDict() },
		object.TUPLE_OBJ:   func() object.Object { return &object.Tuple{Elements: []object.Object{object.NewInteger(1)}} },
		object.SET_OBJ:     func() object.Object { return object.NewSet() },
		object.BYTES_OBJ:   func() object.Object { return object.NewBytes([]byte("a,b")) },
		object.INTEGER_OBJ: func() object.Object { return object.NewInteger(5) },
		object.BOOLEAN_OBJ: func() object.Object { return TRUE },
		object.FLOAT_OBJ:   func() object.Object { return object.NewFloat(1.5) },
		object.FLOAT_ARRAY_OBJ: func() object.Object {
			return &object.FloatArray{Data: []float64{1, 2}, Shape: []int{2}}
		},
	}
	kw := object.NewKwargs(map[string]object.Object{bogusKeyword: object.NewInteger(1)})
	total := 0
	for _, methods := range builtinMethodNames {
		total += len(methods)
	}
	t.Logf("checking %d type methods", total)
	for typ, methods := range builtinMethodNames {
		mk, ok := receivers[typ]
		if !ok {
			t.Errorf("no sample receiver for %s; add one", typ)
			continue
		}
		for _, method := range methods {
			full := getTypeName(mk()) + "." + method
			if _, ok := methodsAcceptingAnyKeyword[full]; ok {
				continue
			}
			for _, args := range [][]object.Object{nil, {object.NewString(",")}} {
				env := object.NewEnvironment()
				ctx := SetEnvInContext(context.Background(), env)
				recv := mk()
				res, p := callCatchingPanic(func() object.Object {
					return callStringMethodWithKeywords(ctx, recv, method, args, kw, env)
				})
				if p != "" {
					t.Errorf("%s(%d args, %s=1) panicked: %s", full, len(args), bogusKeyword, p)
					continue
				}
				if got, ok := keywordErrorText(res, bogusKeyword); !ok {
					t.Errorf("%s(%d args, %s=1) = %s; want a TypeError rejecting the keyword", full, len(args), bogusKeyword, got)
				}
			}
		}
	}
}

// TestKnownKeywordsStillAccepted checks the keyword-taking methods and
// builtins still work with their keywords.
func TestKnownKeywordsStillAccepted(t *testing.T) {
	cases := []struct{ src, want string }{
		{`sorted([3, 1, 2], key=lambda x: -x, reverse=True)`, `[1, 2, 3]`},
		{`max([1, 22, 333], key=lambda s: -s, default=0)`, `1`},
		{`min([], default=7)`, `7`},
		{`sum([1, 2], start=10)`, `13`},
		{`list(enumerate("ab", start=1))`, `[(1, 'a'), (2, 'b')]`},
		{`round(2.567, ndigits=2)`, `2.57`},
		{`int("ff", base=16)`, `255`},
		{`pow(2, 10, mod=1000)`, `24`},
		{`"a,b,c".split(",", maxsplit=1)`, `['a', 'b,c']`},
		{`"a,b,c".replace(",", ";", count=1)`, `'a;b,c'`},
		{`"a\tb".expandtabs(tabsize=2)`, `'a b'`},
		{`"x\ny".splitlines(keepends=True)`, `['x\n', 'y']`},
		{`"é".encode(encoding="utf-8", errors="strict")`, `b'\xc3\xa9'`},
		{`b"hi".decode(encoding="utf-8")`, `'hi'`},
		{`str(b"hi", encoding="utf-8")`, `'hi'`},
		{`(1).to_bytes(2, byteorder="little")`, `b'\x01\x00'`},
		{`(-2).to_bytes(2, "big", signed=True)`, `b'\xff\xfe'`},
		{`int.from_bytes(b"\xff\xfe", byteorder="big", signed=True)`, `-2`},
		{`"{a}-{b}".format(a=1, b=2)`, `'1-2'`},
	}
	for _, c := range cases {
		got := testEval("repr(" + c.src + ")")
		if s, ok := got.(*object.String); !ok || s.StringValue() != c.want {
			t.Errorf("%s = %s; want %s", c.src, got.Inspect(), c.want)
		}
	}
}
