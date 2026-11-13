package kv_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

const nameKV = "scriptling.runtime.kv"

// recorder collects the names a Register function registers.
type recorder struct{ names []string }

func (r *recorder) RegisterLibrary(lib *object.Library) { r.names = append(r.names, lib.Name()) }

func TestLibraryName(t *testing.T) {
	if kv.LibraryName != nameKV {
		t.Errorf("kv.LibraryName = %q, want %q (scripts import it by this name)", kv.LibraryName, nameKV)
	}
}

func TestRegisterRegistersExactlyItsName(t *testing.T) {
	for _, paths := range [][]string{nil, {}, {t.TempDir()}} {
		var r recorder
		kv.Register(&r, paths)
		if !slices.Equal(r.names, []string{nameKV}) {
			t.Errorf("kv.Register(%v) registered %v, want exactly [%s]", paths, r.names, nameKV)
		}
	}
}

// After kv.Register the library imports under its own name, and, with the
// runtime core registered too, as the kv attribute of scriptling.runtime.
func TestImportWorksAfterRegister(t *testing.T) {
	p := scriptling.New()
	kv.Register(p, nil)
	scripttest.Imports(t, p, nameKV)
	if got := scripttest.EvalString(t, p, `import scriptling.runtime.kv as kv
s = kv.open(":memory:kv_registration_sub")
s.set("k", "sub")
r = s.get("k")
s.close()
r`); got != "sub" {
		t.Errorf("kv through its own name = %q, want sub", got)
	}

	runtime.Register(p)
	if got := scripttest.EvalString(t, p, `import scriptling.runtime as rt
s = rt.kv.open(":memory:kv_registration_parent")
s.set("k", "parent")
r = s.get("k")
s.close()
r`); got != "parent" {
		t.Errorf("kv through scriptling.runtime = %q, want parent", got)
	}
}

// kv is independent of the runtime core: registering it alone does not make
// scriptling.runtime (background tasks) importable.
func TestRegisterAloneDoesNotRegisterTheRuntimeCore(t *testing.T) {
	p := scriptling.New()
	kv.Register(p, nil)
	scripttest.ImportFails(t, p, "scriptling.runtime", "kv.Register registers only scriptling.runtime.kv")
}

func TestNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	runtime.Register(p)
	scripttest.ImportFails(t, p, nameKV, "it is only available after kv.Register")
	typ, msg := scripttest.Raises(t, p, "import scriptling.runtime as rt\nrt.kv.open(':memory:x')")
	if typ == "" || !strings.Contains(msg, "kv") {
		t.Errorf("rt.kv without kv.Register: got %s %q, want an error naming kv", typ, msg)
	}
}

// The KV store's file paths follow allowedPaths exactly as the filesystem
// libraries do: a store outside them is refused with a PermissionError a
// script cannot swallow with `except Exception`, an empty list refuses every
// filesystem path, nil allows any, and in-memory stores are always allowed.
func TestRuntimeKVPathPolicy(t *testing.T) {
	allowed := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "store")
	open := func(path string) string {
		return "import scriptling.runtime.kv as kv\nkv.open(" + strconv.Quote(path) + ").close()"
	}

	p := scriptling.New()
	kv.Register(p, []string{allowed})
	runtime.Register(p)

	// Happy paths: in-memory, and a persistent store inside the allowed path
	// that really holds data across reopening.
	scripttest.Eval(t, p, "import scriptling.runtime.kv as kv\nkv.open(':memory:registration_policy').close()")
	inside := filepath.Join(allowed, "store")
	if got := scripttest.EvalString(t, p, `import scriptling.runtime.kv as kv
s = kv.open(`+strconv.Quote(inside)+`)
s.set("k", "kept")
s.close()
s = kv.open(`+strconv.Quote(inside)+`)
r = s.get("k")
s.close()
r`); got != "kept" {
		t.Errorf("store inside allowedPaths: get = %q, want kept", got)
	}

	// Unhappy: refused, protected, and nothing created outside.
	for _, src := range []string{
		open(outside),
		open(filepath.Join(allowed, "..", filepath.Base(outsideDir), "store")),
		"import scriptling.runtime as rt\nrt.kv.open(" + strconv.Quote(outside) + ")",
	} {
		msg := scripttest.Refused(t, p, src)
		if !strings.Contains(msg, "kv.open: access denied") || !strings.Contains(msg, "outside allowed directories") {
			t.Errorf("refusal message = %q, want the kv.open policy message", msg)
		}
	}
	if entries, _ := os.ReadDir(outsideDir); len(entries) != 0 {
		t.Errorf("refused opens created %d entries outside the allowed paths", len(entries))
	}

	// Deny-all: every filesystem path is refused, in-memory stores still work.
	deny := scriptling.New()
	kv.Register(deny, []string{})
	scripttest.Refused(t, deny, open(filepath.Join(allowed, "denyall")))
	scripttest.Eval(t, deny, open(":memory:denyall"))

	// Unrestricted (nil): any path opens.
	free := scriptling.New()
	kv.Register(free, nil)
	scripttest.Eval(t, free, open(filepath.Join(outsideDir, "free")))
}

// The refusal is not swallowed by a broad handler, and code after the handler
// does not run: the script stops with the PermissionError.
func TestRuntimeKVRefusalNotSwallowedByExceptException(t *testing.T) {
	allowed := t.TempDir()
	outside := filepath.Join(t.TempDir(), "store")
	p := scriptling.New()
	kv.Register(p, []string{allowed})
	res, err := p.Eval(`import scriptling.runtime.kv as kv
r = "not reached"
try:
    kv.open(` + strconv.Quote(outside) + `)
    r = "opened"
except Exception as e:
    r = "swallowed"
r = "continued after handler"
r`)
	if err == nil {
		t.Fatalf("kv.open outside allowedPaths was swallowed: script returned %v", res)
	}
	if !strings.Contains(err.Error(), "outside allowed directories") {
		t.Errorf("error = %v, want the path-policy refusal", err)
	}
	if ex, ok := res.(*object.Exception); !ok || !ex.IsPermissionError() {
		t.Errorf("result = %T %v, want a PermissionError", res, res)
	}
}
