package runtime_test

import (
	"archive/tar"
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/filesystem"
)

// Archives a script leaves open are closed, and so finished, when the
// interpreter is closed: a zip gets its directory, a tar its trailer.
func TestArchivesClosedWithInterpreter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "member.txt"), []byte("in the tar"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := scriptling.New()
	filesystem.RegisterZipfile(p, []string{dir})
	filesystem.RegisterTarfile(p, []string{dir})
	if _, err := p.Eval(`
import zipfile
import tarfile
z = zipfile.ZipFile("` + filepath.Join(dir, "left.zip") + `", "w")
z.writestr("member.txt", "in the zip")
t = tarfile.TarFile("` + filepath.Join(dir, "left.tar") + `", "w")
t.add("` + filepath.Join(dir, "member.txt") + `", "member.txt")
`); err != nil {
		t.Fatal(err)
	}
	p.Close()

	zr, err := zip.OpenReader(filepath.Join(dir, "left.zip"))
	if err != nil {
		t.Fatalf("the zip was not finished: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 1 || zr.File[0].Name != "member.txt" {
		t.Fatalf("zip members: %v", zr.File)
	}

	f, err := os.Open(filepath.Join(dir, "left.tar"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tr := tar.NewReader(f)
	h, err := tr.Next()
	if err != nil || h.Name != "member.txt" {
		t.Fatalf("tar member: %v %v", h, err)
	}
	if b, _ := io.ReadAll(tr); string(b) != "in the tar" {
		t.Fatalf("tar data: %q", b)
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("the tar was not finished: %v", err)
	}
}
