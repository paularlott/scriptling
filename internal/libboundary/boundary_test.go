package libboundary

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// stdlibThirdParty is the exact set of third-party import-path prefixes the
// stdlib package may link. Core links all of them already, so registering the
// standard libraries adds no module. Adding an entry means every embedder
// pays for it: put a library that needs a new module in its own extlibs/
// package instead.
var stdlibThirdParty = []string{
	"github.com/google/uuid",             // uuid library
	"github.com/paularlott/gossip/codec", // msgpack codec interface
	"github.com/shamaton/msgpack/v3",     // msgpack encoder behind the codec
}

// coreThirdParty is the exact set of third-party import-path prefixes the
// root package and evaluator may link: stdlib's set plus x/text, which the
// evaluator uses for Unicode-aware str.title/casefold.
var coreThirdParty = append(append([]string(nil), stdlibThirdParty...),
	"golang.org/x/text", // evaluator: Unicode case mapping
)

// stdlibBannedImports are the packages stdlib must not import directly:
// standard libraries have no filesystem, network, process, environment or
// host access. Libraries that need one belong in an extlibs/ package.
var stdlibBannedImports = []rule{
	{"os", "process, environment and filesystem access"},
	{"syscall", "host access"},
	{"plugin", "loads host code"},
	{"path/filepath", "host filesystem paths"},
	{"io/fs", "filesystem access"},
	{"io/ioutil", "filesystem access"},
	{"net", "network access"},
	{"crypto/tls", "network access"},
	{"runtime/cgo", "host code"},
}

// stdlibAllowedImports are exact exceptions to stdlibBannedImports, each with
// the reason it is pure computation.
var stdlibAllowedImports = map[string]string{
	"net/url": "urllib / urllib.parse: pure URL parsing, no network access",
}

// TestStdlibPurity enforces that the always-linked standard libraries are
// pure computation with no extra third-party weight.
func TestStdlibPurity(t *testing.T) {
	pkg := listOne(t, "./stdlib")

	t.Run("no direct host or network imports", func(t *testing.T) {
		if bad := bannedImports(pkg.Imports, stdlibBannedImports, stdlibAllowedImports); len(bad) > 0 {
			t.Errorf("stdlib (always linked, pure compute) imports banned packages:\n  %s\n"+
				"Fix: move the library that needs them to its own extlibs/<name> package with a Register function, "+
				"so only embedders that register it link it.", strings.Join(bad, "\n  "))
		}
	})

	t.Run("no os/exec or net/http transitively", func(t *testing.T) {
		if bad := matchingDeps(pkg.Deps, []string{"os/exec", "net/http"}); len(bad) > 0 {
			t.Errorf("stdlib transitively depends on %v. Some import (possibly a third-party one) pulls them in; "+
				"find it with `go list -deps -f '{{.ImportPath}}: {{.Imports}}' ./stdlib` and move that code out of stdlib.", bad)
		}
	})

	t.Run("third-party modules are exactly the allowlist", func(t *testing.T) {
		unexpected, unused := thirdPartyDiff(pkg.Deps, modulePath, stdlibThirdParty)
		if len(unexpected) > 0 {
			t.Errorf("stdlib links third-party packages not in stdlibThirdParty:\n  %s\n"+
				"Fix: stdlib may only use modules core already links; put a library that needs a new module in extlibs/.",
				strings.Join(unexpected, "\n  "))
		}
		if len(unused) > 0 {
			t.Errorf("stdlibThirdParty lists %v but stdlib no longer depends on them; remove the stale entries so the allowlist stays exact.", unused)
		}
	})
}

