package server

import (
	"github.com/paularlott/scriptling/extlibs/runtime"
	rthttp "github.com/paularlott/scriptling/extlibs/runtime/http"
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	"github.com/paularlott/scriptling/object"
)

// registerRuntimeAll registers scriptling.runtime with every sub-library the
// CLI registers (setup.Scriptling does the same through its disabled-library
// filter), for tests that build a bare interpreter.
func registerRuntimeAll(registrar object.LibraryRegistrar, allowedPaths []string) {
	rthttp.Register(registrar, allowedPaths)
	kv.Register(registrar, allowedPaths)
	runtime.RegisterSync(registrar)
	runtime.RegisterSandbox(registrar, allowedPaths)
	runtime.RegisterJSONRPC(registrar)
	runtime.RegisterMCP(registrar)
	runtime.Register(registrar)
}
