package libboundary

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Symbol substrings that mark heavy code in a linked binary.
const (
	symWebsocket   = "github.com/gorilla/websocket."
	symGoldmark    = "github.com/yuin/goldmark"
	symGopsutil    = "github.com/shirou/gopsutil"
	symSnapshotKV  = "github.com/paularlott/snapshotkv."
	symExecCmd     = "os/exec.(*Cmd)"
	symHTTPClient  = "net/http.(*Client)"
	symHTTPPackage = "net/http."           // any net/http code at all
	symTLSPackage  = "crypto/tls."         // any TLS code at all
	symYAML        = "gopkg.in/yaml%2ev3." // the linker escapes the dot in a final path element
)

// embedders are small programs an application might build. Each must link
// none of its forbidden symbols; required symbols prove the check sees the
// libraries that were registered (so an empty symbol table cannot pass).
var embedders = []struct {
	name      string
	imports   []string
	decls     string // top-level declarations, e.g. a stub type
	register  string
	forbidden []string
	required  []string
}{
	{
		name:      "core+stdlib",
		forbidden: []string{symWebsocket, symGoldmark, symGopsutil, symSnapshotKV, symExecCmd, symHTTPClient, symYAML},
		required:  []string{"github.com/paularlott/scriptling/stdlib."},
	},
	{
		name:      "core+stdlib+yaml",
		imports:   []string{modulePath + "/extlibs/yaml"},
		register:  "yaml.Register(p)",
		forbidden: []string{symWebsocket, symGoldmark, symGopsutil, symSnapshotKV, symExecCmd, symHTTPClient},
		required:  []string{symYAML},
	},
	{
		name:      "core+stdlib+filesystem",
		imports:   []string{modulePath + "/extlibs/filesystem"},
		register:  "filesystem.RegisterOS(p, nil)",
		forbidden: []string{symWebsocket, symGoldmark, symGopsutil, symSnapshotKV, symExecCmd, symHTTPClient, symYAML},
		required:  []string{modulePath + "/extlibs/filesystem."},
	},
	{
		name:      "core+stdlib+mercure",
		imports:   []string{modulePath + "/extlibs/mercure"},
		register:  "mercure.Register(p)",
		forbidden: []string{symWebsocket, symGoldmark, symGopsutil, symSnapshotKV, symExecCmd, symYAML},
		required:  []string{modulePath + "/extlibs/mercure.", symHTTPClient},
	},
	{
		// scriptling.secret with an application-supplied provider: the
		// Vault and 1Password Connect HTTP providers are separate packages,
		// so nothing HTTP is linked unless the embedder registers one.
		name:    "core+stdlib+secret(stub provider)",
		imports: []string{"context", modulePath + "/extlibs/secret", modulePath + "/extlibs/secretprovider"},
		decls: "type stubProvider struct{}\n\n" +
			"func (stubProvider) ID() string { return \"stub\" }\n" +
			"func (stubProvider) Resolve(context.Context, string, string) (string, error) { return \"v\", nil }\n" +
			"func (stubProvider) List(context.Context, string) ([]string, error) { return nil, nil }\n",
		register: "registry := secretprovider.NewRegistry()\n" +
			"\tif err := registry.Register(stubProvider{}, \"stub\", 0); err != nil {\n\t\tpanic(err)\n\t}\n" +
			"\tsecret.Register(p, registry)",
		forbidden: []string{symHTTPPackage, symTLSPackage, symWebsocket, symGoldmark, symGopsutil, symSnapshotKV, symExecCmd, symYAML},
		required:  []string{modulePath + "/extlibs/secret.", modulePath + "/extlibs/secretprovider."},
	},
	{
		// The runtime core (background tasks, sync, sandbox): no KV store,
		// no WebSocket server.
		name:      "core+stdlib+runtime(core,sync,sandbox)",
		imports:   []string{modulePath + "/extlibs/runtime"},
		register:  "runtime.Register(p)\n\truntime.RegisterSync(p)\n\truntime.RegisterSandbox(p, nil)",
		forbidden: []string{symHTTPPackage, symWebsocket, symSnapshotKV, symGoldmark, symGopsutil, symExecCmd, symYAML, modulePath + "/extlibs/runtime/"},
		required:  []string{modulePath + "/extlibs/runtime."},
	},
	{
		// The KV store alone: links snapshotkv, not the runtime core or the
		// WebSocket server.
		name:      "core+stdlib+runtime/kv",
		imports:   []string{modulePath + "/extlibs/runtime/kv"},
		register:  "kv.Register(p, nil)",
		forbidden: []string{symWebsocket, symHTTPClient, symGoldmark, symGopsutil, symExecCmd, symYAML, modulePath + "/extlibs/runtime.", modulePath + "/extlibs/runtime/http."},
		required:  []string{symSnapshotKV, modulePath + "/extlibs/runtime/kv."},
	},
}

