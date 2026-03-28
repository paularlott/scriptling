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

// Singleton state.
var (
	consoleOnce sync.Once
	consoleTUI  *tui.TUI
	consoleW    *tuiWrapper
)

// tuiWrapper holds the *tui.TUI and its callbacks.
// Internal only — not exposed to the scripting language.
type tuiWrapper struct {
	t        *tui.TUI
	escapeCb func()
	submitCb func(context.Context, string)
	mu       sync.Mutex
	cancel   context.CancelFunc
	prevDone chan struct{}
}

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

// TUI returns the shared singleton TUI instance.
func TUI() *tui.TUI {
	consoleOnce.Do(func() {
		w := newTUIWrapper()
		consoleW = w
		consoleTUI = w.t
	})
	return consoleTUI
}

// SetSubmit wires an external submit handler into the singleton console.
func SetSubmit(fn func(ctx context.Context, text string)) {
	TUI()
	consoleW.mu.Lock()
	consoleW.submitCb = fn
	consoleW.mu.Unlock()
}

// ResetConsole resets the singleton for testing.
func ResetConsole() {
	consoleOnce = sync.Once{}
	consoleTUI = nil
	consoleW = nil
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

// ─── Module-level builtins ─────────────────────────────────────────────

var moduleBuiltins = map[string]*object.Builtin{
	"panel": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			name := "main"
			if len(args) > 0 {
				if s, err := args[0].AsString(); err == nil {
					name = s
				}
			}
			if name == "main" {
				return newMainPanelInstance(TUI())
			}
			nativePanel := TUI().Panel(name)
			if nativePanel == nil {
				return &object.Null{}
			}
			return newPanelInstance(nativePanel, TUI())
		},
		HelpText: "panel([name]) — get a Panel by name (default: 'main')",
	},
	"main_panel": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return newMainPanelInstance(TUI())
		},
		HelpText: "main_panel() — get the main chat panel",
	},
	"create_panel": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			cfg := tui.PanelConfig{}
			if len(args) > 0 {
				if s, err := args[0].AsString(); err == nil {
					cfg.Name = s
				}
			}
			if v, err := kwargs.GetInt("width", 0); err == nil && v != 0 {
				cfg.Width = int(v)
			}
			if v, err := kwargs.GetInt("height", 0); err == nil && v != 0 {
				cfg.Height = int(v)
			}
			if v, err := kwargs.GetInt("min_width", 0); err == nil && v != 0 {
				cfg.MinWidth = int(v)
			}
			if v, err := kwargs.GetBool("scrollable", false); err == nil && v {
				cfg.Scrollable = true
			}
			if v, err := kwargs.GetString("title", ""); err == nil && v != "" {
				cfg.Title = v
			}
			if v, err := kwargs.GetBool("no_border", false); err == nil && v {
				cfg.NoBorder = true
			}
			if v, err := kwargs.GetBool("skip_focus", false); err == nil && v {
				cfg.SkipFocus = true
			}
			nativePanel := TUI().CreatePanel(cfg)
			return newPanelInstance(nativePanel, TUI())
		},
		HelpText: "create_panel([name], [width=], [height=], ...) — create a panel",
	},
	"add_left": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("console.add_left: panel required")
			}
			pw, ok := args[0].(*object.Instance).Fields[nativePanelKey].(*panelWrapper)
			if !ok || pw.p == nil {
				return errors.NewError("console.add_left: expected a named Panel")
			}
			TUI().AddLeft(pw.p)
			return &object.Null{}
		},
		HelpText: "add_left(panel) — add a panel to the left of main",
	},
	"add_right": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewError("console.add_right: panel required")
			}
			pw, ok := args[0].(*object.Instance).Fields[nativePanelKey].(*panelWrapper)
			if !ok || pw.p == nil {
				return errors.NewError("console.add_right: expected a named Panel")
			}
			TUI().AddRight(pw.p)
			return &object.Null{}
		},
		HelpText: "add_right(panel) — add a panel to the right of main",
	},
	"clear_layout": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			TUI().ClearLayout()
			return &object.Null{}
		},
		HelpText: "clear_layout() — remove layout but keep panels",
	},
	"has_panels": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Boolean{Value: TUI().HasMultiplePanels()}
		},
		HelpText: "has_panels() — return True if multi-panel layout is active",
	},
	"styled": &object.Builtin{
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
			return &object.String{Value: applyStyle(TUI(), color, text)}
		},
		HelpText: "styled(color, text) — apply theme color to text",
	},
	"set_status": &object.Builtin{
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
			TUI().SetStatus(left, right)
			return &object.Null{}
		},
		HelpText: "set_status(left, right) — set both status bar texts",
	},
	"set_status_left": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) > 0 {
				if s, err := args[0].AsString(); err == nil {
					TUI().SetStatusLeft(s)
				}
			}
			return &object.Null{}
		},
		HelpText: "set_status_left(text) — set left status bar text",
	},
	"set_status_right": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) > 0 {
				if s, err := args[0].AsString(); err == nil {
					TUI().SetStatusRight(s)
				}
			}
			return &object.Null{}
		},
		HelpText: "set_status_right(text) — set right status bar text",
	},
	"set_labels": &object.Builtin{
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
			TUI().SetLabels(user, assistant, system)
			return &object.Null{}
		},
		HelpText: "set_labels(user, assistant, system) — set role labels",
	},
	"register_command": &object.Builtin{
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
			env := envFromCtx(ctx)
			TUI().AddCommand(&tui.Command{
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
			if len(args) > 0 {
				if name, err := args[0].AsString(); err == nil {
					TUI().RemoveCommand(name)
				}
			}
			return &object.Null{}
		},
		HelpText: "remove_command(name) — remove a registered slash command",
	},
	"on_submit": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return &object.Null{}
			}
			fn := args[0]
			eval := evaliface.FromContext(ctx)
			env := envFromCtx(ctx)
			TUI()
			consoleW.mu.Lock()
			consoleW.submitCb = func(submitCtx context.Context, text string) {
				if eval != nil {
					eval.CallObjectFunction(submitCtx, fn,
						[]object.Object{&object.String{Value: text}}, nil, env)
				}
			}
			consoleW.mu.Unlock()
			return &object.Null{}
		},
		HelpText: "on_submit(fn) — register handler called when user submits input",
	},
	"on_escape": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return &object.Null{}
			}
			fn := args[0]
			eval := evaliface.FromContext(ctx)
			env := envFromCtx(ctx)
			TUI()
			consoleW.mu.Lock()
			consoleW.escapeCb = func() {
				if eval != nil {
					eval.CallObjectFunction(context.Background(), fn, nil, nil, env)
				}
			}
			consoleW.mu.Unlock()
			return &object.Null{}
		},
		HelpText: "on_escape(fn) — register a callback for Esc key",
	},
	"spinner_start": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text := "Working"
			if len(args) > 0 {
				if s, err := args[0].AsString(); err == nil {
					text = s
				}
			}
			TUI().StartSpinner(text)
			return &object.Null{}
		},
		HelpText: "spinner_start([text]) — show a spinner",
	},
	"spinner_stop": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			TUI().StopSpinner()
			return &object.Null{}
		},
		HelpText: "spinner_stop() — hide the spinner",
	},
	"set_progress": &object.Builtin{
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
			t := TUI()
			if pct < 0 {
				t.ClearProgress()
			} else {
				t.SetProgress(label, pct)
			}
			return &object.Null{}
		},
		HelpText: "set_progress(label, pct) — set progress bar (0.0–1.0, or <0 to clear)",
	},
	"run": &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := TUI().Run(context.Background()); err != nil {
				return errors.NewError("console.run: %s", err.Error())
			}
			return &object.Null{}
		},
		HelpText: "run() — start the console event loop (blocks until exit)",
	},
}

