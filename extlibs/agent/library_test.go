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
