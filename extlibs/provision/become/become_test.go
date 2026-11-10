package become

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

func TestMain(m *testing.M) {
	ServeIfHelper()
	os.Exit(m.Run())
}

type echoArgs struct{ Text string }
type echoResult struct{ Text string }

func init() {
	Handle("test.echo", func(_ context.Context, paths fssecurity.Config, a echoArgs) (echoResult, error) {
		if a.Text == "denied" {
			return echoResult{}, paths.CheckPath("/nowhere/at/all")
		}
		if a.Text == "fail" {
			return echoResult{}, errors.New("it failed")
		}
		if a.Text == "pid" {
			return echoResult{Text: itoa(os.Getpid())}, nil
		}
		return echoResult{Text: "echo " + a.Text}, nil
	})
}

func itoa(i int) string { return strconv.Itoa(i) }

// fakeSudo writes a stand-in for sudo: it drops -n and -- and runs the rest,
// or, with refuse set, fails with -n as sudo does when it wants a password.
func fakeSudo(t *testing.T, refuse bool) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no shell")
	}
	script := "#!/bin/sh\nwhile [ \"$1\" = -n ] || [ \"$1\" = -- ]; do\n"
	if refuse {
		script += "  if [ \"$1\" = -n ]; then echo 'sudo: a password is required' >&2; exit 1; fi\n"
	}
	script += "  shift\ndone\nexec \"$@\"\n"
	p := filepath.Join(t.TempDir(), "sudo")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func testBecome(t *testing.T, sudo string, paths []string) *Become {
	b := New(Config{Sudo: []string{sudo}, Command: []string{os.Args[0], HelperArg},
		Paths: fssecurity.Config{AllowedPaths: paths}, NonRootHelper: true})
	b.root = func() bool { return false }
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func TestRunsInOneHelper(t *testing.T) {
	b := testBecome(t, fakeSudo(t, false), nil)
	ctx := context.Background()
	r, err := Do[echoArgs, echoResult](ctx, b, true, fssecurity.Config{}, "test.echo", echoArgs{"hi"})
	if err != nil || r.Text != "echo hi" {
		t.Fatalf("%v %v", r, err)
	}
	first, _ := Do[echoArgs, echoResult](ctx, b, true, fssecurity.Config{}, "test.echo", echoArgs{"pid"})
	second, _ := Do[echoArgs, echoResult](ctx, b, true, fssecurity.Config{}, "test.echo", echoArgs{"pid"})
	if first.Text == "" || first.Text != second.Text || first.Text == itoa(os.Getpid()) {
		t.Fatalf("operations ran in %q and %q, want one helper that is not this process", first.Text, second.Text)
	}
	here, _ := Do[echoArgs, echoResult](ctx, b, false, fssecurity.Config{}, "test.echo", echoArgs{"pid"})
	if here.Text != itoa(os.Getpid()) {
		t.Fatalf("without become the operation ran in %s, not here", here.Text)
	}
}

func TestErrorsComeBack(t *testing.T) {
	b := testBecome(t, fakeSudo(t, false), []string{t.TempDir()})
	ctx := context.Background()
	_, err := Do[echoArgs, echoResult](ctx, b, true, fssecurity.Config{}, "test.echo", echoArgs{"fail"})
	if err == nil || err.Error() != "it failed" || errors.Is(err, fssecurity.ErrPathNotAllowed) {
		t.Fatalf("got %v", err)
	}
	// The helper holds to its own allowed paths.
	_, err = Do[echoArgs, echoResult](ctx, b, true, fssecurity.Config{}, "test.echo", echoArgs{"denied"})
	if !errors.Is(err, fssecurity.ErrPathNotAllowed) {
		t.Fatalf("got %v, want a denied path", err)
	}
}

func TestNotEnabled(t *testing.T) {
	if IsRoot() {
		t.Skip("running as root: become needs no helper")
	}
	_, err := Do[echoArgs, echoResult](context.Background(), nil, true, fssecurity.Config{}, "test.echo", echoArgs{"x"})
	if !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("got %v", err)
	}
	r, err := Do[echoArgs, echoResult](context.Background(), nil, false, fssecurity.Config{}, "test.echo", echoArgs{"x"})
	if err != nil || r.Text != "echo x" {
		t.Fatalf("%v %v", r, err)
	}
}

func TestRootRunsHere(t *testing.T) {
	b := New(Config{Sudo: []string{"/nonexistent/sudo"}})
	b.root = func() bool { return true }
	r, err := Do[echoArgs, echoResult](context.Background(), b, true, fssecurity.Config{}, "test.echo", echoArgs{"pid"})
	if err != nil || r.Text != itoa(os.Getpid()) {
		t.Fatalf("%v %v", r, err)
	}
}

