#!/usr/bin/env scriptling
# Test that interact is registered as scriptling.ai.agent.interact

import scriptling.ai.agent.interact as interact

# Verify interact library has Agent class
assert hasattr(interact, "Agent"), "interact should have Agent class"
print("✓ scriptling.ai.agent.interact imported")
print("✓ interact.Agent accessible")

# Test that it's the extended Agent with interact method
class MockClient:
    def __init__(self):
        self.tools = None
    def set_tools(self, schemas):
        self.tools = schemas
    def completion(self, *args):
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
bot = interact.Agent(client, model="test")
assert hasattr(bot, "interact"), "Agent from interact library should have interact method"
print("✓ Agent has interact method")

print("\n✅ Interact library test passed")
