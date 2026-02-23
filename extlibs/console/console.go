package console

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const LibraryName = "scriptling.console"

// ConsoleBackend is the interface scriptling.console calls through.
// The CLI registers a TUI-backed implementation via SetBackend().
type ConsoleBackend interface {
	Input(prompt string, env *object.Environment) (string, error)
	Print(text string, env *object.Environment)
	PrintAs(label, text string, env *object.Environment)
	StreamStart()
	StreamStartAs(label string)
	StreamChunk(chunk string)
	StreamEnd()
	SpinnerStart(text string)
	SpinnerStop()
	SetProgress(label string, pct float64)
	SetLabels(user, assistant, system string)
	SetStatus(left, right string)
	SetStatusLeft(left string)
	SetStatusRight(right string)
	RegisterCommand(name, description string, handler func(args string))
	RemoveCommand(name string)
	OnSubmit(fn func(ctx context.Context, text string))
	OnEscape(fn func())
	ClearOutput()
	Run() error
}

var (
	mu      sync.RWMutex
	backend ConsoleBackend = &lazyBackend{}
)

// SetBackend registers a custom backend (e.g. TUI). Call before running scripts.
func SetBackend(b ConsoleBackend) {
	mu.Lock()
	backend = b
	mu.Unlock()
}

// GetBackend returns the current backend.
func GetBackend() ConsoleBackend {
	mu.RLock()
	defer mu.RUnlock()
	return backend
}

func getBackend() ConsoleBackend {
	mu.RLock()
	defer mu.RUnlock()
	return backend
}

// lazyBackend is the default backend. It stores callbacks and state set by the
// script before console.run() is called, then on Run() creates the real TUI
// backend, transfers everything to it, and starts it. If Run() is never called
// (e.g. telegram bot), plain stdout I/O is used for Input/Print.
type lazyBackend struct {
	mu       sync.Mutex
	submitCb func(context.Context, string)
	escapeCb func()
	initCb   func()
}

func (l *lazyBackend) Input(prompt string, env *object.Environment) (string, error) {
	if prompt != "" {
		fmt.Fprint(env.GetWriter(), prompt)
	}
	scanner := bufio.NewScanner(env.GetReader())
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("EOF")
	}
	return scanner.Text(), nil
}
func (l *lazyBackend) Print(text string, env *object.Environment) { fmt.Fprint(env.GetWriter(), text) }
func (l *lazyBackend) PrintAs(_, text string, env *object.Environment) {
	fmt.Fprint(env.GetWriter(), text)
}
func (l *lazyBackend) StreamStart()                                {}
func (l *lazyBackend) StreamStartAs(_ string)                      {}
func (l *lazyBackend) StreamChunk(_ string)                        {}
func (l *lazyBackend) StreamEnd()                                  {}
func (l *lazyBackend) SpinnerStart(_ string)                       {}
func (l *lazyBackend) SpinnerStop()                                {}
func (l *lazyBackend) SetProgress(_ string, _ float64)             {}
func (l *lazyBackend) SetLabels(_, _, _ string)                    {}
func (l *lazyBackend) SetStatus(_, _ string)                       {}
func (l *lazyBackend) SetStatusLeft(_ string)                      {}
func (l *lazyBackend) SetStatusRight(_ string)                     {}
func (l *lazyBackend) RegisterCommand(_, _ string, _ func(string)) {}
func (l *lazyBackend) RemoveCommand(_ string)                      {}
func (l *lazyBackend) ClearOutput()                                {}
func (l *lazyBackend) OnSubmit(fn func(context.Context, string)) {
	l.mu.Lock()
	l.submitCb = fn
	l.mu.Unlock()
}
func (l *lazyBackend) OnEscape(fn func()) {
	l.mu.Lock()
	l.escapeCb = fn
	l.mu.Unlock()
}
func (l *lazyBackend) OnInit(fn func()) {
	l.mu.Lock()
	l.initCb = fn
	l.mu.Unlock()
}
func (l *lazyBackend) Run() error {
	l.mu.Lock()
	scb := l.submitCb
	ecb := l.escapeCb
	icb := l.initCb
	l.mu.Unlock()
	tb := newTUIBackend()
	if scb != nil {
		tb.OnSubmit(scb)
	}
	if ecb != nil {
		tb.OnEscape(ecb)
	}
	SetBackend(tb)
	if icb != nil {
		icb()
	}
	return tb.Run()
}

