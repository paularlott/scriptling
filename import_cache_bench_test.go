package scriptling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/libloader"
	"github.com/paularlott/scriptling/stdlib"
)

// benchLibSource builds a realistic script library: module constants, a
// class and nFuncs small functions.
func benchLibSource(nFuncs int) string {
	var sb strings.Builder
	sb.WriteString("\"\"\"Benchmark library.\"\"\"\n")
	sb.WriteString("VERSION = \"1.0\"\nLIMITS = {\"a\": 1, \"b\": 2, \"c\": [1, 2, 3]}\n\n")
	sb.WriteString("class Widget:\n    def __init__(self, n):\n        self.n = n\n    def double(self):\n        return self.n * 2\n\n")
	for i := 0; i < nFuncs; i++ {
		fmt.Fprintf(&sb, "def f%d(x, y=1):\n    total = 0\n    for i in range(x):\n        if i %% 2 == 0:\n            total += i * y\n        else:\n            total -= y\n    return total + %d\n\n", i, i)
	}
	return sb.String()
}

func benchLibDir(b *testing.B, nFuncs int) string {
	dir := b.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "biglib.py"), []byte(benchLibSource(nFuncs)), 0o644); err != nil {
		b.Fatal(err)
	}
	// helper imports biglib, so a fresh instance loads two libraries.
	helper := "import biglib\n\ndef run(n):\n    return biglib.f0(n) + biglib.Widget(n).double()\n"
	if err := os.WriteFile(filepath.Join(dir, "helper.py"), []byte(helper), 0o644); err != nil {
		b.Fatal(err)
	}
	// Libraries on disk are normally old; backdate so the files are past any
	// recently-modified window.
	old := time.Now().Add(-time.Hour)
	for _, f := range []string{"biglib.py", "helper.py"} {
		if err := os.Chtimes(filepath.Join(dir, f), old, old); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}

const benchImportScript = "import helper\nresult = helper.run(5)\n"

func benchFresh(b *testing.B, setup func(p *Scriptling)) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New()
		setup(p)
		if _, err := p.Eval(benchImportScript); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFreshImport_Baseline(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New()
		if _, err := p.Eval("result = 5\n"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFreshImport_FS_20(b *testing.B) {
	loader := libloader.NewFilesystem(benchLibDir(b, 20))
	benchFresh(b, func(p *Scriptling) { p.SetLibraryLoader(loader) })
}

func BenchmarkFreshImport_FS_200(b *testing.B) {
	loader := libloader.NewFilesystem(benchLibDir(b, 200))
	benchFresh(b, func(p *Scriptling) { p.SetLibraryLoader(loader) })
}

func BenchmarkFreshImport_Registered_200(b *testing.B) {
	src := benchLibSource(200)
	helper := "import biglib\n\ndef run(n):\n    return biglib.f0(n) + biglib.Widget(n).double()\n"
	benchFresh(b, func(p *Scriptling) {
		p.RegisterScriptLibrary("biglib", src)
		p.RegisterScriptLibrary("helper", helper)
	})
}

func BenchmarkFreshImport_FSStdlib_200(b *testing.B) {
	loader := libloader.NewFilesystem(benchLibDir(b, 200))
	benchFresh(b, func(p *Scriptling) {
		stdlib.RegisterAll(p)
		p.SetLibraryLoader(loader)
	})
}

// BenchmarkFreshImport_Multi_200 mirrors the CLI: several library dirs with
// the libraries living in the last one, so earlier dirs miss every lookup.
func BenchmarkFreshImport_Multi_200(b *testing.B) {
	libDir := benchLibDir(b, 200)
	empty1, empty2 := b.TempDir(), b.TempDir()
	benchFresh(b, func(p *Scriptling) {
		p.SetLibraryLoader(libloader.NewMultiFilesystem(empty1, empty2, libDir))
	})
}
