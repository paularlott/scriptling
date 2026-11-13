package http_test

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
	rthttp "github.com/paularlott/scriptling/extlibs/runtime/http"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

const nameHTTP = "scriptling.runtime.http"

// recorder collects the names a Register function registers.
type recorder struct{ names []string }

func (r *recorder) RegisterLibrary(lib *object.Library) { r.names = append(r.names, lib.Name()) }

func TestLibraryName(t *testing.T) {
	if rthttp.LibraryName != nameHTTP {
		t.Errorf("http.LibraryName = %q, want %q (scripts import it by this name)", rthttp.LibraryName, nameHTTP)
	}
}

func TestRegisterRegistersExactlyItsName(t *testing.T) {
	for _, paths := range [][]string{nil, {}, {t.TempDir()}} {
		var r recorder
		rthttp.Register(&r, paths)
		if !slices.Equal(r.names, []string{nameHTTP}) {
			t.Errorf("http.Register(%v) registered %v, want exactly [%s]", paths, r.names, nameHTTP)
		}
	}
}

// After http.Register the library imports under its own name, and, with the
// runtime core registered too, as the http attribute of scriptling.runtime.
func TestImportWorksAfterRegister(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()
	p := scriptling.New()
	rthttp.Register(p, nil)
	scripttest.Imports(t, p, nameHTTP)
	scripttest.Eval(t, p, "import scriptling.runtime.http as http\nhttp.get('/registration_sub', 'h.sub')")

	runtime.Register(p)
	scripttest.Eval(t, p, "import scriptling.runtime as rt\nrt.http.get('/registration_parent', 'h.parent')")

	runtime.RuntimeState.RLock()
	defer runtime.RuntimeState.RUnlock()
	for path, handler := range map[string]string{"GET /registration_sub": "h.sub", "GET /registration_parent": "h.parent"} {
		if r := rthttp.State.Routes[path]; r == nil || r.Handler != handler {
			t.Errorf("route %s = %+v, want handler %s", path, r, handler)
		}
	}
}

func TestNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	runtime.Register(p)
	scripttest.ImportFails(t, p, nameHTTP, "it is only available after http.Register")
	typ, msg := scripttest.Raises(t, p, "import scriptling.runtime as rt\nrt.http.get('/x', 'h.x')")
	if typ == "" || !strings.Contains(msg, "http") {
		t.Errorf("rt.http without http.Register: got %s %q, want an error naming http", typ, msg)
	}
}

// staticDirs lays out an allowed directory with a served directory inside it,
// a directory outside it, and a symlink inside pointing out.
func staticDirs(t *testing.T) (allowed, inside, outside, escape string) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed = filepath.Join(base, "allowed")
	inside = filepath.Join(allowed, "public")
	outside = filepath.Join(base, "outside")
	for _, d := range []string{inside, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	escape = filepath.Join(allowed, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	return allowed, inside, outside, escape
}

func staticRoute(path string) *rthttp.RouteInfo {
	runtime.RuntimeState.RLock()
	defer runtime.RuntimeState.RUnlock()
	return rthttp.State.Routes["GET "+path]
}

func static(urlPath, dir string) string {
	return "import scriptling.runtime.http as http\nhttp.static(" + strconv.Quote(urlPath) + ", " + strconv.Quote(dir) + ")"
}

// static() under allowedPaths: a directory inside is registered with the
// policy recorded for the server, a directory outside (directly, via "/", or
// through a symlink) is refused with a PermissionError `except Exception`
// cannot swallow, and nothing is registered for it.
func TestStaticPathPolicy(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()
	allowed, inside, outside, escape := staticDirs(t)

	p := scriptling.New()
	rthttp.Register(p, []string{allowed})
	runtime.Register(p)

	scripttest.Eval(t, p, static("/public", inside))
	route := staticRoute("/public")
	if route == nil || !route.Static || route.StaticDir != inside {
		t.Fatalf("static route inside allowedPaths not registered: %+v", route)
	}
	if !slices.Equal(route.StaticAllowed, []string{allowed}) {
		t.Errorf("route.StaticAllowed = %v, want [%s] so the server confines what it serves", route.StaticAllowed, allowed)
	}

	for i, dir := range []string{outside, "/", escape} {
		urlPath := "/refused" + strconv.Itoa(i)
		msg := scripttest.Refused(t, p, static(urlPath, dir))
		if !strings.Contains(msg, "outside allowed directories") {
			t.Errorf("static(%q) refusal = %q, want the path-policy message", dir, msg)
		}
		if r := staticRoute(urlPath); r != nil {
			t.Errorf("refused static(%q) still registered a route: %+v", dir, r)
		}
	}
	// The parent form is held to the same policy.
	scripttest.Refused(t, p, "import scriptling.runtime as rt\nrt.http.static('/parent', "+strconv.Quote(outside)+")")
}

// An empty allowedPaths refuses every directory, even one that would be
// allowed by a real list.
func TestStaticDenyAll(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()
	_, inside, outside, _ := staticDirs(t)
	p := scriptling.New()
	rthttp.Register(p, []string{})
	for _, dir := range []string{inside, outside, "."} {
		scripttest.Refused(t, p, static("/denied", dir))
	}
	if r := staticRoute("/denied"); r != nil {
		t.Errorf("deny-all registered a static route: %+v", r)
	}
}

// nil allowedPaths is the explicit "unrestricted" setting: any directory is
// accepted and the route carries no restriction.
func TestStaticUnrestrictedWhenNil(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()
	_, _, outside, _ := staticDirs(t)
	p := scriptling.New()
	rthttp.Register(p, nil)
	scripttest.Eval(t, p, static("/free", outside))
	if r := staticRoute("/free"); r == nil || r.StaticDir != outside || r.StaticAllowed != nil {
		t.Errorf("unrestricted static route = %+v, want %s with no restriction", r, outside)
	}
}

// The refusal is not swallowed by a broad handler and stops the script.
func TestStaticRefusalNotSwallowedByExceptException(t *testing.T) {
	runtime.ResetRuntime()
	defer runtime.ResetRuntime()
	allowed, _, outside, _ := staticDirs(t)
	p := scriptling.New()
	rthttp.Register(p, []string{allowed})
	res, err := p.Eval(`import scriptling.runtime.http as http
r = "not reached"
try:
    http.static("/x", ` + strconv.Quote(outside) + `)
    r = "registered"
except Exception as e:
    r = "swallowed"
r = "continued after handler"
r`)
	if err == nil {
		t.Fatalf("static() outside allowedPaths was swallowed: script returned %v", res)
	}
	if ex, ok := res.(*object.Exception); !ok || !ex.IsPermissionError() {
		t.Errorf("result = %T %v, want a PermissionError", res, res)
	}
}
