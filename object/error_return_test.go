package object

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A method or function whose ONLY return is an error uses that return as the
// error channel: a non-nil error raises a script error carrying the message,
// a nil error yields null. The regression these tests pin down: the error
// used to be handed to value conversion and surfaced as
// "unsupported constant type: *errors.errorString" instead.

type errorStore struct{ fail bool }

func callBuiltin(t *testing.T, fn *Builtin, args ...Object) Object {
	t.Helper()
	return fn.Fn(context.Background(), NewKwargs(nil), args...)
}

func wantError(t *testing.T, result Object, message string) {
	t.Helper()
	errObj, ok := result.(*Error)
	if !ok {
		t.Fatalf("want *Error, got %T (%s)", result, result.Inspect())
	}
	if errObj.Message != message {
		t.Fatalf("message = %q, want %q", errObj.Message, message)
	}
	if strings.Contains(errObj.Message, "unsupported constant type") {
		t.Fatal("the error leaked into value conversion")
	}
}

func wantNull(t *testing.T, result Object) {
	t.Helper()
	if _, isErr := result.(*Error); isErr {
		t.Fatalf("unexpected error: %s", result.Inspect())
	}
	if result.Type() != NULL_OBJ {
		t.Fatalf("want null, got %s", result.Inspect())
	}
}

// Typed-receiver methods (Constructor + self *T), including the context and
// kwargs forms the plugins use, take the createTypedReceiverWrapper path.
func TestTypedReceiverMethodErrorOnlyReturn(t *testing.T) {
	cb := NewClassBuilder("Store")
	cb.Constructor(func() *errorStore { return &errorStore{} })
	cb.MethodWithHelp("save", func(self *errorStore, key string) error {
		if key == "bad" {
			return errors.New("save failed: bad key")
		}
		return nil
	}, "save(key)")
	cb.MethodWithHelp("store", func(self *errorStore, ctx context.Context, kwargs Kwargs, key, value string) error {
		if key == "bad" {
			return errors.New("store failed: bad key")
		}
		return nil
	}, "store(key, value)")
	class := cb.Build()
	instance := NewReceiverInstance(class, "Store", &errorStore{})

	save := class.Methods["save"].(*Builtin)
	wantError(t, callBuiltin(t, save, instance, NewString("bad")), "save failed: bad key")
	wantNull(t, callBuiltin(t, save, instance, NewString("good")))

	store := class.Methods["store"].(*Builtin)
	wantError(t, callBuiltin(t, store, instance, NewString("bad"), NewString("v")), "store failed: bad key")
	wantNull(t, callBuiltin(t, store, instance, NewString("good"), NewString("v")))
}

// Methods taking a plain *Instance self take the callTypedMethod path.
func TestPlainMethodErrorOnlyReturn(t *testing.T) {
	cb := NewClassBuilder("Thing")
	cb.MethodWithHelp("close", func(self *Instance) error {
		if broken, ok := self.Field("broken").(*Boolean); ok && broken.value {
			return errors.New("close failed: broken")
		}
		return nil
	}, "close()")
	class := cb.Build()

	broken := NewInstanceWithFields(class, map[string]Object{"broken": NewBoolean(true)})
	healthy := NewInstanceWithFields(class, map[string]Object{"broken": NewBoolean(false)})
	closeFn := class.Methods["close"].(*Builtin)

	wantError(t, callBuiltin(t, closeFn, broken), "close failed: broken")
	wantNull(t, callBuiltin(t, closeFn, healthy))
}

// Library functions take the callTypedFunction path.
func TestLibraryFunctionErrorOnlyReturn(t *testing.T) {
	builder := NewLibraryBuilder("test", "test")
	builder.FunctionWithHelp("fail", func(when bool) error {
		if when {
			return errors.New("boom")
		}
		return nil
	}, "fail(when)")
	lib := builder.Build()
	fail := lib.Functions()["fail"]

	wantError(t, callBuiltin(t, fail, NewBoolean(true)), "boom")
	wantNull(t, callBuiltin(t, fail, NewBoolean(false)))
}

// The (value, error) channel keeps its behaviour on every path.
func TestValueErrorReturnsUnchanged(t *testing.T) {
	cb := NewClassBuilder("Store")
	cb.Constructor(func() *errorStore { return &errorStore{} })
	cb.MethodWithHelp("get", func(self *errorStore, key string) (string, error) {
		if key == "bad" {
			return "", errors.New("get failed: bad key")
		}
		return "value", nil
	}, "get(key)")
	cb.MethodWithHelp("plain_get", func(self *Instance, key string) (string, error) {
		if key == "bad" {
			return "", errors.New("plain get failed: bad key")
		}
		return "value", nil
	}, "plain_get(key)")
	class := cb.Build()
	instance := NewReceiverInstance(class, "Store", &errorStore{})

	get := class.Methods["get"].(*Builtin)
	wantError(t, callBuiltin(t, get, instance, NewString("bad")), "get failed: bad key")
	if result := callBuiltin(t, get, instance, NewString("good")); result.Inspect() != "value" {
		t.Fatalf("typed receiver value: %s", result.Inspect())
	}

	plainGet := class.Methods["plain_get"].(*Builtin)
	plain := NewInstance(class)
	wantError(t, callBuiltin(t, plainGet, plain, NewString("bad")), "plain get failed: bad key")
	if result := callBuiltin(t, plainGet, plain, NewString("good")); result.Inspect() != "value" {
		t.Fatalf("plain method value: %s", result.Inspect())
	}

	builder := NewLibraryBuilder("test", "test")
	builder.FunctionWithHelp("get", func(key string) (string, error) {
		if key == "bad" {
			return "", errors.New("function get failed: bad key")
		}
		return "value", nil
	}, "get(key)")
	fn := builder.Build().Functions()["get"]
	wantError(t, callBuiltin(t, fn, NewString("bad")), "function get failed: bad key")
	if result := callBuiltin(t, fn, NewString("good")); result.Inspect() != "value" {
		t.Fatalf("function value: %s", result.Inspect())
	}
}

// A single return that is not an error is still a value, not an error channel.
func TestSingleValueReturnUnchanged(t *testing.T) {
	cb := NewClassBuilder("Store")
	cb.Constructor(func() *errorStore { return &errorStore{} })
	cb.MethodWithHelp("size", func(self *errorStore) int { return 7 }, "size()")
	class := cb.Build()
	instance := NewReceiverInstance(class, "Store", &errorStore{})
	if result := callBuiltin(t, class.Methods["size"].(*Builtin), instance); result.Inspect() != "7" {
		t.Fatalf("value return: %s", result.Inspect())
	}

	builder := NewLibraryBuilder("test", "test")
	builder.Function("answer", func() int { return 42 })
	if result := callBuiltin(t, builder.Build().Functions()["answer"]); result.Inspect() != "42" {
		t.Fatalf("function value return: %s", result.Inspect())
	}
}
