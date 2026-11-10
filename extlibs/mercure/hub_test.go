package mercure

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The tests run against a real gossiper hub: the server from the gossiper
// source folder ($SCRIPTLING_GOSSIPER_DIR, or ../gossiper beside this
// repository), built once per test run and started as a process. scriptling
// depends on gossiper's client only, never on the hub.

var (
	buildOnce   sync.Once
	gossiperBin string
	buildErr    error
	buildDir    string
)

func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

// gossiperDir is the gossiper source folder.
func gossiperDir() string {
	if d := os.Getenv("SCRIPTLING_GOSSIPER_DIR"); d != "" {
		return d
	}
	return filepath.Join("..", "..", "..", "gossiper")
}

// gossiper builds the hub, or skips the test when its source is not there.
func gossiper(t testing.TB) string {
	t.Helper()
	dir := gossiperDir()
	if _, err := os.Stat(filepath.Join(dir, "cmd", "gossiper")); err != nil {
		t.Skipf("no gossiper source at %s: set SCRIPTLING_GOSSIPER_DIR to run the hub tests", dir)
	}
	buildOnce.Do(func() {
		if buildDir, buildErr = os.MkdirTemp("", "scriptling-mercure-test"); buildErr != nil {
			return
		}
		gossiperBin = filepath.Join(buildDir, "gossiper")
		cmd := exec.Command("go", "build", "-o", gossiperBin, "./cmd/gossiper")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildError{err: err, out: string(out)}
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return gossiperBin
}

type buildError struct {
	err error
	out string
}

func (e *buildError) Error() string { return "building gossiper: " + e.err.Error() + "\n" + e.out }

// testHub is a gossiper process behind a server of the test's, over HTTP/1.1
// and HTTP/2 without TLS, that counts what it passes on.
type testHub struct {
	url     string       // the hub, through the counting server
	direct  string       // the hub itself
	streams atomic.Int64 // streaming publishes opened
	forms   atomic.Int64 // form POSTs
	query   atomic.Value // the query of the last subscription
	gets    atomic.Int64 // subscriptions opened (and reopened)
}

// newHub runs a hub until the test ends, args added to its command line.
func newHub(t testing.TB, args ...string) *testHub {
	t.Helper()
	return newHubWith(t, false, args...)
}

// newHubWith is newHub, refusing streaming publishes as any other Mercure hub
// does (it reads the body as a form) when noStreams is set.
func newHubWith(t testing.TB, noStreams bool, args ...string) *testHub {
	t.Helper()
	base := startHub(t, args)
	pass := forward(t, base)
	th := &testHub{direct: base + "/.well-known/mercure"}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			th.query.Store(r.URL.RawQuery)
			th.gets.Add(1)
		}
		if r.Method == http.MethodPost {
			if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-ndjson") {
				th.streams.Add(1)
				if noStreams {
					http.Error(w, `Missing "topic" parameter`, http.StatusBadRequest)
					return
				}
			} else {
				th.forms.Add(1)
			}
		}
		pass.ServeHTTP(w, r)
	}))
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetHTTP1(true)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(func() { // a stream a script left open would keep Close waiting
		srv.CloseClientConnections()
		srv.Close()
	})
	th.url = srv.URL + "/.well-known/mercure"
	return th
}

// startHub starts a gossiper process, args added to its command line, and
// returns its address (http://host:port). It takes Mercure 1.0 tokens from
// the issuer https://localhost signed with testKey, unless args say otherwise.
func startHub(t testing.TB, args []string) string {
	t.Helper()
	bin := gossiper(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	level := os.Getenv("SCRIPTLING_GOSSIPER_LOG") // its log is shown when a test fails
	if level == "" {
		level = "error"
	}
	cmdline := []string{"serve", "--listen", addr, "--http-only", "--log-level", level,
		"--publisher-key", testKey, "--trusted-issuers", "https://localhost", "--trust-forwarded-proto"}
	cmd := exec.Command(bin, append(cmdline, args...)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-exited
		if t.Failed() && out.Len() > 0 {
			t.Logf("gossiper's log:\n%s", out.String())
		}
	})
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		select {
		case <-exited:
			t.Fatalf("gossiper %v ended: %s", args, out.String())
		default:
		}
		if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			_ = c.Close()
			return "http://" + addr
		}
		if time.Now().After(deadline) {
			t.Fatalf("gossiper did not start: %s", out.String())
		}
	}
}

// forward passes requests on to the hub at target, both ways at once (a
// publish stream's lines and answers), over HTTP/2 without TLS. The hub sees
// the host and scheme the client asked for: they make the audience of
// Mercure 1.0 tokens.
func forward(t testing.TB, target string) http.Handler {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	tr := &http.Transport{Protocols: new(http.Protocols)}
	tr.Protocols.SetUnencryptedHTTP2(true)
	t.Cleanup(tr.CloseIdleConnections)
	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.Out.Host = r.In.Host
			r.SetXForwarded()
		},
		Transport:     tr,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).EnableFullDuplex()
		rp.ServeHTTP(w, r)
	})
}