func TestPasswordWithoutTerminal(t *testing.T) {
	if hasTerminal() {
		t.Skip("a terminal is open: sudo would ask on it")
	}
	b := testBecome(t, fakeSudo(t, true), nil)
	_, err := Do[echoArgs, echoResult](context.Background(), b, true, fssecurity.Config{}, "test.echo", echoArgs{"x"})
	if err == nil || !strings.Contains(err.Error(), "no terminal") || !strings.Contains(err.Error(), "a password is required") {
		t.Fatalf("got %v", err)
	}
}

func TestNotRootHelperRefused(t *testing.T) {
	if IsRoot() {
		t.Skip("running as root")
	}
	b := testBecome(t, fakeSudo(t, false), nil)
	b.helperRoot = true
	_, err := Do[echoArgs, echoResult](context.Background(), b, true, fssecurity.Config{}, "test.echo", echoArgs{"x"})
	if err == nil || !strings.Contains(err.Error(), "not running as root") {
		t.Fatalf("got %v", err)
	}
}

func TestRunCommand(t *testing.T) {
	b := testBecome(t, fakeSudo(t, false), nil)
	ctx := context.Background()
	for _, asRoot := range []bool{false, true} {
		r, err := Run(ctx, b, asRoot, RunArgs{Shell: "read x; echo out $x $EXTRA; echo err >&2; exit 3",
			Input: []byte("in\n"), Env: []string{"EXTRA=more"}})
		if err != nil || r.Code != 3 || string(r.Stdout) != "out in more\n" || string(r.Stderr) != "err\n" {
			t.Fatalf("root=%v: %+v %v", asRoot, r, err)
		}
		dir := t.TempDir()
		r, err = Run(ctx, b, asRoot, RunArgs{Argv: []string{"pwd"}, Dir: dir})
		got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(r.Stdout)))
		want, _ := filepath.EvalSymlinks(dir)
		if err != nil || got != want {
			t.Fatalf("root=%v: pwd %q, want %q (%v)", asRoot, got, want, err)
		}
		start := time.Now()
		_, err = Run(ctx, b, asRoot, RunArgs{Argv: []string{"sleep", "30"}, Timeout: 200 * time.Millisecond})
		if err == nil || !strings.Contains(err.Error(), "timed out") || time.Since(start) > 5*time.Second {
			t.Fatalf("root=%v: timeout gave %v after %s", asRoot, err, time.Since(start))
		}
		_, err = Run(ctx, b, asRoot, RunArgs{Argv: []string{"/no/such/program"}})
		if err == nil {
			t.Fatalf("root=%v: a missing program ran", asRoot)
		}
	}
}

// A script stopped mid-command stops the command, and the next operation
// starts a new helper.
func TestCancelStopsHelper(t *testing.T) {
	b := testBecome(t, fakeSudo(t, false), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Run(ctx, b, true, RunArgs{Argv: []string{"sleep", "30"}})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("got %v after %s", err, time.Since(start))
	}
	r, err := Do[echoArgs, echoResult](context.Background(), b, true, fssecurity.Config{}, "test.echo", echoArgs{"again"})
	if err != nil || r.Text != "echo again" {
		t.Fatalf("after a cancel: %v %v", r, err)
	}
}

func TestClosed(t *testing.T) {
	b := testBecome(t, fakeSudo(t, false), nil)
	_ = b.Close()
	if _, err := Do[echoArgs, echoResult](context.Background(), b, true, fssecurity.Config{}, "test.echo", echoArgs{"x"}); err == nil {
		t.Fatal("ran after Close")
	}
}

func TestOwner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	p := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(p, nil, 0o644)
	info, _ := os.Stat(p)
	uid, gid, _ := fileOwner(info)
	changed, err := Owner{User: itoa(uid), Group: itoa(gid)}.Apply(fssecurity.Config{}, p)
	if err != nil || changed {
		t.Fatalf("own ids: changed=%v %v", changed, err)
	}
	if err := (Owner{User: "no-such-user-here"}).Check(); err == nil || !strings.Contains(err.Error(), "no user") {
		t.Fatalf("got %v", err)
	}
	if err := (Owner{Group: "no-such-group-here"}).Check(); err == nil || !strings.Contains(err.Error(), "no group") {
		t.Fatalf("got %v", err)
	}
	if changed, err := (Owner{}).Apply(fssecurity.Config{}, p); changed || err != nil {
		t.Fatalf("no owner: %v %v", changed, err)
	}
}

// A helper whose input closes ends cleanly, with status 0.
func TestServeEndsCleanly(t *testing.T) {
	var out strings.Builder
	if code := serve([]string{"null"}, strings.NewReader(""), &out); code != 0 {
		t.Fatalf("exit %d", code)
	}
	time.Sleep(50 * time.Millisecond) // a stray exit would have ended the test binary by now
	if out.Len() == 0 {
		t.Fatal("no hello")
	}
}
