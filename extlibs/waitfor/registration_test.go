package waitfor_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/extlibs/waitfor"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNameConstant(t *testing.T) {
	if waitfor.LibraryName != "scriptling.wait_for" {
		t.Errorf("waitfor.LibraryName = %q, want %q (scripts import it by this name)", waitfor.LibraryName, "scriptling.wait_for")
	}
}

func TestWaitForRegisterAndWait(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ready")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f := filepath.ToSlash(file)
	missing := filepath.ToSlash(filepath.Join(dir, "never"))

	tests := []struct {
		name     string
		register func(*scriptling.Scriptling)
	}{
		{"Register", func(p *scriptling.Scriptling) { waitfor.Register(p) }},
		{"RegisterConfigured", func(p *scriptling.Scriptling) {
			waitfor.RegisterConfigured(p, []string{dir}, &netsecurity.Config{})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scriptling.New()
			tt.register(p)
			// Already true: returns at once. Never true: a short timeout.
			got := scripttest.Eval(t, p, "import scriptling.wait_for as wait_for\n"+
				"[wait_for.file('"+f+"', timeout=5), wait_for.file('"+missing+"', timeout=1, poll_rate=0.1)]").Inspect()
			if want := "[True, False]"; got != want {
				t.Errorf("wait_for.file = %s, want %s", got, want)
			}
		})
	}
}

func TestWaitForNotImportableWithoutRegister(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	scripttest.ImportFails(t, p, "scriptling.wait_for", "it is only available after waitfor.Register(p, policy)")
}

// A path outside allowedPaths is a protected PermissionError, like the
// filesystem libraries: the script cannot swallow it.
func TestWaitForPathPolicyRefusal(t *testing.T) {
	allowed := t.TempDir()
	outside := filepath.Join(t.TempDir(), "x")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	p := scriptling.New()
	waitfor.RegisterConfigured(p, []string{allowed})
	msg := scripttest.Refused(t, p, "import scriptling.wait_for as wait_for\nwait_for.file('"+filepath.ToSlash(outside)+"', timeout=1)")
	if !strings.Contains(msg, "outside allowed") {
		t.Errorf("wait_for.file outside allowedPaths: got %q, want a path-policy refusal", msg)
	}
}

// A network wait the policy blocks raises an exception the script can catch.
func TestWaitForNetworkPolicyRefusalIsCatchable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(srv.Close)
	p := scriptling.New()
	waitfor.Register(p, &netsecurity.Config{})
	_, msg := scripttest.Raises(t, p, "import scriptling.wait_for as wait_for\nwait_for.http('"+srv.URL+"', timeout=1)")
	if !strings.Contains(msg, "network policy") {
		t.Errorf("blocked wait_for.http raised %q, want a network policy error", msg)
	}
}
