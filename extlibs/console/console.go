package console

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/paularlott/cli/tui"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const LibraryName = "scriptling.console"

const nativeTUIKey = "__tui__"

// tuiWrapper holds the *tui.TUI and its callbacks, stored in Instance.Fields.
type tuiWrapper struct {
	t        *tui.TUI
	escapeCb func()
	submitCb func(context.Context, string)
	mu       sync.Mutex
	cancel   context.CancelFunc
	prevDone chan struct{}
}

func (w *tuiWrapper) Type() object.ObjectType                          { return object.BUILTIN_OBJ }
func (w *tuiWrapper) Inspect() string                                  { return "<Console>" }
func (w *tuiWrapper) AsString() (string, object.Object)                { return "<Console>", nil }
func (w *tuiWrapper) AsInt() (int64, object.Object)                    { return 0, nil }
func (w *tuiWrapper) AsFloat() (float64, object.Object)                { return 0, nil }
func (w *tuiWrapper) AsBool() (bool, object.Object)                    { return true, nil }
func (w *tuiWrapper) AsList() ([]object.Object, object.Object)         { return nil, nil }
func (w *tuiWrapper) AsDict() (map[string]object.Object, object.Object) { return nil, nil }
func (w *tuiWrapper) CoerceString() (string, object.Object)            { return "<Console>", nil }
func (w *tuiWrapper) CoerceInt() (int64, object.Object)                { return 0, nil }
func (w *tuiWrapper) CoerceFloat() (float64, object.Object)            { return 0, nil }

func newTUIWrapper() *tuiWrapper {
	w := &tuiWrapper{prevDone: make(chan struct{})}
	close(w.prevDone)

	var t *tui.TUI
	t = tui.New(tui.Config{
		StatusRight: "Ctrl+C to exit",
		Commands: []*tui.Command{
			{Name: "exit", Description: "Exit", Handler: func(_ string) { t.Exit() }},
		},
		OnEscape: func() {
			w.mu.Lock()
			if w.cancel != nil {
				w.cancel()
			}
			cb := w.escapeCb
			w.mu.Unlock()
			if cb != nil {
				go cb()
			}
		},
		OnSubmit: func(line string) {
			t.AddMessage(tui.RoleUser, line)
			w.mu.Lock()
			scb := w.submitCb
			ecb := w.escapeCb
			if w.cancel != nil {
				w.cancel()
				if ecb != nil {
					go ecb()
				}
			}
			ctx, c := context.WithCancel(context.Background())
			w.cancel = c
			waitFor := w.prevDone
			nextDone := make(chan struct{})
			w.prevDone = nextDone
			w.mu.Unlock()
			if scb == nil {
				c()
				close(nextDone)
				return
			}
			go func() {
				defer func() {
					w.mu.Lock()
					w.cancel = nil
					w.mu.Unlock()
					c()
					close(nextDone)
				}()
				<-waitFor
				scb(ctx, line)
			}()
		},
	})
	w.t = t
	return w
}

// wrapperFrom extracts the tuiWrapper from a Console instance (args[0]).
func wrapperFrom(args []object.Object) *tuiWrapper {
	return args[0].(*object.Instance).Fields[nativeTUIKey].(*tuiWrapper)
}

func envFromCtx(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment()
}

func applyStyle(t *tui.TUI, color, text string) string {
	theme := t.Theme()
	var c tui.Color
	switch color {
	case "primary":
		c = theme.Primary
	case "secondary":
		c = theme.Secondary
	case "error":
		c = theme.Error
	case "dim":
		c = theme.Dim
	case "user":
		c = theme.UserText
	default:
		s := strings.TrimPrefix(color, "#")
		if len(s) == 6 {
			if v, err := strconv.ParseUint(s, 16, 32); err == nil {
				return tui.Styled(tui.Color(v), text)
			}
		}
		c = theme.Text
	}
	return tui.Styled(c, text)
}

