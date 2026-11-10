package become

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
)

// protocolVersion changes when the helper's messages do.
const protocolVersion = 1

type hello struct {
	Version int
	Root    bool
}

type request struct {
	Op   string
	Args []byte
}

type response struct {
	Result []byte
	Err    string
	Denied bool
}

// helper is a running helper process.
type helper struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	enc   *gob.Encoder
	dec   *gob.Decoder
	once  sync.Once
}

// helperDied is the error when the helper is gone mid-operation.
type helperDied struct{ err error }

func (e *helperDied) Error() string {
	return "become: the root helper stopped: " + e.err.Error()
}

func (e *helperDied) Unwrap() error { return e.err }

// helperCommand is the helper's command line, without sudo; its last
// argument is the allowed paths, as JSON.
func (b *Become) helperCommand() ([]string, error) {
	argv := b.cfg.Command
	if len(argv) == 0 {
		exe, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("become: finding this program to run as root: %w", err)
		}
		argv = []string{exe, HelperArg}
	}
	paths := b.cfg.Paths.AllowedPaths
	if paths != nil {
		abs := make([]string, 0, len(paths))
		for _, p := range paths {
			if a, err := filepath.Abs(p); err == nil {
				abs = append(abs, filepath.Clean(a))
			}
		}
		paths = abs
	}
	j, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}
	return append(append([]string{}, argv...), string(j)), nil
}

// start runs the helper through sudo: first without letting sudo prompt, so
// passwordless sudo just works; then, when that fails and there is a terminal
// to ask on, letting sudo ask for the password.
func (b *Become) start(ctx context.Context) (*helper, error) {
	argv, err := b.helperCommand()
	if err != nil {
		return nil, err
	}
	sudo := b.cfg.Sudo
	if len(sudo) == 0 {
		sudo = []string{"sudo"}
	}
	quiet := append(append(append([]string{}, sudo...), "-n", "--"), argv...)
	h, err := b.launch(ctx, quiet, false)
	if err == nil {
		return h, nil
	}
	var se *startError
	if !errors.As(err, &se) || !se.exited {
		return nil, err
	}
	if !hasTerminal() {
		return nil, fmt.Errorf("become: %s could not run scriptling's root helper without a password, and there is "+
			"no terminal to ask for one on: give this user passwordless sudo, or run the script as root (%s)",
			sudo[0], se.detail())
	}
	asking := append(append(append([]string{}, sudo...), "--"), argv...)
	return b.launch(ctx, asking, true)
}

// startError is a helper that did not start.
type startError struct {
	program string
	exited  bool // the program ran and ended before the helper said hello
	stderr  string
	err     error
}

func (e *startError) detail() string {
	if s := strings.TrimSpace(e.stderr); s != "" {
		return s
	}
	return e.err.Error()
}

func (e *startError) Error() string {
	return fmt.Sprintf("become: starting the root helper with %s: %s", e.program, e.detail())
}

// launch starts argv and waits for the helper's hello. interactive lets the
// program (sudo) talk to the terminal.
func (b *Become) launch(ctx context.Context, argv []string, interactive bool) (*helper, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &limitedBuffer{max: 4096}
	if interactive {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = stderr
	}
	if err := cmd.Start(); err != nil {
		return nil, &startError{program: argv[0], err: err}
	}
	h := &helper{cmd: cmd, stdin: stdin, enc: gob.NewEncoder(stdin), dec: gob.NewDecoder(bufio.NewReader(stdout))}

	got := make(chan error, 1)
	var hi hello
	go func() { got <- h.dec.Decode(&hi) }()
	select {
	case err := <-got:
		if err != nil {
			h.kill()
			_ = cmd.Wait()
			return nil, &startError{program: argv[0], exited: true, stderr: stderr.String(), err: err}
		}
	case <-ctx.Done():
		h.stop()
		return nil, ctx.Err()
	}
	if hi.Version != protocolVersion {
		h.stop()
		return nil, fmt.Errorf("become: the root helper speaks version %d, not %d", hi.Version, protocolVersion)
	}
	if !hi.Root && b.helperRoot {
		h.stop()
		return nil, fmt.Errorf("become: the root helper started with %s is not running as root", argv[0])
	}
	return h, nil
}

