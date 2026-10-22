package plugin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/object"
)

// namedPluginServer serves a scriptling plugin whose ping() returns reply.
func namedPluginServer(t *testing.T, reply string) string {
	t.Helper()
	ps := NewServer("anything", "1.0.0", "test plugin")
	ps.RegisterBuiltin("ping", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		return object.NewString(reply)
	})
	srv := httptest.NewServer(ps)
	t.Cleanup(srv.Close)
	return srv.URL
}

func hostLibrary(name, reply string) *object.Library {
	return object.NewLibrary(name, map[string]*object.Builtin{
		"ping": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewString(reply)
		}},
	}, nil, "")
}

func evalString(t *testing.T, p *scriptling.Scriptling, script string) (string, error) {
	t.Helper()
	result, err := p.Eval(script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

// A script's plugin.load() registered its plugin over any library of the
// same name, dotted names verbatim, so a script could replace a host library
// (scriptling.secret) for every module it imported afterwards (CWE-706,
// strix 4250 vuln-0021). Like plugins the host loads at startup, it is now
// refused a name already registered; a free dotted name is still the
// plugin author's namespace.
func TestPluginLoadCannotTakeHostLibraryName(t *testing.T) {
	url := namedPluginServer(t, "impostor")
	p := registerControl(t, NewManager(nil), nil, true)
	p.RegisterLibrary(hostLibrary("scriptling.secret", "host"))

	_, err := p.Eval(`import scriptling.plugin as P
P.load("scriptling.secret", "` + url + `", scriptling=True)`)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("load over a host library: want a refusal, got %v", err)
	}
	if got, err := evalString(t, p, "import scriptling.secret as s\ns.ping()"); err != nil || got != "host" {
		t.Fatalf("host library after the attempt: %q, %v", got, err)
	}

	got, err := evalString(t, p, `import scriptling.plugin as P
P.load("myplugin.hello", "`+url+`", scriptling=True)
import myplugin.hello as h
h.ping()`)
	if err != nil || got != "impostor" {
		t.Fatalf("free dotted name: %q, %v", got, err)
	}
}

// Loading the same plugin again is a no-op, a different plugin under a
// loaded name is refused, and an explicit plugin.* name is accepted.
func TestPluginLoadSamePluginTwice(t *testing.T) {
	first := namedPluginServer(t, "first")
	second := namedPluginServer(t, "second")
	p := registerControl(t, NewManager(nil), nil, true)

	for i := 0; i < 3; i++ {
		got, err := evalString(t, p, `import scriptling.plugin as P
P.load("widgets", "`+first+`", scriptling=True)`)
		if err != nil || got != "plugin.widgets" {
			t.Fatalf("load %d: %q, %v", i, got, err)
		}
	}
	if got, err := evalString(t, p, "import plugin.widgets as w\nw.ping()"); err != nil || got != "first" {
		t.Fatalf("plugin.widgets: %q, %v", got, err)
	}
	if _, err := p.Eval(`import scriptling.plugin as P
P.load("widgets", "` + second + `", scriptling=True)`); err == nil || !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("different plugin under a loaded name: %v", err)
	}
	if got, err := evalString(t, p, `import scriptling.plugin as P
P.load("plugin.gadgets", "`+second+`", scriptling=True)`); err != nil || got != "plugin.gadgets" {
		t.Fatalf("explicit plugin.* name: %q, %v", got, err)
	}
	if got, err := evalString(t, p, "import plugin.widgets as w\nw.ping()"); err != nil || got != "first" {
		t.Fatalf("plugin.widgets after the refusal: %q, %v", got, err)
	}
}