// ─── Panel ────────────────────────────────────────────────────────────

const nativePanelKey = "__panel__"

// panelWrapper holds a *tui.Panel and the parent TUI.
// For the main chat panel, p is nil and methods dispatch to the TUI.
type panelWrapper struct {
	p *tui.Panel // nil for main panel
	t *tui.TUI
}

func (w *panelWrapper) Type() object.ObjectType                          { return object.BUILTIN_OBJ }
func (w *panelWrapper) Inspect() string                                  { return "<Panel>" }
func (w *panelWrapper) AsString() (string, object.Object)                { return "<Panel>", nil }
func (w *panelWrapper) AsInt() (int64, object.Object)                    { return 0, nil }
func (w *panelWrapper) AsFloat() (float64, object.Object)                { return 0, nil }
func (w *panelWrapper) AsBool() (bool, object.Object)                    { return true, nil }
func (w *panelWrapper) AsList() ([]object.Object, object.Object)         { return nil, nil }
func (w *panelWrapper) AsDict() (map[string]object.Object, object.Object) { return nil, nil }
func (w *panelWrapper) CoerceString() (string, object.Object)            { return "<Panel>", nil }
func (w *panelWrapper) CoerceInt() (int64, object.Object)                { return 0, nil }
func (w *panelWrapper) CoerceFloat() (float64, object.Object)            { return 0, nil }