// call sends one operation and waits for its answer, or for ctx: then the
// helper is stopped, and with it what it was running.
func (h *helper) call(ctx context.Context, op string, args []byte) ([]byte, error) {
	if err := h.enc.Encode(request{Op: op, Args: args}); err != nil {
		return nil, &helperDied{err}
	}
	type answer struct {
		r   response
		err error
	}
	got := make(chan answer, 1)
	go func() {
		var a answer
		a.err = h.dec.Decode(&a.r)
		got <- a
	}()
	select {
	case a := <-got:
		if a.err != nil {
			return nil, &helperDied{a.err}
		}
		if a.r.Err != "" {
			return nil, &remoteError{msg: a.r.Err, denied: a.r.Denied}
		}
		return a.r.Result, nil
	case <-ctx.Done():
		h.stop()
		return nil, ctx.Err()
	}
}

// stop ends the helper: its input closes and it is asked to stop (sudo passes
// the signal on), so what it runs is stopped too; after a grace period it is killed.
func (h *helper) stop() {
	h.once.Do(func() {
		_ = h.stdin.Close()
		terminate(h.cmd.Process)
		done := make(chan struct{})
		go func() {
			_ = h.cmd.Wait()
			close(done)
		}()
		go func() {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = h.cmd.Process.Kill()
			}
		}()
	})
}

func (h *helper) kill() {
	_ = h.stdin.Close()
	_ = h.cmd.Process.Kill()
}

func hasTerminal() bool {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// ServeIfHelper runs the helper and exits when this program was started as
// one (its first argument is HelperArg); otherwise it returns at once. A
// program that enables become calls it first thing in main.
func ServeIfHelper() {
	if len(os.Args) < 2 || os.Args[1] != HelperArg {
		return
	}
	os.Exit(serve(os.Args[2:], os.Stdin, os.Stdout))
}

// serve answers operations on in and out until in closes or a signal stops it.
func serve(args []string, in io.Reader, out io.Writer) int {
	var paths fssecurity.Config
	if len(args) > 0 {
		if err := json.Unmarshal([]byte(args[0]), &paths.AllowedPaths); err != nil {
			fmt.Fprintf(os.Stderr, "scriptling root helper: bad allowed paths %q: %v\n", args[0], err)
			return 2
		}
	}
	ctx, stopSignals := signalContext()
	defer stopSignals()

	w := bufio.NewWriter(out)
	enc := gob.NewEncoder(w)
	dec := gob.NewDecoder(bufio.NewReader(in))
	if err := enc.Encode(hello{Version: protocolVersion, Root: IsRoot()}); err != nil || w.Flush() != nil {
		return 1
	}

	var busy sync.Mutex
	finished := make(chan struct{})
	defer close(finished) // runs before stopSignals: a clean end is not a signal
	go func() {
		select {
		case <-ctx.Done(): // a signal: let the running operation see it end, then go
		case <-finished:
			return
		}
		select {
		case <-finished: // ctx ended because serve did: not a signal
			return
		default:
		}
		busy.Lock()
		os.Exit(1)
	}()

	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return 0
			}
			return 1
		}
		busy.Lock()
		resp := answer(ctx, paths, req)
		busy.Unlock()
		if err := enc.Encode(resp); err != nil || w.Flush() != nil {
			return 1
		}
	}
}

func answer(ctx context.Context, paths fssecurity.Config, req request) (resp response) {
	defer func() {
		if p := recover(); p != nil {
			resp = response{Err: fmt.Sprintf("the root helper failed in %s: %v", req.Op, p)}
		}
	}()
	h, ok := handlers[req.Op]
	if !ok {
		return response{Err: fmt.Sprintf("the root helper does not know %q", req.Op)}
	}
	result, err := h(ctx, paths, req.Args)
	if err != nil {
		return response{Err: err.Error(), Denied: errors.Is(err, fssecurity.ErrPathNotAllowed)}
	}
	return response{Result: result}
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode(raw []byte, v any) error {
	return gob.NewDecoder(bytes.NewReader(raw)).Decode(v)
}

// limitedBuffer keeps the first max bytes written to it.
type limitedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

func (l *limitedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}
