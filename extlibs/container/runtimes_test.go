package container

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
)

// fakeCLIs puts docker, podman and container executables first on PATH that
// log each run and succeed. It returns the log path.
func fakeCLIs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "ran.log")
	script := "#!/bin/sh\necho \"$(basename $0) $*\" >> '" + log + "'\nexit 0\n"
	for _, name := range []string{"docker", "podman", "container"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func ranCLIs(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

// pingDaemon serves a Docker API that answers /_ping on a Unix socket.
func pingDaemon(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(shortTempDir(t), "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/_ping") {
			w.Write([]byte("OK"))
			return
		}
		http.NotFound(w, r)
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return sock
}

func evalContainerWith(t *testing.T, opts Options, script string) (string, error) {
	t.Helper()
	resetSockets(t)
	p := scriptling.New()
	RegisterWithOptions(p, opts)
	result, err := p.Eval("import scriptling.container as container\n" + script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

// runtimes() found runtimes by running their CLIs, resolved on PATH, so a
// script that could plant a `docker` executable in a PATH directory got it
// run, even under --no-subprocess (CWE-269, strix 4250 vuln-0019). It now
// starts nothing: Docker and Podman are found by pinging the endpoint
// Client() uses, and Apple by its CLI being installed at its fixed path.
func TestRuntimesStartNoProcess(t *testing.T) {
	log := fakeCLIs(t)
	docker := pingDaemon(t)
	missing := filepath.Join(shortTempDir(t), "none.sock")
	fakeApple := filepath.Join(filepath.Dir(log), "container")

	out, err := evalContainerWith(t, Options{DockerSock: docker, PodmanSock: missing, AppleCLI: fakeApple}, `container.runtimes()`)
	if err != nil || out != "['docker', 'apple']" {
		t.Fatalf("runtimes() = %s, %v", out, err)
	}
	out, err = evalContainerWith(t, Options{DockerSock: docker, PodmanSock: docker, AppleCLI: filepath.Join(t.TempDir(), "absent")}, `container.runtimes()`)
	if err != nil || out != "['docker', 'podman']" {
		t.Fatalf("runtimes() = %s, %v", out, err)
	}
	if ran := ranCLIs(t, log); ran != "" {
		t.Fatalf("runtimes() started processes: %q", ran)
	}
}

// The Apple driver runs its CLI from a fixed path, never one found on PATH,
// where a script that can write to a PATH directory could put its own.
func TestAppleCLINotLookedUpOnPath(t *testing.T) {
	log := fakeCLIs(t)
	missing := filepath.Join(shortTempDir(t), "none.sock")
	absent := filepath.Join(t.TempDir(), "no-such-container")
	out, err := evalContainerWith(t, Options{DockerSock: missing, PodmanSock: missing, AppleCLI: absent}, `container.runtimes()`)
	if err != nil || out != "[]" {
		t.Fatalf("runtimes() = %s, %v", out, err)
	}
	if _, err := evalContainerWith(t, Options{AppleCLI: absent}, `container.Client("apple").list()`); err == nil {
		t.Fatal("apple list with no CLI at the fixed path succeeded")
	}
	if ran := ranCLIs(t, log); ran != "" {
		t.Fatalf("a CLI found on PATH was run: %q", ran)
	}
	if appleCLI() != absent {
		t.Fatalf("appleCLI() = %q", appleCLI())
	}
}

// A daemon that is not answering is not reported, whatever CLI is on PATH.
func TestRuntimesReportsOnlyReachableEndpoints(t *testing.T) {
	fakeCLIs(t)
	missing := filepath.Join(shortTempDir(t), "none.sock")
	out, err := evalContainerWith(t, Options{DockerSock: missing, PodmanSock: missing, AppleCLI: filepath.Join(t.TempDir(), "absent")}, `container.runtimes()`)
	if err != nil || out != "[]" {
		t.Fatalf("runtimes() = %s, %v", out, err)
	}
}
