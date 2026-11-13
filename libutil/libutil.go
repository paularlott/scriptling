package libutil

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/paularlott/scriptling/object"
)

// EnvFromContext returns the environment of the call ctx belongs to, or a
// fresh empty environment when the context carries none.
func EnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment()
}

// NewAliasedLibrary re-names a library so it can be imported under more than
// one module name. The functions and constants are shared by reference.
func NewAliasedLibrary(lib *object.Library, name string) *object.Library {
	functions := map[string]*object.Builtin{}
	for fnName, fn := range lib.Functions() {
		functions[fnName] = fn
	}
	constants := lib.Constants()
	if constants == nil {
		constants = map[string]object.Object{}
	}
	return object.NewLibrary(name, functions, constants, lib.Description())
}

// ErrorJSONBody renders {"error": <message>} as a JSON body with proper
// escaping. Protocol endpoints report failures through it, so an error (or a
// user message) containing quotes or control characters must not be spliced
// into the body with Sprintf, which would emit malformed JSON.
func ErrorJSONBody(message string) string {
	data, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		// Unreachable for map[string]string; keep the body valid regardless.
		return `{"error": "internal error"}`
	}
	return string(data)
}

var (
	cleanupMu    sync.Mutex
	cleanupFuncs []func()
)

// RegisterCleanup registers a function to be called by RunCleanups. Libraries
// that hold process-wide state (open sockets, cluster memberships) register
// one so a host resetting the runtime releases that state, without the
// library having to import the runtime library to say so.
func RegisterCleanup(fn func()) {
	cleanupMu.Lock()
	cleanupFuncs = append(cleanupFuncs, fn)
	cleanupMu.Unlock()
}

// RunCleanups calls every registered cleanup function, in registration
// order. The functions stay registered, so each call runs them all again;
// they manage their own synchronisation and are called without any lock held.
func RunCleanups() {
	cleanupMu.Lock()
	fns := append([]func(){}, cleanupFuncs...)
	cleanupMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}