var consoleClass = &object.Class{
	Name: "Console",
	Methods: map[string]object.Object{
		"__init__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				inst := args[0].(*object.Instance)
				inst.Fields[nativeTUIKey] = newTUIWrapper()
				return &object.Null{}
			},
			HelpText: "__init__() — create a new Console instance backed by a TUI",
		},
		"add_message": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				w := wrapperFrom(args)
				parts := make([]string, len(args)-1)
				for i, a := range args[1:] {
					parts[i] = a.Inspect()
				}
				text := strings.Join(parts, " ")
				label, _ := kwargs.GetString("label", "")
				if label != "" {
					w.t.AddMessageAs(tui.RoleAssistant, label, text)
				} else {
					w.t.AddMessage(tui.RoleAssistant, text)
				}
				return &object.Null{}
			},
			HelpText: "add_message(*args, [label=]) — add a message to the output area",
		},
		"stream_start": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				w := wrapperFrom(args)
				label, _ := kwargs.GetString("label", "")
				if label != "" {
					w.t.StartStreamingAs(label)
				} else {
					w.t.StartStreaming()
				}
				return &object.Null{}
			},
			HelpText: "stream_start([label=]) — begin a streaming message",
		},
		"stream_chunk": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						wrapperFrom(args).t.StreamChunk(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "stream_chunk(text) — append a chunk to the current stream",
		},
		"stream_end": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				wrapperFrom(args).t.StreamComplete()
				return &object.Null{}
			},
			HelpText: "stream_end() — finalise the current stream",
		},
		"spinner_start": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				text := "Working"
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						text = s
					}
				}
				wrapperFrom(args).t.StartSpinner(text)
				return &object.Null{}
			},
			HelpText: "spinner_start([text]) — show a spinner",
		},
		"spinner_stop": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				wrapperFrom(args).t.StopSpinner()
				return &object.Null{}
			},
			HelpText: "spinner_stop() — hide the spinner",
		},
		"set_progress": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				label := ""
				pct := -1.0
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						label = s
					}
				}
				if len(args) > 2 {
					if f, err := args[2].AsFloat(); err == nil {
						pct = f
					}
				}
				t := wrapperFrom(args).t
				if pct < 0 {
					t.ClearProgress()
				} else {
					t.SetProgress(label, pct)
				}
				return &object.Null{}
			},
			HelpText: "set_progress(label, pct) — set progress bar (0.0–1.0, or <0 to clear)",
		},
		"set_labels": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				user, assistant, system := "", "", ""
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						user = s
					}
				}
				if len(args) > 2 {
					if s, err := args[2].AsString(); err == nil {
						assistant = s
					}
				}
				if len(args) > 3 {
					if s, err := args[3].AsString(); err == nil {
						system = s
					}
				}
				wrapperFrom(args).t.SetLabels(user, assistant, system)
				return &object.Null{}
			},
			HelpText: "set_labels(user, assistant, system) — set role labels; empty string leaves label unchanged",
		},
		"set_status": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				left, right := "", ""
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						left = s
					}
				}
				if len(args) > 2 {
					if s, err := args[2].AsString(); err == nil {
						right = s
					}
				}
				wrapperFrom(args).t.SetStatus(left, right)
				return &object.Null{}
			},
			HelpText: "set_status(left, right) — set both status bar texts",
		},
		"set_status_left": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						wrapperFrom(args).t.SetStatusLeft(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_status_left(text) — set left status bar text",
		},
		"set_status_right": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						wrapperFrom(args).t.SetStatusRight(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_status_right(text) — set right status bar text",
		},
		"register_command": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 4 {
					return &object.Null{}
				}
				name, err := args[1].AsString()
				if err != nil {
					return err
				}
				desc, err := args[2].AsString()
				if err != nil {
					return err
				}
				fn := args[3]
				eval := evaliface.FromContext(ctx)
				env := envFromCtx(ctx)
				wrapperFrom(args).t.AddCommand(&tui.Command{
					Name:        name,
					Description: desc,
					Handler: func(cmdArgs string) {
						if eval != nil {
							eval.CallObjectFunction(context.Background(), fn,
								[]object.Object{&object.String{Value: cmdArgs}}, nil, env)
						}
					},
				})
				return &object.Null{}
			},
			HelpText: "register_command(name, description, fn) — register a slash command",
		},
		"remove_command": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					if name, err := args[1].AsString(); err == nil {
						wrapperFrom(args).t.RemoveCommand(name)
					}
				}
				return &object.Null{}
			},
			HelpText: "remove_command(name) — remove a registered slash command",
		},
		"clear_output": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				wrapperFrom(args).t.ClearOutput()
				return &object.Null{}
			},
			HelpText: "clear_output() — clear the output area",
		},
		"styled": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) < 3 {
					return &object.String{Value: ""}
				}
				color, err := args[1].AsString()
				if err != nil {
					return err
				}
				text, err := args[2].AsString()
				if err != nil {
					return err
				}
				return &object.String{Value: applyStyle(wrapperFrom(args).t, color, text)}
			},
			HelpText: "styled(color, text) — apply theme color to text",
		},
		"on_escape": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					fn := args[1]
					eval := evaliface.FromContext(ctx)
					env := envFromCtx(ctx)
					w := wrapperFrom(args)
					w.mu.Lock()
					w.escapeCb = func() {
						if eval != nil {
							eval.CallObjectFunction(context.Background(), fn, nil, nil, env)
						}
					}
					w.mu.Unlock()
				}
				return &object.Null{}
			},
			HelpText: "on_escape(fn) — register a callback for Esc key",
		},
		"on_submit": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					fn := args[1]
					eval := evaliface.FromContext(ctx)
					env := envFromCtx(ctx)
					w := wrapperFrom(args)
					w.mu.Lock()
					w.submitCb = func(submitCtx context.Context, text string) {
						if eval != nil {
							eval.CallObjectFunction(submitCtx, fn,
								[]object.Object{&object.String{Value: text}}, nil, env)
						}
					}
					w.mu.Unlock()
				}
				return &object.Null{}
			},
			HelpText: "on_submit(fn) — register handler called when user submits input",
		},
		"run": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := wrapperFrom(args).t.Run(context.Background()); err != nil {
					return errors.NewError("console.run: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: "run() — start the console event loop (blocks until exit)",
		},
	},
}

// NewLibrary creates the scriptling.console library.
func NewLibrary() *object.Library {
	return object.NewLibrary(LibraryName, nil, map[string]object.Object{
		"Console":   consoleClass,
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
