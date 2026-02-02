#!/usr/bin/env scriptling
# Test nested library import in different orders

# Test 1: Import parent first, then child
import scriptling.ai as ai1
import scriptling.ai.agent as agent1

assert hasattr(ai1, "agent"), "Parent should have child after both imports"
assert hasattr(ai1.agent, "Agent"), "Nested access should work"
print("✓ Import order: parent first, then child")

# Test 2: Import child first, then parent (in new namespace)
import scriptling.ai.agent as agent2
import scriptling.ai as ai2

assert hasattr(ai2, "agent"), "Parent should have child regardless of order"
assert hasattr(ai2.agent, "Agent"), "Nested access should work"
print("✓ Import order: child first, then parent")

# Test 3: Both namespaces point to same objects
assert agent1.Agent is agent2.Agent, "Agent class should be same object"
print("✓ Same objects regardless of import order")

# Test 4: ToolRegistry accessible from both ai imports
reg1 = ai1.ToolRegistry()
reg2 = ai2.ToolRegistry()
assert str(type(reg1)) == str(type(reg2)), "Both should create ToolRegistry instances"
print("✓ ToolRegistry accessible from both ai imports")

print("\n✅ All nested import tests passed")
