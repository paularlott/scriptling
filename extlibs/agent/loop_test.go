package agent

import (
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/stdlib"
)

func runAgentScript(t *testing.T, script string) {
	t.Helper()
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("expected 'OK', got: %v (err: %v)", result, err)
	}
}

// A model that never stops calling tools must still produce an answer: at
// the iteration limit the agent asks once more, without tools, instead of
// returning the last raw tool output as if it were the reply.
func TestAgentIterationLimitAsksForFinalAnswer(t *testing.T) {
	runAgentScript(t, `
import scriptling.ai as ai
import scriptling.ai.agent as agent

class MockClient:
    def __init__(self):
        self.calls = 0
        self.final_had_tools = None
        self.final_last = ""
    def completion(self, model, messages, **kwargs):
        self.calls += 1
        if "tools" not in kwargs:
            self.final_had_tools = False
            self.final_last = messages[-1]["content"]
            return {"choices": [{"message": {"role": "assistant", "content": "Here is what I found so far."}}]}
        return {"choices": [{"message": {"role": "assistant", "content": "", "tool_calls": [
            {"id": "c" + str(self.calls), "type": "function", "function": {"name": "look", "arguments": "{\"n\": " + str(self.calls) + "}"}}
        ]}}]}

tools = ai.ToolRegistry()
tools.add("look", "Look", {"n": "int"}, lambda args: "raw tool output " + str(args["n"]))

client = MockClient()
bot = agent.Agent(client, tools=tools, model="m")
resp = bot.trigger("dig", max_iterations=3)

assert resp["content"] == "Here is what I found so far.", "content: " + str(resp["content"])
assert client.calls == 4, "three tool rounds then one final request, got " + str(client.calls)
assert client.final_had_tools == False
assert "limit on tool calls" in client.final_last
assert bot.stop_reason == "max_iterations", bot.stop_reason
assert bot.get_messages()[-1] == {"role": "assistant", "content": "Here is what I found so far."}

# A normal finish reports "done" and makes no extra request.
class Plain:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "hi"}}]}
bot2 = agent.Agent(Plain(), model="m")
assert bot2.trigger("hello")["content"] == "hi"
assert bot2.stop_reason == "done"

"OK"
`)
}

// trigger() loops by default: one call must be enough to use a tool and
// answer from its result.
func TestAgentTriggerDefaultIterationsCompleteAToolRound(t *testing.T) {
	runAgentScript(t, `
import scriptling.ai as ai
import scriptling.ai.agent as agent

class MockClient:
    def __init__(self):
        self.calls = 0
    def completion(self, model, messages, **kwargs):
        self.calls += 1
        if self.calls == 1:
            return {"choices": [{"message": {"role": "assistant", "content": "", "tool_calls": [
                {"id": "c1", "type": "function", "function": {"name": "look", "arguments": "{}"}}]}}]}
        return {"choices": [{"message": {"role": "assistant", "content": "answer from " + messages[-1]["content"]}}]}

tools = ai.ToolRegistry()
tools.add("look", "Look", {}, lambda args: "the result")
bot = agent.Agent(MockClient(), tools=tools, model="m")
assert bot.trigger("go")["content"] == "answer from the result"

"OK"
`)
}

// The same call returning the same result again and again is pointed out to
// the model; a result that changes is not.
func TestAgentRepeatedIdenticalCallIsFlagged(t *testing.T) {
	runAgentScript(t, `
import scriptling.ai as ai
import scriptling.ai.agent as agent

tools = ai.ToolRegistry()
counter = [0]
def tick(args):
    counter[0] += 1
    return "tick " + str(counter[0])
tools.add("same", "Same", {}, lambda args: "nothing here")
tools.add("tick", "Tick", {}, tick)

bot = agent.Agent(None, tools=tools, model="m")
seen = {}
def call(name):
    return bot._run_tools([{"id": "c", "type": "function", "function": {"name": name, "arguments": "{}"}}], seen)[0]["content"]

assert call("same") == "nothing here"
assert call("same") == "nothing here"
third = call("same")
assert third.startswith("nothing here") and "same result 3 times" in third, third
assert "same result 4 times" in call("same")

for i in range(4):
    assert "same result" not in call("tick")

"OK"
`)
}

// One enormous tool result is cut to a share of the context budget, keeping
// its start and end, and is left alone when there is no budget.
func TestAgentOversizedToolResultIsClipped(t *testing.T) {
	runAgentScript(t, `
import scriptling.ai as ai
import scriptling.ai.agent as agent

tools = ai.ToolRegistry()
tools.add("big", "Big", {}, lambda args: "START" + "x" * 50000 + "END")
calls = [{"id": "c", "type": "function", "function": {"name": "big", "arguments": "{}"}}]

bot = agent.Agent(None, tools=tools, model="m", max_tokens=2000)
content = bot._run_tools(calls, {})[0]["content"]
assert len(content) < 2400, len(content)
assert content.startswith("START") and content.endswith("END")
assert "characters omitted" in content

unlimited = agent.Agent(None, tools=tools, model="m", max_tokens=0)
assert len(unlimited._run_tools(calls, {})[0]["content"]) == 50008

"OK"
`)
}

