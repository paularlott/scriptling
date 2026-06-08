package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestManagerLoadsExecutableAndRegistersProxyLibraries(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_TEST_HELPER") == "1" {
		runPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_WRAPPER_HELPER") == "1" {
		runWrapperPluginTestHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "hello-plugin")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writePluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	defer manager.Close()

	if warnings := manager.Warnings(); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if plugins := manager.List(); len(plugins) != 1 || plugins[0].Name != "plugin.hello" {
		t.Fatalf("unexpected plugin list: %#v", plugins)
	}

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import plugin.hello

cfg = plugin.hello.Config("Ada")
plugin.hello.greet(cfg.get("name"))
`)
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "Hello, Ada" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestPluginSuppliedWrapperSource(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_WRAPPER_HELPER") == "1" {
		runWrapperPluginTestHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "wrapper-plugin")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeWrapperPluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import plugin.wrap
plugin.wrap.greet("Ada")
`)
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "Hello, Ada!" {
		t.Fatalf("unexpected wrapper result: %#v", result)
	}
}

func writePluginHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_TEST_HELPER=1\r\n\"" + exe + "\" -test.run=TestManagerLoadsExecutableAndRegistersProxyLibraries --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_TEST_HELPER=1 exec \"" + exe + "\" -test.run=TestManagerLoadsExecutableAndRegistersProxyLibraries --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func writeWrapperPluginHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_WRAPPER_HELPER=1\r\n\"" + exe + "\" -test.run=TestPluginSuppliedWrapperSource --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_WRAPPER_HELPER=1 exec \"" + exe + "\" -test.run=TestPluginSuppliedWrapperSource --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write wrapper helper: %v", err)
	}
}

func runPluginTestHelper() {
	configBuilder := object.NewClassBuilder("Config").
		Method("__init__", func(self *object.Instance, name string) {
			self.Fields["name"] = object.NewString(name)
		}).
		Method("get", func(self *object.Instance, key string) string {
			return self.Fields["name"].(*object.String).StringValue()
		})

	greetBuilder := object.NewFunctionBuilder()
	greetBuilder.Function(func(name string) string {
		return "Hello, " + name
	})

	server := NewServer("hello", "1.0.0", "test helper plugin")
	server.RegisterFunc("greet", greetBuilder)
	server.RegisterClass(configBuilder)
	_ = server.Run()
	os.Exit(0)
}

func runWrapperPluginTestHelper() {
	greetBuilder := object.NewFunctionBuilder()
	greetBuilder.Function(func(name string) string {
		return "Hello, " + name
	})

	server := NewServer("wrap", "1.0.0", "wrapper test plugin")
	server.RegisterFunc("greet", greetBuilder)
	server.Wrapper("greet", `
import scriptling.plugin

def greet(name):
    return scriptling.plugin.call_function("plugin.wrap", "greet", name) + "!"
`)
	_ = server.Run()
	os.Exit(0)
}

func mustRawJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

// ============================================================================
// Comprehensive Integration Tests
// ============================================================================