// TestCorePurity enforces that the interpreter core links only its small
// third-party set and none of the extension libraries.
func TestCorePurity(t *testing.T) {
	for _, pattern := range []string{".", "./evaluator"} {
		t.Run(pattern, func(t *testing.T) {
			pkg := listOne(t, pattern)
			unexpected, unused := thirdPartyDiff(pkg.Deps, modulePath, coreThirdParty)
			if len(unexpected) > 0 {
				t.Errorf("%s links third-party packages not in coreThirdParty:\n  %s\n"+
					"Fix: every embedder links core, so new modules belong in an extlibs/ package.",
					pkg.ImportPath, strings.Join(unexpected, "\n  "))
			}
			if len(unused) > 0 {
				t.Errorf("coreThirdParty lists %v but %s no longer depends on them; remove the stale entries.", unused, pkg.ImportPath)
			}
			var ext []string
			for _, d := range pkg.Deps {
				if strings.HasPrefix(d, extlibsPrefix) || d == strings.TrimSuffix(extlibsPrefix, "/") {
					ext = append(ext, d)
				}
			}
			if len(ext) > 0 {
				t.Errorf("%s depends on extension packages %v. Core must not link any extension library; "+
					"invert the dependency (the extension registers itself with core).", pkg.ImportPath, ext)
			}
		})
	}
}

// TestNoRootExtlibsPackage enforces that extlibs/ is only a directory of
// extension packages: a Go package at .../extlibs would let one import link
// every library again.
func TestNoRootExtlibsPackage(t *testing.T) {
	dir := filepath.Join(moduleRoot(t), "extlibs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var goFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			goFiles = append(goFiles, e.Name())
		}
	}
	if len(goFiles) > 0 {
		t.Errorf("extlibs/ must contain no .go files (there is no package %s/extlibs), found:\n  %s\n"+
			"Fix: move code to the extlibs/<name> package that owns it, and cross-library tests to extlibs/integration.",
			modulePath, strings.Join(goFiles, "\n  "))
	}
}

// supportPackages are the packages under extlibs/ that are policy or shared
// support code, not script libraries. Any extension package may import them.
var supportPackages = map[string]string{
	"fssecurity":     "filesystem path policy",
	"netsecurity":    "network policy",
	"secretprovider": "secret provider registry and provider-type factory table",
	// The concrete providers are support code too (not script libraries): an
	// embedder links one only by calling its Register.
	"secretprovider/vault":       "HashiCorp Vault secret provider",
	"secretprovider/onepassword": "1Password Connect secret provider",
	"netsecurity/policyfile":     "TOML loader for netsecurity policy files",
	"reqctx":                     "per-request context shared by runtime and mcp",
	"net/internal":               "shared code for net/multicast and net/unicast",
	"messaging/shared":           "shared code for the messaging clients",
	"provision/become":           "privilege-escalation helper for provision/*",
}

// allowedLibraryEdges are the reviewed exceptions to the leaf rule: an
// extension library importing another extension library. Every entry links
// the target whenever the source is registered, so keep this list short.
var allowedLibraryEdges = map[edge]string{
	{"ai", "similarity"}:                        "ai's cosine-similarity helper calls similarity.CosineSimilarity",
	{"ai", "ai/tools"}:                          "ai exposes the ToolRegistry class from ai/tools",
	{"ai/memory", "ai"}:                         "memory takes an ai.Client (AIClientFromObject) for LLM compaction",
	{"ai/memory", "runtime/kv"}:                 "memory persists through the kv store (kv.StoreDB)",
	{"runtime/http", "runtime"}:                 "routes are served by the runtime core's server lifecycle: start_server snapshots them under runtime.RuntimeState's lock, which also guards http.State",
	{"messaging/console", "console"}:            "the console messaging backend drives the console TUI library",
	{"provision/packages", "provision/command"}: "package installs run through provision/command",
}

