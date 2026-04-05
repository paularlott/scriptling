# AI Library Example - Streaming Tool Calling
# This script streams the initial assistant turn, accumulates tool-call deltas,
# executes the local tool, then streams the final assistant answer.

import scriptling.ai as ai

BASE_URL = "http://127.0.0.1:11434/v1"
MODEL = "gemma4:e4b"


def echo_tool(args):
    message = args.get("message", "")
    print()
    print("TOOL CALLED: echo_tool(message=" + message + ")")
    return "Tool says: " + message


def on_event(event):
    if event["type"] == "content":
        print(event["content"], end="", flush=True)


print("Creating OpenAI client...")
client = ai.Client(BASE_URL)

print()
print("Registering tools...")
tools = ai.ToolRegistry()
tools.add("echo_tool", "Echo a message back to the assistant", {"message": "string"}, echo_tool)

messages = [{
    "role": "user",
    "content": "Call the echo_tool with message 'hello from streaming tool test', then briefly tell me what it returned."
}]

print()
print("Streaming initial response with " + MODEL + "...")
stream = client.completion_stream(MODEL, messages, tools=tools.build(), timeout_ms=30000)
result = ai.collect_stream(stream, on_event=on_event)

print()

if len(result["tool_calls"]) == 0:
    print("Model did not call a tool.")
else:
    print("Model requested " + str(len(result["tool_calls"])) + " tool call(s).")

    messages.append(result["assistant_message"])

    tool_results = ai.execute_tool_calls(tools, result["tool_calls"])
    for r in tool_results:
        messages.append(r)

    print()
    print("Streaming final assistant answer...")
    final_stream = client.completion_stream(MODEL, messages, timeout_ms=30000)
    ai.collect_stream(final_stream, on_event=on_event)
    print()
