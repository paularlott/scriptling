package container

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// createRecorder is a Docker API stub that records each container create
// request's HostConfig and answers create and start as the client expects.
type createRecorder struct {
	mu      sync.Mutex
	creates []dockerHostConfig
}

func (r *createRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch {
	case strings.Contains(req.URL.Path, "/containers/create"):
		var body dockerCreateRequest
		data, _ := io.ReadAll(req.Body)
		json.Unmarshal(data, &body)
		r.mu.Lock()
		r.creates = append(r.creates, body.HostConfig)
		r.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"Id":"deadbeefcafe"}`))
	case strings.HasSuffix(req.URL.Path, "/start"):
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (r *createRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.creates)
}

func (r *createRecorder) last() dockerHostConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.creates[len(r.creates)-1]
}

// runFixture serves the stub on the operator's (trusted) Docker socket and
// returns it with an allowed directory and a directory outside it.
func runFixture(t *testing.T) (rec *createRecorder, sock, allowed, outside string) {
	t.Helper()
	rec = &createRecorder{}
	dir := shortTempDir(t)
	sock = filepath.Join(dir, "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: rec}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	allowed, outside = shortTempDir(t), shortTempDir(t)
	return rec, sock, allowed, outside
}

func evalRun(t *testing.T, opts Options, call string) (string, error) {
	t.Helper()
	resetSockets(t)
	p := scriptling.New()
	RegisterWithOptions(p, opts)
	result, err := p.Eval("import scriptling.container as container\nc = container.Client(\"docker\")\n" + call)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

// run(volumes=...) handed any host path to the container runtime, so a
// script under --allowed-paths could mount /, /etc or a secret store into a
// container and read it (CWE-22, strix 4250 vuln-0015). A bind source outside
// the allowed directories is now refused before the runtime is asked, as an
// uncatchable PermissionError; named and anonymous volumes and bind sources
// inside the jail still work.
func TestRunVolumesHeldToAllowedPaths(t *testing.T) {
	rec, sock, allowed, outside := runFixture(t)
	link := filepath.Join(allowed, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	opts := Options{DockerSock: sock, Paths: fssecurity.Config{AllowedPaths: []string{allowed}}}

	for _, spec := range []string{
		outside + ":/mnt/data",
		outside + ":/mnt/data:ro",
		"/:/host",
		"/etc:/hostetc:ro",
		link + ":/mnt/data",
		allowed + "/../" + filepath.Base(outside) + ":/mnt",
		"type=bind,source=" + outside + ",target=/mnt/data",
		"type=bind,src=" + outside + ",dst=/mnt/data",
	} {
		_, err := evalRun(t, opts, `
try:
    c.run("alpine", volumes=["`+spec+`"])
    r = "ran"
except Exception as e:
    r = "caught"
r`)
		if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Errorf("volume %q: want an uncatchable denial, got %v", spec, err)
		}
	}
	if n := rec.count(); n != 0 {
		t.Fatalf("JAIL-BYPASS [container]: %d create requests reached the runtime", n)
	}

	for _, spec := range []string{allowed + ":/mnt/data:ro", "mydata:/data", "/scratch", "my-vol.1:/data:rw"} {
		out, err := evalRun(t, opts, `c.run("alpine", volumes=["`+spec+`"])`)
		if err != nil || out != "deadbeefcafe" {
			t.Errorf("volume %q: out=%q err=%v", spec, out, err)
			continue
		}
		if binds := rec.last().Binds; len(binds) != 1 || binds[0] != spec {
			t.Errorf("volume %q reached the runtime as %v", spec, binds)
		}
	}

	// Without a jail, host paths are mounted as before.
	if out, err := evalRun(t, Options{DockerSock: sock}, `c.run("alpine", volumes=["`+outside+`:/mnt"])`); err != nil || out != "deadbeefcafe" {
		t.Fatalf("unrestricted: out=%q err=%v", out, err)
	}
}

// Privileged containers are allowed by default, jail or not; DenyPrivileged
// refuses them.
func TestRunPrivileged(t *testing.T) {
	rec, sock, allowed, _ := runFixture(t)
	paths := fssecurity.Config{AllowedPaths: []string{allowed}}
	for _, opts := range []Options{{DockerSock: sock}, {DockerSock: sock, Paths: paths}} {
		out, err := evalRun(t, opts, `c.run("alpine", privileged=True)`)
		if err != nil || out != "deadbeefcafe" || !rec.last().Privileged {
			t.Fatalf("privileged allowed by default: out=%q err=%v", out, err)
		}
	}

	before := rec.count()
	for _, opts := range []Options{{DockerSock: sock, DenyPrivileged: true}, {DockerSock: sock, Paths: paths, DenyPrivileged: true}} {
		_, err := evalRun(t, opts, `
try:
    c.run("alpine", privileged=True)
except Exception:
    pass`)
		if err == nil || !strings.Contains(err.Error(), "privileged containers are not allowed") {
			t.Errorf("DenyPrivileged: want an uncatchable denial, got %v", err)
		}
		if out, err := evalRun(t, opts, `c.run("alpine")`); err != nil || out != "deadbeefcafe" {
			t.Errorf("DenyPrivileged, unprivileged run: out=%q err=%v", out, err)
		}
	}
	if got := rec.count() - before; got != 2 {
		t.Fatalf("%d creates under DenyPrivileged, want only the 2 unprivileged runs", got)
	}
}

// Host networking gives the container the host's loopback services, so it is
// refused under a network policy; other networks are unaffected.
func TestRunHostNetworkUnderPolicy(t *testing.T) {
	rec, sock, _, _ := runFixture(t)
	policy := &netsecurity.Config{}
	for _, network := range []string{"host", "HOST"} {
		_, err := evalRun(t, Options{DockerSock: sock, Policy: policy}, `c.run("alpine", network="`+network+`")`)
		if err == nil || !strings.Contains(err.Error(), "not allowed under a network policy") {
			t.Errorf("network=%q under a policy: got %v", network, err)
		}
	}
	if rec.count() != 0 {
		t.Fatalf("host network reached the runtime")
	}
	if out, err := evalRun(t, Options{DockerSock: sock, Policy: policy}, `c.run("alpine", network="bridge")`); err != nil || out != "deadbeefcafe" {
		t.Errorf("bridge under a policy: out=%q err=%v", out, err)
	}
	if out, err := evalRun(t, Options{DockerSock: sock}, `c.run("alpine", network="host")`); err != nil || out != "deadbeefcafe" || rec.last().NetworkMode != "host" {
		t.Errorf("host without a policy: out=%q err=%v", out, err)
	}
}

func TestBindSource(t *testing.T) {
	for spec, want := range map[string]string{
		"/data:/mnt":                       "/data",
		"/data:/mnt:ro":                    "/data",
		"./rel:/mnt":                       "./rel",
		"../up:/mnt":                       "../up",
		"~/home:/mnt":                      "~/home",
		"sub/dir:/mnt":                     "sub/dir",
		"type=bind,source=/x,target=/y":    "/x",
		"target=/y,src=/x,type=bind":       "/x",
		"mydata:/data":                     "",
		"/anonymous":                       "",
		"type=volume,source=vol,target=/y": "",
	} {
		got, ok := bindSource(spec)
		if (want != "") != ok || (ok && got != want) {
			t.Errorf("bindSource(%q) = %q, %v; want %q", spec, got, ok, want)
		}
	}
}
