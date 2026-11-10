package packages

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/provision/become"
)

func TestMain(m *testing.M) {
	become.ServeIfHelper()
	os.Exit(m.Run())
}

// fakeApt puts stand-ins for apt-get and dpkg-query first on PATH. A package
// is installed when $STATE/<name> exists; every apt-get call is logged; with
// $NEED_UPDATE set, installing fails until "apt-get update" has run.
func fakeApt(t *testing.T, needUpdate bool) (state string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	bin, state := t.TempDir(), t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("dpkg-query", `for a; do n=$a; done
[ -e "$STATE/$n" ] && { printf 'ii '; exit 0; }
echo "dpkg-query: no packages found matching $n" >&2; exit 1
`)
	write("apt-get", `echo "$DEBIAN_FRONTEND $*" >> "$STATE/log"
cmd=""; skip=0; names=""
for a; do
  if [ $skip = 1 ]; then skip=0; continue; fi
  case "$a" in -o) skip=1 ;; -*) ;; *) if [ -z "$cmd" ]; then cmd=$a; else names="$names $a"; fi ;; esac
done
case $cmd in
  update) touch "$STATE/.updated" ;;
  install)
    if [ -n "$NEED_UPDATE" ] && [ ! -e "$STATE/.updated" ]; then echo "E: Unable to locate package$names" >&2; exit 100; fi
    for n in $names; do [ "$n" = broken ] && { echo "E: broken is broken" >&2; exit 100; }; touch "$STATE/$n"; done ;;
  remove) for n in $names; do rm -f "$STATE/$n"; done ;;
esac
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("STATE", state)
	if needUpdate {
		t.Setenv("NEED_UPDATE", "1")
	}
	return state
}

func helper(t *testing.T) *become.Become {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sudo")
	script := "#!/bin/sh\nwhile [ \"$1\" = -n ] || [ \"$1\" = -- ]; do shift; done\nexec \"$@\"\n"
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	b := become.New(become.Config{Sudo: []string{p}, Command: []string{os.Args[0], become.HelperArg}, NonRootHelper: true})
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func eval(t *testing.T, b *become.Become, code string) (string, error) {
	t.Helper()
	p := scriptling.New()
	RegisterConfigured(p, WithBecome(b))
	v, err := p.Eval("import scriptling.provision.package as package\n" + code)
	if err != nil {
		return "", err
	}
	return v.Inspect(), nil
}

// calls are the apt-get command lines, in order.
func calls(t *testing.T, state string) []string {
	raw, _ := os.ReadFile(filepath.Join(state, "log"))
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestEnsureAndAbsent(t *testing.T) {
	state := fakeApt(t, false)
	_ = os.WriteFile(filepath.Join(state, "curl"), nil, 0o644)
	got, err := eval(t, helper(t), `
a = package.ensure(["curl", "nginx"], manager="apt")
b = package.ensure("nginx", manager="apt")
c = package.installed(["curl", "nginx"], manager="apt")
d = package.absent(["nginx", "telnet"], manager="apt")
e = package.absent("nginx", manager="apt")
f = package.installed("nginx", manager="apt")
[a, b, c, d, e, f]
`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "['created', 'unchanged', True, 'removed', 'absent', False]"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	// Only what was missing is installed, only what is there removed, and apt
	// runs only when there is something to do.
	log := strings.Join(calls(t, state), " | ")
	want := "noninteractive install -y -q -o DPkg::Lock::Timeout=300 nginx | noninteractive remove -y -q -o DPkg::Lock::Timeout=300 nginx"
	if log != want {
		t.Errorf("apt-get calls:\n%s\nwant\n%s", log, want)
	}
}

// A new machine has no package lists: the install fails, the lists are
// refreshed, and it is tried again.
func TestRefreshesAndRetries(t *testing.T) {
	state := fakeApt(t, true)
	got, err := eval(t, helper(t), `package.ensure("git", manager="apt")`)
	if err != nil || got != "created" {
		t.Fatalf("%s %v", got, err)
	}
	var cmds []string
	for _, c := range calls(t, state) {
		cmds = append(cmds, strings.Fields(c)[1])
	}
	if strings.Join(cmds, ",") != "install,update,install" {
		t.Errorf("apt-get calls %v", cmds)
	}
}

func TestFailureSaysWhy(t *testing.T) {
	fakeApt(t, false)
	_, err := eval(t, helper(t), `package.ensure(["ok", "broken"], manager="apt")`)
	if err == nil || !strings.Contains(err.Error(), "E: broken is broken") || !strings.Contains(err.Error(), "status 100") {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateFirst(t *testing.T) {
	state := fakeApt(t, false)
	if _, err := eval(t, helper(t), `package.ensure("git", update=True, manager="apt")`); err != nil {
		t.Fatal(err)
	}
	if log := calls(t, state); !strings.HasPrefix(log[0], "noninteractive update") {
		t.Errorf("calls: %s", log)
	}
}

func TestNeedsRoot(t *testing.T) {
	if become.IsRoot() {
		t.Skip("running as root")
	}
	fakeApt(t, false)
	_, err := eval(t, nil, `
try:
    package.ensure("git", manager="apt")
except Exception:
    pass
`)
	if err == nil || !strings.Contains(err.Error(), "--deny-become") {
		t.Fatalf("got %v", err)
	}
	// Asking needs no root.
	if got, err := eval(t, nil, `package.installed("git", manager="apt")`); err != nil || got != "False" {
		t.Fatalf("%s %v", got, err)
	}
}

func TestArguments(t *testing.T) {
	for code, want := range map[string]string{
		`package.ensure([])`:                 "no packages named",
		`package.ensure(["-y"])`:             "is not a package name",
		`package.ensure([1])`:                "must be a string",
		`package.ensure("x", manager="pip")`: `no package manager "pip"`,
	} {
		if _, err := eval(t, nil, code); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", code, err, want)
		}
	}
}

func TestDetect(t *testing.T) {
	old := lookPath
	t.Cleanup(func() { lookPath = old })
	for have, want := range map[string]string{"apt-get": "apt", "dnf": "dnf", "yum": "yum", "apk": "apk", "brew": "brew"} {
		lookPath = func(p string) (string, error) {
			if p == have || (have == "yum" && p == "brew") {
				return "/bin/" + p, nil
			}
			return "", os.ErrNotExist
		}
		if got, err := eval(t, nil, `package.manager()`); err != nil || got != want {
			t.Errorf("with %s: %s %v", have, got, err)
		}
	}
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	if got, _ := eval(t, nil, `package.manager()`); got != "None" {
		t.Errorf("with none: %s", got)
	}
}