var goDirective = regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)

// TestEmbedderLinkWeight builds tiny embedding programs against this
// checkout and inspects their symbol tables: the end-to-end proof that an
// application links only the libraries it registers.
func TestEmbedderLinkWeight(t *testing.T) {
	if testing.Short() {
		t.Skip("builds several binaries; run without -short")
	}
	root := moduleRoot(t)
	goTool(t)

	rootMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	m := goDirective.FindSubmatch(rootMod)
	if m == nil {
		t.Fatal("no go directive in the module's go.mod")
	}
	goVersion := string(m[1])
	sum, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	// Build offline against the module cache the repo already uses.
	env := []string{"GOFLAGS=-mod=mod", "GOPROXY=off", "CGO_ENABLED=0"}

	for _, e := range embedders {
		t.Run(e.name, func(t *testing.T) {
			dir := t.TempDir()
			gomod := fmt.Sprintf("module example.com/embedder\n\ngo %s\n\nrequire %s v0.0.0\n\nreplace %s => %s\n",
				goVersion, modulePath, modulePath, root)
			writeFile(t, filepath.Join(dir, "go.mod"), gomod)
			writeFile(t, filepath.Join(dir, "go.sum"), string(sum))

			var src strings.Builder
			src.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\n")
			src.WriteString("\t\"" + modulePath + "\"\n\t\"" + modulePath + "/stdlib\"\n")
			for _, imp := range e.imports {
				src.WriteString("\t\"" + imp + "\"\n")
			}
			src.WriteString(")\n\n")
			if e.decls != "" {
				src.WriteString(e.decls + "\n")
			}
			src.WriteString("func main() {\n\tp := scriptling.New()\n\tstdlib.RegisterAll(p)\n")
			if e.register != "" {
				src.WriteString("\t" + e.register + "\n")
			}
			src.WriteString("\tif len(os.Args) > 1 {\n\t\tif _, err := p.Eval(os.Args[1]); err != nil {\n\t\t\tfmt.Println(err)\n\t\t\tos.Exit(1)\n\t\t}\n\t}\n}\n")
			writeFile(t, filepath.Join(dir, "main.go"), src.String())

			bin := filepath.Join(dir, "embedder")
			runGo(t, dir, env, "build", "-o", bin, ".")
			nm := runGo(t, dir, env, "tool", "nm", bin)

			if hits := symbolHits(nm, e.forbidden); len(hits) > 0 {
				t.Errorf("an embedder registering only %s links %v.\n"+
					"Some package it imports pulls that code in. TestWeightIsolation and TestLeafRule usually point at "+
					"the import; otherwise trace it with `go list -deps` on the embedder.", e.name, hits)
			}
			for _, req := range e.required {
				if len(symbolHits(nm, []string{req})) == 0 {
					t.Errorf("expected the %s binary to contain symbols matching %q; the symbol check is not "+
						"seeing what was linked (stripped binary or renamed package?), so its other results cannot be trusted", e.name, req)
				}
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
