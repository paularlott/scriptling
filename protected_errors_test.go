package scriptling

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/stdlib"
)

// swallowers are the ways a script tries to keep going past an error: an
// except clause, a with block whose __exit__ suppresses everything, and
// contextlib.suppress.
type swallower struct {
	setup, open, close string
	depth              int // block levels the body sits under open; 0 means 1
}

var swallowers = map[string]swallower{
	"try/except": {
		open:  "try:\n",
		close: "except:\n    pass\n",
	},
	"__exit__ returns True": {
		setup: "class Swallow:\n    def __enter__(self):\n        return self\n    def __exit__(self, t, v, tb):\n        return True\n",
		open:  "with Swallow():\n",
	},
	"contextlib.suppress": {
		setup: "import contextlib\n",
		open:  "with contextlib.suppress(Exception):\n",
	},
	// An __exit__ that raises replaces the in-flight exception in Python, so
	// a handler around the with block could catch the replacement: the
	// protected error must keep propagating instead.
	"__exit__ raises": {
		setup: "class Launder:\n    def __enter__(self):\n        return self\n    def __exit__(self, t, v, tb):\n        raise ValueError('laundered: ' + str(t))\n",
		open:  "try:\n    with Launder():\n",
		close: "except ValueError:\n    pass\n",
		depth: 2,
	},
}

// wrapIn indents body under a swallower and marks whether the script got past it.
func wrapIn(s swallower, body string) string {
	prefix := strings.Repeat("    ", max(s.depth, 1))
	indented := prefix + strings.ReplaceAll(strings.TrimSpace(body), "\n", "\n"+prefix) + "\n"
	return s.setup + "survived = False\n" + s.open + indented + s.close + "survived = True\n"
}

// A filesystem-jail PermissionError and sys.exit() end the script whatever
// tries to swallow them.
func TestProtectedExceptionsCannotBeSwallowed(t *testing.T) {
	allowed := t.TempDir()
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, s := range swallowers {
		t.Run(name+"/PermissionError", func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			extlibs.RegisterOSLibrary(p, []string{allowed})
			_, err := p.Eval("import os\n" + wrapIn(s, `os.read_file("`+secret+`")`))
			if err == nil || !strings.Contains(err.Error(), "outside allowed directories") {
				t.Fatalf("expected the PermissionError to end the script, got: %v", err)
			}
			if v, _ := p.GetVar("survived"); v == true {
				t.Fatal("the script continued past the PermissionError")
			}
		})
		t.Run(name+"/SystemExit", func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			extlibs.RegisterSysLibrary(p, nil, nil)
			_, err := p.Eval("import sys\n" + wrapIn(s, "sys.exit(3)"))
			if err == nil || !strings.Contains(err.Error(), "SystemExit") {
				t.Fatalf("expected SystemExit to end the script, got: %v", err)
			}
			if v, _ := p.GetVar("survived"); v == true {
				t.Fatal("the script continued past sys.exit()")
			}
		})
	}
}

// A timeout or cancellation from the host ends the script promptly: no
// except clause or with block can catch it and keep the script running.
func TestRunStopsCannotBeSwallowed(t *testing.T) {
	for name, s := range swallowers {
		for _, stop := range []string{"timeout", "cancel"} {
			t.Run(name+"/"+stop, func(t *testing.T) {
				p := New()
				stdlib.RegisterAll(p)
				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				defer cancel()
				if stop == "cancel" {
					ctx, cancel = context.WithCancel(context.Background())
					defer cancel()
					time.AfterFunc(200*time.Millisecond, cancel)
				}
				// Each pass catches the stop and goes round again; only an
				// uncatchable stop ends the outer loop.
				script := "caught = 0\nfor attempt in range(1000000):\n" +
					indentBy(wrapIn(s, "while True:\n    pass"), "    ") +
					"    caught += 1\n"
				start := time.Now()
				_, err := p.EvalWithContext(ctx, script)
				if err == nil {
					t.Fatal("the script ran to completion despite being stopped")
				}
				want := "execution timeout"
				if stop == "cancel" {
					want = "execution cancelled"
				}
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("expected %q, got: %v", want, err)
				}
				if c, _ := p.GetVar("caught"); c != int64(0) {
					t.Fatalf("the script swallowed the stop %v times", c)
				}
				if d := time.Since(start); d > 5*time.Second {
					t.Fatalf("stop took %v", d)
				}
			})
		}
	}
}

// Ordinary errors are still caught and suppressed as before.
func TestOrdinaryErrorsStillCatchable(t *testing.T) {
	for name, s := range swallowers {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(wrapIn(s, "x = 1 / 0")); err != nil {
				t.Fatalf("ZeroDivisionError should be swallowed: %v", err)
			}
			if v, _ := p.GetVar("survived"); v != true {
				t.Fatal("the script did not continue past an ordinary error")
			}
		})
	}
}

func indentBy(s, prefix string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}
