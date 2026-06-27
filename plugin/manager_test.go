package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

type capturedLog struct {
	level string
	msg   string
	args  []any
}

type captureLogger struct {
	mu   sync.Mutex
	logs []capturedLog
}

func (l *captureLogger) Trace(msg string, keysAndValues ...any) {
	l.record("trace", msg, keysAndValues...)
}
func (l *captureLogger) Debug(msg string, keysAndValues ...any) {
	l.record("debug", msg, keysAndValues...)
}
func (l *captureLogger) Info(msg string, keysAndValues ...any) {
	l.record("info", msg, keysAndValues...)
}
func (l *captureLogger) Warn(msg string, keysAndValues ...any) {
	l.record("warn", msg, keysAndValues...)
}
func (l *captureLogger) Error(msg string, keysAndValues ...any) {
	l.record("error", msg, keysAndValues...)
}
func (l *captureLogger) Fatal(msg string, keysAndValues ...any) {
	l.record("fatal", msg, keysAndValues...)
}
func (l *captureLogger) With(key string, value any) logger.Logger {
	return &captureLoggerWith{parent: l, prefix: []any{key, value}}
}
func (l *captureLogger) WithError(err error) logger.Logger { return l.With("error", err) }
func (l *captureLogger) WithGroup(group string) logger.Logger {
	return l.With("group", group)
}
func (l *captureLogger) record(level, msg string, keysAndValues ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	args := append([]any(nil), keysAndValues...)
	l.logs = append(l.logs, capturedLog{level: level, msg: msg, args: args})
}
func (l *captureLogger) entries() []capturedLog {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]capturedLog, len(l.logs))
	copy(out, l.logs)
	return out
}

type captureLoggerWith struct {
	parent *captureLogger
	prefix []any
}

func (l *captureLoggerWith) Trace(msg string, keysAndValues ...any) {
	l.record("trace", msg, keysAndValues...)
}
func (l *captureLoggerWith) Debug(msg string, keysAndValues ...any) {
	l.record("debug", msg, keysAndValues...)
}
func (l *captureLoggerWith) Info(msg string, keysAndValues ...any) {
	l.record("info", msg, keysAndValues...)
}
func (l *captureLoggerWith) Warn(msg string, keysAndValues ...any) {
	l.record("warn", msg, keysAndValues...)
}
func (l *captureLoggerWith) Error(msg string, keysAndValues ...any) {
	l.record("error", msg, keysAndValues...)
}
func (l *captureLoggerWith) Fatal(msg string, keysAndValues ...any) {
	l.record("fatal", msg, keysAndValues...)
}
func (l *captureLoggerWith) With(key string, value any) logger.Logger {
	next := append([]any(nil), l.prefix...)
	next = append(next, key, value)
	return &captureLoggerWith{parent: l.parent, prefix: next}
}
func (l *captureLoggerWith) WithError(err error) logger.Logger { return l.With("error", err) }
func (l *captureLoggerWith) WithGroup(group string) logger.Logger {
	return l.With("group", group)
}
func (l *captureLoggerWith) record(level, msg string, keysAndValues ...any) {
	args := append([]any(nil), l.prefix...)
	args = append(args, keysAndValues...)
	l.parent.record(level, msg, args...)
}

