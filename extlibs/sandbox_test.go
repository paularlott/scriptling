package extlibs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// setupSandboxTest creates a Scriptling instance with runtime (including sandbox)
// and sets the sandbox factory.
func setupSandboxTest(t *testing.T) *scriptling.Scriptling {
	t.Helper()
	ResetRuntime()

	// Clear any previous sandbox state
	SetSandboxFactory(nil)
	SetSandboxAllowedPaths(nil)

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p)

	// Set up factory that creates instances with stdlib + runtime
	SetSandboxFactory(func() SandboxInstance {
		newP := scriptling.New()
		stdlib.RegisterAll(newP)
		RegisterRuntimeLibraryAll(newP)
		return newP
	})

	return p
}

func TestSandboxCreate(t *testing.T) {
	p := setupSandboxTest(t)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("test", True)
env.get("test")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	if b, _ := result.AsBool(); !b {
		t.Error("Expected True from sandbox get")
	}
}

func TestSandboxSetGet(t *testing.T) {
	p := setupSandboxTest(t)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("x", 42)
env.set("name", "hello")
env.set("data", {"key": "value"})

results = [
    env.get("x"),
    env.get("name"),
    env.get("data"),
    env.get("missing"),
]
results
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %T", result)
	}

	if len(list.Elements) != 4 {
		t.Fatalf("Expected 4 elements, got %d", len(list.Elements))
	}

	// x == 42
	if i, _ := list.Elements[0].AsInt(); i != 42 {
		t.Errorf("Expected x=42, got %d", i)
	}

	// name == "hello"
	if s, _ := list.Elements[1].AsString(); s != "hello" {
		t.Errorf("Expected name='hello', got '%s'", s)
	}

	// data is a dict
	if _, ok := list.Elements[2].(*object.Dict); !ok {
		t.Errorf("Expected dict for data, got %T", list.Elements[2])
	}

	// missing returns null
	if _, ok := list.Elements[3].(*object.Null); !ok {
		t.Errorf("Expected Null for missing, got %T", list.Elements[3])
	}
}

func TestSandboxExec(t *testing.T) {
	p := setupSandboxTest(t)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("a", 10)
env.set("b", 20)
env.exec("result = a + b")
env.get("result")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 30 {
		t.Errorf("Expected 30, got %d", i)
	}
}

func TestSandboxExecFile(t *testing.T) {
	p := setupSandboxTest(t)

	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "test_script.py")
	err := os.WriteFile(scriptFile, []byte("output = input_val * 2\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("input_val", 21)
env.exec_file("` + scriptFile + `")
env.get("output")
`
	result, evalErr := p.Eval(script)
	if evalErr != nil {
		t.Fatalf("Script error: %v", evalErr)
	}

	if i, _ := result.AsInt(); i != 42 {
		t.Errorf("Expected 42, got %d", i)
	}
}

func TestSandboxExitCode(t *testing.T) {
	p := setupSandboxTest(t)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("x = 1 + 1")
code = env.exit_code()
code
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 0 {
		t.Errorf("Expected exit code 0, got %d", i)
	}
}

func TestSandboxExitCodeOnError(t *testing.T) {
	p := setupSandboxTest(t)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("x = undefined_variable")
env.exit_code()
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 1 {
		t.Errorf("Expected exit code 1 for error, got %d", i)
	}
}

func TestSandboxIsolation(t *testing.T) {
	p := setupSandboxTest(t)

	// Variables in one sandbox should not be visible in another
	script := `
import scriptling.runtime as runtime

env1 = runtime.sandbox.create()
env2 = runtime.sandbox.create()

env1.set("x", 100)
env2.set("x", 200)

results = [env1.get("x"), env2.get("x")]
results
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %T", result)
	}

	if i, _ := list.Elements[0].AsInt(); i != 100 {
		t.Errorf("Expected env1.x=100, got %d", i)
	}

	if i, _ := list.Elements[1].AsInt(); i != 200 {
		t.Errorf("Expected env2.x=200, got %d", i)
	}
}

