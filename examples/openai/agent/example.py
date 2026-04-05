# AI Library Example - Agent with Tool Calling
# This script creates an agent that can call tools in an agentic loop,
# automatically handling tool call extraction and execution.

import scriptling.ai as ai
import scriptling.ai.agent as agent

BASE_URL = "http://127.0.0.1:11434/v1"
MODEL = "gemma4:e4b"


def echo_tool(args):
    message = args.get("message", "")
    print("TOOL CALLED: echo_tool(message=" + message + ")")
    return "Tool says: " + message


print("Creating OpenAI client...")
client = ai.Client(BASE_URL)

print()
print("Registering tools...")
tools = ai.ToolRegistry()
tools.add("echo_tool", "Echo a message back to the assistant", {"message": "string"}, echo_tool)

print()
print("Creating agent...")
bot = agent.Agent(
    client,
    tools=tools,
    system_prompt="You are a helpful assistant. When asked to call a tool, call it exactly once then report the result.",
    model=MODEL,
    max_tokens=4096,
    compaction_threshold=80
)

print()
print("Triggering agent with " + MODEL + "...")
response = bot.trigger(
    "Call the echo_tool with message 'hello from agent test', then briefly tell me what it returned.",
    max_iterations=5
)

print()
print("Agent response:")
print(response["content"] if isinstance(response, dict) else response.content)

print()
print("Conversation history:")
messages = bot.get_messages()
for msg in messages:
    role = msg.get("role", "?") if isinstance(msg, dict) else "?"
    content = msg.get("content", "") if isinstance(msg, dict) else ""
    if content:
        preview = content[:80] + "..." if len(content) > 80 else content
        print("  [" + role + "] " + preview)
