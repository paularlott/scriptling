package extlibs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

const bombSize = 8 << 20 // 8 MiB of zeros: compresses to a few KB

func writeZip(t *testing.T, path string, members map[string]int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, size := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(bytes.Repeat([]byte{'z'}, size))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func writeTarGz(t *testing.T, path string, members map[string]int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, size := range members {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(size)})
		tw.Write(bytes.Repeat([]byte{'t'}, size))
	}
	tw.Close()
	gz.Close()
	f.Close()
}

func archiveInterp(t *testing.T, limit int64) *scriptling.Scriptling {
	t.Helper()
	p := limitsNewInterp(t, limit)
	RegisterZipfileLibrary(p, nil)
	RegisterTarfileLibrary(p, nil)
	return p
}

func dirSize(t *testing.T, dir string) int64 {
	t.Helper()
	var total int64
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// A small archive extracted with no bound, so a few KB expanded to megabytes
// on disk even under a value limit that refused read() of the same member
// (CWE-409, strix 4250 vuln-0011). Each member is now held to the value
// limit, like read(): an oversized member is a MemoryError and nothing of it
// is left on disk.
func TestArchiveExtractHeldToValueLimit(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bomb.zip")
	tgzPath := filepath.Join(dir, "bomb.tar.gz")
	writeZip(t, zipPath, map[string]int{"big.bin": bombSize})
	writeTarGz(t, tgzPath, map[string]int{"big.bin": bombSize})

	for name, tc := range map[string]struct{ setup, call string }{
		"zip extractall": {`import zipfile` + "\n" + `a = zipfile.ZipFile("` + zipPath + `")`, `a.extractall("OUT")`},
		"zip extract":    {`import zipfile` + "\n" + `a = zipfile.ZipFile("` + zipPath + `")`, `a.extract("big.bin", "OUT")`},
		"tgz extractall": {`import tarfile` + "\n" + `a = tarfile.TarFile("` + tgzPath + `", "r:gz")`, `a.extractall("OUT")`},
		"tgz extract":    {`import tarfile` + "\n" + `a = tarfile.TarFile("` + tgzPath + `", "r:gz")`, `a.extract("big.bin", "OUT")`},
	} {
		out := t.TempDir()
		got := limitsCatch(t, archiveInterp(t, 1<<20), tc.setup, strings.ReplaceAll(tc.call, "OUT", out))
		limitsExpectMemoryError(t, got, "archive entry too large")
		if n := dirSize(t, out); n != 0 {
			t.Errorf("%s: %d bytes left on disk", name, n)
		}
	}
}

// Members within the limit extract exactly as before, including one exactly
// at the limit; with no limit, a large member extracts whole.
func TestArchiveExtractWithinLimit(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "ok.zip")
	tgzPath := filepath.Join(dir, "ok.tar.gz")
	members := map[string]int{"a.txt": 10, "d/b.txt": 1000, "edge.bin": 1 << 20}
	writeZip(t, zipPath, members)
	writeTarGz(t, tgzPath, members)

	for _, setup := range []string{
		`import zipfile` + "\n" + `a = zipfile.ZipFile("` + zipPath + `")`,
		`import tarfile` + "\n" + `a = tarfile.TarFile("` + tgzPath + `", "r:gz")`,
	} {
		out := t.TempDir()
		limitsExpectOK(t, limitsCatch(t, archiveInterp(t, 1<<20), setup, `len(a.extractall("`+out+`"))`), "3")
		for name, size := range members {
			info, err := os.Stat(filepath.Join(out, name))
			if err != nil || info.Size() != int64(size) {
				t.Errorf("%s: %v %v", name, info, err)
			}
		}
	}

	bigZip := filepath.Join(dir, "big.zip")
	writeZip(t, bigZip, map[string]int{"big.bin": bombSize})
	out := t.TempDir()
	limitsExpectOK(t, limitsCatch(t, archiveInterp(t, 0), `import zipfile`+"\n"+`a = zipfile.ZipFile("`+bigZip+`")`, `len(a.extractall("`+out+`"))`), "1")
	if n := dirSize(t, out); n != bombSize {
		t.Fatalf("unlimited extraction wrote %d bytes, want %d", n, bombSize)
	}
}

// A zip that declares a small size but holds more is refused (Go's zip
// reader rejects data past the declared size) and leaves nothing behind.
func TestArchiveExtractLyingSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "liar.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	var comp bytes.Buffer
	fw, _ := flate.NewWriter(&comp, flate.BestCompression)
	fw.Write(make([]byte, bombSize))
	fw.Close()
	w, err := zw.CreateRaw(&zip.FileHeader{Name: "liar.bin", Method: zip.Deflate,
		CompressedSize64: uint64(comp.Len()), UncompressedSize64: 16})
	if err != nil {
		t.Fatal(err)
	}
	w.Write(comp.Bytes())
	zw.Close()
	f.Close()

	for _, limit := range []int64{1 << 20, 0} {
		out := t.TempDir()
		_, err := archiveInterp(t, limit).Eval(`import zipfile` + "\n" + `zipfile.ZipFile("` + path + `").extractall("` + out + `")`)
		if err == nil {
			t.Errorf("limit %d: expected the lying member to be refused", limit)
		}
		if n := dirSize(t, out); n != 0 {
			t.Errorf("limit %d: %d bytes left on disk", limit, n)
		}
	}
}
