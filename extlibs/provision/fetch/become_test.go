package fetch

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/provision/become"
)

func TestMain(m *testing.M) {
	become.ServeIfHelper()
	os.Exit(m.Run())
}

func helper(t *testing.T) *become.Become {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	p := filepath.Join(t.TempDir(), "sudo")
	script := "#!/bin/sh\nwhile [ \"$1\" = -n ] || [ \"$1\" = -- ]; do shift; done\nexec \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	b := become.New(become.Config{Sudo: []string{p}, Command: []string{os.Args[0], become.HelperArg}, NonRootHelper: true})
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func TestBecomeFetch(t *testing.T) {
	var zipped bytes.Buffer
	zw := zip.NewWriter(&zipped)
	w, _ := zw.Create("sub/a.txt")
	_, _ = w.Write([]byte("A"))
	_ = zw.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/z.zip" {
			_, _ = w.Write(zipped.Bytes())
			return
		}
		_, _ = w.Write([]byte("tool"))
	}))
	defer server.Close()

	dir := t.TempDir()
	me, _ := user.Current()
	p := scriptling.New()
	RegisterConfigured(p, fssecurity.Config{}, nil, WithBecome(helper(t)))
	v, err := p.Eval(strings.NewReplacer("D", dir, "U", server.URL, "ME", me.Username).Replace(`
import scriptling.provision.fetch as fetch
r = []
r.append(fetch.file("U/tool", "D/bin/tool", mode=0o755, become=True)["status"])
r.append(fetch.file("U/tool", "D/bin/tool", mode=0o755, owner="ME", become=True)["status"])
r.append(fetch.file("U/tool", "D/bin/tool", provides=["D/bin/tool"], become=True)["status"])
z = fetch.file("U/z.zip", "D/site", unpack_zip=True, owner="ME", become=True)
r.append(z["status"])
r.append(len(z["files"]))
r
`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v.Inspect(), "['created', 'unchanged', 'unchanged', 'created', 1]"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if info, err := os.Stat(filepath.Join(dir, "bin", "tool")); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("tool: %v %v", info, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "site", "sub", "a.txt")); string(b) != "A" {
		t.Errorf("unpacked %q", b)
	}
}
