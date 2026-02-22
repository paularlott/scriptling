package agent

import (
	"context"
	"strings"
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/ai"
	consolepkg "github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// testBackend captures console output for assertions.
type testBackend struct {
	printed   []string
	statusL   string
	statusR   string
	escapeCb  func()
	submitCb  func(context.Context, string)
	commands  map[string]func(string)
	cleared   bool
}

func (b *testBackend) Input(_ string, _ *object.Environment) (string, error) { return "", nil }
func (b *testBackend) Print(text string, _ *object.Environment)              { b.printed = append(b.printed, text) }
func (b *testBackend) StreamStart()                                          {}
func (b *testBackend) StreamChunk(s string)                                  { b.printed = append(b.printed, s) }
func (b *testBackend) StreamEnd()                                            {}
func (b *testBackend) SpinnerStart(_ string)                                 {}
func (b *testBackend) SpinnerStop()                                          {}
func (b *testBackend) SetProgress(_ string, _ float64)                       {}
func (b *testBackend) SetStatus(l, r string)                                 { b.statusL = l; b.statusR = r }
func (b *testBackend) SetStatusLeft(l string)                                { b.statusL = l }
func (b *testBackend) SetStatusRight(r string)                               { b.statusR = r }
func (b *testBackend) RegisterCommand(name, _ string, h func(string))        {
	if b.commands == nil {
		b.commands = map[string]func(string){}
	}
	b.commands[name] = h
}
func (b *testBackend) RemoveCommand(name string) { delete(b.commands, name) }
func (b *testBackend) OnSubmit(fn func(context.Context, string)) { b.submitCb = fn }
func (b *testBackend) OnEscape(fn func())        { b.escapeCb = fn }
func (b *testBackend) ClearOutput()              { b.cleared = true }
func (b *testBackend) Run() error                { return nil }

func (b *testBackend) allOutput() string { return strings.Join(b.printed, "") }

func newInteractInterpreter(t *testing.T) (*scriptlib.Scriptling, *testBackend) {
	t.Helper()
	tb := &testBackend{}
	orig := consolepkg.GetBackend()
	t.Cleanup(func() { consolepkg.SetBackend(orig) })
	consolepkg.SetBackend(tb)

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	consolepkg.Register(p)
	if err := RegisterInteract(p); err != nil {
		t.Fatalf("RegisterInteract: %v", err)
	}
	return p, tb
}

func TestInteractModelCommand(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}

bot = interact_lib.Agent(FakeClient(), model="gpt-4")
assert bot.model == "gpt-4"
"OK"
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	// The /model command handler is registered when interact() is called;
	// verify the Agent was constructed correctly.
	_ = tb
}

func TestInteractStatusSetOnStart(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	// The interact loop sets status on entry; test via set_status directly
	_, err := p.Eval(`
import scriptling.console as console
console.set_status("scriptling", "gpt-4")
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if tb.statusL != "scriptling" || tb.statusR != "gpt-4" {
		t.Errorf("Expected status 'scriptling'/'gpt-4', got %q/%q", tb.statusL, tb.statusR)
	}
}

func TestInteractClearResetsMessages(t *testing.T) {
	p, _ := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib
import scriptling.ai as ai

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}

bot = interact_lib.Agent(FakeClient(), system_prompt="You are helpful.")
bot.trigger("hello")
assert len(bot.messages) > 0

# Simulate /clear
bot.messages = []
if bot.system_prompt:
    bot.messages.append({"role": "system", "content": bot.system_prompt})

assert len(bot.messages) == 1
assert bot.messages[0]["role"] == "system"

"OK"
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}

func TestInteractHistoryTracked(t *testing.T) {
	p, _ := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib
import scriptling.ai as ai

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "response"}}]}

bot = interact_lib.Agent(FakeClient())
bot.trigger("first message")
bot.trigger("second message")

msgs = bot.get_messages()
user_msgs = [m for m in msgs if m["role"] == "user"]
assert len(user_msgs) == 2
assert user_msgs[0]["content"] == "first message"
assert user_msgs[1]["content"] == "second message"

"OK"
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}

func TestInteractEscapeCallbackRegistered(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.console as console
console.on_escape(lambda: None)
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if tb.escapeCb == nil {
		t.Error("Expected escape callback to be registered in backend")
	}
}

func TestInteractOnSubmitFired(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeChunk:
    def __init__(self, content):
        class Delta:
            pass
        d = Delta()
        d.content = content
        class Choice:
            pass
        c = Choice()
        c.delta = d
        self.choices = [c]

class FakeStream:
    def __init__(self, content):
        self._chunks = [FakeChunk(content), None]
        self._idx = 0
    def next(self):
        val = self._chunks[self._idx]
        self._idx = self._idx + 1
        return val

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": ""}}]}
    def completion_stream(self, model, messages, **kwargs):
        return FakeStream("pong")

bot = interact_lib.Agent(FakeClient(), model="test-model")
bot.interact()
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if tb.submitCb == nil {
		t.Fatal("Expected on_submit callback to be registered")
	}
	if tb.statusL != "scriptling" || tb.statusR != "test-model" {
		t.Errorf("Expected status 'scriptling'/'test-model', got %q/%q", tb.statusL, tb.statusR)
	}

	// Fire the submit callback — should run the agentic loop and stream "pong"
	tb.printed = nil
	tb.submitCb(context.Background(), "hello")

	out := tb.allOutput()
	if !strings.Contains(out, "pong") {
		t.Errorf("Expected streamed response 'pong', got %q", out)
	}
}

func TestInteractCommandsClear(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeStream:
    def next(self): return None

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}
    def completion_stream(self, model, messages, **kwargs):
        return FakeStream()

bot = interact_lib.Agent(FakeClient(), system_prompt="You are helpful.")
bot.interact()
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	// Simulate a message to populate history
	if tb.submitCb != nil {
		tb.submitCb(context.Background(), "hello")
	}

	// Fire /clear command
	if tb.commands == nil || tb.commands["clear"] == nil {
		t.Fatal("Expected /clear command to be registered")
	}
	tb.cleared = false
	tb.commands["clear"]("")

	if !tb.cleared {
		t.Error("Expected ClearOutput to be called by /clear command")
	}
}

func TestInteractCommandsModel(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}

bot = interact_lib.Agent(FakeClient(), model="old-model")
bot.interact()
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if tb.commands == nil || tb.commands["model"] == nil {
		t.Fatal("Expected /model command to be registered")
	}

	// Switch model
	tb.commands["model"]("new-model")
	if tb.statusR != "new-model" {
		t.Errorf("Expected status right 'new-model', got %q", tb.statusR)
	}
}

func TestInteractCommandsHistory(t *testing.T) {
	p, tb := newInteractInterpreter(t)

	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeStream:
    def next(self): return None

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "response"}}]}
    def completion_stream(self, model, messages, **kwargs):
        return FakeStream()

bot = interact_lib.Agent(FakeClient())
bot.interact()
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	// Submit a message to build history
	if tb.submitCb != nil {
		tb.submitCb(context.Background(), "test question")
	}

	if tb.commands == nil || tb.commands["history"] == nil {
		t.Fatal("Expected /history command to be registered")
	}

	tb.printed = nil
	tb.commands["history"]("")

	out := tb.allOutput()
	if !strings.Contains(out, "user") {
		t.Errorf("Expected history output to contain 'user', got %q", out)
	}
}