// Compaction keeps the recent part of the history word for word, never
// splits a tool round, repeats the user's request when it falls in the
// summarised part, and gives the summariser enough of each tool result to
// work from.
func TestAgentCompactionKeepsRecentWorkAndTheRequest(t *testing.T) {
	runAgentScript(t, `
import json
import scriptling.ai as ai
import scriptling.ai.agent as agent

class MockClient:
    def __init__(self):
        self.summary_prompt = ""
        self.requests = 0
    def completion(self, model, messages, **kwargs):
        self.requests += 1
        self.summary_prompt = messages[-1]["content"]
        return {"choices": [{"message": {"role": "assistant", "content": "<think>hmm</think>SUMMARY TEXT"}}]}

def round(n, size):
    return [
        {"role": "assistant", "content": "", "tool_calls": [{"id": "c" + str(n), "type": "function", "function": {
            "name": "read", "arguments": json.dumps({"path": "some/long/directory/name/that/goes/on/file" + str(n) + ".go"})}}]},
        {"role": "tool", "tool_call_id": "c" + str(n), "content": "result " + str(n) + " " + "y" * size + " tail" + str(n)},
    ]

client = MockClient()
bot = agent.Agent(client, model="m", max_tokens=2000, compaction_threshold=50)
request = "Find the bug in the parser and fix it without changing the public API."
bot.messages = [{"role": "system", "content": "sys"}, {"role": "user", "content": request}]
for n in range(8):
    bot.messages.extend(round(n, 1200))

assert bot._should_compact()
bot._compact_messages()
msgs = bot.get_messages()

# system, summary, then the kept tail opening with an assistant tool call
assert msgs[0]["role"] == "system"
assert msgs[1]["role"] == "user" and msgs[1]["content"].startswith("[Previous conversation summary]\nSUMMARY TEXT"), msgs[1]["content"][:80]
assert "think" not in msgs[1]["content"]
assert msgs[1]["content"].endswith(request), "the request must survive word for word"
assert msgs[2]["role"] == "assistant" and msgs[2].get("tool_calls"), "no filler acknowledgement before an assistant message"

# more than the last round is kept, whole rounds only, newest last
tail = msgs[2:]
assert len(tail) >= 2 and len(tail) % 2 == 0, len(tail)
for i in range(0, len(tail), 2):
    assert tail[i]["role"] == "assistant" and tail[i + 1]["role"] == "tool"
    assert tail[i]["tool_calls"][0]["id"] == tail[i + 1]["tool_call_id"]
assert tail[-1]["content"].endswith("tail7")
assert len(tail[-1]["content"]) > 1200, "kept results are not shortened"
assert len(tail) < 16, "the older rounds were summarised"

# the summariser saw the full file path and both ends of an old result
assert "file0.go" in client.summary_prompt
assert "result 0" in client.summary_prompt and "tail0" in client.summary_prompt
assert "word for word" not in client.summary_prompt

# With nothing new before the kept tail, a second pass makes no request.
before = client.requests
count = len(bot.get_messages())
bot._compact_messages()
assert client.requests == before and len(bot.get_messages()) == count

"OK"
`)
}

// The tool schemas count against the budget, so a history that fits on its
// own can still need compacting once they are added.
func TestAgentCompactionCountsToolSchemas(t *testing.T) {
	runAgentScript(t, `
import scriptling.ai as ai
import scriptling.ai.agent as agent

history = [{"role": "system", "content": "sys"}]
for i in range(6):
    history.append({"role": "user", "content": "q" * 200})
    history.append({"role": "assistant", "content": "a" * 200})

bare = agent.Agent(None, model="m", max_tokens=1000, compaction_threshold=80)
bare.messages = history
assert not bare._should_compact()

tools = ai.ToolRegistry()
for i in range(10):
    tools.add("tool_" + str(i), "A tool with a fairly long description to take up room " * 2, {"value": "string"}, lambda args: "x")
loaded = agent.Agent(None, tools=tools, model="m", max_tokens=1000, compaction_threshold=80)
loaded.messages = history
assert loaded._should_compact()

"OK"
`)
}

// An interrupted interactive turn keeps the tool rounds that already ran and
// notes the interruption; a turn that got nowhere is dropped.
func TestInteractAbandonedTurnKeepsCompletedToolRounds(t *testing.T) {
	p := newInteractInterpreter(t)
	_, err := p.Eval(`
import scriptling.ai.agent.interact as interact_lib

bot = interact_lib.Agent(None)

bot.messages = [{"role": "system", "content": "sys"}, {"role": "user", "content": "do it"}]
bot._abandon_turn(0, "[interrupted]")
assert bot.messages == [{"role": "system", "content": "sys"}], bot.messages

bot.messages = [
    {"role": "user", "content": "edit the file"},
    {"role": "assistant", "content": "", "tool_calls": [{"id": "c1", "type": "function", "function": {"name": "edit", "arguments": "{}"}}]},
    {"role": "tool", "tool_call_id": "c1", "content": "ok"},
]
bot._abandon_turn(1, "[interrupted]")
assert len(bot.messages) == 4
assert bot.messages[2]["content"] == "ok"
assert bot.messages[3] == {"role": "assistant", "content": "[interrupted]"}
`)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
}
