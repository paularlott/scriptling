package libboundary

import (
	"reflect"
	"testing"
)

// These self-checks feed the checkers synthetic input, so a checker that
// silently stopped reporting (and let every architecture test pass) fails
// here instead.

func TestPathMatches(t *testing.T) {
	tests := []struct {
		pattern, pkg string
		want         bool
	}{
		{"net/http", "net/http", true},
		{"net/http", "net/http/httptrace", true},
		{"net/http", "golang.org/x/net/http2", false},
		{"net/http", "net/httpx", false},
		{"os", "os/exec", true},
		{"os", "osext", false},
		{"github.com/shirou/gopsutil", "github.com/shirou/gopsutil/v3/process", true},
		{modulePath + "/extlibs/runtime", modulePath + "/extlibs/runtimex", false},
	}
	for _, tt := range tests {
		if got := pathMatches(tt.pattern, tt.pkg); got != tt.want {
			t.Errorf("pathMatches(%q, %q) = %v, want %v", tt.pattern, tt.pkg, got, tt.want)
		}
	}
}

func TestBannedImportsChecker(t *testing.T) {
	imports := []string{"fmt", "net/url", "os", "net", "net/http", "syscall", "strings"}
	got := bannedImports(imports, stdlibBannedImports, stdlibAllowedImports)
	want := []string{
		"net (network access)",
		"net/http (network access)",
		"os (process, environment and filesystem access)",
		"syscall (host access)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bannedImports reported %q, want %q (net/url is an allowed exception, fmt/strings are fine)", got, want)
	}
	if got := bannedImports([]string{"fmt", "net/url"}, stdlibBannedImports, stdlibAllowedImports); len(got) != 0 {
		t.Errorf("clean import list reported %q", got)
	}
}

func TestMatchingDepsChecker(t *testing.T) {
	deps := []string{"fmt", "net", "net/http", "net/http/internal", "github.com/gorilla/websocket", "golang.org/x/net/http2"}
	got := matchingDeps(deps, []string{"net/http", "github.com/gorilla/websocket", "os/exec"})
	want := []string{"github.com/gorilla/websocket", "net/http", "net/http/internal"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchingDeps = %q, want %q", got, want)
	}
	if got := matchingDeps([]string{"fmt", "net"}, []string{"net/http"}); len(got) != 0 {
		t.Errorf("transitive net must not count as net/http, got %q", got)
	}
}

func TestThirdPartyChecker(t *testing.T) {
	if isThirdParty("vendor/golang.org/x/net/dns/dnsmessage", modulePath) {
		t.Error("vendored standard-library packages are not third-party")
	}
	if isThirdParty(modulePath+"/object", modulePath) {
		t.Error("the module's own packages are not third-party")
	}
	if !isThirdParty("gopkg.in/yaml.v3", modulePath) {
		t.Error("gopkg.in/yaml.v3 is third-party")
	}

	deps := []string{"fmt", modulePath + "/object", "github.com/google/uuid", "gopkg.in/yaml.v3"}
	unexpected, unused := thirdPartyDiff(deps, modulePath, []string{"github.com/google/uuid", "github.com/shamaton/msgpack/v3"})
	if !reflect.DeepEqual(unexpected, []string{"gopkg.in/yaml.v3"}) {
		t.Errorf("unexpected = %q, want [gopkg.in/yaml.v3]", unexpected)
	}
	if !reflect.DeepEqual(unused, []string{"github.com/shamaton/msgpack/v3"}) {
		t.Errorf("unused = %q, want the stale msgpack entry", unused)
	}
}

func TestLeafViolationsChecker(t *testing.T) {
	imports := map[string][]string{
		"yaml":      {"fmt", modulePath + "/object", extlibsPrefix + "fssecurity"},
		"requests":  {extlibsPrefix + "runtime", extlibsPrefix + "netsecurity"},
		"ai/memory": {extlibsPrefix + "ai"},
	}
	support := map[string]string{"fssecurity": "", "netsecurity": ""}
	allowed := map[edge]string{
		{"ai/memory", "ai"}:   "",
		{"gone", "elsewhere"}: "",
	}
	violations, stale := leafViolations(imports, support, allowed)
	if !reflect.DeepEqual(violations, []edge{{"requests", "runtime"}}) {
		t.Errorf("violations = %v, want [requests -> runtime] (support imports and allowed edges are fine)", violations)
	}
	if !reflect.DeepEqual(stale, []edge{{"gone", "elsewhere"}}) {
		t.Errorf("stale = %v, want [gone -> elsewhere]", stale)
	}
}

func TestSymbolHitsChecker(t *testing.T) {
	nm := []byte(`  100a20 T main.main
  200b40 T net/http.(*Client).Do
  300c60 R type:struct { F *github.com/gorilla/websocket.Conn }
         U os/exec.(*Cmd).Start
`)
	got := symbolHits(nm, []string{"net/http.(*Client)", "github.com/gorilla/websocket.", "os/exec.(*Cmd)", "github.com/yuin/goldmark"})
	want := []string{"github.com/gorilla/websocket.", "net/http.(*Client)", "os/exec.(*Cmd)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("symbolHits = %q, want %q", got, want)
	}
	if got := symbolHits([]byte("  1 T main.main\n"), []string{"net/http."}); len(got) != 0 {
		t.Errorf("clean binary reported %q", got)
	}
}

func TestWithoutHelper(t *testing.T) {
	got := without(depYAML, depTOML)
	for _, d := range got {
		if d == depYAML || d == depTOML {
			t.Fatalf("without() kept %s", d)
		}
	}
	if len(got) != len(allHeavy)-2 {
		t.Errorf("without() dropped the wrong number of entries: %v", got)
	}
}
