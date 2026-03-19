package telegram_test

import (
	"testing"

	scriptling "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/messaging/telegram"
	"github.com/paularlott/scriptling/stdlib"
)

func newInterp() *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	telegram.Register(p, nil)
	return p
}

func eval(t *testing.T, p *scriptling.Scriptling, code string) string {
	t.Helper()
	result, err := p.Eval(code)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	s, errObj := result.AsString()
	if errObj != nil {
		t.Fatalf("result not a string: %v", result.Inspect())
	}
	return s
}

func TestLibraryRegisters(t *testing.T) {
	p := newInterp()
	result := eval(t, p, `import scriptling.messaging.telegram as telegram
"ok"`)
	if result != "ok" {
		t.Errorf("got %q, want \"ok\"", result)
	}
}

func TestClientCreation(t *testing.T) {
	p := newInterp()
	result := eval(t, p, `import scriptling.messaging.telegram as telegram
c = telegram.client("test-token")
str(type(c))`)
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestClientEmptyTokenErrors(t *testing.T) {
	p := newInterp()
	_, err := p.Eval(`import scriptling.messaging.telegram as telegram
telegram.client("")`)
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestClientWithAllowedUsers(t *testing.T) {
	p := newInterp()
	result := eval(t, p, `import scriptling.messaging.telegram as telegram
c = telegram.client("test-token", allowed_users=["123", "456"])
"ok"`)
	if result != "ok" {
		t.Errorf("got %q, want \"ok\"", result)
	}
}

func TestKeyboardBuilder(t *testing.T) {
	p := newInterp()
	result := eval(t, p, `import scriptling.messaging.telegram as telegram
kb = telegram.keyboard([[{"text": "Yes", "data": "yes"}, {"text": "Go", "url": "https://scriptling.dev"}]])
kb[0][0]["text"]`)
	if result != "Yes" {
		t.Errorf("got %q, want \"Yes\"", result)
	}
}

func TestCommandRegistration(t *testing.T) {
	p := newInterp()
	result := eval(t, p, `import scriptling.messaging.telegram as telegram
c = telegram.client("tok")
def handle(ctx):
    pass
telegram.command(c, "/start", "Start the bot", handle)
"ok"`)
	if result != "ok" {
		t.Errorf("got %q, want \"ok\"", result)
	}
}

func TestOnMessageRegistration(t *testing.T) {
	p := newInterp()
	result := eval(t, p, `import scriptling.messaging.telegram as telegram
c = telegram.client("tok")
def handle(ctx):
    pass
telegram.on_message(c, handle)
"ok"`)
	if result != "ok" {
		t.Errorf("got %q, want \"ok\"", result)
	}
}

func TestSendMessageArgValidation(t *testing.T) {
	p := newInterp()
	_, err := p.Eval(`import scriptling.messaging.telegram as telegram
c = telegram.client("tok")
telegram.send_message(c, "123")`)
	if err == nil {
		t.Error("expected error for too few args")
	}
}

func TestClientNotPassedToSendMessage(t *testing.T) {
	p := newInterp()
	_, err := p.Eval(`import scriptling.messaging.telegram as telegram
telegram.send_message("not-a-client", "123", "hello")`)
	if err == nil {
		t.Error("expected error when non-client passed")
	}
}
