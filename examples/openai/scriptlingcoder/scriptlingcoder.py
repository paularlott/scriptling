#!/usr/bin/env scriptling
"""
scriptlingcoder - AI coding assistant with tool execution
Inspired by https://github.com/1rgs/nanocode

WARNING: This is an example that executes AI-generated code and shell commands.
It may modify or delete files. Use at your own risk!
"""

import scriptling.ai as ai, scriptling.ai.agent.interact as agent, scriptling.console as console, glob, os, re, subprocess

# Configuration from environment
BASE_URL = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:1234/v1")
MODEL = os.getenv("OPENAI_MODEL", "mistralai/ministral-3-3b")
API_KEY = os.getenv("OPENAI_API_KEY", "")

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
    files = sorted(files, key=lambda f: os.getmtime(f) if os.isfile(f) else 0, reverse=True)

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
    result = subprocess.run(args["cmd"], capture_output=True, shell=True, text=True)
    output = result.stdout
    if result.stderr:
        output = output + result.stderr if output else result.stderr
    return output if output else "(empty)"

# --- Setup ---

# Create AI client
client = ai.Client(BASE_URL, api_key=API_KEY)

# Create tool registry
tools = ai.ToolRegistry()
tools.add("read", "Read file with line numbers", {"path": "string", "offset": "integer?", "limit": "integer?"}, read_file)
tools.add("write", "Write content to file", {"path": "string", "content": "string"}, write_file)
tools.add("edit", "Replace old with new in file", {"path": "string", "old": "string", "new": "string", "all": "boolean?"}, edit_file)
tools.add("glob", "Find files by pattern, sorted by mtime", {"pat": "string", "path": "string?"}, glob_files)
tools.add("grep", "Search files for regex pattern", {"pat": "string", "path": "string?"}, grep_files)
tools.add("bash", "Run shell command", {"cmd": "string"}, run_bash)

# Create agent
bot = agent.Agent(
    client,
    tools=tools,
    system_prompt="Concise coding assistant. cwd: " + os.getcwd() + ". Tools: read, write, edit files; glob to find files; grep to search; bash for shell commands.",
    model=MODEL
)

# Run interactive session
bot.interact()
