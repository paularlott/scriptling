# AI Library Example - Tool Calling with Chat Completions
# This script exposes a local tool, lets the model call it, executes it,
# then sends the tool result back for a final answer.

import scriptling.ai as ai

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
print("Requesting a tool call with " + MODEL + "...")
messages = [{
    "role": "user",
    "content": "Call the echo_tool with message 'hello from tool test', then briefly tell me what it returned."
}]

response = client.completion(MODEL, messages, tools=tools.build(), timeout=30)
calls = ai.tool_calls(response)

if len(calls) == 0:
    print("Model did not call a tool.")
    content = ai.text(response)
    if content:
        print("Assistant response:")
        print(content)
else:
    print("Model requested " + str(len(calls)) + " tool call(s).")

    messages.append({
        "role": "assistant",
        "content": response.choices[0].message.content or "",
        "tool_calls": calls
    })

    for result in ai.execute_tool_calls(tools, calls):
        messages.append(result)

    print()
    print("Requesting final assistant answer...")
    final_response = client.completion(MODEL, messages, timeout=30)

    print()
    print("Final response:")
    print(ai.text(final_response))