// Dispatch helpers for methods that work on both main and named panels.

func (pw *panelWrapper) panelAddMessage(text, label string) {
	if pw.p != nil {
		if label != "" {
			pw.p.AddMessageAs(tui.RoleAssistant, label, text)
		} else {
			pw.p.AddMessage(tui.RoleAssistant, text)
		}
	} else {
		if label != "" {
			pw.t.AddMessageAs(tui.RoleAssistant, label, text)
		} else {
			pw.t.AddMessage(tui.RoleAssistant, text)
		}
	}
}

func (pw *panelWrapper) panelStartStreaming(label string) {
	if pw.p != nil {
		if label != "" {
			pw.p.StartStreamingAs(label)
		} else {
			pw.p.StartStreaming()
		}
	} else {
		if label != "" {
			pw.t.StartStreamingAs(label)
		} else {
			pw.t.StartStreaming()
		}
	}
}

func (pw *panelWrapper) panelStreamChunk(text string) {
	if pw.p != nil {
		pw.p.StreamChunk(text)
	} else {
		pw.t.StreamChunk(text)
	}
}

func (pw *panelWrapper) panelStreamComplete() {
	if pw.p != nil {
		pw.p.StreamComplete()
	} else {
		pw.t.StreamComplete()
	}
}

func (pw *panelWrapper) panelClear() {
	if pw.p != nil {
		pw.p.Clear()
	} else {
		pw.t.ClearOutput()
	}
}

func newPanelInstance(nativePanel *tui.Panel, t *tui.TUI) *object.Instance {
	pw := &panelWrapper{p: nativePanel, t: t}
	name := nativePanel.Name()
	return &object.Instance{
		Class: panelClass,
		Fields: map[string]object.Object{
			nativePanelKey: pw,
			"__str_repr__": &object.String{Value: "<Panel: " + name + ">"},
		},
	}
}

func newMainPanelInstance(t *tui.TUI) *object.Instance {
	pw := &panelWrapper{p: nil, t: t}
	return &object.Instance{
		Class: panelClass,
		Fields: map[string]object.Object{
			nativePanelKey: pw,
			"__str_repr__": &object.String{Value: "<Panel: main>"},
		},
	}
}

func panelWrapperFrom(args []object.Object) *panelWrapper {
	return args[0].(*object.Instance).Fields[nativePanelKey].(*panelWrapper)
}

