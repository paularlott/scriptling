//go:build darwin

package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// fakeAppleCLI points the Apple driver at a fake `container` executable that
// records its arguments, one per line, and prints a container ID. It returns
// the log path.
func fakeAppleCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "argv.log")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\" >> '" + log + "'; done\nprintf -- '--END--\\n' >> '" + log + "'\necho abc123\n"
	if err := os.WriteFile(filepath.Join(dir, "container"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	saved := overrideAppleCLI
	overrideAppleCLI = filepath.Join(dir, "container")
	t.Cleanup(func() { overrideAppleCLI = saved })
	return log
}

func appleCalls(t *testing.T, log string) [][]string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	var cur []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "--END--" {
			calls = append(calls, cur)
			cur = nil
			continue
		}
		cur = append(cur, line)
	}
	return calls
}

// The Apple driver passed script values straight to the `container` CLI, so
// an image of "-v" (with the mount and the real image in command) became a
// bind mount run() never saw. Script-supplied positionals now follow "--",
// and option values that would read as flags are refused.
func TestAppleArgvCannotInjectOptions(t *testing.T) {
	log := fakeAppleCLI(t)
	d := &appleClient{}
	ctx := context.Background()

	if _, err := d.Run(ctx, "-v", RunOptions{Command: []string{"/:/host", "alpine"}}); err != nil {
		t.Fatal(err)
	}
	calls := appleCalls(t, log)
	if got := strings.Join(calls[0], " "); got != "run -d -- -v /:/host alpine" {
		t.Fatalf("run argv = %q", got)
	}

	for name, opts := range map[string]RunOptions{
		"name":    {Name: "-v"},
		"env":     {Env: []string{"--volume=/:/host"}},
		"ports":   {Ports: []string{"-v"}},
		"volumes": {Volumes: []string{"-v"}},
		"network": {Network: "--privileged"},
	} {
		if _, err := d.Run(ctx, "alpine", opts); err == nil || !strings.Contains(err.Error(), `must not start with "-"`) {
			t.Errorf("%s: want a refusal, got %v", name, err)
		}
	}
	if _, err := d.ExecStream(ctx, "c1", []string{"sh"}, ExecOptions{User: "-v"}, func(string, string) {}); err == nil {
		t.Error("exec user starting with - was not refused")
	}
	if n := len(appleCalls(t, log)); n != 1 {
		t.Fatalf("%d CLI calls, want only the first run", n)
	}

	// Every script-supplied positional follows "--".
	d.Pull(ctx, "-x")
	d.Stop(ctx, "-x")
	d.Remove(ctx, "-x")
	d.Inspect(ctx, "-x")
	d.VolumeCreate(ctx, "-x", "1G")
	d.VolumeRemove(ctx, "-x")
	d.ExecStream(ctx, "-x", []string{"sh", "-c", "true"}, ExecOptions{}, func(string, string) {})
	for _, call := range appleCalls(t, log)[1:] {
		i := strings.Index(strings.Join(call, " "), " -- -x")
		if i < 0 {
			t.Errorf("call %q does not put the positional after --", strings.Join(call, " "))
		}
	}
}

// The library's run() checks apply before the Apple CLI is invoked.
func TestAppleRunHeldToAllowedPaths(t *testing.T) {
	log := fakeAppleCLI(t)
	allowed := t.TempDir()
	p := scriptling.New()
	RegisterWithOptions(p, Options{Paths: fssecurity.Config{AllowedPaths: []string{allowed}}, DenyPrivileged: true})
	_, err := p.Eval(`import scriptling.container as container
container.Client("apple").run("alpine", volumes=["/etc:/hostetc"])`)
	if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
		t.Fatalf("want a denial, got %v", err)
	}
	_, err = p.Eval(`import scriptling.container as container
container.Client("apple").run("alpine", privileged=True)`)
	if err == nil || !strings.Contains(err.Error(), "privileged containers are not allowed") {
		t.Fatalf("want a denial, got %v", err)
	}
	if calls := appleCalls(t, log); len(calls) != 0 {
		t.Fatalf("the CLI was invoked: %v", calls)
	}
}
