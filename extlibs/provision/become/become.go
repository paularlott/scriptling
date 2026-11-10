// Package become runs provisioning work as root for a script that runs as an
// ordinary user.
//
// The first operation a script asks for with become=True starts one helper:
// this program again, run through sudo. sudo is first asked not to prompt
// (sudo -n), so a user with passwordless sudo is never asked anything; when
// sudo wants a password and there is a terminal, it asks once on it. Every
// later root operation goes to that helper over a pipe, and it ends with the
// program (or when Close is called).
//
// The helper does the same work as the unprivileged path, with the same
// functions (registered with Handle), and holds paths to the same allowed
// directories. A program that offers become must let the helper start: call
// ServeIfHelper first thing in main.
package become

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// HelperArg, as a program's first argument, makes ServeIfHelper run the helper.
const HelperArg = "__scriptling-provision-helper"

// ErrNotEnabled is returned for a root operation where become is not enabled.
var ErrNotEnabled = errors.New("become=True needs root, and running as root is turned off here " +
	"(the scriptling CLI turns it off with --deny-become, or when the subprocess library is disabled)")

// Config says how to start the helper.
type Config struct {
	// Paths confines the helper's file operations, as the libraries'.
	Paths fssecurity.Config
	// Command is the helper's command line, without sudo. Empty: this
	// program with HelperArg.
	Command []string
	// Sudo runs Command as root. Empty: "sudo". Its -n option must make it
	// fail instead of prompting, as sudo's and doas's do.
	Sudo []string
	// NonRootHelper accepts a helper that is not root. For tests, which run
	// the helper without sudo.
	NonRootHelper bool
}

// Become hands root operations to a helper running as root. Its methods are
// safe to call from several goroutines; operations run one at a time.
type Become struct {
	cfg Config

	mu     sync.Mutex
	helper *helper
	closed bool

	// root reports whether this process already runs as root; tests replace it.
	root func() bool
	// helperRoot is whether the helper must report running as root; tests
	// run a helper without sudo and turn it off.
	helperRoot bool
}

// New returns a Become that starts its helper when it is first needed.
func New(cfg Config) *Become {
	return &Become{cfg: cfg, root: func() bool { return os.Geteuid() == 0 }, helperRoot: !cfg.NonRootHelper}
}

// Close stops the helper, if it runs. Later operations fail.
func (b *Become) Close() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	if b.helper != nil {
		b.helper.stop()
		b.helper = nil
	}
	return nil
}

// IsRoot reports whether this process runs as root.
func IsRoot() bool { return os.Geteuid() == 0 }

// handler is an operation as the helper runs it: arguments and result gob-encoded.
type handler func(ctx context.Context, paths fssecurity.Config, args []byte) ([]byte, error)

var (
	handlers = map[string]handler{}
	locals   = map[string]any{}
)

// Handle registers op, so Do can run it here or in the helper. Call it from an
// init function: the helper knows the operations its program registered.
// A and R must be gob-encodable.
func Handle[A, R any](op string, fn func(ctx context.Context, paths fssecurity.Config, a A) (R, error)) {
	if _, dup := handlers[op]; dup {
		panic("become: operation registered twice: " + op)
	}
	locals[op] = fn
	handlers[op] = func(ctx context.Context, paths fssecurity.Config, raw []byte) ([]byte, error) {
		var a A
		if err := decode(raw, &a); err != nil {
			return nil, fmt.Errorf("become: decoding %s arguments: %w", op, err)
		}
		r, err := fn(ctx, paths, a)
		if err != nil {
			return nil, err
		}
		return encode(r)
	}
}

// Do runs op with a. As root it runs in the helper when asRoot is set and this
// process is not root already; otherwise here, confined to paths. b may be nil
// when become is not enabled: an operation as root then fails with
// ErrNotEnabled, unless this process is root.
func Do[A, R any](ctx context.Context, b *Become, asRoot bool, paths fssecurity.Config, op string, a A) (R, error) {
	var zero R
	fn, ok := locals[op].(func(context.Context, fssecurity.Config, A) (R, error))
	if !ok {
		return zero, fmt.Errorf("become: unknown operation %q", op)
	}
	if !asRoot || isRoot(b) {
		return fn(ctx, paths, a)
	}
	if b == nil {
		return zero, ErrNotEnabled
	}
	raw, err := encode(a)
	if err != nil {
		return zero, err
	}
	out, err := b.call(ctx, op, raw)
	if err != nil {
		return zero, err
	}
	var r R
	if err := decode(out, &r); err != nil {
		return zero, fmt.Errorf("become: decoding %s result: %w", op, err)
	}
	return r, nil
}

func isRoot(b *Become) bool {
	if b != nil {
		return b.root()
	}
	return IsRoot()
}

// call runs op in the helper, starting it if it is not running.
func (b *Become) call(ctx context.Context, op string, args []byte) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("become: closed")
	}
	if b.helper == nil {
		h, err := b.start(ctx)
		if err != nil {
			return nil, err
		}
		b.helper = h
	}
	out, err := b.helper.call(ctx, op, args)
	var dead *helperDied
	if errors.As(err, &dead) || ctx.Err() != nil {
		// The helper is gone (or was stopped with the script): the next
		// operation starts another.
		b.helper.stop()
		b.helper = nil
	}
	return out, err
}

// remoteError is an error the helper returned.
type remoteError struct {
	msg    string
	denied bool
}

func (e *remoteError) Error() string { return e.msg }

func (e *remoteError) Unwrap() error {
	if e.denied {
		return fssecurity.ErrPathNotAllowed
	}
	return nil
}