var panelClass = &object.Class{
	Name: "Panel",
	Methods: map[string]object.Object{
		"write": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						pw.p.WriteString(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "write(text) — append text to the panel",
		},
		"set_content": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						pw.p.SetContent(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_content(text) — replace all panel content",
		},
		"clear": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				panelWrapperFrom(args).panelClear()
				return &object.Null{}
			},
			HelpText: "clear() — remove all panel content",
		},
		"set_title": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						pw.p.SetTitle(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_title(title) — set the panel border title",
		},
		"set_color": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) > 1 {
					color, err := args[1].AsString()
					if err != nil {
						return err
					}
					pw.p.SetColor(colorFromName(pw.t, color))
				}
				return &object.Null{}
			},
			HelpText: "set_color(color) — set the panel border/accent color",
		},
		"set_scrollable": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) > 1 {
					if b, ok := args[1].(*object.Boolean); ok {
						pw.p.SetScrollable(b.Value)
					}
				}
				return &object.Null{}
			},
			HelpText: "set_scrollable(bool) — set whether panel content scrolls",
		},
		"add_message": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				parts := make([]string, len(args)-1)
				for i, a := range args[1:] {
					parts[i] = a.Inspect()
				}
				text := strings.Join(parts, " ")
				label, _ := kwargs.GetString("label", "")
				pw.panelAddMessage(text, label)
				return &object.Null{}
			},
			HelpText: "add_message(*args, [label=]) — add a message to the panel",
		},
		"stream_start": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				label, _ := kwargs.GetString("label", "")
				pw.panelStartStreaming(label)
				return &object.Null{}
			},
			HelpText: "stream_start([label=]) — begin a streaming message in this panel",
		},
		"stream_chunk": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if len(args) > 1 {
					if s, err := args[1].AsString(); err == nil {
						pw.panelStreamChunk(s)
					}
				}
				return &object.Null{}
			},
			HelpText: "stream_chunk(text) — append a chunk to the current stream",
		},
		"stream_end": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				panelWrapperFrom(args).panelStreamComplete()
				return &object.Null{}
			},
			HelpText: "stream_end() — finalise the current stream",
		},
		"scroll_to_top": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p != nil {
					pw.p.ScrollToTop()
				}
				return &object.Null{}
			},
			HelpText: "scroll_to_top() — scroll to top of panel content",
		},
		"scroll_to_bottom": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p != nil {
					pw.p.ScrollToBottom()
				}
				return &object.Null{}
			},
			HelpText: "scroll_to_bottom() — scroll to bottom of panel content",
		},
		"size": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p != nil {
					w, h := pw.p.Size()
					return &object.List{
						Elements: []object.Object{
							&object.Integer{Value: int64(w)},
							&object.Integer{Value: int64(h)},
						},
					}
				}
				return &object.List{
					Elements: []object.Object{
						&object.Integer{Value: 0},
						&object.Integer{Value: 0},
					},
				}
			},
			HelpText: "size() — return [width, height] of the panel",
		},
		"styled": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
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
				return &object.String{Value: applyStyle(pw.t, color, text)}
			},
			HelpText: "styled(color, text) — apply theme color to text",
		},
		"write_at": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) < 4 {
					return &object.Null{}
				}
				row, _ := args[1].AsInt()
				col, _ := args[2].AsInt()
				s, err := args[3].AsString()
				if err != nil {
					return err
				}
				pw.p.WriteAt(int(row), int(col), s)
				return &object.Null{}
			},
			HelpText: "write_at(row, col, text) — write text at a specific position",
		},
		"clear_line": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) > 1 {
					if row, err := args[1].AsInt(); err == nil {
						pw.p.ClearLine(int(row))
					}
				}
				return &object.Null{}
			},
			HelpText: "clear_line(row) — clear a specific line",
		},
		"add_column": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) < 2 {
					return &object.Null{}
				}
				childPw, ok := args[1].(*object.Instance)
				if !ok {
					return &object.Null{}
				}
				childNative, ok := childPw.Fields[nativePanelKey].(*panelWrapper)
				if !ok || childNative.p == nil {
					return &object.Null{}
				}
				pw.p.AddColumn(childNative.p)
				return &object.Null{}
			},
			HelpText: "add_column(panel) — add a child panel as a horizontal column",
		},
		"add_row": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p == nil {
					return &object.Null{}
				}
				if len(args) < 2 {
					return &object.Null{}
				}
				childPw, ok := args[1].(*object.Instance)
				if !ok {
					return &object.Null{}
				}
				childNative, ok := childPw.Fields[nativePanelKey].(*panelWrapper)
				if !ok || childNative.p == nil {
					return &object.Null{}
				}
				pw.p.AddRow(childNative.p)
				return &object.Null{}
			},
			HelpText: "add_row(panel) — add a child panel as a vertical row",
		},
		"__name__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p != nil {
					return &object.String{Value: pw.p.Name()}
				}
				return &object.String{Value: "main"}
			},
			HelpText: "__name__() — return the panel name",
		},
		"__str_repr__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				pw := panelWrapperFrom(args)
				if pw.p != nil {
					return &object.String{Value: "<Panel: " + pw.p.Name() + ">"}
				}
				return &object.String{Value: "<Panel: main>"}
			},
			HelpText: "__str_repr__() — return string representation",
		},
	},
}

// colorFromName converts a color name string to a tui.Color.
func colorFromName(t *tui.TUI, name string) tui.Color {
	theme := t.Theme()
	switch name {
	case "primary":
		return theme.Primary
	case "secondary":
		return theme.Secondary
	case "error":
		return theme.Error
	case "dim":
		return theme.Dim
	case "user":
		return theme.UserText
	default:
		s := strings.TrimPrefix(name, "#")
		if len(s) == 6 {
			if v, err := strconv.ParseUint(s, 16, 32); err == nil {
				return tui.Color(v)
			}
		}
		return theme.Text
	}
}

// NewLibrary creates the scriptling.console library.
func NewLibrary() *object.Library {
	return object.NewLibrary(LibraryName, moduleBuiltins, map[string]object.Object{
		"Panel":     panelClass,
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
