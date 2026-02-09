#!/usr/bin/env scriptling
# Test scriptling.ai.agent namespace structure

import scriptling.ai as ai
import scriptling.ai.agent as agent

# Test 1: ToolRegistry accessible from ai namespace
registry = ai.ToolRegistry()
assert str(type(registry)) == "Registry", "ToolRegistry should create Registry instance"
print("✓ ToolRegistry accessible from ai namespace")

# Test 2: Agent accessible from agent namespace
class MockClient:
    def __init__(self):
        self.tools = None
    def set_tools(self, schemas):
        self.tools = schemas
    def completion(self, *args, **kwargs):
        class MockMessage:
            def __init__(self):
                self.content = "test"
                self.tool_calls = None
        class MockChoice:
            def __init__(self):
                self.message = MockMessage()
        class MockResponse:
            def __init__(self):
                self.choices = [MockChoice()]
        return MockResponse()

client = MockClient()
bot = agent.Agent(client, system_prompt="test", model="test")
assert str(type(bot)) == "Agent", "Agent should create Agent instance"
print("✓ Agent accessible from agent namespace")

# Test 3: Nested access via attribute
assert hasattr(ai, "agent"), "ai should have agent attribute"
assert hasattr(ai.agent, "Agent"), "ai.agent should have Agent attribute"
print("✓ Nested access works: ai.agent.Agent")

# Test 4: Tool registry integration
def test_handler(args):
    return "result: " + args["param"]

registry.add("test_tool", "Test tool", {"param": "string"}, test_handler)
schemas = registry.build()
assert type(schemas) == type([]), "build() should return list"
assert len(schemas) == 1, "Should have 1 tool"
print("✓ Tool registry builds schemas correctly")

# Test 5: Agent with tools
bot2 = agent.Agent(client, tools=registry, model="test")
# Agent stores tool schemas internally and passes via kwargs
assert bot2.tool_schemas is not None, "Agent should store tool schemas"
assert type(bot2.tool_schemas) == type([]), "tool_schemas should be a list"
print("✓ Agent stores tool schemas for passing via kwargs")

print("\n✅ All agent namespace tests passed")
