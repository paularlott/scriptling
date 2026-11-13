package runtime_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	rt "github.com/paularlott/scriptling/extlibs/runtime"
	rthttp "github.com/paularlott/scriptling/extlibs/runtime/http"
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// recorder collects the names a Register function registers.
type recorder struct{ names []string }

func (r *recorder) RegisterLibrary(lib *object.Library) { r.names = append(r.names, lib.Name()) }

const (
	nameRuntime = "scriptling.runtime"
	nameHTTP    = "scriptling.runtime.http"
	nameKV      = "scriptling.runtime.kv"
	nameSync    = "scriptling.runtime.sync"
	nameSandbox = "scriptling.runtime.sandbox"
	nameJSONRPC = "scriptling.runtime.jsonrpc"
	nameMCP     = "scriptling.runtime.mcp"
	namePlugin  = "scriptling.runtime.plugin"
)

func TestLibraryNameConstants(t *testing.T) {
	for got, want := range map[string]string{
		rt.RuntimeLibraryName: nameRuntime, rthttp.LibraryName: nameHTTP,
		kv.LibraryName: nameKV, rt.RuntimeSyncLibraryName: nameSync,
		rt.RuntimeSandboxLibraryName: nameSandbox, rt.RuntimeJSONRPCLibraryName: nameJSONRPC,
		rt.RuntimeMCPLibraryName: nameMCP, rt.RuntimePluginLibraryName: namePlugin,
	} {
		if got != want {
			t.Errorf("runtime library name constant is %q, want %q (scripts import it by this name)", got, want)
		}
	}
}

type registrar = interface{ RegisterLibrary(*object.Library) }

// registerEverything registers the runtime core and every sub-library, the
// set the CLI registers (plugin is registered separately there, as here).
// There is no Go function for this: kv and http live in their own packages
// so an application links only the pieces it registers.
func registerEverything(r registrar, allowedPaths []string) {
	rthttp.Register(r, allowedPaths)
	kv.Register(r, allowedPaths)
	rt.RegisterSync(r)
	rt.RegisterSandbox(r, allowedPaths)
	rt.RegisterJSONRPC(r)
	rt.RegisterMCP(r)
	rt.Register(r)
}

// runtimeRegisters maps each runtime Register function to exactly the names it
// must register.
var runtimeRegisters = []struct {
	fn       string
	register func(registrar)
	names    []string
}{
	{"Register", func(r registrar) { rt.Register(r) }, []string{nameRuntime}},
	{"http.Register", func(r registrar) { rthttp.Register(r, nil) }, []string{nameHTTP}},
	{"RegisterJSONRPC", func(r registrar) { rt.RegisterJSONRPC(r) }, []string{nameJSONRPC}},
	{"RegisterMCP", func(r registrar) { rt.RegisterMCP(r) }, []string{nameMCP}},
	{"kv.Register", func(r registrar) { kv.Register(r, nil) }, []string{nameKV}},
	{"kv.Register(deny-all)", func(r registrar) { kv.Register(r, []string{}) }, []string{nameKV}},
	{"RegisterSync", func(r registrar) { rt.RegisterSync(r) }, []string{nameSync}},
	{"RegisterSandbox", func(r registrar) { rt.RegisterSandbox(r, nil) }, []string{nameSandbox}},
	{"RegisterPlugin", func(r registrar) { rt.RegisterPlugin(r) }, []string{namePlugin}},
	{"everything", func(r registrar) { registerEverything(r, nil) },
		[]string{nameRuntime, nameHTTP, nameKV, nameSync, nameSandbox, nameJSONRPC, nameMCP}},
}

func TestRuntimeRegisterFunctionsRegisterExactlyTheirNames(t *testing.T) {
	for _, tt := range runtimeRegisters {
		t.Run(tt.fn, func(t *testing.T) {
			var r recorder
			tt.register(&r)
			got, want := slices.Clone(r.names), slices.Clone(tt.names)
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("runtime.%s registered %v, want exactly %v", tt.fn, got, want)
			}

			p := scriptling.New()
			tt.register(p)
			for _, n := range tt.names {
				scripttest.Imports(t, p, n)
			}
		})
	}
}