// getEnv retrieves the environment from context.
func getEnv(ctx context.Context) *object.Environment {
	// Use the evaluator's context key via the object package helper
	type envKey = string
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment()
}

// NewLibrary creates the scriptling.console library.
func NewLibrary() *object.Library {
	fns := map[string]*object.Builtin{
		"input": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				prompt := ""
				if len(args) > 0 {
					s, err := args[0].AsString()
					if err != nil {
						return err
					}
					prompt = s
				}
				env := getEnv(ctx)
				text, err := getBackend().Input(prompt, env)
				if err != nil {
					return errors.NewError("input: %s", err.Error())
				}
				return &object.String{Value: text}
			},
			HelpText: "input([prompt]) -> str — read a line from input",
		},
		"print": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				parts := make([]string, len(args))
				for i, a := range args {
					parts[i] = a.Inspect()
				}
				text := strings.Join(parts, " ") + "\n"
				if kwargs.Has("label") {
					label, _ := kwargs.GetString("label", "")
					getBackend().PrintAs(label, text, getEnv(ctx))
					return &object.Null{}
				}
				getBackend().Print(text, getEnv(ctx))
				return &object.Null{}
			},
			HelpText: "print(*args, [label=]) — write to console output, optionally with a custom label",
		},
		"stream_start": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if kwargs.Has("label") {
					label, _ := kwargs.GetString("label", "")
					getBackend().StreamStartAs(label)
					return &object.Null{}
				}
				getBackend().StreamStart()
				return &object.Null{}
			},
			HelpText: "stream_start([label=]) — begin a streaming message, optionally with a custom label",
		},
		"stream_chunk": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					s, err := args[0].AsString()
					if err != nil {
						return err
					}
					getBackend().StreamChunk(s)
				}
				return &object.Null{}
			},
			HelpText: "stream_chunk(text) — append a chunk to the current stream",
		},
		"stream_end": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				getBackend().StreamEnd()
				return &object.Null{}
			},
			HelpText: "stream_end() — finalise the current stream",
		},
		"spinner_start": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				text := "Working"
				if len(args) > 0 {
					if s, err := args[0].AsString(); err == nil {
						text = s
					}
				}
				getBackend().SpinnerStart(text)
				return &object.Null{}
			},
			HelpText: "spinner_start([text]) — show a spinner",
		},
		"spinner_stop": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				getBackend().SpinnerStop()
				return &object.Null{}
			},
			HelpText: "spinner_stop() — hide the spinner",
		},
		"set_progress": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				label := ""
				pct := -1.0
				if len(args) > 0 {
					if s, err := args[0].AsString(); err == nil {
						label = s
					}
				}
				if len(args) > 1 {
					if f, err := args[1].AsFloat(); err == nil {
						pct = f
					}
				}
				getBackend().SetProgress(label, pct)
				return &object.Null{}
			},
			HelpText: "set_progress(label, pct) — set progress bar (0.0–1.0, or <0 to clear)",
		},
		"set_labels": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				user, assistant, system := "", "", ""
				if len(args) > 0 {
					if s, err := args[0].AsString(); err == nil {
						user = s
					}
				}
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						assistant = s
					}
				}
				if len(args) > 2 {
					if s, err := args[2].AsString(); err == nil {
						system = s
					}
				}
				getBackend().SetLabels(user, assistant, system)
				return &object.Null{}
			},
			HelpText: "set_labels(user, assistant, system) — set default role labels; empty string leaves label unchanged",
		},
		"set_status": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				left, right := "", ""
				if len(args) > 0 {
					if s, err := args[0].AsString(); err == nil {
						left = s
					}
				}
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						right = s
					}
				}
				getBackend().SetStatus(left, right)
				return &object.Null{}
			},
			HelpText: "set_status(left, right) — set both status bar texts",
		},
		"set_status_left": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					if s, err := args[0].AsString(); err == nil {
						getBackend().SetStatusLeft(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_status_left(text) — set left status bar text",
		},
		"set_status_right": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					if s, err := args[0].AsString(); err == nil {
						getBackend().SetStatusRight(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_status_right(text) — set right status bar text",
		},
		"register_command": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 3 {
					return &object.Null{}
				}
				name, err := args[0].AsString()
				if err != nil {
					return err
				}
				desc, err := args[1].AsString()
				if err != nil {
					return err
				}
				fn := args[2]
				eval := evaliface.FromContext(ctx)
				env := getEnv(ctx)
				getBackend().RegisterCommand(name, desc, func(cmdArgs string) {
					if eval != nil {
						eval.CallObjectFunction(context.Background(), fn,
							[]object.Object{&object.String{Value: cmdArgs}}, nil, env)
					}
				})
				return &object.Null{}
			},
			HelpText: "register_command(name, description, fn) — register a slash command with the backend",
		},
		"remove_command": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					if name, err := args[0].AsString(); err == nil {
						getBackend().RemoveCommand(name)
					}
				}
				return &object.Null{}
			},
			HelpText: "remove_command(name) — remove a registered slash command",
		},
		"clear_output": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				getBackend().ClearOutput()
				return &object.Null{}
			},
			HelpText: "clear_output() — clear the output area",
		},
		"styled": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 2 {
					return &object.String{Value: ""}
				}
				color, err := args[0].AsString()
				if err != nil {
					return err
				}
				text, err := args[1].AsString()
				if err != nil {
					return err
				}
				type styledBackend interface{ Styled(string, string) string }
				if sb, ok := getBackend().(styledBackend); ok {
					return &object.String{Value: sb.Styled(color, text)}
				}
				return &object.String{Value: text}
			},
			HelpText: "styled(color, text) — apply theme color to text; colors: primary, secondary, error, dim, user, text",
		},
		"on_init": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					fn := args[0]
					eval := evaliface.FromContext(ctx)
					env := getEnv(ctx)
					if lb, ok := getBackend().(*lazyBackend); ok {
						lb.OnInit(func() {
							if eval != nil {
								eval.CallObjectFunction(context.Background(), fn, nil, nil, env)
							}
						})
					}
				}
				return &object.Null{}
			},
			HelpText: "on_init(fn) — register a callback invoked once when the console starts (before run() blocks)",
		},
		"on_escape": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					fn := args[0]
					eval := evaliface.FromContext(ctx)
					env := getEnv(ctx)
					getBackend().OnEscape(func() {
						if eval != nil {
							eval.CallObjectFunction(context.Background(), fn, nil, nil, env)
						}
					})
				}
				return &object.Null{}
			},
			HelpText: "on_escape(fn) — register a callback for Esc key (TUI only)",
		},
		"on_submit": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 0 {
					fn := args[0]
					eval := evaliface.FromContext(ctx)
					env := getEnv(ctx)
					getBackend().OnSubmit(func(submitCtx context.Context, text string) {
						if eval != nil {
							eval.CallObjectFunction(submitCtx, fn,
								[]object.Object{&object.String{Value: text}}, nil, env)
						}
					})
				}
				return &object.Null{}
			},
			HelpText: "on_submit(fn) — register handler called when user submits input",
		},
		"run": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := getBackend().Run(); err != nil {
					return errors.NewError("console.run: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: "run() — start the console event loop (blocks until exit)",
		},
	}
	return object.NewLibrary(LibraryName, fns, map[string]object.Object{
		"PRIMARY":   &object.String{Value: "primary"},
		"SECONDARY": &object.String{Value: "secondary"},
		"ERROR":     &object.String{Value: "error"},
		"DIM":       &object.String{Value: "dim"},
		"USER":      &object.String{Value: "user"},
		"TEXT":      &object.String{Value: "text"},
	}, "Console I/O with TUI backend")
}

// Register registers the console library with a scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(NewLibrary())
}