func TestManagerLoadsExecutableAndRegistersProxyLibraries(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_TEST_HELPER") == "1" {
		runPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_WRAPPER_HELPER") == "1" {
		runWrapperPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_CRASH_HELPER") == "1" {
		runCrashPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_BAD_PROTOCOL_HELPER") == "1" {
		runBadProtocolPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_PREFIXED_HELPER") == "1" {
		runPrefixedPluginTestHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "hello-plugin")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writePluginHelper(t, helper)

	manager := NewManager(nil)
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

	manager := NewManager(nil)
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

	result, err = p.Eval(`
import plugin.wrap
c = plugin.wrap.Config("Ada")
c.name = "Grace"
c.name + ":" + c.label
`)
	if err != nil {
		t.Fatalf("property wrapper eval returned error: %v", err)
	}
	str, ok = result.(*object.String)
	if !ok || str.StringValue() != "Grace:cfg:Grace" {
		t.Fatalf("unexpected property wrapper result: %#v", result)
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

func writeCrashPluginHelper(t *testing.T, path string) {
	t.Helper()
	writeEnvPluginHelper(t, path, "SCRIPTLING_PLUGIN_CRASH_HELPER")
}

func writeBadProtocolPluginHelper(t *testing.T, path string) {
	t.Helper()
	writeEnvPluginHelper(t, path, "SCRIPTLING_PLUGIN_BAD_PROTOCOL_HELPER")
}

func writePrefixedPluginHelper(t *testing.T, path string) {
	t.Helper()
	writeEnvPluginHelper(t, path, "SCRIPTLING_PLUGIN_PREFIXED_HELPER")
}

func writeEnvPluginHelper(t *testing.T, path string, envName string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset " + envName + "=1\r\n\"" + exe + "\" -test.run=TestManagerLoadsExecutableAndRegistersProxyLibraries --\r\n"
	} else {
		script = "#!/bin/sh\n" + envName + "=1 exec \"" + exe + "\" -test.run=TestManagerLoadsExecutableAndRegistersProxyLibraries --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
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

	type config struct {
		name string
	}
	configBuilder := object.NewClassBuilder("Config").
		Constructor(func(name string) *config {
			return &config{name: name}
		}).
		PropertyWithSetter("name",
			func(self *config) string {
				return self.name
			},
			func(self *config, name string) {
				self.name = name
			},
		).
		Property("label", func(self *config) string {
			return "cfg:" + self.name
		})

	server := NewServer("wrap", "1.0.0", "wrapper test plugin")
	server.RegisterFunc("greet", greetBuilder)
	server.RegisterClass(configBuilder)
	server.Wrapper("greet", `
import scriptling.plugin

def greet(name):
    return scriptling.plugin.call_function("plugin.wrap", "greet", name) + "!"
`)
	_ = server.Run()
	os.Exit(0)
}

func runCrashPluginTestHelper() {
	writeRawHandshakeAndExit("crash", ProtocolVersion, 2)
}

func runBadProtocolPluginTestHelper() {
	writeRawHandshakeAndExit("badproto", "2.0", 0)
}

func runPrefixedPluginTestHelper() {
	writeRawHandshakeAndExit("plugin.hello", ProtocolVersion, 0)
}

func writeRawHandshakeAndExit(name, protocol string, code int) {
	decoder := json.NewDecoder(os.Stdin)
	var req rpcRequest
	if err := decoder.Decode(&req); err != nil {
		os.Exit(2)
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	result := handshakeResult{
		Protocol:  protocol,
		Transport: "json",
		Library: libraryInfo{
			Name:        name,
			Version:     "1.0.0",
			Description: "raw test plugin",
		},
		Capabilities: []string{"remote_objects"},
		Schema:       Schema{},
	}
	raw, err := json.Marshal(result)
	if err != nil {
		os.Exit(2)
	}
	resp.Result = raw
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(code)
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
		manager := NewManager(nil)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		for _, tc := range []struct {
			code  string
			check func(t *testing.T, result object.Object)
		}{
			{`import plugin.comprehensive; plugin.comprehensive.echo_int(42)`, func(t *testing.T, r object.Object) {
				if i, ok := r.(*object.Integer); !ok || i.IntValue() != 42 {
					t.Fatalf("expected 42, got %v", r)
				}
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_float(3.14)`, func(t *testing.T, r object.Object) {
				if f, ok := r.(*object.Float); !ok || f.FloatValue() != 3.14 {
					t.Fatalf("expected 3.14, got %v", r)
				}
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_string("test")`, func(t *testing.T, r object.Object) {
				if s, ok := r.(*object.String); !ok || s.StringValue() != "test" {
					t.Fatalf("expected 'test', got %v", r)
				}
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_bool(True)`, func(t *testing.T, r object.Object) {
				if b, ok := r.(*object.Boolean); !ok || !b.BoolValue() {
					t.Fatalf("expected true, got %v", r)
				}
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_list([1, "two", True])`, func(t *testing.T, r object.Object) {
				if l, ok := r.(*object.List); !ok || len(l.Elements) != 3 {
					t.Fatalf("expected 3 elements, got %v", r)
				}
			}},
			{`import plugin.comprehensive; plugin.comprehensive.echo_dict({"a": 1, "b": "two"})`, func(t *testing.T, r object.Object) {
				if d, ok := r.(*object.Dict); !ok || len(d.Pairs) != 2 {
					t.Fatalf("expected 2 pairs, got %v", r)
				}
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
		manager := NewManager(nil)
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
		manager := NewManager(nil)
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
counter.value = counter.value + 3
counter.get() + counter.value + len(counter.label)
`)
		if err != nil {
			t.Fatalf("Counter: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() != 46 {
			t.Fatalf("Counter: expected 46, got %v", result)
		}

		_, err = p.Eval(`
import plugin.comprehensive
counter = plugin.comprehensive.Counter(1)
counter.label = "nope"
`)
		if err == nil {
			t.Fatal("expected read-only property assignment to fail")
		}
	})

	t.Run("PluginLogger", func(t *testing.T) {
		logs := &captureLogger{}
		manager := NewManager(logs)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`
import plugin.comprehensive
plugin.comprehensive.log_event({"kind": "demo"}, ["a", 2, True])
`)
		if err != nil {
			t.Fatalf("log_event: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "logged" {
			t.Fatalf("log_event result: %#v", result)
		}
		entries := logs.entries()
		if len(entries) != 1 {
			t.Fatalf("expected one log entry, got %#v", entries)
		}
		entry := entries[0]
		if entry.level != "info" || entry.msg != "plugin event" {
			t.Fatalf("unexpected log entry: %#v", entry)
		}
		if len(entry.args) != 8 {
			t.Fatalf("unexpected log args: %#v", entry.args)
		}
		if entry.args[0] != "source" || entry.args[1] != "plugin" || entry.args[2] != "payload" || entry.args[4] != "values" || entry.args[6] != "count" {
			t.Fatalf("unexpected log keys: %#v", entry.args)
		}
		payload, ok := entry.args[3].(map[string]any)
		if !ok || payload["kind"] != "demo" {
			t.Fatalf("unexpected payload arg: %#v", entry.args[3])
		}
		values, ok := entry.args[5].([]any)
		if !ok || len(values) != 3 || values[0] != "a" || values[1] != int64(2) || values[2] != true {
			t.Fatalf("unexpected values arg: %#v", entry.args[5])
		}
		if entry.args[7] != int64(4) {
			t.Fatalf("unexpected count arg: %#v", entry.args[7])
		}
	})

	t.Run("InstanceClass", func(t *testing.T) {
		manager := NewManager(nil)
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
		manager := NewManager(nil)
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
		manager := NewManager(nil)
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
		manager := NewManager(nil)
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

	t.Run("FunctionCallbacks", func(t *testing.T) {
		manager := NewManager(nil)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		result, err := p.Eval(`
import plugin.comprehensive

events = []

def on_event(event):
    events.append(event)
    return "ack"

status = plugin.comprehensive.stream_events(on_event)
events[0] + ":" + events[1][1] + ":" + events[2]["token"] + ":" + str(events[2]["index"]) + ":" + events[3]["nested"]["kind"] + ":" + status
`)
		if err != nil {
			t.Fatalf("callback eval: %v", err)
		}
		str, ok := result.(*object.String)
		if !ok {
			t.Fatalf("expected string result, got %#v", result)
		}
		want := "start:two:done:3:map:complete"
		if str.StringValue() != want {
			t.Fatalf("expected %q, got %q", want, str.StringValue())
		}
	})

	t.Run("CallbacksExpireAfterOuterCall", func(t *testing.T) {
		manager := NewManager(nil)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		_, err := p.Eval(`
import plugin.comprehensive

def on_event(event):
    return "ack"

plugin.comprehensive.save_callback(on_event)
plugin.comprehensive.fire_saved_callback()
`)
		if err == nil {
			t.Fatal("expected expired callback to fail")
		}
		if !strings.Contains(err.Error(), "unknown callback") {
			t.Fatalf("expected unknown callback error, got %v", err)
		}
	})

	t.Run("ParallelCallbacks", func(t *testing.T) {
		manager := NewManager(nil)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		var wg sync.WaitGroup
		var errors atomic.Int64

		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				p := scriptling.New()
				RegisterLibraries(p, manager)

				code := fmt.Sprintf(`
import plugin.comprehensive

events = []

def on_event(value):
    events.append(value)

plugin.comprehensive.delayed_callback(%d, on_event)
events[0]
`, id)
				result, err := p.Eval(code)
				if err != nil {
					t.Logf("parallel callback %d error: %v", id, err)
					errors.Add(1)
					return
				}
				if got, ok := result.(*object.Integer); !ok || got.IntValue() != int64(id) {
					t.Logf("parallel callback %d: expected %d, got %#v", id, id, result)
					errors.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if e := errors.Load(); e > 0 {
			t.Fatalf("%d parallel callback goroutines failed", e)
		}
	})

	t.Run("ParallelSeparateEnvs", func(t *testing.T) {
		manager := NewManager(nil)
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

	t.Run("SharedEnvConcurrentPluginCall", func(t *testing.T) {
		// All goroutines share ONE scriptling instance (one environment, one GIL).
		// Each call acquires the GIL, runs the plugin RPC with the lock released,
		// then re-acquires it — proving plugin calls are safe on a shared
		// environment under the interpreter lock (the shared-state model the GIL
		// enables). Complementary to ParallelSeparateEnvs above.
		manager := NewManager(nil)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		p := scriptling.New()
		RegisterLibraries(p, manager)

		var wg sync.WaitGroup
		var errors atomic.Int64

		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				code := fmt.Sprintf(`import plugin.comprehensive; plugin.comprehensive.echo_int(%d)`, id)
				result, err := p.Eval(code)
				if err != nil {
					t.Logf("shared-env goroutine %d error: %v", id, err)
					errors.Add(1)
					return
				}
				if got, ok := result.(*object.Integer); !ok || got.IntValue() != int64(id) {
					t.Logf("shared-env goroutine %d: expected %d, got %v", id, id, result)
					errors.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if e := errors.Load(); e > 0 {
			t.Fatalf("%d shared-env concurrent plugin calls failed", e)
		}
	})

	t.Run("ParallelObjectCreation", func(t *testing.T) {
		manager := NewManager(nil)
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

	t.Run("OverlappingClientIO", func(t *testing.T) {
		manager := NewManager(nil)
		manager.AddDir(dir)
		if err := manager.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer manager.Close()

		client, ok := manager.Get("plugin.comprehensive")
		if !ok {
			t.Fatal("missing comprehensive plugin")
		}

		const calls = 12
		var wg sync.WaitGroup
		var errors atomic.Int64

		for i := 0; i < calls; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				got, err := client.CallFunction(ctx, "delayed_echo", []Value{{Type: valueInt, Value: int64(id)}}, nil)
				if err != nil {
					t.Logf("call %d error: %v", id, err)
					errors.Add(1)
					return
				}
				if got.Type != valueInt || numberToInt64(got.Value) != int64(id) {
					t.Logf("call %d: expected %d, got %#v", id, id, got)
					errors.Add(1)
				}
			}(i)
		}
		wg.Wait()

		if e := errors.Load(); e > 0 {
			t.Fatalf("%d overlapping client calls failed", e)
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
	var savedCallback Callback

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
		}).
		PropertyWithSetter("value",
			func(self *counter) int {
				return int(self.value)
			},
			func(self *counter, value int) {
				self.value = int64(value)
			},
		).
		Property("label", func(self *counter) string {
			return fmt.Sprintf("counter:%d", self.value)
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

	type fragile struct{}
	fragileClass := object.NewClassBuilder("Fragile").
		Constructor(func(shouldFail bool) (*fragile, error) {
			if shouldFail {
				return nil, fmt.Errorf("construction failed")
			}
			return &fragile{}, nil
		}).
		Method("ok", func(self *fragile) string {
			return "yes"
		})

	strictFn := object.NewFunctionBuilder()
	strictFn.Function(func(s string) string { return "got:" + s })

	type streamEvent struct {
		Token string `json:"token"`
		Index int    `json:"index"`
	}

	streamEventsFn := object.NewFunctionBuilder()
	streamEventsFn.Function(func(ctx context.Context, callback Callback) (string, error) {
		if _, err := callback.Call(ctx, "start"); err != nil {
			return "", err
		}
		if _, err := callback.Call(ctx, []any{"one", "two", 3}); err != nil {
			return "", err
		}
		if _, err := callback.Call(ctx, streamEvent{Token: "done", Index: 3}); err != nil {
			return "", err
		}
		if _, err := callback.Call(ctx, map[string]any{
			"nested": map[string]any{"kind": "map"},
		}); err != nil {
			return "", err
		}
		return "complete", nil
	})

	saveCallbackFn := object.NewFunctionBuilder()
	saveCallbackFn.Function(func(callback Callback) string {
		savedCallback = callback
		return "saved"
	})

	fireSavedCallbackFn := object.NewFunctionBuilder()
	fireSavedCallbackFn.Function(func(ctx context.Context) (string, error) {
		if savedCallback == nil {
			return "", fmt.Errorf("no saved callback")
		}
		if _, err := savedCallback.Call(ctx, "late"); err != nil {
			return "", err
		}
		return "called", nil
	})

	delayedCallbackFn := object.NewFunctionBuilder()
	delayedCallbackFn.Function(func(ctx context.Context, value int, callback Callback) (string, error) {
		time.Sleep(time.Duration(20-value%10) * 5 * time.Millisecond)
		if _, err := callback.Call(ctx, value); err != nil {
			return "", err
		}
		return "ok", nil
	})

	delayedEchoFn := object.NewFunctionBuilder()
	delayedEchoFn.Function(func(value int) int {
		time.Sleep(time.Duration(15-value%10) * 10 * time.Millisecond)
		return value
	})

	logEventFn := object.NewFunctionBuilder()
	logEventFn.Function(func(ctx context.Context, payload map[string]any, values []any) string {
		Logger(ctx).With("source", "plugin").Info("plugin event",
			"payload", payload,
			"values", values,
			"count", len(payload)+len(values),
		)
		return "logged"
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
	server.RegisterFunc("strict", strictFn)
	server.RegisterFunc("stream_events", streamEventsFn)
	server.RegisterFunc("save_callback", saveCallbackFn)
	server.RegisterFunc("fire_saved_callback", fireSavedCallbackFn)
	server.RegisterFunc("delayed_callback", delayedCallbackFn)
	server.RegisterFunc("delayed_echo", delayedEchoFn)
	server.RegisterFunc("log_event", logEventFn)
	server.RegisterClass(counterClass)
	server.RegisterClass(resourceClass)
	server.RegisterClass(kvClass)
	server.RegisterClass(fragileClass)
	server.Constant("VERSION", "1.0.0")
	server.Constant("MAX_SIZE", 100)
	_ = server.Run()
	os.Exit(0)
}

func TestManagerEdgeCases(t *testing.T) {
	t.Run("AddDirEmpty", func(t *testing.T) {
		m := NewManager(nil)
		m.AddDir("")
		if len(m.dirs) != 0 {
			t.Error("empty dir should be ignored")
		}
	})

	t.Run("LoadNonexistentDir", func(t *testing.T) {
		m := NewManager(nil)
		m.AddDir("/nonexistent/path/that/does/not/exist")
		if err := m.Load(context.Background()); err != nil {
			t.Fatalf("Load should not error on missing dirs: %v", err)
		}
		if len(m.Warnings()) == 0 {
			t.Error("expected warning for nonexistent dir")
		}
	})

	t.Run("LoadRejectsProtocolMismatch", func(t *testing.T) {
		dir := t.TempDir()
		helper := filepath.Join(dir, "bad-protocol-plugin")
		if runtime.GOOS == "windows" {
			helper += ".bat"
		}
		writeBadProtocolPluginHelper(t, helper)

		m := NewManager(nil)
		m.AddDir(dir)
		if err := m.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer m.Close()

		if plugins := m.List(); len(plugins) != 0 {
			t.Fatalf("expected no loaded plugins, got %#v", plugins)
		}
		warnings := m.Warnings()
		if len(warnings) != 1 || !strings.Contains(warnings[0], "unsupported protocol") {
			t.Fatalf("expected unsupported protocol warning, got %#v", warnings)
		}
	})

	t.Run("LoadRejectsNormalizedNamespaceCollision", func(t *testing.T) {
		dir := t.TempDir()
		hello := filepath.Join(dir, "a-hello")
		prefixed := filepath.Join(dir, "b-prefixed")
		if runtime.GOOS == "windows" {
			hello += ".bat"
			prefixed += ".bat"
		}
		writePluginHelper(t, hello)
		writePrefixedPluginHelper(t, prefixed)

		m := NewManager(nil)
		m.AddDir(dir)
		if err := m.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer m.Close()

		plugins := m.List()
		if len(plugins) != 1 || plugins[0].Name != "plugin.hello" {
			t.Fatalf("expected one plugin.hello, got %#v", plugins)
		}
		warnings := m.Warnings()
		if len(warnings) != 1 || !strings.Contains(warnings[0], "duplicate library plugin.hello") {
			t.Fatalf("expected duplicate warning, got %#v", warnings)
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
		m := NewManager(nil)
		_, ok := m.Get("nonexistent")
		if ok {
			t.Error("expected false for nonexistent plugin")
		}
	})

	t.Run("ManagerWarnings", func(t *testing.T) {
		m := NewManager(nil)
		m.addWarning("test warning")
		w := m.Warnings()
		if len(w) != 1 || w[0] != "test warning" {
			t.Fatalf("expected ['test warning'], got %v", w)
		}
	})

	t.Run("ManagerHealthReportsExitedPlugin", func(t *testing.T) {
		dir := t.TempDir()
		helper := filepath.Join(dir, "crash-plugin")
		if runtime.GOOS == "windows" {
			helper += ".bat"
		}
		writeCrashPluginHelper(t, helper)

		m := NewManager(nil)
		m.AddDir(dir)
		if err := m.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer m.Close()

		deadline := time.Now().Add(2 * time.Second)
		for {
			health := m.Health()
			if err := health["plugin.crash"]; err != nil {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("expected plugin.crash to become unhealthy, health=%v", health)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("ManagerCrashHandlerReportsExitedPlugin", func(t *testing.T) {
		dir := t.TempDir()
		helper := filepath.Join(dir, "crash-plugin")
		if runtime.GOOS == "windows" {
			helper += ".bat"
		}
		writeCrashPluginHelper(t, helper)

		type crashEvent struct {
			name string
			err  error
		}
		events := make(chan crashEvent, 1)

		m := NewManager(nil, func(name string, err error) {
			events <- crashEvent{name: name, err: err}
		})
		m.AddDir(dir)
		if err := m.Load(context.Background()); err != nil {
			t.Fatalf("Load: %v", err)
		}
		defer m.Close()

		select {
		case event := <-events:
			if event.name != "plugin.crash" {
				t.Fatalf("expected plugin.crash, got %q", event.name)
			}
			if event.err == nil {
				t.Fatal("expected crash error")
			}
		case <-time.After(2 * time.Second):
			t.Fatal("expected crash handler event")
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

	manager := NewManager(nil)
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

	manager := NewManager(nil)
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

func TestProxyErrorPropagation(t *testing.T) {
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

	manager := NewManager(nil)
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	t.Run("constructor fail propagates as error", func(t *testing.T) {
		result, err := p.Eval(`import plugin.comprehensive
plugin.comprehensive.Fragile(true)
`)
		if err == nil {
			t.Fatal("expected error from failed constructor")
		}
		if _, ok := result.(*object.Error); !ok {
			t.Fatalf("expected *object.Error result, got %T: %v", result, result)
		}
	})

	t.Run("constructor fail prevents method call", func(t *testing.T) {
		_, err := p.Eval(`import plugin.comprehensive
f = plugin.comprehensive.Fragile(true)
f.ok()
`)
		if err == nil {
			t.Fatal("expected error — constructor fail should stop execution")
		}
	})

	t.Run("constructor success allows method call", func(t *testing.T) {
		result, err := p.Eval(`import plugin.comprehensive
f = plugin.comprehensive.Fragile(0)
f.ok()
`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		s, ok := result.(*object.String)
		if !ok || s.StringValue() != "yes" {
			t.Fatalf("expected 'yes', got %v", result)
		}
	})

	t.Run("wrong arg types to constructor propagates as error", func(t *testing.T) {
		result, err := p.Eval(`import plugin.comprehensive
plugin.comprehensive.Fragile("not a bool")
`)
		if err == nil {
			t.Fatal("expected error from wrong argument type to constructor")
		}
		if _, ok := result.(*object.Error); !ok {
			t.Fatalf("expected *object.Error result, got %T: %v", result, result)
		}
	})

	t.Run("method call on released object", func(t *testing.T) {
		result, err := p.Eval(`import plugin.comprehensive
r = plugin.comprehensive.Resource("test")
scriptling.plugin.release(r)
r.name()
`)
		if err == nil {
			t.Fatal("expected error from method call on released object")
		}
		if _, ok := result.(*object.Error); !ok {
			t.Fatalf("expected *object.Error result, got %T: %v", result, result)
		}
	})

	t.Run("call_function unknown propagates as error", func(t *testing.T) {
		result, err := p.Eval(`import plugin.comprehensive
scriptling.plugin.call_function("plugin.comprehensive", "nonexistent")
`)
		if err == nil {
			t.Fatal("expected error from unknown function")
		}
		if _, ok := result.(*object.Error); !ok {
			t.Fatalf("expected *object.Error result, got %T: %v", result, result)
		}
	})
}

// ============================================================================
// Scope Tests
// ============================================================================

// TestScopeGetChainsToParent verifies that Get on a scope falls back to the
// parent manager when a name is not found locally.
func TestScopeGetChainsToParent(t *testing.T) {
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

	parent := NewManager(nil)
	parent.AddDir(dir)
	if err := parent.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer parent.Close()

	scope := parent.NewScope()
	defer scope.Close()

	// scope has no local plugins, but should see the parent's plugin via Get.
	client, ok := scope.Get("plugin.comprehensive")
	if !ok {
		t.Fatal("expected Get to chain to parent")
	}
	if client.Metadata().Name != "plugin.comprehensive" {
		t.Fatalf("unexpected name: %s", client.Metadata().Name)
	}
}

// TestScopeListMergesParentLocalWins verifies that List returns both parent and
// local plugins, and that a local plugin with the same name shadows the parent.
func TestScopeListMergesParentLocalWins(t *testing.T) {
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

	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })
	httpSrv := newPluginHTTPServer(t, "comprehensive", echoFn)
	defer httpSrv.Close()

	parent := NewManager(nil)
	parent.AddDir(dir)
	if err := parent.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer parent.Close()

	// Confirm parent has exactly one plugin.
	if n := len(parent.List()); n != 1 {
		t.Fatalf("expected 1 parent plugin, got %d", n)
	}

	scope := parent.NewScope()
	defer scope.Close()

	// Load a plugin that has a DIFFERENT name in the scope — both must appear.
	ctx := context.Background()
	if _, err := scope.LoadURL(ctx, "extra", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}

	list := scope.List()
	names := make(map[string]bool)
	for _, m := range list {
		names[m.Name] = true
	}
	if !names["plugin.comprehensive"] {
		t.Error("expected parent plugin in scope List")
	}
	if !names["plugin.extra"] {
		t.Error("expected local scope plugin in scope List")
	}

	// Attempting to load the SAME name as the parent (different URL) is blocked.
	scope2 := parent.NewScope()
	defer scope2.Close()
	if _, err := scope2.LoadURL(ctx, "comprehensive", httpSrv.URL, true, false); err == nil {
		t.Fatal("expected error: child must not shadow parent's plugin.comprehensive")
	}
}

// TestScopeCloseReleasesLocalOnly verifies that closing a scope terminates its
// locally loaded processes/connections without touching the parent's plugins.
func TestScopeCloseReleasesLocalOnly(t *testing.T) {
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

	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })
	httpSrv := newPluginHTTPServer(t, "scoped", echoFn)
	defer httpSrv.Close()

	parent := NewManager(nil)
	parent.AddDir(dir)
	if err := parent.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer parent.Close()

	scope := parent.NewScope()
	ctx := context.Background()
	if _, err := scope.LoadURL(ctx, "scoped", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}

	// Both visible before close.
	if _, ok := scope.Get("plugin.scoped"); !ok {
		t.Fatal("scope plugin not found before close")
	}
	if _, ok := scope.Get("plugin.comprehensive"); !ok {
		t.Fatal("parent plugin not found via scope before close")
	}

	// Close the scope.
	if err := scope.Close(); err != nil {
		t.Fatalf("scope.Close: %v", err)
	}

	// Parent plugin is unaffected — still callable.
	parentClient, ok := parent.Get("plugin.comprehensive")
	if !ok {
		t.Fatal("parent plugin disappeared after scope.Close")
	}
	result, err := parentClient.CallFunction(ctx, "echo_string",
		[]Value{{Type: valueString, Value: "still-alive"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction after scope close: %v", err)
	}
	if result.Type != valueString || result.Value != "still-alive" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// TestScopeTransportHTTPOnly verifies that a scope with TransportHTTP refuses
// to load executable (stdio) plugins.
func TestScopeTransportHTTPOnly(t *testing.T) {
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

	parent := NewManager(nil)
	defer parent.Close()

	scope := parent.NewScope(WithTransport(TransportHTTP))
	defer scope.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := scope.LoadPath(ctx, "exe", helper, true, nil)
	if err == nil {
		t.Fatal("expected error loading executable in HTTP-only scope")
	}
	if !strings.Contains(err.Error(), "stdio/executable plugins are not permitted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestScopeTransportStdioOnly verifies that a scope with TransportStdio refuses
// to load HTTP(S) plugins.
func TestScopeTransportStdioOnly(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })
	httpSrv := newPluginHTTPServer(t, "remote", echoFn)
	defer httpSrv.Close()

	parent := NewManager(nil)
	defer parent.Close()

	scope := parent.NewScope(WithTransport(TransportStdio))
	defer scope.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := scope.LoadURL(ctx, "remote", httpSrv.URL, true, false)
	if err == nil {
		t.Fatal("expected error loading HTTP URL in stdio-only scope")
	}
	if !strings.Contains(err.Error(), "http/https plugins are not permitted") {
		t.Fatalf("unexpected error: %v", err)
	}

	// LoadPath with an HTTP URL should also be rejected.
	_, err = scope.LoadPath(ctx, "remote2", httpSrv.URL, true, nil)
	if err == nil {
		t.Fatal("expected error loading HTTP URL via LoadPath in stdio-only scope")
	}
	if !strings.Contains(err.Error(), "http/https plugins are not permitted") {
		t.Fatalf("unexpected LoadPath error: %v", err)
	}
}

// TestScopeTransportAll verifies that a scope with the default TransportAll
// mode accepts both stdio and HTTP plugins.
func TestScopeTransportAll(t *testing.T) {
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

	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })
	httpSrv := newPluginHTTPServer(t, "remote", echoFn)
	defer httpSrv.Close()

	parent := NewManager(nil)
	defer parent.Close()

	// Default scope (TransportAll) accepts both.
	scope := parent.NewScope()
	defer scope.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := scope.LoadPath(ctx, "exe", helper, true, nil); err != nil {
		t.Fatalf("LoadPath in TransportAll scope: %v", err)
	}
	if _, err := scope.LoadURL(ctx, "remote", httpSrv.URL, true, false); err != nil {
		t.Fatalf("LoadURL in TransportAll scope: %v", err)
	}
	if n := len(scope.List()); n != 2 {
		t.Fatalf("expected 2 local plugins, got %d", n)
	}
}

// TestScopeParallelIsolation verifies that two concurrent scopes loading the
// same plugin name are fully independent and don't interfere with each other.
func TestScopeParallelIsolation(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v string) string { return v })
	httpSrv := newPluginHTTPServer(t, "echo", echoFn)
	defer httpSrv.Close()

	parent := NewManager(nil)
	defer parent.Close()

	var wg sync.WaitGroup
	var errors atomic.Int64

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			scope := parent.NewScope(WithTransport(TransportHTTP))
			defer scope.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Each scope loads the same URL under the same name — completely isolated.
			client, err := scope.LoadURL(ctx, "echo", httpSrv.URL, true, false)
			if err != nil {
				t.Logf("scope %d LoadURL: %v", id, err)
				errors.Add(1)
				return
			}

			payload := fmt.Sprintf("hello-%d", id)
			result, err := client.CallFunction(ctx, "echo",
				[]Value{{Type: valueString, Value: payload}}, nil)
			if err != nil {
				t.Logf("scope %d CallFunction: %v", id, err)
				errors.Add(1)
				return
			}
			if result.Type != valueString || result.Value != payload {
				t.Logf("scope %d: expected %q, got %#v", id, payload, result)
				errors.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if e := errors.Load(); e > 0 {
		t.Fatalf("%d parallel scope goroutines failed", e)
	}
}

// newPluginHTTPServer starts a minimal scriptling plugin HTTP server and returns
// its httptest.Server. The server exports the provided function builder under
// the given name, responding to the scriptling plugin handshake.
func newPluginHTTPServer(t *testing.T, name string, fn *object.FunctionBuilder) *httptest.Server {
	t.Helper()
	server := NewServer(name, "1.0.0", name+" test server")
	server.RegisterFunc("echo", fn)
	return httptest.NewServer(server)
}

// TestScopeInsecureTransportIsShared verifies that a Manager and its scopes
// share the same insecure transport instance, meaning connections made with
// insecure_skip_tls are pooled across the scope hierarchy.
func TestScopeInsecureTransportIsShared(t *testing.T) {
	parent := NewManager(nil)
	defer parent.Close()

	scope1 := parent.NewScope(WithTransport(TransportHTTP))
	defer scope1.Close()

	scope2 := parent.NewScope(WithTransport(TransportHTTP))
	defer scope2.Close()

	// Both scopes should hold the exact same insecure transport pointer as parent.
	if parent.httpInsecureTransport == nil {
		t.Fatal("parent insecure transport is nil")
	}
	if scope1.httpInsecureTransport != parent.httpInsecureTransport {
		t.Error("scope1 insecure transport is not shared with parent")
	}
	if scope2.httpInsecureTransport != parent.httpInsecureTransport {
		t.Error("scope2 insecure transport is not shared with parent")
	}
	// Same for the verified transport.
	if scope1.httpTransport != parent.httpTransport {
		t.Error("scope1 secure transport is not shared with parent")
	}
}

// TestManagerLoadURLInsecureSkipTLS verifies that LoadURL with insecureSkipTLS=true
// succeeds against a TLS server with a self-signed certificate.
func TestManagerLoadURLInsecureSkipTLS(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })
	server := NewServer("tlsecho", "1.0.0", "tls echo").RegisterFunc("echo", echoFn)

	tlsSrv := httptest.NewTLSServer(server)
	defer tlsSrv.Close()

	manager := NewManager(nil)
	defer manager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Secure (default) should fail — self-signed cert.
	_, err := manager.LoadURL(ctx, "tlsfail", tlsSrv.URL, true, false)
	if err == nil {
		t.Fatal("expected TLS error without insecure_skip_tls")
	}

	// Insecure should succeed.
	client, err := manager.LoadURL(ctx, "tlsok", tlsSrv.URL, true, true)
	if err != nil {
		t.Fatalf("LoadURL insecure: %v", err)
	}

	result, err := client.CallFunction(ctx, "echo",
		[]Value{{Type: valueString, Value: "skip-verify"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueString || result.Value != "skip-verify" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// TestScopeLoadURLInsecureSkipTLS verifies that a scope inherits the insecure
// transport and can load TLS-skip-verify plugins.
func TestScopeLoadURLInsecureSkipTLS(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })
	server := NewServer("scopetls", "1.0.0", "scope tls echo").RegisterFunc("echo", echoFn)

	tlsSrv := httptest.NewTLSServer(server)
	defer tlsSrv.Close()

	parent := NewManager(nil)
	defer parent.Close()

	scope := parent.NewScope(WithTransport(TransportHTTP))
	defer scope.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := scope.LoadURL(ctx, "scopetls", tlsSrv.URL, true, true)
	if err != nil {
		t.Fatalf("scope LoadURL insecure: %v", err)
	}

	result, err := client.CallFunction(ctx, "echo",
		[]Value{{Type: valueString, Value: "scope-ok"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueString || result.Value != "scope-ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// ============================================================================
// Deep-stack (multi-level scope) tests
// ============================================================================

// TestDeepStackGetChains verifies that Get traverses an arbitrarily deep chain:
// global → scope1 → scope2. Each level should see all plugins above it.
// Since shadowing is blocked, a child scope attempting to load a parent-owned
// name gets an error; attempting with the same URL is idempotent.
func TestDeepStackGetChains(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })

	srv0 := newPluginHTTPServer(t, "global", echoFn)
	defer srv0.Close()
	srv1 := newPluginHTTPServer(t, "scope1", echoFn)
	defer srv1.Close()
	srv2 := newPluginHTTPServer(t, "scope2", echoFn)
	defer srv2.Close()

	ctx := context.Background()

	global := NewManager(nil)
	defer global.Close()
	if _, err := global.LoadURL(ctx, "global", srv0.URL, true, false); err != nil {
		t.Fatalf("global LoadURL: %v", err)
	}

	scope1 := global.NewScope()
	defer scope1.Close()
	if _, err := scope1.LoadURL(ctx, "scope1", srv1.URL, true, false); err != nil {
		t.Fatalf("scope1 LoadURL: %v", err)
	}

	scope2 := scope1.NewScope()
	defer scope2.Close()
	if _, err := scope2.LoadURL(ctx, "scope2", srv2.URL, true, false); err != nil {
		t.Fatalf("scope2 LoadURL: %v", err)
	}

	// scope2 can see all three levels via chain.
	for _, name := range []string{"plugin.global", "plugin.scope1", "plugin.scope2"} {
		if _, ok := scope2.Get(name); !ok {
			t.Errorf("scope2.Get(%q) returned false, want true", name)
		}
	}
	// scope1 sees global + scope1, but not scope2.
	if _, ok := scope1.Get("plugin.scope2"); ok {
		t.Error("scope1.Get(plugin.scope2) returned true — child must not be visible to parent")
	}
	// global sees only itself.
	if _, ok := global.Get("plugin.scope1"); ok {
		t.Error("global.Get(plugin.scope1) returned true — child must not be visible")
	}

	// Attempting to shadow a parent name from scope1 is blocked.
	srvOther := newPluginHTTPServer(t, "other", echoFn)
	defer srvOther.Close()
	_, err := scope1.LoadURL(ctx, "global", srvOther.URL, true, false)
	if err == nil {
		t.Fatal("expected error when scope1 tries to shadow parent plugin.global")
	}
	if !strings.Contains(err.Error(), "already loaded in a parent scope") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Loading the SAME URL under the same name is idempotent — returns parent's client.
	client, err := scope1.LoadURL(ctx, "global", srv0.URL, true, false)
	if err != nil {
		t.Fatalf("idempotent LoadURL for parent plugin: %v", err)
	}
	globalClient, _ := global.Get("plugin.global")
	if client != globalClient {
		t.Error("idempotent load should return the parent's client")
	}
}

// TestDeepStackListMerges verifies that List at each level returns the correct
// merged view of all plugins visible in that scope's chain, with no duplicates.
// Since shadowing is now blocked, each level only adds NEW plugin names.
func TestDeepStackListMerges(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })

	srvA := newPluginHTTPServer(t, "a", echoFn)
	defer srvA.Close()
	srvB := newPluginHTTPServer(t, "b", echoFn)
	defer srvB.Close()
	srvC := newPluginHTTPServer(t, "c", echoFn)
	defer srvC.Close()
	srvD := newPluginHTTPServer(t, "d", echoFn)
	defer srvD.Close()

	ctx := context.Background()

	global := NewManager(nil)
	defer global.Close()
	if _, err := global.LoadURL(ctx, "a", srvA.URL, true, false); err != nil {
		t.Fatalf("global a: %v", err)
	}
	if _, err := global.LoadURL(ctx, "b", srvB.URL, true, false); err != nil {
		t.Fatalf("global b: %v", err)
	}

	scope1 := global.NewScope()
	defer scope1.Close()
	if _, err := scope1.LoadURL(ctx, "c", srvC.URL, true, false); err != nil {
		t.Fatalf("scope1 c: %v", err)
	}
	// Attempting to load a parent-owned name is blocked.
	srvOther := newPluginHTTPServer(t, "b", echoFn)
	defer srvOther.Close()
	if _, err := scope1.LoadURL(ctx, "b", srvOther.URL, true, false); err == nil {
		t.Fatal("expected error when scope1 tries to shadow parent plugin.b")
	}

	scope2 := scope1.NewScope()
	defer scope2.Close()
	if _, err := scope2.LoadURL(ctx, "d", srvD.URL, true, false); err != nil {
		t.Fatalf("scope2 d: %v", err)
	}

	// scope2 sees a, b (from global), c (from scope1), d (local) — 4 total.
	list2 := scope2.List()
	if len(list2) != 4 {
		names := make([]string, len(list2))
		for i, m := range list2 {
			names[i] = m.Name
		}
		t.Fatalf("scope2 list: expected 4 entries, got %d: %v", len(list2), names)
	}
	// "b" is global's — verify client identity.
	bClient, ok := scope2.Get("plugin.b")
	if !ok {
		t.Fatal("scope2.Get(plugin.b) returned false")
	}
	if bClient.Path() != srvB.URL {
		t.Errorf("expected global's plugin.b, got path %q", bClient.Path())
	}

	// scope1 sees a, b (from global), c (local) — 3 entries.
	list1 := scope1.List()
	if len(list1) != 3 {
		t.Fatalf("scope1 list: expected 3 entries, got %d", len(list1))
	}

	// global sees a, b — 2 entries.
	listG := global.List()
	if len(listG) != 2 {
		t.Fatalf("global list: expected 2 entries, got %d", len(listG))
	}
}

// TestDeepStackTransportInherited verifies that a scope-of-a-scope shares the
// same transport instances as the root manager.
func TestDeepStackTransportInherited(t *testing.T) {
	root := NewManager(nil)
	defer root.Close()

	scope1 := root.NewScope()
	defer scope1.Close()

	scope2 := scope1.NewScope()
	defer scope2.Close()

	scope3 := scope2.NewScope()
	defer scope3.Close()

	if scope1.httpTransport != root.httpTransport {
		t.Error("scope1 secure transport not shared with root")
	}
	if scope2.httpTransport != root.httpTransport {
		t.Error("scope2 secure transport not shared with root")
	}
	if scope3.httpTransport != root.httpTransport {
		t.Error("scope3 secure transport not shared with root")
	}
	if scope1.httpInsecureTransport != root.httpInsecureTransport {
		t.Error("scope1 insecure transport not shared with root")
	}
	if scope2.httpInsecureTransport != root.httpInsecureTransport {
		t.Error("scope2 insecure transport not shared with root")
	}
	if scope3.httpInsecureTransport != root.httpInsecureTransport {
		t.Error("scope3 insecure transport not shared with root")
	}
}

// TestDeepStackCloseDoesNotAffectSiblings verifies that closing one scope at
// depth does not affect sibling scopes at the same level.
func TestDeepStackCloseDoesNotAffectSiblings(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })

	srv := newPluginHTTPServer(t, "echo", echoFn)
	defer srv.Close()

	ctx := context.Background()

	root := NewManager(nil)
	defer root.Close()

	scopeA := root.NewScope()
	scopeB := root.NewScope()
	defer scopeB.Close()

	if _, err := scopeA.LoadURL(ctx, "a", srv.URL, true, false); err != nil {
		t.Fatalf("scopeA LoadURL: %v", err)
	}
	if _, err := scopeB.LoadURL(ctx, "b", srv.URL, true, false); err != nil {
		t.Fatalf("scopeB LoadURL: %v", err)
	}

	// Close scopeA — scopeB should be unaffected.
	if err := scopeA.Close(); err != nil {
		t.Fatalf("scopeA.Close: %v", err)
	}

	// scopeB still sees its own plugin and root is unaffected.
	if _, ok := scopeB.Get("plugin.b"); !ok {
		t.Error("scopeB.Get(plugin.b) returned false after sibling close")
	}
	// scopeA's plugin is gone from scopeA.
	if _, ok := scopeA.Get("plugin.a"); ok {
		t.Error("scopeA.Get(plugin.a) returned true after close, expected false for local")
	}

	// scopeB can still call its plugin.
	bClient, _ := scopeB.Get("plugin.b")
	ctxT, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := bClient.CallFunction(ctxT, "echo",
		[]Value{{Type: valueString, Value: "sibling-ok"}}, nil)
	if err != nil {
		t.Fatalf("bClient.CallFunction after sibling close: %v", err)
	}
	if result.Type != valueString || result.Value != "sibling-ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// ============================================================================
// Proxy library registration with stacked managers
// ============================================================================

// TestScopeProxyLibrariesBothVisible verifies the two-lib case: parent has
// plugin.base, child scope has plugin.extra. RegisterLibraries on the scope
// must register proxies for BOTH so scripts can import and call either.
func TestScopeProxyLibrariesBothVisible(t *testing.T) {
	// plugin.base lives in the parent — registered before the scope exists.
	baseFn := object.NewFunctionBuilder()
	baseFn.Function(func(name string) string { return "hello-" + name })
	baseSrv := httptest.NewServer(
		NewServer("base", "1.0.0", "base test").RegisterFunc("greet", baseFn),
	)
	defer baseSrv.Close()

	// plugin.extra lives only in the child scope.
	extraFn := object.NewFunctionBuilder()
	extraFn.Function(func(name string) string { return "extra-" + name })
	extraSrv := httptest.NewServer(
		NewServer("extra", "1.0.0", "extra test").RegisterFunc("shout", extraFn),
	)
	defer extraSrv.Close()

	ctx := context.Background()

	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(ctx, "base", baseSrv.URL, true, false); err != nil {
		t.Fatalf("parent LoadURL: %v", err)
	}

	scope := parent.NewScope()
	defer scope.Close()
	if _, err := scope.LoadURL(ctx, "extra", extraSrv.URL, true, false); err != nil {
		t.Fatalf("scope LoadURL: %v", err)
	}

	// RegisterLibraries walks scope.List() which returns both plugin.base
	// (via parent chain) and plugin.extra (local). Both proxies must be
	// registered so the script can import and call either.
	p := scriptling.New()
	RegisterLibraries(p, scope)

	result, err := p.Eval(`
import plugin.base
import plugin.extra
plugin.base.greet("Ada") + ":" + plugin.extra.shout("World")
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	s, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected string, got %T: %v", result, result)
	}
	if s.StringValue() != "hello-Ada:extra-World" {
		t.Fatalf("expected hello-Ada:extra-World, got %q", s.StringValue())
	}
}

// TestScopeChildBlockedFromShadowingParentPlugin verifies that a child scope
// cannot load a plugin under a name that already exists in the parent chain.
// Loading the SAME endpoint under the same name is idempotent (allowed).
func TestScopeChildBlockedFromShadowingParentPlugin(t *testing.T) {
	parentFn := object.NewFunctionBuilder()
	parentFn.Function(func() string { return "from-parent" })
	parentSrv := httptest.NewServer(
		NewServer("shared", "1.0.0", "parent shared").RegisterFunc("which", parentFn),
	)
	defer parentSrv.Close()

	otherFn := object.NewFunctionBuilder()
	otherFn.Function(func() string { return "from-other" })
	otherSrv := httptest.NewServer(
		NewServer("shared", "1.0.0", "other shared").RegisterFunc("which", otherFn),
	)
	defer otherSrv.Close()

	ctx := context.Background()

	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(ctx, "shared", parentSrv.URL, true, false); err != nil {
		t.Fatalf("parent LoadURL: %v", err)
	}

	scope := parent.NewScope()
	defer scope.Close()

	// Different endpoint under same name → blocked.
	_, err := scope.LoadURL(ctx, "shared", otherSrv.URL, true, false)
	if err == nil {
		t.Fatal("expected error when child tries to shadow parent plugin with different endpoint")
	}
	if !strings.Contains(err.Error(), "already loaded in a parent scope") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Same endpoint under same name → idempotent, returns parent's client.
	client, err := scope.LoadURL(ctx, "shared", parentSrv.URL, true, false)
	if err != nil {
		t.Fatalf("idempotent load: %v", err)
	}
	parentClient, _ := parent.Get("plugin.shared")
	if client != parentClient {
		t.Error("idempotent load should return the parent's client")
	}

	// Script using parent's proxy still works correctly.
	p := scriptling.New()
	RegisterLibraries(p, scope)
	result, err := p.Eval(`import plugin.shared; plugin.shared.which()`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "from-parent" {
		t.Fatalf("expected from-parent, got %#v", result)
	}
}

// TestScopeSiblingsDontAffectEachOther confirms that two sibling scopes are
// fully independent: each sees the parent's plugins plus only its own local
// plugins, and neither can shadow the parent or each other.
func TestScopeSiblingsDontAffectEachOther(t *testing.T) {
	parentFn := object.NewFunctionBuilder()
	parentFn.Function(func() string { return "from-parent" })
	parentSrv := httptest.NewServer(
		NewServer("shared", "1.0.0", "parent shared").RegisterFunc("which", parentFn),
	)
	defer parentSrv.Close()

	aFn := object.NewFunctionBuilder()
	aFn.Function(func() string { return "scope-a" })
	aSrv := httptest.NewServer(
		NewServer("scopea", "1.0.0", "scope A plugin").RegisterFunc("id", aFn),
	)
	defer aSrv.Close()

	bFn := object.NewFunctionBuilder()
	bFn.Function(func() string { return "scope-b" })
	bSrv := httptest.NewServer(
		NewServer("scopeb", "1.0.0", "scope B plugin").RegisterFunc("id", bFn),
	)
	defer bSrv.Close()

	ctx := context.Background()

	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(ctx, "shared", parentSrv.URL, true, false); err != nil {
		t.Fatalf("parent LoadURL: %v", err)
	}

	// scopeA loads its own unique plugin.
	scopeA := parent.NewScope()
	defer scopeA.Close()
	if _, err := scopeA.LoadURL(ctx, "scopea", aSrv.URL, true, false); err != nil {
		t.Fatalf("scopeA LoadURL: %v", err)
	}

	// scopeB loads its own unique plugin.
	scopeB := parent.NewScope()
	defer scopeB.Close()
	if _, err := scopeB.LoadURL(ctx, "scopeb", bSrv.URL, true, false); err != nil {
		t.Fatalf("scopeB LoadURL: %v", err)
	}

	pA := scriptling.New()
	RegisterLibraries(pA, scopeA)
	pB := scriptling.New()
	RegisterLibraries(pB, scopeB)

	// Both see the parent's plugin.shared with the same (parent's) implementation.
	resultA, err := pA.Eval(`import plugin.shared; plugin.shared.which()`)
	if err != nil {
		t.Fatalf("pA shared: %v", err)
	}
	resultB, err := pB.Eval(`import plugin.shared; plugin.shared.which()`)
	if err != nil {
		t.Fatalf("pB shared: %v", err)
	}
	if s, ok := resultA.(*object.String); !ok || s.StringValue() != "from-parent" {
		t.Fatalf("scopeA: expected from-parent, got %#v", resultA)
	}
	if s, ok := resultB.(*object.String); !ok || s.StringValue() != "from-parent" {
		t.Fatalf("scopeB: expected from-parent, got %#v", resultB)
	}

	// scopeA cannot see scopeB's plugin and vice-versa.
	if _, ok := scopeA.Get("plugin.scopeb"); ok {
		t.Error("scopeA.Get(plugin.scopeb) should return false — siblings are not visible to each other")
	}
	if _, ok := scopeB.Get("plugin.scopea"); ok {
		t.Error("scopeB.Get(plugin.scopea) should return false — siblings are not visible to each other")
	}

	// Neither scope can shadow the parent's plugin.
	otherFn := object.NewFunctionBuilder()
	otherFn.Function(func() string { return "impostor" })
	otherSrv := httptest.NewServer(
		NewServer("shared", "1.0.0", "impostor").RegisterFunc("which", otherFn),
	)
	defer otherSrv.Close()
	if _, err := scopeA.LoadURL(ctx, "shared", otherSrv.URL, true, false); err == nil {
		t.Fatal("expected scopeA to be blocked from shadowing parent's plugin.shared")
	}
	if _, err := scopeB.LoadURL(ctx, "shared", otherSrv.URL, true, false); err == nil {
		t.Fatal("expected scopeB to be blocked from shadowing parent's plugin.shared")
	}
}

// ============================================================================
// Remote object lifecycle with dynamic shadowing

// TestScopeExistingInstanceSurvivesDynamicShadow covers the critical edge case
// where a parent scope has plugin.testing already loaded and a child scope
// tries to load the same name.
//
// Rules verified:
//  1. Child loading a DIFFERENT endpoint under an already-taken parent name is BLOCKED.
//  2. Child loading the SAME endpoint under the same name is idempotent (returns parent's client).
//  3. Existing remote objects created from the parent's plugin continue to work
//     regardless of child scope activity (remote.Client is immutable on the object).
func TestScopeExistingInstanceSurvivesDynamicShadow(t *testing.T) {
	aFn := object.NewFunctionBuilder()
	aFn.Function(func() string { return "from-A" })
	srvA := httptest.NewServer(
		NewServer("testing", "1.0.0", "A").
			RegisterFunc("which", aFn).
			RegisterClass(object.NewClassBuilder("Widget").
				Constructor(func(id string) *struct{ id string } { return &struct{ id string }{id} }).
				Method("id", func(self *struct{ id string }) string { return self.id }),
			),
	)
	defer srvA.Close()

	bFn := object.NewFunctionBuilder()
	bFn.Function(func() string { return "from-B" })
	srvB := httptest.NewServer(
		NewServer("testing", "1.0.0", "B").RegisterFunc("which", bFn),
	)
	defer srvB.Close()

	ctx := context.Background()
	parent := NewManager(nil)
	defer parent.Close()
	if _, err := parent.LoadURL(ctx, "testing", srvA.URL, true, false); err != nil {
		t.Fatalf("parent LoadURL A: %v", err)
	}

	scope := parent.NewScope()
	defer scope.Close()
	p := scriptling.New()
	RegisterLibraries(p, scope)

	// Phase 1: create an object from the parent's plugin.
	result, err := p.Eval(`
import plugin.testing
obj = plugin.testing.Widget("original")
obj.id()
`)
	if err != nil {
		t.Fatalf("phase 1: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "original" {
		t.Fatalf("phase 1: expected original, got %#v", result)
	}

	// Phase 2: child tries to load a DIFFERENT endpoint under the same name — must be blocked.
	_, err = p.Eval(fmt.Sprintf(`
import scriptling.plugin
scriptling.plugin.load("testing", %q, scriptling=True)
`, srvB.URL))
	if err == nil {
		t.Fatal("expected error when child shadows parent plugin name with different endpoint; got nil")
	}
	if !strings.Contains(err.Error(), "already loaded in a parent scope") {
		t.Fatalf("unexpected error: %v", err)
	}

	// Phase 3: loading the SAME endpoint under the same name is idempotent — returns parent's client.
	result, err = p.Eval(fmt.Sprintf(`
import scriptling.plugin
scriptling.plugin.load("testing", %q, scriptling=True)
`, srvA.URL))
	if err != nil {
		t.Fatalf("phase 3 idempotent load: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "plugin.testing" {
		t.Fatalf("phase 3: expected plugin.testing, got %#v", result)
	}

	// Phase 4: the original object still works (remote.Client stored directly — immutable).
	result, err = p.Eval(`obj.id()`)
	if err != nil {
		t.Fatalf("phase 4 obj.id(): %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "original" {
		t.Fatalf("phase 4: expected original (server A), got %#v", result)
	}

	// Phase 5: Go-level block also applies — scope.LoadURL with different endpoint errors.
	_, err = scope.LoadURL(ctx, "testing", srvB.URL, true, false)
	if err == nil {
		t.Fatal("expected LoadURL to block child from shadowing parent plugin")
	}
	if !strings.Contains(err.Error(), "already loaded in a parent scope") {
		t.Fatalf("unexpected LoadURL error: %v", err)
	}

	// Phase 6: Go-level idempotent via parent — same URL returns parent's client.
	client, err := scope.LoadURL(ctx, "testing", srvA.URL, true, false)
	if err != nil {
		t.Fatalf("phase 6 idempotent LoadURL: %v", err)
	}
	parentClient, _ := parent.Get("plugin.testing")
	if client != parentClient {
		t.Fatal("phase 6: expected idempotent load to return parent's client instance")
	}
}