func TestSandboxNoFactory(t *testing.T) {
	ResetRuntime()
	SetSandboxFactory(nil)
	SetSandboxAllowedPaths(nil)

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p)

	// When no factory is set, create() returns an error which
	// propagates as a script error
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
`
	_, err := p.Eval(script)
	if err == nil {
		t.Error("Expected error when factory not set")
	}
}

func TestSandboxCaptureOutputDefault(t *testing.T) {
	p := setupSandboxTest(t)

	// By default, print output is discarded — script should not error
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("print('this should be discarded')")
env.exit_code()
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 0 {
		t.Errorf("Expected exit code 0, got %d", i)
	}
}

func TestSandboxCaptureOutputTrue(t *testing.T) {
	p := setupSandboxTest(t)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create(capture_output=True)
env.exec("print('hello world')")
env.exit_code()
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 0 {
		t.Errorf("Expected exit code 0, got %d", i)
	}
}

func TestSandboxExecFileNotFound(t *testing.T) {
	p := setupSandboxTest(t)

	// exec_file on nonexistent path sets exit_code to 1
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec_file("/nonexistent/path/script.py")
env.exit_code()
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 1 {
		t.Errorf("Expected exit code 1 for missing file, got %d", i)
	}
}

func TestSandboxExecFilePathRestriction(t *testing.T) {
	// Create temp directory and a script file
	allowedDir := t.TempDir()
	deniedDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "allowed.py")
	deniedFile := filepath.Join(deniedDir, "denied.py")

	os.WriteFile(allowedFile, []byte("result = 'allowed'\n"), 0644)
	os.WriteFile(deniedFile, []byte("result = 'denied'\n"), 0644)

	p := setupSandboxTest(t)

	// Restrict to allowedDir only
	SetSandboxAllowedPaths([]string{allowedDir})

	// Test allowed path
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec_file("` + allowedFile + `")
env.get("result")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if s, _ := result.AsString(); s != "allowed" {
		t.Errorf("Expected 'allowed', got '%s'", s)
	}

	// Test denied path — sets exit_code to 1
	script2 := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec_file("` + deniedFile + `")
env.exit_code()
`
	result2, err2 := p.Eval(script2)
	if err2 != nil {
		t.Fatalf("Script error: %v", err2)
	}

	if i, _ := result2.AsInt(); i != 1 {
		t.Errorf("Expected exit code 1 for denied path, got %d", i)
	}
}

func TestSandboxExecFileNoRestriction(t *testing.T) {
	// With no path restrictions, all paths should be allowed
	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "test.py")
	os.WriteFile(scriptFile, []byte("result = 'ok'\n"), 0644)

	p := setupSandboxTest(t)
	// Explicitly clear restrictions (default is unrestricted)
	SetSandboxAllowedPaths(nil)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec_file("` + scriptFile + `")
env.get("result")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if s, _ := result.AsString(); s != "ok" {
		t.Errorf("Expected 'ok', got '%s'", s)
	}
}

func TestSandboxExecMultiple(t *testing.T) {
	p := setupSandboxTest(t)

	// Multiple exec calls to the same sandbox should accumulate state
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("x = 1")
env.exec("y = 2")
env.exec("z = x + y")
env.get("z")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if i, _ := result.AsInt(); i != 3 {
		t.Errorf("Expected 3, got %d", i)
	}
}

func TestSandboxImportedLibraries(t *testing.T) {
	p := setupSandboxTest(t)

	// Sandbox instances should have stdlib available (from factory)
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("import json; result = json.dumps({'a': 1})")
env.get("result")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if s, _ := result.AsString(); s != `{"a":1}` && s != `{"a": 1}` {
		t.Errorf("Expected JSON string, got '%s'", s)
	}
}

func TestSandboxComplexDataTypes(t *testing.T) {
	p := setupSandboxTest(t)

	// Test passing complex data types through set/get
	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("config", {"debug": True, "port": 8080, "tags": ["a", "b"]})
env.exec("result = config['debug']")
r1 = env.get("result")
env.exec("result2 = config['port']")
r2 = env.get("result2")
[r1, r2]
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("Expected List, got %T", result)
	}

	if b, _ := list.Elements[0].AsBool(); !b {
		t.Error("Expected True for debug, got False")
	}

	if i, _ := list.Elements[1].AsInt(); i != 8080 {
		t.Errorf("Expected port 8080, got %d", i)
	}
}

func TestSandboxMCPPattern(t *testing.T) {
	// Test the MCP script execution pattern: set __mcp_params, exec script,
	// read __mcp_response — this is how fortix.mcp.call_script works
	p := setupSandboxTest(t)

	tmpDir := t.TempDir()
	scriptFile := filepath.Join(tmpDir, "mcp_script.py")
	os.WriteFile(scriptFile, []byte(`
name = __mcp_params.get("name", "world")
__mcp_response = "Hello, " + name + "!"
`), 0644)

	script := `
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("__mcp_params", {"name": "Alice"})
env.exec_file("` + scriptFile + `")
env.get("__mcp_response")
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if s, _ := result.AsString(); s != "Hello, Alice!" {
		t.Errorf("Expected 'Hello, Alice!', got '%s'", s)
	}
}
