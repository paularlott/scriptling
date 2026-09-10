package scriptling

import (
	"testing"
)

// Class bodies must execute all statements, not just method definitions.
// A plain assignment becomes a class attribute (Python: C.x == 5).
func TestClassBodyAttributes(t *testing.T) {
	got := evalString(t, `
class C:
    x = 5
    label = "hi"
"%d|%s" % (C.x, C.label)
`)
	if got != "5|hi" {
		t.Fatalf("class attributes: got %q, want %q", got, "5|hi")
	}
}

// Class attributes are visible via an instance (attribute fallback to class).
func TestClassAttributeVisibleOnInstance(t *testing.T) {
	got := evalString(t, `
class C:
    kind = "widget"
    def __init__(self):
        self.n = 1
c = C()
c.kind
`)
	if got != "widget" {
		t.Fatalf("got %q, want %q", got, "widget")
	}
}

// A raise in a class body propagates out of the class definition rather than
// being silently dropped.
func TestClassBodyRaisePropagates(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("class-boom")
caught = "no"
try:
    class C:
        y = boom()
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes (class-body raise was swallowed)", got)
	}
}

// Methods and attributes coexist and both work.
func TestClassMethodsStillWorkWithAttributes(t *testing.T) {
	got := evalString(t, `
class Counter:
    kind = "counter"
    def __init__(self):
        self.n = 0
    def inc(self):
        self.n = self.n + 1
        return self.n
c = Counter()
"%s|%d|%d" % (c.kind, c.inc(), c.inc())
`)
	if got != "counter|1|2" {
		t.Fatalf("got %q, want %q", got, "counter|1|2")
	}
}

// A decorator EXPRESSION that raises propagates the real exception, both for a
// function and for a method inside a class body (where the method previously
// vanished silently).
func TestDecoratorExpressionRaisePropagates(t *testing.T) {
	t.Run("function", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("dec-boom")
caught = "no"
try:
    @boom()
    def f():
        pass
except ValueError:
    caught = "yes"
caught
`)
		if got != "yes" {
			t.Fatalf("got %q, want yes", got)
		}
	})
	t.Run("class method", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("dec-boom-method")
caught = "no"
try:
    class C:
        @boom()
        def m(self):
            pass
except ValueError:
    caught = "yes"
caught
`)
		if got != "yes" {
			t.Fatalf("got %q, want yes", got)
		}
	})
}

// Out-of-range subscript assignment raises a catchable IndexError, consistent
// with out-of-range reads.
func TestSubscriptAssignOutOfRangeRaises(t *testing.T) {
	got := evalString(t, `
l = [1, 2, 3]
caught = "no"
try:
    l[9] = 5
except IndexError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes", got)
	}
}

// An exception raised by a module's top-level code propagates to the import
// site with its ORIGINAL type, not flattened into an ImportError.
func TestImportPropagatesModuleRaiseWithType(t *testing.T) {
	p := New()
	if err := p.RegisterScriptLibrary("modboom", `raise ValueError("mod-boom")`); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	got, err := p.Eval(`
caught = "no"
try:
    import modboom
except ValueError as e:
    caught = "ValueError:" + str(e)
except ImportError:
    caught = "ImportError"
caught
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got.Inspect() != "ValueError:mod-boom" {
		t.Fatalf("got %q, want %q (module raise should keep its type)", got.Inspect(), "ValueError:mod-boom")
	}
}

// from-import likewise propagates the original module exception type.
func TestFromImportPropagatesModuleRaiseWithType(t *testing.T) {
	p := New()
	if err := p.RegisterScriptLibrary("modboom2", `raise ValueError("mod-boom2")`); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	got, err := p.Eval(`
caught = "no"
try:
    from modboom2 import anything
except ValueError as e:
    caught = "ValueError:" + str(e)
except ImportError:
    caught = "ImportError"
caught
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got.Inspect() != "ValueError:mod-boom2" {
		t.Fatalf("got %q, want %q", got.Inspect(), "ValueError:mod-boom2")
	}
}