// The manager is shared by every interpreter, so a plugin one script loads
// is global: a later interpreter sees it. It still never replaces a library
// another interpreter registered under that name, at setup or when its own
// script loads the same plugin.
func TestPluginNamesAreGlobalButNeverReplace(t *testing.T) {
	url := namedPluginServer(t, "plugin")
	manager := NewManager(nil)
	defer manager.Close()

	a := registerControl(t, manager, nil, true)
	if _, err := a.Eval(`import scriptling.plugin as P
P.load("widgets", "` + url + `", scriptling=True)`); err != nil {
		t.Fatal(err)
	}

	b := registerControl(t, manager, nil, true)
	if got, err := evalString(t, b, "import plugin.widgets as w\nw.ping()"); err != nil || got != "plugin" {
		t.Fatalf("later interpreter: %q, %v", got, err)
	}

	c := scriptling.New()
	c.RegisterLibrary(hostLibrary("plugin.widgets", "host"))
	RegisterLibrariesConfigured(c, manager, nil, true)
	if got, err := evalString(t, c, "import plugin.widgets as w\nw.ping()"); err != nil || got != "host" {
		t.Fatalf("host library at setup: %q, %v", got, err)
	}
	// A fresh interpreter, so nothing is imported before its script loads
	// the plugin.
	d := scriptling.New()
	d.RegisterLibrary(hostLibrary("plugin.widgets", "host"))
	RegisterLibrariesConfigured(d, manager, nil, true)
	if got, err := evalString(t, d, `import scriptling.plugin as P
P.load("widgets", "`+url+`", scriptling=True)
import plugin.widgets as w
w.ping()`); err != nil || got != "host" {
		t.Fatalf("host library after its script loaded the plugin: %q, %v", got, err)
	}
}

// A module the library loader supplies from disk is a library the host has
// too: a registered library wins over the loader for every later import, so
// a plugin loaded under that name would shadow the on-disk module.
func TestPluginLoadCannotTakeLoaderBackedModuleName(t *testing.T) {
	url := namedPluginServer(t, "impostor")
	p := registerControl(t, NewManager(nil), nil, true)
	p.SetLibraryLoader(libloader.NewMemoryLoader(map[string]string{
		"mypkg.util": "def ping():\n    return 'on disk'\n",
	}))

	_, err := p.Eval(`import scriptling.plugin as P
P.load("mypkg.util", "` + url + `", scriptling=True)`)
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("load over a loader-backed module: want a refusal, got %v", err)
	}
	if got, err := evalString(t, p, "import mypkg.util as u\nu.ping()"); err != nil || got != "on disk" {
		t.Fatalf("on-disk module after the attempt: %q, %v", got, err)
	}
	if got, err := evalString(t, p, `import scriptling.plugin as P
P.load("mypkg.extra", "`+url+`", scriptling=True)
import mypkg.extra as e
e.ping()`); err != nil || got != "impostor" {
		t.Fatalf("free name beside the module: %q, %v", got, err)
	}
}

// The manager is shared, so a plugin one interpreter loads under a free name
// is offered to every later interpreter at setup. Where a later interpreter's
// library loader has a module of that name, the module wins there, whether
// the plugin arrives at setup or through that interpreter's own script.
func TestPluginNeverShadowsLoaderBackedModuleAcrossInterpreters(t *testing.T) {
	url := namedPluginServer(t, "impostor")
	manager := NewManager(nil)
	defer manager.Close()
	onDisk := func() scriptling.LibraryLoader {
		return libloader.NewMemoryLoader(map[string]string{
			"mypkg.util": "def ping():\n    return 'on disk'\n",
		})
	}

	// No module here, so the name is free and the load succeeds.
	a := registerControl(t, manager, nil, true)
	if got, err := evalString(t, a, `import scriptling.plugin as P
P.load("mypkg.util", "`+url+`", scriptling=True)
import mypkg.util as u
u.ping()`); err != nil || got != "impostor" {
		t.Fatalf("first interpreter, free name: %q, %v", got, err)
	}

	// Set up after the load: the plugin is skipped for the module.
	b := scriptling.New()
	b.SetLibraryLoader(onDisk())
	RegisterLibrariesConfigured(b, manager, nil, true)
	if got, err := evalString(t, b, "import mypkg.util as u\nu.ping()"); err != nil || got != "on disk" {
		t.Fatalf("later interpreter with the module: %q, %v", got, err)
	}
	if !strings.Contains(strings.Join(manager.Warnings(), "\n"), "mypkg.util ignored") {
		t.Fatalf("expected a warning that the plugin was skipped, got %q", manager.Warnings())
	}

	// Its own script loading the already-loaded plugin is a no-op that
	// still leaves the module in place.
	c := scriptling.New()
	c.SetLibraryLoader(onDisk())
	RegisterLibrariesConfigured(c, manager, nil, true)
	if got, err := evalString(t, c, `import scriptling.plugin as P
P.load("mypkg.util", "`+url+`", scriptling=True)
import mypkg.util as u
u.ping()`); err != nil || got != "on disk" {
		t.Fatalf("module after its own script loaded the plugin: %q, %v", got, err)
	}
}
