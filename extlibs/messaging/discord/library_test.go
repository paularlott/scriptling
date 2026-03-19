package discord

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func setupInterpreter() *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	Register(p, nil)
	return p
}

func TestLibraryRegistration(t *testing.T) {
	p := setupInterpreter()
	_, err := p.Eval(`import scriptling.messaging.discord as discord`)
	if err != nil {
		t.Fatalf("failed to import: %v", err)
	}
}

func TestClientCreation(t *testing.T) {
	p := setupInterpreter()
	result, err := p.Eval(`import scriptling.messaging.discord as discord
c = discord.client("test-token")
type(c)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Inspect() != "DiscordClient" {
		t.Errorf("expected DiscordClient, got %s", result.Inspect())
	}
}

func TestClientWithAllowedUsers(t *testing.T) {
	p := setupInterpreter()
	_, err := p.Eval(`import scriptling.messaging.discord as discord
discord.client("tok", allowed_users=["111", "222"])`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientEmptyTokenError(t *testing.T) {
	p := setupInterpreter()
	result, _ := p.Eval(`import scriptling.messaging.discord as discord
discord.client("")`)
	if result == nil || result.Inspect() == "DiscordClient" {
		t.Error("expected error for empty token")
	}
}

func TestKeyboardBuilder(t *testing.T) {
	p := setupInterpreter()
	result, err := p.Eval(`import scriptling.messaging.discord as discord
kb = discord.keyboard([[{"text": "Yes", "data": "yes"}, {"text": "Go", "url": "https://scriptling.dev"}]])
kb[0][0]["text"]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Inspect() != "Yes" {
		t.Errorf("expected 'Yes', got %s", result.Inspect())
	}
}

func TestCommandRegistration(t *testing.T) {
	p := setupInterpreter()
	_, err := p.Eval(`import scriptling.messaging.discord as discord
c = discord.client("tok")
def handle(ctx):
    pass
discord.command(c, "/start", "Start", handle)`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendMessageArgValidation(t *testing.T) {
	p := setupInterpreter()
	result, _ := p.Eval(`import scriptling.messaging.discord as discord
c = discord.client("tok")
discord.send_message(c)`)
	if result == nil {
		t.Fatal("expected error for missing args")
	}
}