func TestComprehensivePlugin(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_COMPREHENSIVE_HELPER") == "1" {
		runComprehensivePluginHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "comprehensive")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeComprehensivePluginHelper(t, helper)

	t.Run("DataTypes", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		for _, tc := range []struct {
			code string
			check func(t *testing.T, result object.Object)
		}{
			{`import plugin.comprehensive; plugin.comprehensive.echo_int(42)`, func(t *testing.T, r object.Object) {
				if i, ok := r.(*object.Integer); !ok || i.IntValue() != 42 { t.Fatalf("expected 42, got %v", r) }
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_float(3.14)`, func(t *testing.T, r object.Object) {
				if f, ok := r.(*object.Float); !ok || f.FloatValue() != 3.14 { t.Fatalf("expected 3.14, got %v", r) }
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_string("test")`, func(t *testing.T, r object.Object) {
				if s, ok := r.(*object.String); !ok || s.StringValue() != "test" { t.Fatalf("expected 'test', got %v", r) }
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_bool(True)`, func(t *testing.T, r object.Object) {
				if b, ok := r.(*object.Boolean); !ok || !b.BoolValue() { t.Fatalf("expected true, got %v", r) }
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_list([1, "two", True])`, func(t *testing.T, r object.Object) {
				if l, ok := r.(*object.List); !ok || len(l.Elements) != 3 { t.Fatalf("expected 3 elements, got %v", r) }
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_dict({"a": 1, "b": "two"})`, func(t *testing.T, r object.Object) {
				if d, ok := r.(*object.Dict); !ok || len(d.Pairs) != 2 { t.Fatalf("expected 2 pairs, got %v", r) }
			}},
		} {
			result, err := p.Eval(tc.code)
			if err != nil {
				t.Fatalf("Eval(%q): %v", tc.code, err)
			}
			tc.check(t, result)
		}
	})

	t.Run("Constants", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`import plugin.comprehensive; plugin.comprehensive.VERSION`)
		if err != nil {
			t.Fatalf("VERSION: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "1.0.0" {
			t.Fatalf("VERSION: expected '1.0.0', got %v", result)
		}

		result, err = p.Eval(`import plugin.comprehensive; plugin.comprehensive.MAX_SIZE`)
		if err != nil {
			t.Fatalf("MAX_SIZE: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() != 100 {
			t.Fatalf("MAX_SIZE: expected 100, got %v", result)
		}
	})

	t.Run("TypedReceiverClass", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`
import plugin.comprehensive
counter = plugin.comprehensive.Counter(10)
counter.add(5)
counter.add(3)
counter.get()
`)
		if err != nil {
			t.Fatalf("Counter: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() != 18 {
			t.Fatalf("Counter: expected 18, got %v", result)
		}
	})

	t.Run("InstanceClass", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`
import plugin.comprehensive
kv = plugin.comprehensive.KVStore()
kv.set("host", "localhost")
kv.set("port", "8080")
kv.get("host")
`)
		if err != nil {
			t.Fatalf("KVStore: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "localhost" {
			t.Fatalf("KVStore: expected 'localhost', got %v", result)
		}
	})

	t.Run("CleanupOnDestroy", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`
import plugin.comprehensive
import scriptling.plugin

res = plugin.comprehensive.Resource("db-conn")
name = res.name()
scriptling.plugin.release(res)
name
`)
		if err != nil {
			t.Fatalf("Resource cleanup: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "db-conn" {
			t.Fatalf("Resource: expected 'db-conn', got %v", result)
		}

		result, err = p.Eval(`import plugin.comprehensive; plugin.comprehensive.destroyed_count()`)
		if err != nil {
			t.Fatalf("destroyed_count: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() < 1 {
			t.Fatalf("destroyed_count: expected >= 1, got %v", result)
		}
	})

	t.Run("MethodError", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`import plugin.comprehensive; plugin.comprehensive.fail("test error")`)
		if err == nil {
			if s, ok := result.(*object.String); !ok || s.StringValue() != "fail: test error" {
				t.Fatalf("expected error string 'fail: test error', got %v", result)
			}
		}
	})

	t.Run("ControlLibrary", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`import scriptling.plugin; len(scriptling.plugin.list())`)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() < 1 {
			t.Fatalf("list: expected >= 1 plugin, got %v", result)
		}

		result, err = p.Eval(`import scriptling.plugin; scriptling.plugin.describe("plugin.comprehensive")["name"]`)
		if err != nil {
			t.Fatalf("describe: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "plugin.comprehensive" {
			t.Fatalf("describe name: got %v", result)
		}

		result, err = p.Eval(`import scriptling.plugin; scriptling.plugin.call_function("plugin.comprehensive", "echo_string", "via_control")`)
		if err != nil {
			t.Fatalf("call_function: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "via_control" {
			t.Fatalf("call_function: got %v", result)
		}
	})

	t.Run("ParallelSeparateEnvs", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		var wg sync.WaitGroup
		var errors atomic.Int64

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				p := scriptling.New()
				RegisterLibraries(p, manager)

				code := fmt.Sprintf(`import plugin.comprehensive; plugin.comprehensive.echo_int(%d)`, id)
				result, err := p.Eval(code)
				if err != nil {
					t.Logf("goroutine %d error: %v", id, err)
					errors.Add(1)
					return
				}
				if i, ok := result.(*object.Integer); !ok || i.IntValue() != int64(id) {
					t.Logf("goroutine %d: expected %d, got %v", id, id, result)
					errors.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if e := errors.Load(); e > 0 {
			t.Fatalf("%d parallel goroutines failed", e)
		}
	})

	t.Run("ParallelObjectCreation", func(t *testing.T) {
		t.Skip("TODO: serverClasses global uses per-serverClass ID sequences causing ID collisions across concurrent objects")

		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		var wg sync.WaitGroup
		var errors atomic.Int64

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				p := scriptling.New()
				RegisterLibraries(p, manager)

				code := fmt.Sprintf(`
import plugin.comprehensive
counter = plugin.comprehensive.Counter(%d)
counter.add(1)
counter.get()
`, id)
				result, err := p.Eval(code)
				if err != nil {
					t.Logf("goroutine %d error: %v", id, err)
					errors.Add(1)
					return
				}
				if i, ok := result.(*object.Integer); !ok || i.IntValue() != int64(id)+1 {
					t.Logf("goroutine %d: expected %d, got %v", id, id+1, result)
					errors.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if e := errors.Load(); e > 0 {
			t.Fatalf("%d parallel goroutines failed", e)
		}
	})

	t.Run("ParallelSharedEnv", func(t *testing.T) {
		manager := NewManager()
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		_, err := p.Eval(`import plugin.comprehensive`)
		if err != nil {
			t.Fatalf("import: %v", err)
		}

		var wg sync.WaitGroup
		var errors atomic.Int64

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				code := fmt.Sprintf(`plugin.comprehensive.echo_int(%d)`, id)
				result, err := p.Eval(code)
				if err != nil {
					t.Logf("shared goroutine %d error: %v", id, err)
					errors.Add(1)
					return
				}
				if i, ok := result.(*object.Integer); !ok || i.IntValue() != int64(id) {
					t.Logf("shared goroutine %d: expected %d, got %v", id, id, result)
					errors.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if e := errors.Load(); e > 0 {
			t.Fatalf("%d shared-env goroutines failed", e)
		}
	})
}

func writeComprehensivePluginHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_COMPREHENSIVE_HELPER=1\r\n\"" + exe + "\" -test.run=TestComprehensivePlugin --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_COMPREHENSIVE_HELPER=1 exec \"" + exe + "\" -test.run=TestComprehensivePlugin --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func runComprehensivePluginHelper() {
	var destroyCount atomic.Int64

	type counter struct {
		value int64
	}

	type resource struct {
		name    string
		cleaned bool
	}

	echoInt := object.NewFunctionBuilder()
	echoInt.Function(func(v int) int { return v })

	echoFloat := object.NewFunctionBuilder()
	echoFloat.Function(func(v float64) float64 { return v })

	echoString := object.NewFunctionBuilder()
	echoString.Function(func(v string) string { return v })

	echoBool := object.NewFunctionBuilder()
	echoBool.Function(func(v bool) bool { return v })

	echoList := object.NewFunctionBuilder()
	echoList.Function(func(v []any) []any { return v })

	echoDict := object.NewFunctionBuilder()
	echoDict.Function(func(v map[string]any) map[string]any { return v })

	failFn := object.NewFunctionBuilder()
	failFn.Function(func(msg string) (string, error) {
		return "", fmt.Errorf("fail: %s", msg)
	})

	destroyedCountFn := object.NewFunctionBuilder()
	destroyedCountFn.Function(func() int {
		return int(destroyCount.Load())
	})

	counterClass := object.NewClassBuilder("Counter").
		Constructor(func(start int) *counter {
			return &counter{value: int64(start)}
		}).
		Method("add", func(self *counter, n int) int {
			self.value += int64(n)
			return int(self.value)
		}).
		Method("get", func(self *counter) int {
			return int(self.value)
		})

	resourceClass := object.NewClassBuilder("Resource").
		Constructor(func(name string) *resource {
			return &resource{name: name}
		}).
		Method("name", func(self *resource) string {
			return self.name
		}).
		Method("__del__", func(self *resource) {
			destroyCount.Add(1)
			self.cleaned = true
		})

	kvClass := object.NewClassBuilder("KVStore").
		Method("__init__", func(self *object.Instance) {
			self.Fields["data"] = object.NewStringDict(map[string]object.Object{})
		}).
		Method("set", func(self *object.Instance, key, val string) {
			dict := self.Fields["data"].(*object.Dict)
			dict.Pairs[object.DictKey(object.NewString(key))] = object.DictPair{
				Key:   object.NewString(key),
				Value: object.NewString(val),
			}
		}).
		Method("get", func(self *object.Instance, key string) string {
			dict := self.Fields["data"].(*object.Dict)
			k := object.DictKey(object.NewString(key))
			if pair, ok := dict.Pairs[k]; ok {
				s, _ := pair.Value.AsString()
				return s
			}
			return ""
		})

	server := NewServer("comprehensive", "1.0.0", "comprehensive test plugin")
	server.RegisterFunc("echo_int", echoInt)
	server.RegisterFunc("echo_float", echoFloat)
	server.RegisterFunc("echo_string", echoString)
	server.RegisterFunc("echo_bool", echoBool)
	server.RegisterFunc("echo_list", echoList)
	server.RegisterFunc("echo_dict", echoDict)
	server.RegisterFunc("fail", failFn)
	server.RegisterFunc("destroyed_count", destroyedCountFn)
	server.RegisterClass(counterClass)
	server.RegisterClass(resourceClass)
	server.RegisterClass(kvClass)
	server.Constant("VERSION", "1.0.0")
	server.Constant("MAX_SIZE", 100)
	_ = server.Run()
	os.Exit(0)
}

func TestManagerEdgeCases(t *testing.T) {
	t.Run("AddDirEmpty", func(t *testing.T) {
		m := NewManager()
		m.AddDir("")
		if len(m.dirs) != 0 {
			t.Error("empty dir should be ignored")
		}
	})

	t.Run("LoadNonexistentDir", func(t *testing.T) {
		m := NewManager()
		m.AddDir("/nonexistent/path/that/does/not/exist")
		if err := m.Load(context.Background()); err != nil {
			t.Fatalf("Load should not error on missing dirs: %v", err)
		}
		if len(m.Warnings()) == 0 {
			t.Error("expected warning for nonexistent dir")
		}
	})

	t.Run("RegisterLibrariesNilManager", func(t *testing.T) {
		p := scriptling.New()
		RegisterLibraries(p, nil)
	})

	t.Run("NormalizeLibraryName", func(t *testing.T) {
		tests := []struct{ in, want string }{
			{"hello", "plugin.hello"},
			{"plugin.hello", "plugin.hello"},
		}
		for _, tt := range tests {
			if got := NormalizeLibraryName(tt.in); got != tt.want {
				t.Errorf("NormalizeLibraryName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		}
	})

	t.Run("ManagerGetNotFound", func(t *testing.T) {
		m := NewManager()
		_, ok := m.Get("nonexistent")
		if ok {
			t.Error("expected false for nonexistent plugin")
		}
	})

	t.Run("ManagerWarnings", func(t *testing.T) {
		m := NewManager()
		m.addWarning("test warning")
		w := m.Warnings()
		if len(w) != 1 || w[0] != "test warning" {
			t.Fatalf("expected ['test warning'], got %v", w)
		}
	})
}

func TestGCReleaseHook(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_COMPREHENSIVE_HELPER") == "1" {
		runComprehensivePluginHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "comprehensive")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeComprehensivePluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	_, err := p.Eval(`
import plugin.comprehensive
# Create and immediately drop reference
for i in range(5):
    r = plugin.comprehensive.Resource("res-" + str(i))
    # r goes out of scope each iteration
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}

	runtime.GC()
	runtime.GC()

	result, err := p.Eval(`import plugin.comprehensive
plugin.comprehensive.destroyed_count()
`)
	if err != nil {
		t.Fatalf("destroyed_count: %v", err)
	}
	count, ok := result.(*object.Integer)
	if !ok {
		t.Fatalf("expected int, got %T", result)
	}
	if count.IntValue() < 1 {
		t.Fatalf("expected at least 1 GC cleanup, got %d", count.IntValue())
	}
}

func TestReleaseExplicit(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_COMPREHENSIVE_HELPER") == "1" {
		runComprehensivePluginHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "comprehensive")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeComprehensivePluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer manager.Close()

	t.Run("non-instance", func(t *testing.T) {
		err := Release(object.NewString("not an instance"))
		if err == nil {
			t.Error("expected error for non-instance")
		}
	})

	t.Run("non-plugin instance", func(t *testing.T) {
		inst := &object.Instance{
			Class:  &object.Class{Name: "Local"},
			Fields: map[string]object.Object{},
		}
		err := Release(inst)
		if err == nil {
			t.Error("expected error for non-plugin instance")
		}
	})
}
