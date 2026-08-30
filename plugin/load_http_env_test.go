package plugin

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// These tests cover the batch loader's http(s) and environment support:
// --plugin accepts a plugin-server URL the same way it accepts an executable,
// --plugin-env entries layer onto the spawned process's environment, and
// --plugin-insecure skips TLS verification for self-signed certificates.

// TestLoadPluginsEnvironmentVariables proves spec.Env reaches the spawned
// plugin layered on the inherited environment (an inherited key is still
// visible, an injected key arrives, and an injected key overrides a parent
// one).
func TestLoadPluginsEnvironmentVariables(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_ENV_HELPER") == "1" {
		server := NewServer("envprobe", "1.0.0", "env probe helper")
		fb := object.NewFunctionBuilder()
		fb.Function(func(name string) string { return os.Getenv(name) })
		server.RegisterFunc("env", fb)
		if err := server.Run(); err != nil {
			panic(err)
		}
		return
	}

	helper := writeEnvProbeHelper(t, t.TempDir())

	// Present in the test process: the child must still see it.
	t.Setenv("SCRIPTLING_PLUGIN_ENV_PARENT", "inherited")
	// Also present in the parent, but the spec overrides it.
	t.Setenv("SCRIPTLING_PLUGIN_ENV_OVERRIDE", "parent-value")

	m := NewManager(nil)
	defer m.Close()
	err := m.LoadPlugins(context.Background(), []PluginSpec{{
		Path: helper,
		Env: []string{
			"SCRIPTLING_PLUGIN_ENV_INJECTED=hello-from-flag",
			"SCRIPTLING_PLUGIN_ENV_OVERRIDE=flag-value",
		},
	}})
	if err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	client, ok := m.Get("plugin.envprobe")
	if !ok {
		t.Fatal("plugin.envprobe not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	env := func(name string) string {
		t.Helper()
		result, err := client.CallFunction(ctx, "env", []Value{{Type: valueString, Value: name}}, nil)
		if err != nil {
			t.Fatalf("env(%s): %v", name, err)
		}
		value, _ := result.Value.(string)
		return value
	}
	if got := env("SCRIPTLING_PLUGIN_ENV_PARENT"); got != "inherited" {
		t.Fatalf("inherited variable lost: %q", got)
	}
	if got := env("SCRIPTLING_PLUGIN_ENV_INJECTED"); got != "hello-from-flag" {
		t.Fatalf("injected variable missing: %q", got)
	}
	if got := env("SCRIPTLING_PLUGIN_ENV_OVERRIDE"); got != "flag-value" {
		t.Fatalf("injected variable did not override the parent: %q", got)
	}
}

// TestLoadPluginsHTTPURL loads a plugin served over plain HTTP through the
// same LoadPlugins batch the CLI uses for --plugin.
func TestLoadPluginsHTTPURL(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("httpdemo", "1.0.0", "http echo demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)

	m := NewManager(nil)
	defer m.Close()
	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: srv.URL}}); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	client, ok := m.Get("plugin.httpdemo")
	if !ok {
		t.Fatal("plugin.httpdemo not registered")
	}
	result, err := client.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "over-http"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueString || result.Value != "over-http" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// TestLoadPluginsHTTPScopesRefusedByTransport checks the scope transport
// restrictions apply to the batch path too.
func TestLoadPluginsHTTPScopesRefusedByTransport(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("scopedemo", "1.0.0", "scope demo").RegisterFunc("echo", echo)
	srv := httptestServer(t, server)
	helper := writeEnvProbeHelper(t, t.TempDir())

	stdioOnly := NewManager(nil).NewScope(WithTransport(TransportStdio))
	defer stdioOnly.Close()
	err := stdioOnly.LoadPlugins(context.Background(), []PluginSpec{{Path: srv.URL}})
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected stdio-only refusal, got: %v", err)
	}

	httpOnly := NewManager(nil).NewScope(WithTransport(TransportHTTP))
	defer httpOnly.Close()
	err = httpOnly.LoadPlugins(context.Background(), []PluginSpec{{Path: helper}})
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected http-only refusal, got: %v", err)
	}
}

