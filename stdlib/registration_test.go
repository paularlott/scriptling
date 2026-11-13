package stdlib_test

import (
	"slices"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/internal/scripttest"
	"github.com/paularlott/scriptling/stdlib"
)

// extensionNames are script names that belong to extension packages under
// extlibs/. The standard tier must never provide them: each needs host,
// filesystem, network or process access, or a third-party module, and is
// linked only when an embedder registers it.
var extensionNames = []string{
	// extlibs/filesystem
	"os", "os.path", "pathlib", "glob", "fs", "shutil", "tempfile", "zipfile", "tarfile",
	"scriptling.grep", "scriptling.find", "scriptling.sed",
	// single-library extension packages
	"subprocess", "sys", "requests", "platform", "yaml", "toml", "logging",
	"scriptling.template.html", "scriptling.template.text", "scriptling.markdown",
	"scriptling.wait_for", "scriptling.secret", "scriptling.net.websocket",
	// extlibs/runtime
	"scriptling.runtime", "scriptling.runtime.http", "scriptling.runtime.kv",
	"scriptling.runtime.sync", "scriptling.runtime.sandbox", "scriptling.runtime.jsonrpc",
	"scriptling.runtime.mcp", "scriptling.runtime.plugin",
}

// newStdlibOnly returns an interpreter with only the standard tier.
func newStdlibOnly() *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	return p
}

func TestLibraryNamesContents(t *testing.T) {
	names := stdlib.LibraryNames()
	mustHave := []string{
		stdlib.CSVLibraryName, stdlib.CSVAliasLibraryName, // scriptling.csv, csv
		stdlib.XMLLibraryName, stdlib.XMLAliasLibraryName, // scriptling.xml, xml
		stdlib.HTMLParserLibraryName, stdlib.ShlexLibraryName, stdlib.SecretsLibraryName,
		stdlib.JSONLibraryName, stdlib.URLParseLibraryName,
	}
	for _, n := range mustHave {
		if !slices.Contains(names, n) {
			t.Errorf("stdlib.LibraryNames() lacks %q; add the library to stdlib.RegisterAll", n)
		}
	}
	// Pin the literal script names too, so renaming a constant cannot
	// silently change what scripts import.
	for _, n := range []string{"csv", "scriptling.csv", "xml", "scriptling.xml", "html.parser", "shlex", "secrets"} {
		if !slices.Contains(names, n) {
			t.Errorf("stdlib.LibraryNames() lacks script name %q", n)
		}
	}
	for _, n := range extensionNames {
		if slices.Contains(names, n) {
			t.Errorf("stdlib.LibraryNames() includes %q, an extension library; it must be registered by its extlibs package, not stdlib.RegisterAll", n)
		}
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Errorf("stdlib.RegisterAll registers %q more than once", n)
		}
		seen[n] = true
	}
}

func TestStdlibImportsEveryListedName(t *testing.T) {
	for _, name := range stdlib.LibraryNames() {
		t.Run(name, func(t *testing.T) {
			scripttest.Imports(t, newStdlibOnly(), name)
		})
	}
}

func TestStdlibRepresentativeCalls(t *testing.T) {
	p := newStdlibOnly()
	tests := []struct {
		name, src, want string
	}{
		{"csv", "import csv\ncsv.loads('a,b\\n')[0][1]", "b"},
		{"scriptling.csv", "import scriptling.csv as c\nc.loads('x,y\\n')[0][0]", "x"},
		{"xml", "import xml\nxml.loads('<a><b>t</b></a>')['a']['b']", "t"},
		{"scriptling.xml", "import scriptling.xml as x\nx.dumps(x.loads('<a><b>t</b></a>'))", "<a><b>t</b></a>"},
		{"html.parser", "import html.parser\n'HTMLParser' in dir(html.parser)", "True"},
		{"shlex", "import shlex\nshlex.split('a \"b c\"')[1]", "b c"},
		{"secrets", "import secrets\nstr(len(secrets.token_hex(8)))", "16"},
		{"json", "import json\njson.dumps({'a': 1})", `{"a": 1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := scripttest.Eval(t, p, tt.src)
			got := res.Inspect()
			if s, err := res.AsString(); err == nil {
				got = s
			}
			if got != tt.want {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestStdlibCannotImportExtensionLibraries(t *testing.T) {
	for _, name := range extensionNames {
		t.Run(name, func(t *testing.T) {
			scripttest.ImportFails(t, newStdlibOnly(), name,
				"the standard tier must not provide extension libraries (stdlib.RegisterAll registered it, or core does)")
		})
	}
}

func TestBareInterpreterHasNoLibraries(t *testing.T) {
	// Without RegisterAll even standard libraries are unavailable.
	for _, name := range []string{"json", "csv", "shlex", "secrets"} {
		scripttest.ImportFails(t, scriptling.New(), name, "register the standard tier with stdlib.RegisterAll(p)")
	}
}

func TestRegisterAllTwice(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	stdlib.RegisterAll(p)
	if got := scripttest.EvalString(t, p, "import json\njson.dumps([1])"); got != "[1]" {
		t.Errorf("after registering twice: json.dumps([1]) = %q", got)
	}

	// Registering again after a library is imported keeps the bound module
	// working and re-imports cleanly.
	stdlib.RegisterAll(p)
	if got := scripttest.EvalString(t, p, "import json\njson.dumps([2])"); got != "[2]" {
		t.Errorf("after re-registering an imported library: json.dumps([2]) = %q", got)
	}
}

func TestLibrariesBindOnlyOnImport(t *testing.T) {
	p := newStdlibOnly()
	if !p.HasLibrary("shlex") {
		t.Fatal("HasLibrary(shlex) = false after stdlib.RegisterAll")
	}
	// Registration makes a library importable; it is not bound into the
	// script's namespace until imported.
	typ, _ := scripttest.Raises(t, p, "shlex.split('a b')")
	if typ != "NameError" {
		t.Errorf("using shlex before import raised %s, want NameError", typ)
	}
	if n := scripttest.EvalInt(t, p, "import shlex\nlen(shlex.split('a b'))"); n != 2 {
		t.Errorf("shlex.split after import: len = %d, want 2", n)
	}
}
