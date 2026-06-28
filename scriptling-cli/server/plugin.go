package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	scriptling "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
)

// buildPluginServer creates a plugin.Server from registered state and stores it
// on s.pluginServer. Called by NewServer after the setup script finishes so
// that both the stdio path (RunPluginServerStdio) and the HTTP path
// (buildMux → /json-rpc handler) can use the same pre-built instance.
// No-op if runtime.plugin.serve() was not called.
func (s *Server) buildPluginServer() {
	extlibs.RuntimeState.RLock()
	name := extlibs.RuntimeState.PluginName
	if name == "" {
		extlibs.RuntimeState.RUnlock()
		return
	}
	version := extlibs.RuntimeState.PluginVersion
	desc := extlibs.RuntimeState.PluginDescription
	handlers := make(map[string]string, len(extlibs.RuntimeState.PluginFunctions))
	for k, v := range extlibs.RuntimeState.PluginFunctions {
		handlers[k] = v
	}
	constants := make(map[string]object.Object, len(extlibs.RuntimeState.PluginConstants))
	for k, v := range extlibs.RuntimeState.PluginConstants {
		constants[k] = v
	}
	classes := make(map[string]string, len(extlibs.RuntimeState.PluginClasses))
	for k, v := range extlibs.RuntimeState.PluginClasses {
		classes[k] = v
	}
	extlibs.RuntimeState.RUnlock()

	ps := scriptlingplugin.NewServer(name, version, desc)

	for funcName, handlerRef := range handlers {
		ref := handlerRef // capture for closure
		ps.RegisterBuiltin(funcName, func(callCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return s.runPluginHandler(callCtx, ref, args, kwargs.Kwargs)
		})
		Log.Info("Registered plugin function", "name", funcName, "handler", ref)
	}

	for constName, val := range constants {
		ps.Constant(constName, val)
		Log.Info("Registered plugin constant", "name", constName)
	}

	for className, classRef := range classes {
		class, err := s.resolveClass(classRef)
		if err != nil {
			Log.Error("Failed to resolve plugin class", "class", classRef, "error", err)
			continue
		}
		ps.RegisterBuiltinClass(className, class)
		Log.Info("Registered plugin class", "name", className, "handler", classRef)
	}

	Log.Info("Plugin server ready", "name", name, "version", version,
		"functions", len(handlers), "constants", len(constants), "classes", len(classes))
	s.pluginServer = ps
}

// resolveClass imports the module and evaluates the class reference on a fresh
// evaluator, returning the *object.Class for registration in the plugin server.
// The class and its method closures remain valid for the lifetime of the server.
func (s *Server) resolveClass(classRef string) (*object.Class, error) {
	libName, _, ok := strings.Cut(classRef, ".")
	if !ok {
		return nil, fmt.Errorf("class handler %q must be in \"module.ClassName\" form", classRef)
	}

	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	if err := p.Import(libName); err != nil {
		return nil, fmt.Errorf("import %s: %w", libName, err)
	}

	result, err := p.Eval(classRef)
	if err != nil {
		return nil, fmt.Errorf("eval %s: %w", classRef, err)
	}

	class, ok := result.(*object.Class)
	if !ok {
		return nil, fmt.Errorf("%s is a %T, not a class", classRef, result)
	}
	return class, nil
}

// RunPluginServerStdio serves the Scriptling plugin protocol over stdio using
// the pre-built plugin server.
func (s *Server) RunPluginServerStdio(ctx context.Context) error {
	return s.runPluginServer(ctx, os.Stdin, os.Stdout)
}

// runPluginServer serves the full Scriptling plugin protocol over the given
// reader/writer pair. Used by RunPluginServerStdio (os.Stdin/Stdout) and tests.
func (s *Server) runPluginServer(ctx context.Context, in io.Reader, out io.Writer) error {
	return s.pluginServer.RunIO(in, out)
}

// runPluginHandler imports the handler library on a fresh evaluator and calls
// the named function with the decoded plugin arguments and kwargs.
func (s *Server) runPluginHandler(ctx context.Context, handlerRef string, args []object.Object, kwargs map[string]object.Object) object.Object {
	libName, _, ok := strings.Cut(handlerRef, ".")
	if !ok {
		Log.Error("Invalid plugin handler reference", "handler", handlerRef)
		return &object.Error{Message: "invalid plugin handler reference: " + handlerRef}
	}

	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import plugin handler library", "library", libName, "error", err)
		return &object.Error{Message: fmt.Sprintf("failed to import %s: %v", libName, err)}
	}

	// Build interface{} slice for CallFunctionWithContext.
	// object.Object satisfies interface{} and is passed through as-is.
	// A scriptling.Kwargs entry as the last element carries keyword arguments.
	ifaces := make([]interface{}, 0, len(args)+1)
	for _, a := range args {
		ifaces = append(ifaces, a)
	}
	if len(kwargs) > 0 {
		kw := make(scriptling.Kwargs, len(kwargs))
		for k, v := range kwargs {
			kw[k] = v
		}
		ifaces = append(ifaces, kw)
	}

	result, err := p.CallFunctionWithContext(ctx, handlerRef, ifaces...)
	if err != nil {
		Log.Error("Plugin handler error", "handler", handlerRef, "error", err)
		return &object.Error{Message: err.Error()}
	}
	return result
}
