package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	scriptling "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	plugin "github.com/paularlott/scriptling/plugin"
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
	metadata := extlibs.RuntimeState.PluginMetadata
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
	if metadata != nil {
		ps.SetMetadata(metadata)
	}

	for funcName, handlerRef := range handlers {
		ref := handlerRef // capture for closure
		ps.RegisterBuiltin(funcName, func(callCtx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return s.runPluginHandler(callCtx, ref, args, kwargs.Kwargs)
		})
		Log.Debug("Registered plugin function", "name", funcName, "handler", ref)
	}

	for constName, val := range constants {
		ps.Constant(constName, val)
		Log.Debug("Registered plugin constant", "name", constName)
	}

	for className, classRef := range classes {
		class, err := s.resolveClass(classRef)
		if err != nil {
			Log.Error("Failed to resolve plugin class", "class", classRef, "error", err)
			continue
		}
		ps.RegisterBuiltinClass(className, class)
		Log.Debug("Registered plugin class", "name", className, "handler", classRef)
	}

	// A script-declared fetcher (runtime.plugin.register_fetcher): the peer
	// serves sources from script handlers — how a script peer carries a
	// host's declared assets inside itself.
	if scheme, read := extlibs.RuntimeState.PluginFetchScheme, extlibs.RuntimeState.PluginFetchRead; read != "" {
		ps.RegisterFetcher(scheme, scriptFetcher{s: s, read: read, glob: extlibs.RuntimeState.PluginFetchGlob})
		Log.Debug("Registered plugin fetcher", "scheme", scheme, "read", read)
	}

	Log.Info("Plugin server ready", "name", name, "version", version,
		"functions", len(handlers), "constants", len(constants), "classes", len(classes))
	s.pluginServer = ps
}

// scriptFetcher adapts script handlers to the plugin.Fetcher interface: the
// read ref answers (source, path) with the file's contents, the optional
// glob ref answers (source, pattern) with a list of {name, is_dir} dicts.
// Conventions the handlers are held to: None from read is a miss
// (ErrFetchNotFound), a script error fails the call, and the contents may
// be a string or bytes (bytes survive the wire's base64 intact).
type scriptFetcher struct {
	s    *Server
	read string
	glob string // optional; empty serves no glob matches
}

func (f scriptFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	res := f.s.runPluginHandler(ctx, f.read,
		[]object.Object{object.NewString(source), object.NewString(path)}, nil)
	if res == nil {
		return nil, plugin.ErrFetchNotFound
	}
	if err, isErr := res.(*object.Error); isErr {
		return nil, errors.New(err.Message)
	}
	if _, isNull := res.(*object.Null); isNull {
		return nil, plugin.ErrFetchNotFound
	}
	switch v := res.(type) {
	case *object.String:
		return []byte(v.StringValue()), nil
	case *object.Bytes:
		return v.BytesValue(), nil
	}
	return nil, fmt.Errorf("fetch read %s returned %s, want string, bytes or None", f.read, res.Type())
}

func (f scriptFetcher) Glob(ctx context.Context, source, pattern string) ([]plugin.FetchEntry, error) {
	if f.glob == "" {
		// No glob handler: no matches is a valid answer per the contract.
		return nil, nil
	}
	res := f.s.runPluginHandler(ctx, f.glob,
		[]object.Object{object.NewString(source), object.NewString(pattern)}, nil)
	if res == nil {
		return nil, nil
	}
	if err, isErr := res.(*object.Error); isErr {
		return nil, errors.New(err.Message)
	}
	if _, isNull := res.(*object.Null); isNull {
		return nil, nil
	}
	list, isList := res.(*object.List)
	if !isList {
		return nil, fmt.Errorf("fetch glob %s returned %s, want a list", f.glob, res.Type())
	}
	entries := make([]plugin.FetchEntry, 0, len(list.Elements))
	for _, el := range list.Elements {
		dict, isDict := el.(*object.Dict)
		if !isDict {
			continue
		}
		entry := plugin.FetchEntry{}
		if namePair, ok := dict.GetByString("name"); ok {
			if name, err := namePair.Value.AsString(); err == nil {
				entry.Name = name
			}
		}
		if dirPair, ok := dict.GetByString("is_dir"); ok {
			if dir, err := dirPair.Value.AsBool(); err == nil {
				entry.IsDir = dir
			}
		}
		if entry.Name != "" {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// resolveClass imports the module and evaluates the class reference on a fresh
// evaluator, returning the *object.Class for registration in the plugin server.
// The class and its method closures remain valid for the lifetime of the server.
func (s *Server) resolveClass(classRef string) (*object.Class, error) {
	libName, _, ok := splitHandlerRef(classRef)
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
	libName, _, ok := splitHandlerRef(handlerRef)
	if !ok {
		Log.Error("Invalid plugin handler reference", "handler", handlerRef)
		return &object.Error{Message: "invalid plugin handler reference: " + handlerRef}
	}

	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	if err := p.ImportWithContext(ctx, libName); err != nil {
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
