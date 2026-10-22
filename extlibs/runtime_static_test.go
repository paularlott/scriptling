package extlibs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

// Under --allowed-paths, runtime.http.static refuses a directory outside the
// allowed paths when it is registered, and records the restriction for the
// server; without it any directory is accepted as before (vuln-0016).
func TestRuntimeHTTPStaticHonoursAllowedPaths(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	allowed := filepath.Join(base, "allowed")
	inside := filepath.Join(allowed, "public")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{inside, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(allowed, "escape")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	register := func(allowedPaths []string, dir string) (*RouteInfo, error) {
		ResetRuntime()
		p := scriptling.New()
		RegisterRuntimeLibraryAll(p, allowedPaths)
		_, err := p.Eval(`import scriptling.runtime as runtime
runtime.http.static("/files", "` + dir + `")`)
		RuntimeState.Lock()
		route := RuntimeState.Routes["GET /files"]
		RuntimeState.Unlock()
		return route, err
	}

	for _, dir := range []string{outside, "/", filepath.Join(allowed, "escape")} {
		if _, err := register([]string{allowed}, dir); err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Errorf("static(%q) under --allowed-paths should be refused, got: %v", dir, err)
		}
	}
	route, err := register([]string{allowed}, inside)
	if err != nil {
		t.Fatalf("a directory inside the allowed paths was refused: %v", err)
	}
	if route == nil || len(route.StaticAllowed) != 1 || route.StaticAllowed[0] != allowed {
		t.Fatalf("the restriction was not recorded for the server: %+v", route)
	}

	route, err = register(nil, outside)
	if err != nil || route == nil || route.StaticAllowed != nil {
		t.Fatalf("unrestricted static() should accept any directory unrestricted: route=%+v err=%v", route, err)
	}
	ResetRuntime()
}
