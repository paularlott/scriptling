package extlibs

import (
	"archive/tar"
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// buildZipSlipArchive writes a zip containing entries that escape via ../
// plus one safe entry.
func buildZipSlipArchive(t *testing.T, path string) {
	t.Helper()
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	w := zip.NewWriter(fh)
	entries := []struct{ name, content string }{
		{"../slipped.txt", "escaped"},
		{"safe.txt", "inside"},
		{"../../deeper-slip.txt", "escaped2"},
	}
	for _, e := range entries {
		entry, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(e.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// buildTarSlipArchive writes a tar containing ../slipped.txt and safe.txt.
func buildTarSlipArchive(t *testing.T, path string) {
	t.Helper()
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	w := tar.NewWriter(fh)
	for name, content := range map[string]string{
		"../slipped.txt":        "escaped",
		"safe.txt":              "inside",
		"sub/../../double.html": "escaped2",
	} {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content))}
		if err := w.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPathRestrictionTarSlip(t *testing.T) {
	f := newFSFixture(t)
	tarPath := filepath.Join(f.allowed, "evil.tar")
	buildTarSlipArchive(t, tarPath)

	p := newAllFSInterpreter(t, []string{f.allowed})
	out, ok := evalFS(t, p, `import tarfile
t = tarfile.open("`+tarPath+`")
t.extractall("`+f.allowed+`")
t.close()
"extracted"`)
	if !ok {
		t.Logf("extractall errored: %s", out)
	}
	if _, err := os.Stat(filepath.Join(f.outside, "slipped.txt")); err == nil {
		t.Fatal("TAR-SLIP: extraction wrote outside the allowed directory")
	}
}