// TestLeafRule enforces that extension packages are leaves: registering one
// library links only that library (plus core, stdlib and support packages).
func TestLeafRule(t *testing.T) {
	imports := map[string][]string{}
	for _, p := range listPackages(t, "./extlibs/...") {
		if !strings.HasPrefix(p.ImportPath, extlibsPrefix) {
			continue // the root extlibs package, covered by TestNoRootExtlibsPackage
		}
		rel := strings.TrimPrefix(p.ImportPath, extlibsPrefix)
		if _, ok := supportPackages[rel]; ok {
			continue
		}
		imports[rel] = p.Imports
	}
	if len(imports) == 0 {
		t.Fatal("go list ./extlibs/... found no extension packages; the checker is looking in the wrong place")
	}
	violations, stale := leafViolations(imports, supportPackages, allowedLibraryEdges)
	for _, v := range violations {
		t.Errorf("leaf rule broken: extlibs/%s imports extension library extlibs/%s.\n"+
			"Fix: move the shared code to a support package (and add it to supportPackages), pass it in at "+
			"registration time, or, if the edge is truly intended, add it to allowedLibraryEdges with a reason.", v.from, v.to)
	}
	for _, s := range stale {
		t.Errorf("allowedLibraryEdges lists %s but that import no longer exists; remove the entry.", s)
	}
}

// Heavy dependencies the weight-isolation table refers to.
const (
	depHTTP       = "net/http"
	depExec       = "os/exec"
	depWebsocket  = "github.com/gorilla/websocket"
	depSnapshotKV = "github.com/paularlott/snapshotkv"
	depGoldmark   = "github.com/yuin/goldmark"
	depGopsutil   = "github.com/shirou/gopsutil"
	depYAML       = "gopkg.in/yaml.v3"
	depTOML       = "github.com/BurntSushi/toml"
	depRuntime    = modulePath + "/extlibs/runtime" // also matches runtime/kv and runtime/http
	depFilesystem = modulePath + "/extlibs/filesystem"

	depRuntimeKV   = modulePath + "/extlibs/runtime/kv"
	depRuntimeHTTP = modulePath + "/extlibs/runtime/http"
)

// allHeavy is every dependency a light library must not pull.
var allHeavy = []string{depHTTP, depExec, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depYAML, depTOML, depRuntime, depFilesystem}

// without returns allHeavy minus the given entries.
func without(drop ...string) []string {
	var out []string
outer:
	for _, d := range allHeavy {
		for _, x := range drop {
			if d == x {
				continue outer
			}
		}
		out = append(out, d)
	}
	return out
}

// weightTable lists, per extension package, the dependencies registering it
// must not link. This is the point of the split: each row is a cost an
// embedder would otherwise pay for a library it never registered.
var weightTable = []struct {
	pkg       string
	forbidden []string
}{
	{"yaml", without(depYAML)},
	{"toml", without(depTOML)},
	{"markdown", without(depGoldmark)},
	{"template", allHeavy},
	{"logging", allHeavy},
	{"subprocess", without(depExec)},
	{"sys", allHeavy},
	{"platform", allHeavy},
	{"similarity", allHeavy},
	// The concrete providers (Vault, 1Password Connect) are in their own
	// packages, so the secret library and the registry link no HTTP client.
	{"secret", allHeavy},
	{"secretprovider", allHeavy},
	{"secretprovider/vault", without(depHTTP)},
	{"secretprovider/onepassword", without(depHTTP)},
	// netsecurity is imported by every network library: TOML policy-file
	// loading lives in netsecurity/policyfile so none of them link a parser.
	{"netsecurity", without(depHTTP)},
	{"filesystem", []string{depHTTP, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depExec, depYAML, depTOML, depRuntime}},
	{"requests", []string{depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depExec, depYAML, depTOML, depFilesystem, depRuntime}},
	{"net/websocket", []string{depSnapshotKV, depRuntime, depFilesystem, depGoldmark, depGopsutil, depExec, depYAML, depTOML}},
	// mcp links os/exec through github.com/paularlott/jsonrpc (stdio transport).
	{"mcp", []string{depRuntime, depFilesystem, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depYAML, depTOML}},
	{"net/multicast", []string{depRuntime, depFilesystem, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depExec, depYAML, depTOML}},
	{"net/unicast", []string{depRuntime, depFilesystem, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depExec, depYAML, depTOML}},
	{"net/gossip", []string{depRuntime, depFilesystem, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depExec, depYAML, depTOML}},
	{"mercure", []string{depRuntime, depFilesystem, depWebsocket, depSnapshotKV, depGoldmark, depGopsutil, depExec, depYAML, depTOML}},
	// wait_for.process_name uses gopsutil, which brings os/exec.
	{"waitfor", []string{depRuntime, depFilesystem, depWebsocket, depSnapshotKV, depGoldmark, depYAML, depTOML}},
	{"reqctx", allHeavy}, // support package imported by runtime and mcp
	// The runtime family: the core (background tasks, sync, sandbox, server
	// lifecycle, jsonrpc, mcp, plugin) links neither the KV store nor the
	// WebSocket server; kv and http each link only their own.
	{"runtime", append(without(depRuntime), depRuntimeKV, depRuntimeHTTP)},
	{"runtime/kv", without(depSnapshotKV)}, // depRuntime also bans the core and http
	{"runtime/http", append(without(depHTTP, depWebsocket, depRuntime), depRuntimeKV)},
	// memory links ai (net/http, os/exec) and the kv store, and nothing of the
	// runtime core or http (see exactForbidden for the core).
	{"ai/memory", []string{depWebsocket, depRuntimeHTTP, depFilesystem, depGoldmark, depGopsutil, depYAML, depTOML}},
}

