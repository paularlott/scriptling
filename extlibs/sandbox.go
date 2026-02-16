package extlibs

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

// SandboxFactory creates new Scriptling instances for sandbox execution.
// Must be set by the host application before sandbox.create() can be used.
// The factory should return a fully configured instance with all required
// libraries registered and import paths configured.
type SandboxFactory func() SandboxInstance

// SandboxInstance is the minimal interface a sandbox environment needs.
// This matches the Scriptling public API without importing the scriptling package.
// It is also used by the background task factory in scriptling.runtime.
type SandboxInstance interface {
	SetObjectVar(name string, obj object.Object) error
	GetVarAsObject(name string) (object.Object, error)
	EvalWithContext(ctx context.Context, input string) (object.Object, error)
	SetSourceFile(name string)
	LoadLibraryIntoEnv(name string, env *object.Environment) error
	SetOutputWriter(w io.Writer)
}

// sandboxState holds the factory and path restrictions for sandbox instances
var sandboxState = struct {
	sync.RWMutex
	factory      SandboxFactory
	allowedPaths fssecurity.Config
}{}

// SetSandboxFactory sets the factory function for creating sandbox instances.
// Must be called before sandbox.create() is used in scripts.
//
// Example:
//
//	extlibs.SetSandboxFactory(func() extlibs.SandboxInstance {
//	    p := scriptling.New()
//	    setupMyLibraries(p)
//	    return p
//	})
func SetSandboxFactory(factory SandboxFactory) {
	sandboxState.Lock()
	sandboxState.factory = factory
	sandboxState.Unlock()
}

// SetSandboxAllowedPaths restricts exec_file() to the given directories.
// If allowedPaths is empty or nil, all paths are allowed (no restrictions).
// Paths are normalized to absolute paths at set time.
//
// SECURITY: When running untrusted scripts, ALWAYS provide allowedPaths to restrict
// which script files the sandbox can load and execute.
//
// Example:
//
//	extlibs.SetSandboxAllowedPaths([]string{"/opt/scripts"})
func SetSandboxAllowedPaths(allowedPaths []string) {
	normalizedPaths := make([]string, 0, len(allowedPaths))
	for _, p := range allowedPaths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		normalizedPaths = append(normalizedPaths, filepath.Clean(absPath))
	}

	sandboxState.Lock()
	sandboxState.allowedPaths = fssecurity.Config{AllowedPaths: normalizedPaths}
	sandboxState.Unlock()
}

// sandboxEnv wraps a SandboxInstance and tracks execution state
type sandboxEnv struct {
	instance SandboxInstance
	exitCode int
	executed bool
}

// SandboxSubLibrary is the sandbox sub-library for scriptling.runtime.
// Access it as scriptling.runtime.sandbox in scripts.
var SandboxSubLibrary = buildSandboxLibrary()

func buildSandboxLibrary() *object.Library {
	builder := object.NewLibraryBuilder("sandbox", "Isolated script execution environments")

	// create() - Create a new sandbox environment
	builder.FunctionWithHelp("create", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		sandboxState.RLock()
		factory := sandboxState.factory
		sandboxState.RUnlock()

		if factory == nil {
			return errors.NewError("sandbox.create() requires a factory — call extlibs.SetSandboxFactory() in Go first")
		}

		instance := factory()
		if instance == nil {
			return errors.NewError("sandbox factory returned nil")
		}

		// Check for capture_output kwarg (default: false = discard output)
		captureOutput, kwErr := kwargs.GetBool("capture_output", false)
		if kwErr != nil {
			return kwErr
		}

		// By default, discard print output from sandbox scripts
		if !captureOutput {
			instance.SetOutputWriter(io.Discard)
		}

		env := &sandboxEnv{
			instance: instance,
			exitCode: 0,
			executed: false,
		}

		return env.buildObject()
	}, `create(capture_output=False) - Create a new isolated sandbox environment

Creates a fresh script execution environment with its own variable scope.
The sandbox inherits the same library registrations and import paths as
the parent, but variables are completely isolated.

By default, print output from the sandbox is discarded. Set capture_output=True
to capture output (retrievable via the sandbox's output methods).

Requires the host application to configure a sandbox factory via
extlibs.SetSandboxFactory() in Go. Available in CLI mode by default.

Parameters:
  capture_output (bool, optional): If True, capture print output. Default: False

Returns:
  Sandbox object with set(), get(), exec(), exec_file(), and exit_code() methods

Example:
  import scriptling.runtime.sandbox as sandbox

  env = sandbox.create()
  env.set("config", {"debug": True})
  env.exec("result = config['debug']")
  print(env.get("result"))  # True`)

	return builder.Build()
}

