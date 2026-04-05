# AI Library Example - Agent Interactive Chat
# This script creates an interactive agent using the TUI console.

import scriptling.ai as ai
import scriptling.ai.agent.interact as interact
import scriptling.console as console

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
print("Creating interactive agent...")
bot = interact.Agent(
    client,
    tools=tools,
    system_prompt="You are a helpful assistant.",
    model=MODEL
)

main = console.main_panel()
main.add_message("Interactive Agent — type your messages. Use /exit to quit.")

bot.interact()
