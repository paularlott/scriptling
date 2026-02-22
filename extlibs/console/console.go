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
// The default implementation uses env.GetReader()/GetWriter().
// The CLI registers a TUI-backed implementation via SetBackend().
type ConsoleBackend interface {
	Input(prompt string, env *object.Environment) (string, error)
	Print(text string, env *object.Environment)
	StreamStart()
	StreamChunk(chunk string)
	StreamEnd()
	SpinnerStart(text string)
	SpinnerStop()
	SetProgress(label string, pct float64)
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
	backend ConsoleBackend = &plainBackend{}
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

// plainBackend is the default plain-I/O implementation.
type plainBackend struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (p *plainBackend) Input(prompt string, env *object.Environment) (string, error) {
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

func (p *plainBackend) Print(text string, env *object.Environment) {
	fmt.Fprint(env.GetWriter(), text)
}

func (p *plainBackend) StreamStart() {
	p.mu.Lock()
	p.buf.Reset()
	p.mu.Unlock()
}

func (p *plainBackend) StreamChunk(chunk string) {
	p.mu.Lock()
	p.buf.WriteString(chunk)
	p.mu.Unlock()
}

func (p *plainBackend) StreamEnd() {
	p.mu.Lock()
	s := p.buf.String()
	p.buf.Reset()
	p.mu.Unlock()
	fmt.Println(s)
}

func (p *plainBackend) SpinnerStart(text string) { fmt.Println(text + "...") }
func (p *plainBackend) SpinnerStop()              { fmt.Println() }

func (p *plainBackend) SetProgress(label string, pct float64) {
	if pct >= 0 {
		fmt.Printf("%s: %.0f%%\n", label, pct*100)
	}
}

func (p *plainBackend) SetStatus(_, _ string)                        {}
func (p *plainBackend) SetStatusLeft(_ string)                       {}
func (p *plainBackend) SetStatusRight(_ string)                      {}
func (p *plainBackend) RegisterCommand(_, _ string, _ func(string))  {}
func (p *plainBackend) RemoveCommand(_ string)                        {}
func (p *plainBackend) OnSubmit(_ func(context.Context, string)) {}
func (p *plainBackend) OnEscape(_ func())                            {}
func (p *plainBackend) ClearOutput()                                 {}
func (p *plainBackend) Run() error                                   { return nil }

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
				getBackend().Print(strings.Join(parts, " ")+"\n", getEnv(ctx))
				return &object.Null{}
			},
			HelpText: "print(*args) — write to console output",
		},
		"stream_start": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				getBackend().StreamStart()
				return &object.Null{}
			},
			HelpText: "stream_start() — begin a streaming message",
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
	return object.NewLibrary(LibraryName, fns, nil, "Console I/O with optional TUI backend")
}

// Register registers the console library with a scriptling instance.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(NewLibrary())
}