// buildObject creates the scriptling object representing a sandbox environment
func (env *sandboxEnv) buildObject() object.Object {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"set": &object.Builtin{
				Fn:       env.setVar,
				HelpText: "set(name, value) - Set a variable in the sandbox",
			},
			"get": &object.Builtin{
				Fn:       env.getVar,
				HelpText: "get(name) - Get a variable from the sandbox",
			},
			"exec": &object.Builtin{
				Fn:       env.execCode,
				HelpText: "exec(code) - Execute script code in the sandbox",
			},
			"exec_file": &object.Builtin{
				Fn:       env.execFile,
				HelpText: "exec_file(path) - Load and execute a script file in the sandbox",
			},
			"exit_code": &object.Builtin{
				Fn:       env.getExitCode,
				HelpText: "exit_code() - Get the exit code from the last execution (0 = success)",
			},
		},
		HelpText: "Sandbox environment — an isolated script execution context",
	}
}

// set(name, value) - Set a variable in the sandbox
func (env *sandboxEnv) setVar(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 2); err != nil {
		return err
	}

	name, nameErr := args[0].AsString()
	if nameErr != nil {
		return nameErr
	}

	if setErr := env.instance.SetObjectVar(name, args[1]); setErr != nil {
		return errors.NewError("failed to set variable: %v", setErr)
	}

	return &object.Null{}
}

// get(name) - Get a variable from the sandbox
func (env *sandboxEnv) getVar(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	name, nameErr := args[0].AsString()
	if nameErr != nil {
		return nameErr
	}

	obj, getErr := env.instance.GetVarAsObject(name)
	if getErr != nil {
		return &object.Null{}
	}

	return obj
}

// exec(code) - Execute code in the sandbox
func (env *sandboxEnv) execCode(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	code, codeErr := args[0].AsString()
	if codeErr != nil {
		return codeErr
	}

	return env.runScript(ctx, code, "<sandbox>")
}

// exec_file(path) - Load and execute a script file in the sandbox.
// File read errors and path restriction violations are captured internally
// (check via exit_code()) rather than propagated.
func (env *sandboxEnv) execFile(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	if err := errors.ExactArgs(args, 1); err != nil {
		return err
	}

	path, pathErr := args[0].AsString()
	if pathErr != nil {
		return pathErr
	}

	// Check path restrictions
	sandboxState.RLock()
	config := sandboxState.allowedPaths
	sandboxState.RUnlock()

	if !config.IsPathAllowed(path) {
		env.exitCode = 1
		return &object.Null{}
	}

	content, readErr := os.ReadFile(path)
	if readErr != nil {
		env.exitCode = 1
		return &object.Null{}
	}

	env.instance.SetSourceFile(path)
	return env.runScript(ctx, string(content), path)
}

// exit_code() - Get exit code from last execution
func (env *sandboxEnv) getExitCode(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
	return conversion.FromGo(int64(env.exitCode))
}

// runScript executes code in the sandbox and handles SystemExit.
// Errors are captured internally (check via exit_code()) rather than
// propagated, so the calling script can continue after a failed exec.
func (env *sandboxEnv) runScript(ctx context.Context, code string, source string) object.Object {
	env.executed = true
	env.exitCode = 0

	result, evalErr := env.instance.EvalWithContext(ctx, code)

	// Check for SystemExit (used by tool.return_* functions)
	if exc, ok := object.AsException(result); ok && exc.IsSystemExit() {
		env.exitCode = exc.GetExitCode()
		return &object.Null{}
	}

	// Check for evaluation errors — capture internally, don't propagate
	if evalErr != nil {
		env.exitCode = 1
		return &object.Null{}
	}

	return &object.Null{}
}

// Verify SandboxInstance interface is satisfied at compile time.
// SandboxInstance matches the public API of *scriptling.Scriptling.
// The factory function should return a *scriptling.Scriptling or compatible type.
var _ SandboxInstance = (interface {
	SetObjectVar(string, object.Object) error
	GetVarAsObject(string) (object.Object, error)
	EvalWithContext(context.Context, string) (object.Object, error)
	SetSourceFile(string)
	LoadLibraryIntoEnv(string, *object.Environment) error
	SetOutputWriter(io.Writer)
})(nil)
