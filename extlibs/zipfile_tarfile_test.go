package extlibs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
)

// ===================== zipfile =====================

func createTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(content))
	}
	w.Close()
}

func TestZipfileReadAndExtract(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		"hello.txt":         "hello world",
		"sub/deep/file.txt": "deep content",
	})

	p := scriptling.New()
	RegisterZipfileLibrary(p, nil)

	// namelist + read
	result, err := p.Eval(`import zipfile
zf = zipfile.ZipFile("` + zipPath + `")
names = zf.namelist()
content = zf.read("hello.txt")
zf.close()
str(len(names)) + ":" + content`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != "2:hello world" {
		t.Errorf("got %q", s)
	}

	// extractall
	extractDir := filepath.Join(dir, "extracted")
	result, err = p.Eval(`import zipfile
zf = zipfile.ZipFile("` + zipPath + `")
paths = zf.extractall("` + extractDir + `")
zf.close()
str(len(paths))`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = result.AsString()
	if s != "2" {
		t.Errorf("extractall: expected 2 files, got %s", s)
	}
	data, _ := os.ReadFile(filepath.Join(extractDir, "sub", "deep", "file.txt"))
	if string(data) != "deep content" {
		t.Errorf("extracted content mismatch: %q", string(data))
	}
}

func TestZipfileWrite(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "data.txt")
	os.WriteFile(srcFile, []byte("file content"), 0644)
	zipPath := filepath.Join(dir, "output.zip")

	p := scriptling.New()
	RegisterZipfileLibrary(p, nil)

	_, err := p.Eval(`import zipfile
zf = zipfile.ZipFile("` + zipPath + `", "w")
zf.write("` + srcFile + `", "stored.txt")
zf.writestr("inline.txt", "inline data")
zf.close()`)
	if err != nil {
		t.Fatal(err)
	}

	// Verify by reopening
	zf, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zf.Close()
	if len(zf.File) != 2 {
		t.Errorf("expected 2 entries, got %d", len(zf.File))
	}
	for _, f := range zf.File {
		rc, _ := f.Open()
		data, _ := readAll(rc)
		rc.Close()
		switch f.Name {
		case "stored.txt":
			if string(data) != "file content" {
				t.Errorf("stored.txt: %q", data)
			}
		case "inline.txt":
			if string(data) != "inline data" {
				t.Errorf("inline.txt: %q", data)
			}
		}
	}
}

func TestZipfileIsZipfile(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "real.zip")
	txtPath := filepath.Join(dir, "not.txt")
	createTestZip(t, zipPath, map[string]string{"a": "b"})
	os.WriteFile(txtPath, []byte("not a zip"), 0644)

	p := scriptling.New()
	RegisterZipfileLibrary(p, nil)

	result, err := p.Eval(`import zipfile
str(zipfile.is_zipfile("` + zipPath + `")) + ":" + str(zipfile.is_zipfile("` + txtPath + `"))`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != "true:false" {
		t.Errorf("is_zipfile: got %q", s)
	}
}

func TestZipfileSecurity(t *testing.T) {
	allowed := t.TempDir()
	denied := t.TempDir()
	zipPath := filepath.Join(denied, "secret.zip")
	createTestZip(t, zipPath, map[string]string{"a": "b"})

	p := scriptling.New()
	RegisterZipfileLibrary(p, []string{allowed})

	_, err := p.Eval(`import zipfile
zipfile.ZipFile("` + zipPath + `")`)
	if err == nil {
		t.Error("expected permission error opening zip outside allowed paths")
	}
}

// ===================== tarfile =====================

func createTestTar(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := tar.NewWriter(f)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		w.WriteHeader(hdr)
		w.Write([]byte(content))
	}
	w.Close()
}

func createTestTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	w := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		w.WriteHeader(hdr)
		w.Write([]byte(content))
	}
	w.Close()
}

