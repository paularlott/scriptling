package runtime

import (
	"github.com/paularlott/scriptling/extlibs/runtime/kv"
	"github.com/paularlott/scriptling/object"
)

// registerAll registers the runtime core with every sub-library these
// internal tests can reach: kv (a separate package that does not import this
// one) and the sub-libraries in this package. scriptling.runtime.http imports
// this package, so its tests live in extlibs/runtime/http.
func registerAll(registrar object.LibraryRegistrar, allowedPaths []string) {
	kv.Register(registrar, allowedPaths)
	RegisterSync(registrar)
	RegisterSandbox(registrar, allowedPaths)
	RegisterJSONRPC(registrar)
	RegisterMCP(registrar)
	Register(registrar)
}
