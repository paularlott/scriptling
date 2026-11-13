package http

// Route-registration and response tests, moved here from
// extlibs/runtime/runtime_test.go with the library.

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
	"github.com/paularlott/scriptling/object"
)

// registerAll registers http with the runtime core and its in-package
// sub-libraries, as the CLI does, so both the sub-library and the parent
// (`runtime.http`) import forms are available.
func registerAll(registrar object.LibraryRegistrar, allowedPaths []string) {
	Register(registrar, allowedPaths)
	runtime.RegisterSync(registrar)
	runtime.RegisterSandbox(registrar, allowedPaths)
	runtime.RegisterJSONRPC(registrar)
	runtime.RegisterMCP(registrar)
	runtime.Register(registrar)
}

func TestRuntimeHTTP(t *testing.T) {
	runtime.ResetRuntime()
	p := scriptling.New()
	registerAll(p, nil)

	script := `
import scriptling.runtime as runtime

runtime.http.get("/test", "handler.test")
runtime.http.post("/api", "handler.api")
runtime.http.middleware("auth.check")
runtime.http.static("/assets", "./public")
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Failed to execute script: %v", err)
	}

	runtime.RuntimeState.RLock()
	defer runtime.RuntimeState.RUnlock()

	if len(State.Routes) != 3 {
		t.Errorf("Expected 3 routes, got %d", len(State.Routes))
	}

	if route, ok := State.Routes["GET /test"]; !ok || route.Handler != "handler.test" {
		t.Error("GET route not registered correctly")
	}

	if State.Middleware != "auth.check" {
		t.Errorf("Middleware not set correctly: %s", State.Middleware)
	}
}

func TestRuntimeHTTPResponses(t *testing.T) {
	p := scriptling.New()
	registerAll(p, nil)

	tests := []struct {
		name   string
		script string
		check  func(*object.Dict) error
	}{
		{
			name:   "json response",
			script: `import scriptling.runtime as runtime; runtime.http.json(200, {"test": "data"})`,
			check: func(d *object.Dict) error {
				if status, ok := d.GetByString("status"); !ok {
					t.Error("Missing status")
				} else if s, _ := status.Value.AsInt(); s != 200 {
					t.Errorf("Expected status 200, got %d", s)
				}
				return nil
			},
		},
		{
			name:   "redirect response",
			script: `import scriptling.runtime as runtime; runtime.http.redirect("/new")`,
			check: func(d *object.Dict) error {
				if headers, ok := d.GetByString("headers"); ok {
					if hDict, err := headers.Value.AsDict(); err == nil {
						if loc, ok := hDict["Location"]; ok {
							if locStr, err := loc.AsString(); err == nil && locStr != "/new" {
								t.Error("Location header not set correctly")
							}
						} else {
							t.Error("Location header missing")
						}
					}
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("Script error: %v", err)
			}
			if dict, ok := result.(*object.Dict); ok {
				tt.check(dict)
			} else {
				t.Error("Expected Dict result")
			}
		})
	}
}
