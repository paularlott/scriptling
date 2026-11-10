package extlibs

import (
	"archive/tar"
	"archive/zip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

// A background task closes what it opened and left open when its function
// returns; what the script that started it opened stays open until that
// interpreter closes it.
func TestBackgroundTaskClosesItsResources(t *testing.T) {
	ResetRuntime()
	p := scriptling.New()
	RegisterRuntimeLibraryAll(p, nil)
	closed := make(chan string, 4)
	lib := object.NewLibraryBuilder("res", "test resources")
	lib.Function("open", func(ctx context.Context, name string) object.Object {
		evaluator.TrackResource(ctx, nil, func() { closed <- name })
		return &object.Null{}
	})
	p.RegisterLibrary(lib.Build())

	if _, err := p.Eval(`
import scriptling.runtime as runtime
import res
res.open("caller")

def task():
    import res
    res.open("task")
    return 1

runtime.background("t1", "task").get()
`); err != nil {
		t.Fatal(err)
	}
	select {
	case name := <-closed:
		if name != "task" {
			t.Fatalf("closed %q when the task returned", name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the task's resource is still open after it returned")
	}
	select {
	case name := <-closed:
		t.Fatalf("closed %q before the interpreter was closed", name)
	default:
	}
	p.Close()
	if name := <-closed; name != "caller" {
		t.Fatalf("closed %q", name)
	}
}

// Archives a script leaves open are closed, and so finished, when the
// interpreter is closed: a zip gets its directory, a tar its trailer.
func TestArchivesClosedWithInterpreter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "member.txt"), []byte("in the tar"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := scriptling.New()
	RegisterZipfileLibrary(p, []string{dir})
	RegisterTarfileLibrary(p, []string{dir})
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

// A WebSocket connection a script leaves open is closed when the interpreter
// is closed.
func TestWebSocketClosedWithInterpreter(t *testing.T) {
	ended := make(chan struct{})
	up := websocket.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(ended)
				return
			}
		}
	}))
	defer ts.Close()

	p := scriptling.New()
	RegisterWebSocketLibrary(p)
	if _, err := p.Eval(`
import scriptling.net.websocket as ws
c = ws.connect("ws` + ts.URL[4:] + `")
`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ended:
		t.Fatal("the connection ended while the script still held it")
	case <-time.After(100 * time.Millisecond):
	}
	p.Close()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the connection is still open after the interpreter closed")
	}
}
