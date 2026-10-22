package container

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// dockerAPI answers the container list endpoint and counts every request.
func dockerAPI(hits *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"Id":"abc","Names":["/web"],"Image":"nginx","State":"running"}]`))
	}
}

// resetSockets restores the package-level operator endpoints after a test.
func resetSockets(t *testing.T) {
	t.Helper()
	docker, podman, apple := overrideDockerSocket, overrridePodmanSocket, overrideAppleCLI
	t.Cleanup(func() { overrideDockerSocket, overrridePodmanSocket, overrideAppleCLI = docker, podman, apple })
}

func evalContainer(t *testing.T, dockerSock string, paths fssecurity.Config, policy *netsecurity.Config, script string) (string, error) {
	t.Helper()
	resetSockets(t)
	p := scriptling.New()
	RegisterConfigured(p, dockerSock, "", paths, policy)
	result, err := p.Eval("import scriptling.container as container\n" + script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

// serveUnix serves the Docker API stub on a Unix socket at path.
func serveUnix(t *testing.T, path string, hits *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := &http.Server{Handler: dockerAPI(hits)}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
}

// shortTempDir keeps Unix socket paths under the platform length limit.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ct")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestContainerScriptEndpointNetworkPolicy(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(dockerAPI(&hits))
	defer srv.Close()
	tlsSrv := httptest.NewTLSServer(dockerAPI(&hits))
	defer tlsSrv.Close()
	hostPort := strings.TrimPrefix(srv.URL, "http://")

	cases := []struct {
		name   string
		policy *netsecurity.Config
		socket string
	}{
		{"tcp literal", &netsecurity.Config{AllowLoopback: true}, "tcp://" + hostPort},
		{"host:port shorthand", &netsecurity.Config{AllowLoopback: true}, hostPort},
		{"https literal", &netsecurity.Config{AllowLoopback: true}, tlsSrv.URL},
		{"loopback name", &netsecurity.Config{}, "tcp://" + strings.Replace(hostPort, "127.0.0.1", "localhost", 1)},
	}
	for _, tc := range cases {
		_, err := evalContainer(t, "", fssecurity.Config{}, tc.policy, `container.Client("docker", socket="`+tc.socket+`").list()`)
		if err == nil || !strings.Contains(err.Error(), "network policy") {
			t.Errorf("%s: expected network policy error, got: %v", tc.name, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("POLICY-BYPASS [container]: denied requests reached the server %d times", hits.Load())
	}

	// A permitted script endpoint still works under the policy.
	out, err := evalContainer(t, "", fssecurity.Config{}, &netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true},
		`container.Client("docker", socket="tcp://`+hostPort+`").list()[0]["name"]`)
	if err != nil || out != "web" {
		t.Errorf("permitted endpoint: out=%q err=%v", out, err)
	}
}

// The operator's endpoint is trusted even when the policy would deny it,
// whether the script relies on the default or names it explicitly.
func TestContainerOperatorEndpointTrusted(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(dockerAPI(&hits))
	defer srv.Close()
	operator := "tcp://" + strings.TrimPrefix(srv.URL, "http://")

	for _, script := range []string{
		`container.Client("docker").list()[0]["name"]`,
		`container.Client("docker", socket="` + operator + `").list()[0]["name"]`,
	} {
		out, err := evalContainer(t, operator, fssecurity.Config{AllowedPaths: []string{}}, &netsecurity.Config{}, script)
		if err != nil || out != "web" {
			t.Errorf("%s: out=%q err=%v", script, out, err)
		}
	}
}

func TestContainerRejectsQueryAndFragment(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(dockerAPI(&hits))
	defer srv.Close()

	for _, socket := range []string{srv.URL + "/secret#", srv.URL + "/secret?x="} {
		_, err := evalContainer(t, "", fssecurity.Config{}, nil, `container.Client("docker", socket="`+socket+`").list()`)
		if err == nil || !strings.Contains(err.Error(), "query or fragment") {
			t.Errorf("socket=%q should be rejected, got: %v", socket, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("rejected endpoints must not send requests, got %d", hits.Load())
	}
}

func TestContainerScriptSocketAllowedPaths(t *testing.T) {
	var hits atomic.Int32
	allowed := shortTempDir(t)
	outside := shortTempDir(t)
	inSock := filepath.Join(allowed, "d.sock")
	outSock := filepath.Join(outside, "d.sock")
	serveUnix(t, inSock, &hits)
	serveUnix(t, outSock, &hits)
	link := filepath.Join(allowed, "link.sock")
	if err := os.Symlink(outSock, link); err != nil {
		t.Fatal(err)
	}
	paths := fssecurity.Config{AllowedPaths: []string{allowed}}

	out, err := evalContainer(t, "", paths, nil, `container.Client("docker", socket="unix://`+inSock+`").list()[0]["name"]`)
	if err != nil || out != "web" {
		t.Fatalf("socket inside allowed paths: out=%q err=%v", out, err)
	}

	hits.Store(0)
	for _, socket := range []string{outSock, "unix://" + outSock, link} {
		_, err := evalContainer(t, "", paths, nil, `container.Client("docker", socket="`+socket+`").list()`)
		if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
			t.Errorf("socket=%q should be denied, got: %v", socket, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("JAIL-BYPASS [container]: denied sockets were reached %d times", hits.Load())
	}

	// Without --allowed-paths a script-supplied socket is unrestricted.
	out, err = evalContainer(t, "", fssecurity.Config{}, &netsecurity.Config{}, `container.Client("docker", socket="`+outSock+`").list()[0]["name"]`)
	if err != nil || out != "web" {
		t.Errorf("unrestricted socket: out=%q err=%v", out, err)
	}
}