// TestLoadPluginsHTTPSInsecure covers --plugin-insecure: a self-signed
// certificate (as httptest serves) is rejected by default and accepted with
// Insecure set on the spec.
func TestLoadPluginsHTTPSInsecure(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("tlsdemo", "1.0.0", "tls echo demo").RegisterFunc("echo", echo)
	tlsSrv := newTLSServer(t, server)

	m := NewManager(nil)
	defer m.Close()
	err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: tlsSrv.URL}})
	if err == nil {
		t.Fatal("expected the self-signed certificate to be rejected by default")
	}

	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: tlsSrv.URL, Insecure: true}}); err != nil {
		t.Fatalf("LoadPlugins insecure: %v", err)
	}
	client, ok := m.Get("plugin.tlsdemo")
	if !ok {
		t.Fatal("plugin.tlsdemo not registered")
	}
	result, err := client.CallFunction(context.Background(), "echo",
		[]Value{{Type: valueString, Value: "skip-verify"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueString || result.Value != "skip-verify" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

// TestLoadPluginsPHPExample runs the PHP example server from
// examples/plugins/php-server through the plugin manager, proving a plugin
// written in another language loads through the same --plugin path scripts
// use. Skipped when php is not on PATH.
func TestLoadPluginsPHPExample(t *testing.T) {
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("php not on PATH")
	}
	server := filepath.Join("..", "examples", "plugins", "php-server", "index.php")
	if _, err := os.Stat(server); err != nil {
		t.Fatalf("example server missing: %v", err)
	}

	// A free port for the built-in server, and the process environment the
	// server reads (an HTTP plugin owns its environment; --plugin-env applies
	// to executables the host spawns).
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cmd := exec.Command(php, "-S", fmt.Sprintf("127.0.0.1:%d", port), server)
	cmd.Env = append(os.Environ(), "PHPDEMO_FROM=php-test")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start php: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitHTTPReady(t, url+"/", 10*time.Second)

	m := NewManager(nil)
	defer m.Close()
	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: url}}); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	client, ok := m.Get("plugin.phpdemo")
	if !ok {
		t.Fatal("plugin.phpdemo not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := client.CallFunction(ctx, "greet", []Value{{Type: valueString, Value: "Ada"}}, nil)
	if err != nil {
		t.Fatalf("greet: %v", err)
	}
	if result.Type != valueString || result.Value != "Hello, Ada (from php-test)" {
		t.Fatalf("unexpected greet result: %#v", result)
	}

	result, err = client.CallFunction(ctx, "echo", []Value{{
		Type:    valueDict,
		Entries: map[string]Value{"a": {Type: valueInt, Value: int64(1)}},
	}}, nil)
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	if result.Type != valueDict || fmt.Sprint(result.Entries["a"].Value) != "1" {
		t.Fatalf("unexpected echo result: %#v", result)
	}
}

// httptestServer serves a plugin server over plain HTTP for the test's
// lifetime. httptest.NewTLSServer serves a self-signed certificate, which is
// exactly what the insecure flag exists to tolerate.
func httptestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func newTLSServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// writeEnvProbeHelper writes the shell shim that re-execs the test binary as
// the env-probe plugin.
func writeEnvProbeHelper(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "env-probe")
	if runtime.GOOS == "windows" {
		path += ".bat"
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_ENV_HELPER=1\r\n\"" + exe + "\" -test.run=TestLoadPluginsEnvironmentVariables --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_ENV_HELPER=1 exec \"" + exe + "\" -test.run=TestLoadPluginsEnvironmentVariables --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	return path
}

func waitHTTPReady(t *testing.T, url string, timeout time.Duration) {
	waitHTTPReadyAuth(t, url, "", timeout)
}

// waitHTTPReadyAuth polls the handshake until the server answers 200. A
// token-protected server 401s an unauthenticated probe, so the bearer can be
// supplied; any HTTP answer at all also counts as ready, since it proves the
// server is listening.
func waitHTTPReadyAuth(t *testing.T, url, bearer string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	reqBody := strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"scriptling.handshake","params":{}}`)
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, url, reqBody)
		if err != nil {
			t.Fatalf("probe request: %v", err)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("plugin server at %s not ready within %s", url, timeout)
}

// headerRecorder wraps a plugin server and captures the Authorization header
// and Host of the last request, so auth plumbing can be asserted.
type headerRecorder struct {
	http.Handler
	mu            sync.Mutex
	authorization string
	host          string
}

func (r *headerRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.authorization = req.Header.Get("Authorization")
	r.host = req.Host
	r.mu.Unlock()
	r.Handler.ServeHTTP(w, req)
}

func (r *headerRecorder) snapshot() (string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.authorization, r.host
}

// TestLoadPluginsHTTPAuth covers the two ways credentials reach an HTTP
// plugin: user:pass inside the URL becomes the Basic auth header (and never
// leaks into the Host header), and an explicit Authorization header is sent
// verbatim, winning over URL credentials when both are present.
func TestLoadPluginsHTTPAuth(t *testing.T) {
	echo := object.NewFunctionBuilder()
	echo.Function(func(v any) any { return v })
	server := NewServer("authdemo", "1.0.0", "auth echo demo").RegisterFunc("echo", echo)
	recorder := &headerRecorder{Handler: server}
	srv := httptestServer(t, recorder)

	withURL := func(url string, headers map[string]string) {
		t.Helper()
		m := NewManager(nil)
		defer m.Close()
		if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: url, Headers: headers}}); err != nil {
			t.Fatalf("LoadPlugins %s: %v", url, err)
		}
		if _, ok := m.Get("plugin.authdemo"); !ok {
			t.Fatal("plugin.authdemo not registered")
		}
	}

	// Credentials in the URL: Basic auth on the wire, clean Host header.
	withURL(strings.Replace(srv.URL, "http://", "http://ada:secret@", 1), nil)
	auth, host := recorder.snapshot()
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("ada:secret"))
	if auth != expected {
		t.Fatalf("Authorization = %q, want %q", auth, expected)
	}
	if strings.Contains(host, "ada:") {
		t.Fatalf("URL credentials leaked into the Host header: %q", host)
	}

	// An explicit header is sent verbatim.
	withURL(srv.URL, map[string]string{"Authorization": "Bearer tok-123"})
	auth, _ = recorder.snapshot()
	if auth != "Bearer tok-123" {
		t.Fatalf("Authorization = %q, want the bearer token", auth)
	}

	// Explicit Authorization wins over URL credentials.
	withURL(strings.Replace(srv.URL, "http://", "http://ada:secret@", 1),
		map[string]string{"Authorization": "Bearer explicit-wins"})
	auth, _ = recorder.snapshot()
	if auth != "Bearer explicit-wins" {
		t.Fatalf("Authorization = %q, want the explicit header to win", auth)
	}
}

// TestLoadPluginsPHPExampleAuth runs the PHP example with its optional bearer
// enforcement enabled: without the header the load is refused, with the right
// token it answers. Skipped when php is not on PATH.
func TestLoadPluginsPHPExampleAuth(t *testing.T) {
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("php not on PATH")
	}
	server := filepath.Join("..", "examples", "plugins", "php-server", "index.php")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	cmd := exec.Command(php, "-S", fmt.Sprintf("127.0.0.1:%d", port), server)
	cmd.Env = append(os.Environ(), "PHPDEMO_TOKEN=seekrit", "PHPDEMO_FROM=php-auth")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start php: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	waitHTTPReadyAuth(t, url+"/", "seekrit", 10*time.Second)

	m := NewManager(nil)
	defer m.Close()
	if err := m.LoadPlugins(context.Background(), []PluginSpec{{Path: url}}); err == nil {
		t.Fatal("expected the token-protected server to refuse an unauthenticated load")
	}

	if err := m.LoadPlugins(context.Background(), []PluginSpec{
		{Path: url, Headers: map[string]string{"Authorization": "Bearer seekrit"}},
	}); err != nil {
		t.Fatalf("LoadPlugins with bearer token: %v", err)
	}
	client, ok := m.Get("plugin.phpdemo")
	if !ok {
		t.Fatal("plugin.phpdemo not registered")
	}
	result, err := client.CallFunction(context.Background(), "greet",
		[]Value{{Type: valueString, Value: "Ada"}}, nil)
	if err != nil {
		t.Fatalf("greet: %v", err)
	}
	if result.Type != valueString || result.Value != "Hello, Ada (from php-auth)" {
		t.Fatalf("unexpected greet result: %#v", result)
	}
}