// exactForbidden lists a package that must not link one exact package while
// it may link that package's subpackages, which weightTable's prefix match
// cannot express.
var exactForbidden = []struct {
	pkg, dep, why string
}{
	{"ai/memory", depRuntime, "memory needs only the kv store (extlibs/runtime/kv), not the runtime core's background tasks and server lifecycle"},
}

// TestWeightIsolationExact enforces exactForbidden.
func TestWeightIsolationExact(t *testing.T) {
	for _, row := range exactForbidden {
		t.Run(row.pkg, func(t *testing.T) {
			p := listOne(t, "./extlibs/"+row.pkg)
			if p.Error != nil {
				t.Fatalf("extlibs/%s does not build: %s", row.pkg, p.Error.Err)
			}
			if len(p.Deps) == 0 {
				t.Fatalf("go list reported no dependencies for extlibs/%s; the check would pass vacuously", row.pkg)
			}
			for _, d := range p.Deps {
				if d == row.dep {
					t.Errorf("registering extlibs/%s links %s: %s", row.pkg, row.dep, row.why)
				}
			}
		})
	}
}

// TestWeightIsolation enforces that each extension package links only its
// own heavy dependencies.
func TestWeightIsolation(t *testing.T) {
	patterns := make([]string, len(weightTable))
	for i, row := range weightTable {
		patterns[i] = "./extlibs/" + row.pkg
	}
	byPath := map[string]listedPackage{}
	for _, p := range listPackages(t, patterns...) {
		byPath[p.ImportPath] = p
	}
	for _, row := range weightTable {
		t.Run(row.pkg, func(t *testing.T) {
			p, ok := byPath[extlibsPrefix+row.pkg]
			if !ok {
				t.Fatalf("extlibs/%s not found by go list; update weightTable if the package moved", row.pkg)
			}
			if p.Error != nil {
				t.Fatalf("extlibs/%s does not build, so its dependencies cannot be checked: %s", row.pkg, p.Error.Err)
			}
			if len(p.Deps) == 0 {
				t.Fatalf("go list reported no dependencies for extlibs/%s; the check would pass vacuously", row.pkg)
			}
			if bad := matchingDeps(p.Deps, row.forbidden); len(bad) > 0 {
				sort.Strings(bad)
				t.Errorf("registering extlibs/%s links %v, which it must not.\n"+
					"Find the import chain with `go list -deps -f '{{.ImportPath}}: {{.Imports}}' ./extlibs/%s` and move the "+
					"heavy code into its own extension package so only embedders that register it pay for it.",
					row.pkg, bad, row.pkg)
			}
		})
	}
}
