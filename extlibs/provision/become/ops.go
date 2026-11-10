package become

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"time"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// RunArgs is a command to run.
type RunArgs struct {
	Argv    []string // the program and its arguments; or
	Shell   string   // a command line for /bin/sh -c
	Dir     string   // working directory; empty: the current one
	Env     []string // KEY=value entries added to the environment
	Input   []byte   // standard input; nil: none
	Timeout time.Duration
}

// RunResult is how a command ended.
type RunResult struct {
	Code   int
	Stdout []byte
	Stderr []byte
}

// maxOutput is how much of each of a command's stdout and stderr is kept.
const maxOutput = 64 << 20

func init() {
	Handle("run", runCommand)
}

// Run runs a command, as root when asRoot is set.
func Run(ctx context.Context, b *Become, asRoot bool, a RunArgs) (RunResult, error) {
	return Do[RunArgs, RunResult](ctx, b, asRoot, fssecurity.Config{}, "run", a)
}

func runCommand(ctx context.Context, _ fssecurity.Config, a RunArgs) (RunResult, error) {
	argv := a.Argv
	if a.Shell != "" {
		argv = []string{"/bin/sh", "-c", a.Shell}
	}
	if len(argv) == 0 || argv[0] == "" {
		return RunResult{}, errors.New("no command given")
	}
	if a.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.Timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Cancel = func() error { return interrupt(cmd.Process) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = a.Dir
	if len(a.Env) > 0 {
		cmd.Env = append(os.Environ(), a.Env...)
	}
	if a.Input != nil {
		cmd.Stdin = bytesReader(a.Input)
	}
	stdout, stderr := &limitedBuffer{max: maxOutput}, &limitedBuffer{max: maxOutput}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	res := RunResult{Stdout: []byte(stdout.String()), Stderr: []byte(stderr.String())}
	if ctx.Err() == context.DeadlineExceeded && a.Timeout > 0 {
		return res, fmt.Errorf("%s timed out after %s", argv[0], a.Timeout)
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		res.Code = exit.ExitCode()
	default:
		return res, err
	}
	return res, nil
}

// Owner names who a file belongs to: a user and a group, by name or number.
// Empty leaves that part as it is.
type Owner struct {
	User  string
	Group string
}

// Given reports whether an owner or a group is named.
func (o Owner) Given() bool { return o.User != "" || o.Group != "" }

// ids resolves the names; -1 is "leave as it is".
func (o Owner) ids() (uid, gid int, err error) {
	uid, gid = -1, -1
	if o.User != "" {
		if uid, err = strconv.Atoi(o.User); err != nil {
			u, lerr := user.Lookup(o.User)
			if lerr != nil {
				return 0, 0, fmt.Errorf("no user %q", o.User)
			}
			uid, _ = strconv.Atoi(u.Uid)
		}
	}
	if o.Group != "" {
		if gid, err = strconv.Atoi(o.Group); err != nil {
			g, lerr := user.LookupGroup(o.Group)
			if lerr != nil {
				return 0, 0, fmt.Errorf("no group %q", o.Group)
			}
			gid, _ = strconv.Atoi(g.Gid)
		}
	}
	return uid, gid, nil
}

// Check resolves the names, so a bad one fails before anything is written.
func (o Owner) Check() error {
	_, _, err := o.ids()
	return err
}

// Apply gives path the named owner and group, if it has not got them, and
// reports whether it changed anything. path must be within paths.
func (o Owner) Apply(paths fssecurity.Config, path string) (bool, error) {
	if !o.Given() {
		return false, nil
	}
	if err := paths.CheckPath(path); err != nil {
		return false, err
	}
	uid, gid, err := o.ids()
	if err != nil {
		return false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	curUID, curGID, ok := fileOwner(info)
	if !ok {
		return false, errors.New("setting owner and group is not supported on this system")
	}
	if (uid == -1 || uid == curUID) && (gid == -1 || gid == curGID) {
		return false, nil
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return false, fmt.Errorf("setting the owner of %s: %w", path, err)
	}
	return true, nil
}
