#!/usr/bin/env scriptling
"""
scriptlingcoder - AI coding assistant with tool execution
Inspired by https://github.com/1rgs/nanocode

WARNING: This is an example that executes AI-generated code and shell commands.
It may modify or delete files. Use at your own risk!
"""

import scriptling.ai as ai, scriptling.console as console, glob, json, os, re, subprocess

# Configuration from environment
BASE_URL = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:1234/v1")
MODEL = os.getenv("OPENAI_MODEL", "qwen3-coder-30b-a3b-instruct-mlx")
API_KEY = os.getenv("OPENAI_API_KEY", "")

# ANSI colors
ESC = chr(27)
RESET = ESC + "[0m"
BOLD = ESC + "[1m"
DIM = ESC + "[2m"
BLUE = ESC + "[34m"
CYAN = ESC + "[36m"
GREEN = ESC + "[32m"
YELLOW = ESC + "[33m"
RED = ESC + "[31m"


# --- Tool implementations ---

def read_file(args):
    content = os.read_file(args["path"])
    lines = content.split("\n")
    offset = args.get("offset", 0)
    limit = args.get("limit", len(lines))
    result = []
    for idx in range(limit):
        line_num = offset + idx
        if line_num < len(lines):
            result.append(str(line_num + 1).rjust(4) + "| " + lines[line_num])
    return "\n".join(result)


def write_file(args):
    os.write_file(args["path"], args["content"])
    return "ok"


def edit_file(args):
    text = os.read_file(args["path"])
    old = args["old"]
    new = args["new"]

    if old not in text:
        return "error: old_string not found"

    count = text.count(old)
    if not args.get("all") and count > 1:
        return "error: old_string appears " + str(count) + " times, must be unique (use all=true)"

    if args.get("all"):
        replacement = text.replace(old, new)
    else:
        replacement = text.replace(old, new, 1)

    os.write_file(args["path"], replacement)
    return "ok"


def glob_files(args):
    pattern = args.get("path", ".") + "/" + args["pat"]
    pattern = pattern.replace("//", "/")
    files = glob.glob(pattern, ".")

    # Sort by mtime descending
    files = sorted(files, key=lambda f: os.path.getmtime(f) if os.path.isfile(f) else 0, reverse=True)

    return "\n".join(files) if len(files) > 0 else "none"


def grep_files(args):
    pattern = re.compile(args["pat"])
    hits = []
    files = glob.glob("**/*", args.get("path", "."))

    for filepath in files:
        try:
            content = os.read_file(filepath)
            lines = content.split("\n")
            for line_num in range(len(lines)):
                if pattern.search(lines[line_num]):
                    hits.append(filepath + ":" + str(line_num + 1) + ":" + lines[line_num].rstrip())
                if len(hits) >= 50:
                    break
        except:
            pass
        if len(hits) >= 50:
            break

    return "\n".join(hits) if len(hits) > 0 else "none"


def run_bash(args):
    result = subprocess.run(args["cmd"])
    if result:
        return result if type(result) == type("") else str(result)
    return "(empty)"


# --- Tool registry ---

TOOLS = {
    "read": ["Read file with line numbers", {"path": "string", "offset": "number?", "limit": "number?"}, read_file],
    "write": ["Write content to file", {"path": "string", "content": "string"}, write_file],
    "edit": ["Replace old with new in file", {"path": "string", "old": "string", "new": "string", "all": "boolean?"}, edit_file],
    "glob": ["Find files by pattern, sorted by mtime", {"pat": "string", "path": "string?"}, glob_files],
    "grep": ["Search files for regex pattern", {"pat": "string", "path": "string?"}, grep_files],
    "bash": ["Run shell command", {"cmd": "string"}, run_bash]
}


def run_tool(name, args):
    try:
        return TOOLS[name][2](args)
    except Exception as err:
        return "error: " + str(err)


def make_tool_schema():
    result = []
    for name in TOOLS.keys():
        description = TOOLS[name][0]
        params = TOOLS[name][1]
        properties = {}
        required = []

        for param_name in params.keys():
            param_type = params[param_name]
            is_optional = param_type.endswith("?")
            base_type = param_type.rstrip("?")

            if base_type == "number":
                properties[param_name] = {"type": "integer"}
            else:
                properties[param_name] = {"type": base_type}

            if not is_optional:
                required.append(param_name)

        result.append({
            "type": "function",
            "function": {
                "name": name,
                "description": description,
                "parameters": {
                    "type": "object",
                    "properties": properties,
                    "required": required
                }
            }
        })

    return result


