package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/cli"
)

// TestNoSubprocessFlagReachesServerModes pins the fold that makes
// --no-subprocess an alias for --disable-lib subprocess in every mode: the
// fold used to happen after the server dispatches returned, so HTTP,
// JSON-RPC, MCP and app-bundle request handlers kept the subprocess library
// while plain script runs lost it. The probe is a JSON-RPC server driven over
// piped stdio whose handler reports whether `import subprocess` resolves.
func TestNoSubprocessFlagReachesServerModes(t *testing.T) {
	run := func(t *testing.T, extraArgs []string) string {
		t.Helper()
		dir := t.TempDir()
		setup := filepath.Join(dir, "probe.py")
		if err := os.WriteFile(setup, []byte(`
import scriptling.runtime as runtime

def check(params):
    try:
        import subprocess
        return "registered"
    except:
        return "disabled"

runtime.jsonrpc.method("probe", "probe.check")
`), 0o644); err != nil {
			t.Fatal(err)
		}

		root := buildRootCommand()
		args := append([]string{"scriptling", "--json-rpc", "-L", dir}, extraArgs...)
		args = append(args, setup)

		inR, inW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		outR, outW, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		savedIn, savedOut, savedArgs := os.Stdin, os.Stdout, os.Args
		os.Stdin, os.Stdout, os.Args = inR, outW, args
		defer func() { os.Stdin, os.Stdout, os.Args = savedIn, savedOut, savedArgs }()

		done := make(chan struct{})
		go func() {
			_, _ = inW.WriteString(`{"jsonrpc":"2.0","id":1,"method":"probe","params":{}}` + "\n")
			_ = inW.Close()
		}()
		go func() {
			_ = root.Execute(context.Background())
			_ = outW.Close()
			close(done)
		}()

		buf := make([]byte, 0, 4096)
		tmp := make([]byte, 4096)
		answer := make(chan string, 1)
		go func() {
			for {
				n, err := outR.Read(tmp)
				buf = append(buf, tmp[:n]...)
				if strings.Contains(string(buf), "disabled") || strings.Contains(string(buf), "registered") {
					answer <- string(buf)
					return
				}
				if err != nil {
					answer <- string(buf)
					return
				}
			}
		}()

		select {
		case got := <-answer:
			_ = outR.Close()
			return got
		case <-time.After(20 * time.Second):
			t.Fatal("jsonrpc server did not answer the probe")
			return ""
		case <-done:
			_ = outR.Close()
			got := <-answer
			return got
		}
	}

	t.Run("with the flag the library is gone", func(t *testing.T) {
		if got := run(t, []string{"--no-subprocess"}); !strings.Contains(got, "disabled") {
			t.Fatalf("--no-subprocess did not disable subprocess in server mode: %s", got)
		}
	})
	t.Run("without the flag the library is there", func(t *testing.T) {
		if got := run(t, nil); !strings.Contains(got, "registered") {
			t.Fatalf("control run lost the subprocess library: %s", got)
		}
	})
}

var _ = cli.Command{}