func TestRuntimeCoreOnlyIsNotTheSubLibraries(t *testing.T) {
	p := scriptling.New()
	rt.Register(p)
	if got := scripttest.Eval(t, p, "import scriptling.runtime as runtime\ncallable(runtime.background)").Inspect(); got != "True" {
		t.Errorf("runtime.background not callable after runtime.Register: %s", got)
	}
	for _, n := range []string{nameSync, nameKV, nameHTTP, nameSandbox} {
		scripttest.ImportFails(t, p, n, "runtime.Register registers the core library only; sub-libraries need their own Register call")
	}
}

func TestRuntimeSyncPrimitives(t *testing.T) {
	p := scriptling.New()
	rt.RegisterSync(p)
	got := scripttest.Eval(t, p, `import scriptling.runtime.sync as sync
c = sync.Atomic("registration_test_counter", initial=0)
c.add(2)
c.add(3)
q = sync.Queue("registration_test_queue", maxsize=4)
q.put("a")
[c.get(), q.size(), q.get()]`).Inspect()
	if want := "[5, 1, 'a']"; got != want {
		t.Errorf("sync primitives = %s, want %s", got, want)
	}
}

func TestRuntimeRegisterAllExposesSubLibrariesOnParent(t *testing.T) {
	p := scriptling.New()
	registerEverything(p, nil)
	rt.RegisterPlugin(p)
	got := scripttest.Eval(t, p, `import scriptling.runtime as runtime
a = runtime.sync.Atomic("registration_test_parent", initial=7)
s = runtime.kv.open(":memory:registration_test")
s.set("k", "v")
r = [a.get(), s.get("k")]
s.close()
r`).Inspect()
	if want := "[7, 'v']"; got != want {
		t.Errorf("parent access with every sub-library registered = %s, want %s", got, want)
	}
	got = scripttest.Eval(t, p, `import scriptling.runtime as runtime
[callable(runtime.background), callable(runtime.http.get), callable(runtime.kv.open), callable(runtime.sync.Atomic),
 callable(runtime.sandbox.create), callable(runtime.jsonrpc.method), callable(runtime.mcp.tool), callable(runtime.plugin.serve)]`).Inspect()
	if want := "[True, True, True, True, True, True, True, True]"; got != want {
		t.Errorf("sub-library attributes on scriptling.runtime = %s, want %s", got, want)
	}
}

// Each sub-library registered with the same registrar shows up as an
// attribute of scriptling.runtime whether it was registered before or after
// the core library, and one that was not registered does not.
func TestRuntimeParentExposesOnlyRegisteredSubLibraries(t *testing.T) {
	for _, order := range []string{"subs first", "core first"} {
		t.Run(order, func(t *testing.T) {
			p := scriptling.New()
			if order == "core first" {
				rt.Register(p)
			}
			kv.Register(p, nil)
			rt.RegisterSync(p)
			if order == "subs first" {
				rt.Register(p)
			}
			got := scripttest.Eval(t, p, `import scriptling.runtime as runtime
s = runtime.kv.open(":memory:registration_partial")
s.set("k", 1)
r = [s.get("k"), runtime.sync.Atomic("registration_partial", initial=2).get()]
s.close()
r`).Inspect()
			if want := "[1, 2]"; got != want {
				t.Errorf("registered sub-libraries through the parent = %s, want %s", got, want)
			}
			for _, attr := range []string{"http", "sandbox", "jsonrpc", "mcp", "plugin"} {
				typ, msg := scripttest.Raises(t, p, "import scriptling.runtime as runtime\nruntime."+attr+".x")
				if typ == "" || !strings.Contains(msg, attr) {
					t.Errorf("runtime.%s without registering it: got %s %q, want an error naming it", attr, typ, msg)
				}
			}
			scripttest.ImportFails(t, p, nameHTTP, "http was not registered")
		})
	}
}

func TestRuntimeNotImportableWithoutRegister(t *testing.T) {
	for _, n := range []string{nameRuntime, nameHTTP, nameKV, nameSync, nameSandbox, nameJSONRPC, nameMCP, namePlugin} {
		t.Run(n, func(t *testing.T) {
			p := scriptling.New()
			stdlib.RegisterAll(p)
			scripttest.ImportFails(t, p, n, "it is only available after the matching runtime.Register* call")
		})
	}
}

func TestRuntimeRegisterAllTwice(t *testing.T) {
	p := scriptling.New()
	registerEverything(p, nil)
	registerEverything(p, nil)
	if got := scripttest.EvalInt(t, p, "import scriptling.runtime as runtime\nruntime.sync.Atomic('registration_test_twice', initial=3).get()"); got != 3 {
		t.Errorf("after registering twice: Atomic.get() = %d, want 3", got)
	}
}
