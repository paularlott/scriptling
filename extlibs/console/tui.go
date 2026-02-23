package console

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/paularlott/cli/tui"
	"github.com/paularlott/scriptling/object"
)

// tuiBackend implements ConsoleBackend using the TUI library.
type tuiBackend struct {
	t        *tui.TUI
	escapeCb func()
	submitCb func(context.Context, string)
	mu       sync.Mutex
}

func (b *tuiBackend) Input(prompt string, _ *object.Environment) (string, error) {
	if prompt != "" {
		b.t.StreamChunk(prompt)
	}
	return "", nil
}
func (b *tuiBackend) Print(text string, _ *object.Environment) {
	b.t.AddMessage(tui.RoleAssistant, strings.TrimRight(text, "\n"))
}
func (b *tuiBackend) PrintAs(label, text string, _ *object.Environment) {
	b.t.AddMessageAs(tui.RoleAssistant, label, strings.TrimRight(text, "\n"))
}
func (b *tuiBackend) Styled(color, text string) string {
	theme := b.t.Theme()
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
		// Try parsing as hex: #RRGGBB or RRGGBB
		s := strings.TrimPrefix(color, "#")
		if len(s) == 6 {
			if v, err := strconv.ParseUint(s, 16, 32); err == nil {
				c = tui.Color(v)
				break
			}
		}
		c = theme.Text
	}
	return tui.Styled(c, text)
}
func (b *tuiBackend) StreamStart()               { b.t.StartStreaming() }
func (b *tuiBackend) StreamStartAs(label string) { b.t.StartStreamingAs(label) }
func (b *tuiBackend) StreamChunk(s string)       { b.t.StreamChunk(s) }
func (b *tuiBackend) StreamEnd()                 { b.t.StreamComplete() }
func (b *tuiBackend) SpinnerStart(text string)   { b.t.StartSpinner(text) }
func (b *tuiBackend) SpinnerStop()               { b.t.StopSpinner() }
func (b *tuiBackend) SetProgress(label string, pct float64) {
	if pct < 0 {
		b.t.ClearProgress()
	} else {
		b.t.SetProgress(label, pct)
	}
}
func (b *tuiBackend) SetLabels(user, assistant, system string) {
	b.t.SetLabels(user, assistant, system)
}
func (b *tuiBackend) SetStatus(left, right string) { b.t.SetStatus(left, right) }
func (b *tuiBackend) SetStatusLeft(s string)       { b.t.SetStatusLeft(s) }
func (b *tuiBackend) SetStatusRight(s string)      { b.t.SetStatusRight(s) }
func (b *tuiBackend) RegisterCommand(name, desc string, handler func(args string)) {
	b.t.AddCommand(&tui.Command{Name: name, Description: desc, Handler: handler})
}
func (b *tuiBackend) RemoveCommand(name string) { b.t.RemoveCommand(name) }
func (b *tuiBackend) ClearOutput()              { b.t.ClearOutput() }
func (b *tuiBackend) OnSubmit(fn func(context.Context, string)) {
	b.mu.Lock()
	b.submitCb = fn
	b.mu.Unlock()
}
func (b *tuiBackend) OnEscape(fn func()) {
	b.mu.Lock()
	b.escapeCb = fn
	b.mu.Unlock()
}
func (b *tuiBackend) Run() error {
	return b.t.Run(context.Background())
}

// newTUIBackend builds a tuiBackend and registers it as the active backend.
// Called lazily the first time console.run() is invoked without a custom backend.
func newTUIBackend() *tuiBackend {
	var (
		tb        *tuiBackend
		cancel    context.CancelFunc
		runningMu sync.Mutex
		prevDone  = make(chan struct{})
	)
	close(prevDone)

	var t *tui.TUI
	t = tui.New(tui.Config{
		StatusRight: "Ctrl+C to exit",
		Commands: []*tui.Command{
			{Name: "exit", Description: "Exit", Handler: func(_ string) { t.Exit() }},
		},
		OnEscape: func() {
			runningMu.Lock()
			if cancel != nil {
				cancel()
			}
			runningMu.Unlock()
			tb.mu.Lock()
			cb := tb.escapeCb
			tb.mu.Unlock()
			if cb != nil {
				go cb()
			}
		},
		OnSubmit: func(line string) {
			t.AddMessage(tui.RoleUser, line)
			tb.mu.Lock()
			scb := tb.submitCb
			ecb := tb.escapeCb
			tb.mu.Unlock()
			if scb == nil {
				return
			}
			ctx, c := context.WithCancel(context.Background())
			runningMu.Lock()
			if cancel != nil {
				cancel()
				if ecb != nil {
					go ecb()
				}
			}
			cancel = c
			waitFor := prevDone
			nextDone := make(chan struct{})
			prevDone = nextDone
			runningMu.Unlock()
			go func() {
				defer func() {
					runningMu.Lock()
					cancel = nil
					runningMu.Unlock()
					c()
					close(nextDone)
				}()
				<-waitFor
				scb(ctx, line)
			}()
		},
	})

	tb = &tuiBackend{t: t}
	return tb
}