func TestTarfileReadAndExtract(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "test.tar")
	createTestTar(t, tarPath, map[string]string{
		"hello.txt":    "hello tar",
		"sub/deep.txt": "deep tar",
	})

	p := scriptling.New()
	RegisterTarfileLibrary(p, nil)

	result, err := p.Eval(`import tarfile
tf = tarfile.TarFile("` + tarPath + `")
names = tf.getnames()
content = tf.read("hello.txt")
tf.close()
str(len(names)) + ":" + content`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != "2:hello tar" {
		t.Errorf("got %q", s)
	}

	// extractall
	extractDir := filepath.Join(dir, "extracted")
	result, err = p.Eval(`import tarfile
tf = tarfile.TarFile("` + tarPath + `")
paths = tf.extractall("` + extractDir + `")
tf.close()
str(len(paths))`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ = result.AsString()
	if s != "2" {
		t.Errorf("extractall: expected 2, got %s", s)
	}
	data, _ := os.ReadFile(filepath.Join(extractDir, "sub", "deep.txt"))
	if string(data) != "deep tar" {
		t.Errorf("extracted: %q", data)
	}
}

func TestTarfileGzRead(t *testing.T) {
	dir := t.TempDir()
	tarGzPath := filepath.Join(dir, "test.tar.gz")
	createTestTarGz(t, tarGzPath, map[string]string{
		"compressed.txt": "gz content",
	})

	p := scriptling.New()
	RegisterTarfileLibrary(p, nil)

	result, err := p.Eval(`import tarfile
tf = tarfile.TarFile("` + tarGzPath + `", "r:gz")
content = tf.read("compressed.txt")
tf.close()
content`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != "gz content" {
		t.Errorf("got %q", s)
	}
}

func TestTarfileWrite(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "data.txt")
	os.WriteFile(srcFile, []byte("tar file content"), 0644)
	tarPath := filepath.Join(dir, "output.tar")

	p := scriptling.New()
	RegisterTarfileLibrary(p, nil)

	_, err := p.Eval(`import tarfile
tf = tarfile.TarFile("` + tarPath + `", "w")
tf.add("` + srcFile + `", "stored.txt")
tf.addstr("inline.txt", "inline tar data")
tf.close()`)
	if err != nil {
		t.Fatal(err)
	}

	// Verify
	f, _ := os.Open(tarPath)
	defer f.Close()
	tr := tar.NewReader(f)
	count := 0
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		count++
		data := make([]byte, hdr.Size)
		tr.Read(data)
		if hdr.Name == "inline.txt" && string(data) != "inline tar data" {
			t.Errorf("inline.txt: %q", data)
		}
	}
	if count != 2 {
		t.Errorf("expected 2 entries, got %d", count)
	}
}

func TestTarfileWriteGz(t *testing.T) {
	dir := t.TempDir()
	tarGzPath := filepath.Join(dir, "output.tar.gz")

	p := scriptling.New()
	RegisterTarfileLibrary(p, nil)

	_, err := p.Eval(`import tarfile
tf = tarfile.TarFile("` + tarGzPath + `", "w:gz")
tf.addstr("file.txt", "gzipped content")
tf.close()`)
	if err != nil {
		t.Fatal(err)
	}

	// Verify by reading back with r:gz
	result, err := p.Eval(`import tarfile
tf = tarfile.TarFile("` + tarGzPath + `", "r:gz")
c = tf.read("file.txt")
tf.close()
c`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := result.AsString()
	if s != "gzipped content" {
		t.Errorf("got %q", s)
	}
}

func TestTarfileSecurity(t *testing.T) {
	allowed := t.TempDir()
	denied := t.TempDir()
	tarPath := filepath.Join(denied, "secret.tar")
	createTestTar(t, tarPath, map[string]string{"a": "b"})

	p := scriptling.New()
	RegisterTarfileLibrary(p, []string{allowed})

	_, err := p.Eval(`import tarfile
tarfile.TarFile("` + tarPath + `")`)
	if err == nil {
		t.Error("expected permission error opening tar outside allowed paths")
	}
}

// readAll is a minimal io.ReadAll replacement to avoid importing io in tests.
func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf []byte
	chunk := make([]byte, 4096)
	for {
		n, err := r.Read(chunk)
		buf = append(buf, chunk[:n]...)
		if err != nil {
			return buf, nil
		}
	}
}
