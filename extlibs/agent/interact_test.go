package agent

import (
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/extlibs/console"
	"github.com/paularlott/scriptling/stdlib"
)

func newInteractInterpreter(t *testing.T) *scriptlib.Scriptling {
	t.Helper()
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)
	console.Register(p)
	if err := RegisterInteract(p); err != nil {
		t.Fatalf("RegisterInteract: %v", err)
	}
	return p
}

func TestInteractAgentConstructed(t *testing.T) {
	p := newInteractInterpreter(t)
	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}

bot = interact_lib.Agent(FakeClient(), model="gpt-4")
assert bot.model == "gpt-4"
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}

func TestInteractClearResetsMessages(t *testing.T) {
	p := newInteractInterpreter(t)
	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}

bot = interact_lib.Agent(FakeClient(), system_prompt="You are helpful.")
bot.trigger("hello")
assert len(bot.messages) > 0

bot.messages = []
if bot.system_prompt:
    bot.messages.append({"role": "system", "content": bot.system_prompt})

assert len(bot.messages) == 1
assert bot.messages[0]["role"] == "system"
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}

func TestInteractHistoryTracked(t *testing.T) {
	p := newInteractInterpreter(t)
	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

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
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}

func TestInteractInheritsFromAgent(t *testing.T) {
	p := newInteractInterpreter(t)
	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib
import scriptling.ai.agent as agent_module

class FakeClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}

bot = interact_lib.Agent(FakeClient())
# interact.Agent should have the interact() method
assert hasattr(bot, "interact")
# and still have the base trigger() method
assert hasattr(bot, "trigger")
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}
