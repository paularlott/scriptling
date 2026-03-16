package agent

import (
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/stdlib"
)

func TestAgentBasic(t *testing.T) {
	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent

# Mock client that returns simple responses
class MockClient:
    def __init__(self):
        self.tools = []

    def set_tools(self, tools):
        self.tools = tools

    def completion(self, model, messages, **kwargs):
        # Return a simple response without tool calls
        return {
            "choices": [{
                "message": {
                    "role": "assistant",
                    "content": "Hello! I'm a mock assistant."
                }
            }]
        }

# Create mock client
client = MockClient()

# Create tools
tools = ai.ToolRegistry()
def read_func(args):
    return "file content"
tools.add("read", "Read file", {"path": "string"}, read_func)

# Create agent
bot = agent.Agent(client, tools=tools, system_prompt="Test assistant")

# Trigger a message
response = bot.trigger("Hello")

# Verify response
assert response["role"] == "assistant"
assert response["content"] == "Hello! I'm a mock assistant."

# Verify messages were added
messages = bot.get_messages()
assert len(messages) == 3  # system + user + assistant
assert messages[0]["role"] == "system"
assert messages[1]["role"] == "user"
assert messages[2]["role"] == "assistant"

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

func TestAgentWithToolCalls(t *testing.T) {
	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent
import json

# Mock client that simulates tool calls
class MockClient:
    def __init__(self):
        self.tools = []
        self.call_count = 0

    def set_tools(self, tools):
        self.tools = tools

    def completion(self, model, messages, **kwargs):
        self.call_count = self.call_count + 1

        # First call: return tool call
        if self.call_count == 1:
            return {
                "choices": [{
                    "message": {
                        "role": "assistant",
                        "content": "Let me read that file",
                        "tool_calls": [{
                            "id": "call_123",
                            "function": {
                                "name": "read",
                                "arguments": json.dumps({"path": "test.txt"})
                            }
                        }]
                    }
                }]
            }

        # Second call: return final response
        return {
            "choices": [{
                "message": {
                    "role": "assistant",
                    "content": "The file contains: mock content"
                }
            }]
        }

# Create mock client
client = MockClient()

# Create tools
tools = ai.ToolRegistry()
def read_func(args):
    return "mock content"
tools.add("read", "Read file", {"path": "string"}, read_func)

# Create agent
bot = agent.Agent(client, tools=tools)

# Trigger with tool execution
response = bot.trigger("Read test.txt", max_iterations=5)

# Verify final response
assert response["content"] == "The file contains: mock content"

# Verify messages include tool call and result
messages = bot.get_messages()
assert len(messages) >= 3
# Should have: user, assistant (with tool_calls), tool, assistant

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

func TestAgentMemoryToolsAutoWired(t *testing.T) {
	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent
import json

# Minimal mock memory object
class MockMemory:
    def __init__(self):
        self.remembered = []
        self.recalled = []
        self.forgotten = []

    def remember(self, content, type="note", importance=0.5):
        self.remembered.append({"content": content, "type": type, "importance": importance})
        return {"id": "mock-id-1", "content": content, "type": type, "importance": importance}

    def recall(self, query="", limit=10, type=""):
        self.recalled.append({"query": query, "type": type})
        return []

    def forget(self, id):
        self.forgotten.append(id)
        return True

mem = MockMemory()

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

bot = agent.Agent(MockClient(), memory=mem)

# Memory tools should be in tool_schemas
schema_names = [s["function"]["name"] for s in bot.tool_schemas]
assert "memory_remember" in schema_names, "memory_remember not in schemas: " + str(schema_names)
assert "memory_recall" in schema_names, "memory_recall not in schemas: " + str(schema_names)
assert "memory_forget" in schema_names, "memory_forget not in schemas: " + str(schema_names)

# System prompt should contain memory instructions
assert "## Memory" in bot.system_prompt, "memory instructions not in system_prompt"
assert "memory_remember" in bot.system_prompt
assert "memory_recall" in bot.system_prompt

# Preferences were loaded at init (recall called with type="preference")
pref_calls = [r for r in mem.recalled if r["type"] == "preference"]
assert len(pref_calls) == 1, "expected 1 preference recall at init, got " + str(len(pref_calls))

# Handlers should work
handler = bot.tools.get_handler("memory_remember")
result = handler({"content": "User likes Go", "type": "preference", "importance": 0.9})
assert mem.remembered[-1]["content"] == "User likes Go"
assert mem.remembered[-1]["type"] == "preference"

handler = bot.tools.get_handler("memory_recall")
result = handler({"query": "Python", "limit": 5})
assert mem.recalled[-1]["query"] == "Python"

handler = bot.tools.get_handler("memory_forget")
result = handler({"id": "mock-id-1"})
assert mem.forgotten[-1] == "mock-id-1"

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

func TestAgentMemoryPreferencesInjected(t *testing.T) {
	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent

class MockMemory:
    def remember(self, content, type="note", importance=0.5):
        return {"id": "x", "content": content, "type": type, "importance": importance}
    def recall(self, query="", limit=10, type=""):
        if type == "preference":
            return [
                {"id": "p1", "content": "User prefers dark mode", "type": "preference", "importance": 0.8},
                {"id": "p2", "content": "User codes in Go", "type": "preference", "importance": 0.7},
            ]
        return []
    def forget(self, id):
        return True

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

bot = agent.Agent(MockClient(), system_prompt="You are helpful.", memory=MockMemory())

# Original system prompt preserved
assert "You are helpful." in bot.system_prompt
# Memory instructions appended
assert "## Memory" in bot.system_prompt
# Preferences injected
assert "User prefers dark mode" in bot.system_prompt
assert "User codes in Go" in bot.system_prompt
assert "## Remembered Preferences" in bot.system_prompt

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

func TestAgentMemoryWithExistingTools(t *testing.T) {
	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent

class MockMemory:
    def remember(self, content, type="note", importance=0.5):
        return {"id": "x", "content": content, "type": type, "importance": importance}
    def recall(self, query="", limit=10, type=""):
        return []
    def forget(self, id):
        return True

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

tools = ai.ToolRegistry()
tools.add("my_tool", "A custom tool", {"x": "string"}, lambda args: "result")

bot = agent.Agent(MockClient(), tools=tools, memory=MockMemory())

schema_names = [s["function"]["name"] for s in bot.tool_schemas]
assert "my_tool" in schema_names, "existing tool lost: " + str(schema_names)
assert "memory_remember" in schema_names, "memory_remember missing: " + str(schema_names)
assert "memory_recall" in schema_names, "memory_recall missing: " + str(schema_names)
assert "memory_forget" in schema_names, "memory_forget missing: " + str(schema_names)
assert len(schema_names) == 4

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

func TestAgentNoMemory_NoMemoryTools(t *testing.T) {
	script := `
import scriptling.ai as ai
import scriptling.ai.agent as agent

class MockClient:
    def completion(self, model, messages, **kwargs):
        return {"choices": [{"message": {"role": "assistant", "content": "ok"}}]}

bot = agent.Agent(MockClient())
assert len(bot.tool_schemas) == 0

tools = ai.ToolRegistry()
tools.add("my_tool", "A tool", {"x": "string"}, lambda args: "result")
bot2 = agent.Agent(MockClient(), tools=tools)
schema_names = [s["function"]["name"] for s in bot2.tool_schemas]
assert "my_tool" in schema_names
assert "memory_remember" not in schema_names

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)
	Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}