def call_ai(client, messages):
    response = client.completion(MODEL, messages)

    # Check for API errors
    if hasattr(response, "error"):
        print("\n" + RED + "⏺ API Error: " + str(response.error) + RESET)
        return None

    return response


def separator():
    return DIM + ("─" * 80) + RESET


def render_markdown(text):
    result = ""
    i = 0
    while i < len(text):
        if i + 1 < len(text) and text[i:i+2] == "**":
            j = text.find("**", i + 2)
            if j != -1:
                result = result + BOLD + text[i+2:j] + RESET
                i = j + 2
            else:
                result = result + text[i]
                i = i + 1
        else:
            result = result + text[i]
            i = i + 1
    return result


def main():
    print(BOLD + "scriptlingcoder" + RESET + " | " + DIM + MODEL + " | " + os.getcwd() + RESET)
    print(DIM + "Inspired by https://github.com/1rgs/nanocode" + RESET)
    print(YELLOW + "⚠ WARNING: This tool executes AI-generated code. Use at your own risk!" + RESET + "\n")

    # Create AI client
    client = ai.new_client(BASE_URL, api_key=API_KEY)

    # Set custom tools
    client.set_tools(make_tool_schema())

    messages = []
    system_prompt = "Concise coding assistant. cwd: " + os.getcwd()

    while True:
        try:
            print(separator())
            user_input = console.input(BOLD + BLUE + "❯" + RESET + " ").strip()
            print(separator())

            if not user_input:
                continue
            if user_input == "/q" or user_input == "exit":
                break
            if user_input == "/c":
                messages = []
                print(GREEN + "⏺ Cleared conversation" + RESET)
                continue

            # Add user message
            messages.append({"role": "user", "content": user_input})

            # Agentic loop
            for iteration in range(20):
                # Add system prompt to first message
                if len(messages) == 1:
                    messages.insert(0, {"role": "system", "content": system_prompt})

                response = call_ai(client, messages)
                if not response:
                    break

                # Get message from response
                if not response.choices or len(response.choices) == 0:
                    break

                choice = response.choices[0]
                message = choice.message

                # Display text content
                if message.content:
                    print("\n" + CYAN + "⏺" + RESET + " " + render_markdown(message.content))

                # Check for tool calls
                tool_calls = message.tool_calls if hasattr(message, "tool_calls") else []

                if not tool_calls or len(tool_calls) == 0:
                    # Add assistant message and break
                    messages.append({"role": "assistant", "content": message.content})
                    break

                # Process tool calls
                tool_results = []
                for tool_call in tool_calls:
                    # tool_call is a dict
                    tool_func = tool_call["function"]
                    tool_name = tool_func["name"]
                    tool_args_str = tool_func["arguments"]
                    tool_id = tool_call["id"]

                    # Arguments come as JSON string, parse it
                    tool_args = json.loads(tool_args_str)

                    # Display tool call
                    arg_values = list(tool_args)
                    arg_preview = str(tool_args[arg_values[0]])[:50] if len(arg_values) > 0 else ""
                    print("\n" + GREEN + "⏺ " + tool_name.capitalize() + RESET + "(" + DIM + arg_preview + RESET + ")")

                    # Execute tool
                    result = run_tool(tool_name, tool_args)

                    # Display result preview
                    result_lines = result.split("\n")
                    preview = result_lines[0][:60]
                    if len(result_lines) > 1:
                        preview = preview + " ... +" + str(len(result_lines) - 1) + " lines"
                    elif len(result_lines[0]) > 60:
                        preview = preview + "..."
                    print("  " + DIM + "⎿  " + preview + RESET)

                    # Add tool result
                    tool_results.append({
                        "role": "tool",
                        "tool_call_id": tool_id,
                        "content": result
                    })

                # Add assistant message with tool calls
                messages.append({
                    "role": "assistant",
                    "content": message.content,
                    "tool_calls": tool_calls
                })

                # Add tool results
                for tr in tool_results:
                    messages.append(tr)

            print()

        except Exception as e:
            print(RED + "Error: " + str(e) + RESET)
            break


main()
