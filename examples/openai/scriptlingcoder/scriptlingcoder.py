#!/usr/bin/env scriptling
"""
scriptlingcoder - a small coding assistant for the terminal.

It gives a model six tools (read, write, edit, glob, grep, bash), plus the
tools of any MCP servers listed in .mcp.json, and runs them in the agent loop
from scriptling.ai.agent.

WARNING: This example executes AI-generated code and shell commands without
asking first. It may modify or delete files. Use at your own risk!
"""

import glob
import json
import os
import re
import subprocess
import time

import scriptling.ai as ai
import scriptling.ai.agent.interact as interact
import scriptling.console as console
import scriptling.mcp as mcp


# Configuration from environment
BASE_URL = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:1234/v1")
MODEL = os.getenv("OPENAI_MODEL", "mistralai/ministral-3-3b")
API_KEY = os.getenv("OPENAI_API_KEY", "")
# The model's context window in tokens. The agent summarises older history as
# the conversation approaches it, so set it to what the model really has.
CONTEXT_TOKENS = int(os.getenv("OPENAI_CONTEXT_TOKENS", "32000"))

BASH_TIMEOUT = 120
MAX_LINE = 500
MAX_GREP_HITS = 100
MAX_GREP_FILE_BYTES = 1000000
SKIP_DIRS = [".git", "node_modules", "vendor", "dist", "build", "bin", "__pycache__", ".venv"]

SYSTEM_PROMPT = """You are a coding assistant working in the user's project from a terminal.

Working directory: {cwd}

How to work:
- Look before you answer: use the tools to read the code rather than guessing what it contains.
- Use paths relative to the working directory.
- Use grep and glob to find things and read to view them; keep bash for running commands such as tests and builds.
- Read a file before you edit it. Make the smallest change that does the job, and prefer edit to rewriting a whole file with write.
- After changing code, run the project's tests or build if there are any, and report what actually happened.
- If a tool returns an error, read the message and change the call; do not repeat the same call.
- When you are done, say briefly what you found or changed, naming files and line numbers."""


def to_int(value, default):
    """Models send numbers as 5, 5.0 or "5.0": accept them all."""
    try:
        return int(float(str(value)))
    except:
        return default


def read_file(args):
    try:
        content = os.read_file(args["path"])
    except Exception as err:
        return "error: " + str(err)
    lines = content.split("\n")
    if len(lines) > 0 and lines[-1] == "":
        lines = lines[:-1]
    offset = max(1, to_int(args.get("offset"), 1))
    limit = max(1, to_int(args.get("limit"), 200))
    last = min(len(lines), offset + limit - 1)
    result = []
    for line_num in range(offset, last + 1):
        line = lines[line_num - 1]
        if len(line) > MAX_LINE:
            line = line[:MAX_LINE] + " ... (line cut)"
        result.append("%5d| %s" % (line_num, line))
    if len(result) == 0:
        return "(no lines: the file has " + str(len(lines)) + " lines)"
    if last < len(lines):
        result.append("... " + str(len(lines) - last) + " more lines; read again with offset=" + str(last + 1))
    return "\n".join(result)


def write_file(args):
    try:
        os.write_file(args["path"], args["content"])
        return "ok"
    except Exception as err:
        return "error: " + str(err)


def edit_file(args):
    try:
        text = os.read_file(args["path"])
    except Exception as err:
        return "error: " + str(err)
    old = args["old_string"]
    new = args["new_string"]

    count = text.count(old)
    if count == 0:
        return "error: old_string not found in the file; it must match exactly, including indentation (and without the line-number prefix that read shows)"
    if count > 1 and not args.get("replace_all"):
        return "error: old_string appears " + str(count) + " times; include more surrounding lines to make it unique, or set replace_all=true"

    if args.get("replace_all"):
        replacement = text.replace(old, new)
    else:
        replacement = text.replace(old, new, 1)

    try:
        os.write_file(args["path"], replacement)
        return "ok: replaced " + str(count if args.get("replace_all") else 1) + " occurrence(s)"
    except Exception as err:
        return "error: " + str(err)


def is_skipped(filepath):
    for part in filepath.split("/"):
        if part in SKIP_DIRS:
            return True
    return False


def glob_files(args):
    files = [f for f in glob.glob(args["pattern"], args.get("path", "."), recursive=True) if not is_skipped(f)]
    files = sorted(files, key=lambda f: os.path.getmtime(f) if os.path.isfile(f) else 0, reverse=True)
    if len(files) == 0:
        return "no files match"
    if len(files) > 200:
        return "\n".join(files[:200]) + "\n... " + str(len(files) - 200) + " more; use a narrower pattern"
    return "\n".join(files)


def grep_files(args):
    try:
        pattern = re.compile(args["pattern"])
    except Exception as err:
        return "error: bad regular expression: " + str(err)

    root = args.get("path", ".")
    if os.path.isfile(root):
        files = [root]
    else:
        files = glob.glob(args.get("glob", "**/*"), root, recursive=True)

    hits = []
    for filepath in files:
        if is_skipped(filepath) or not os.path.isfile(filepath) or os.path.getsize(filepath) > MAX_GREP_FILE_BYTES:
            continue
        try:
            content = os.read_file(filepath)
        except:
            continue
        if "\0" in content:
            continue
        lines = content.split("\n")
        for line_num in range(len(lines)):
            if pattern.search(lines[line_num]):
                hits.append(filepath + ":" + str(line_num + 1) + ": " + lines[line_num].strip()[:MAX_LINE])
                if len(hits) >= MAX_GREP_HITS:
                    break
        if len(hits) >= MAX_GREP_HITS:
            hits.append("... stopped at " + str(MAX_GREP_HITS) + " matches; use a more specific pattern, path or glob")
            break

    return "\n".join(hits) if len(hits) > 0 else "no matches"


def run_bash(args):
    timeout = max(1, to_int(args.get("timeout"), BASH_TIMEOUT))
    started = time.time()
    try:
        result = subprocess.run(args["command"], capture_output=True, shell=True, text=True, timeout=timeout)
    except Exception as err:
        return "error: " + str(err)

    output = result.stdout
    if result.stderr:
        output = output + result.stderr
    if result.returncode != 0:
        if time.time() - started >= timeout:
            output = output + "\n[timed out after " + str(timeout) + " seconds and was stopped]"
        else:
            output = output + "\n[exit code " + str(result.returncode) + "]"
    return output.strip() if output.strip() else "(no output, exit code 0)"


def build_tools():
    tools = ai.ToolRegistry()
    tools.add("read", "Read a text file. Returns numbered lines, 200 at a time by default. offset is the line number to start from (1 is the first line); limit is how many lines to return.", {"path": "string", "offset": "integer?", "limit": "integer?"}, read_file)
    tools.add("write", "Create a file, or replace the whole of an existing one, with content.", {"path": "string", "content": "string"}, write_file)
    tools.add("edit", "Replace old_string with new_string in a file. old_string must match the file exactly and appear only once, unless replace_all is true.", {"path": "string", "old_string": "string", "new_string": "string", "replace_all": "boolean?"}, edit_file)
    tools.add("glob", "List files matching a glob pattern such as **/*.py, newest first. path is the directory to search (default: working directory).", {"pattern": "string", "path": "string?"}, glob_files)
    tools.add("grep", "Search file contents for a regular expression. Returns file:line: text. path is a file or directory (default: working directory); glob limits which files are searched, e.g. **/*.go.", {"pattern": "string", "path": "string?", "glob": "string?"}, grep_files)
    tools.add("bash", "Run a shell command in the working directory and return its output and exit code. timeout is in seconds (default " + str(BASH_TIMEOUT) + ").", {"command": "string", "timeout": "integer?"}, run_bash)
    return tools


def load_project_instructions():
    """Project notes for the model, from AGENTS.md in the working directory."""
    if not os.path.isfile("AGENTS.md"):
        return ""
    try:
        return "\n\nProject instructions (from AGENTS.md):\n" + os.read_file("AGENTS.md")[:8000]
    except:
        return ""


def load_mcp_servers():
    """Connect to the MCP servers listed in .mcp.json, if the file is there.

    The file uses the usual layout:
      {"mcpServers": {"name": {"command": "prog", "args": ["--flag"], "env": {"KEY": "value"}},
                      "other": {"url": "https://example.com/mcp", "bearer_token": "..."}}}
    Each server's tools are offered to the model as <name>__<tool>.
    """
    servers = []
    notes = []
    if not os.path.isfile(".mcp.json"):
        return servers, notes
    try:
        config = json.loads(os.read_file(".mcp.json")).get("mcpServers", {})
    except Exception as err:
        return servers, ["could not read .mcp.json: " + str(err)]

    for name, entry in config.items():
        namespace = re.sub("[^A-Za-z0-9_]", "_", name)
        try:
            if entry.get("url"):
                kwargs = {"namespace": namespace}
                if entry.get("bearer_token"):
                    kwargs["bearer_token"] = entry["bearer_token"]
                server = mcp.Client(entry["url"], **kwargs)
            else:
                kwargs = {"namespace": namespace, "args": entry.get("args", [])}
                if entry.get("env"):
                    kwargs["env"] = entry["env"]
                server = mcp.Client(entry["command"], **kwargs)
            notes.append(name + " (" + str(len(server.tools())) + " tools)")
            servers.append(server)
        except Exception as err:
            notes.append(name + " failed: " + str(err))
    return servers, notes


client = ai.Client(BASE_URL, api_key=API_KEY)
mcp_servers, mcp_notes = load_mcp_servers()

bot = interact.Agent(
    client,
    tools=build_tools(),
    mcp_servers=mcp_servers,
    system_prompt=SYSTEM_PROMPT.replace("{cwd}", os.getcwd()) + load_project_instructions(),
    model=MODEL,
    max_tokens=CONTEXT_TOKENS
)

main = console.main_panel()
main.add_message(
    console.styled(console.PRIMARY, "ScriptlingCoder") + " - type your coding requests.\n" +
    console.styled(console.DIM, "Type '/exit' to quit, '/clear' to start again, Esc to interrupt.") + "\n\n" +
    console.styled(console.DIM, "Model: ") + MODEL + "\n" +
    console.styled(console.DIM, "Base URL: ") + BASE_URL + "\n" +
    console.styled(console.DIM, "Context: ") + str(CONTEXT_TOKENS) + " tokens\n" +
    console.styled(console.DIM, "MCP servers: ") + (", ".join(mcp_notes) if len(mcp_notes) > 0 else "none (add a .mcp.json to use some)") + "\n\n" +
    "Tools: read, write, edit, glob, grep, bash.\n\n" +
    console.styled(console.SECONDARY, "WARNING: This example executes AI-generated code and shell commands without asking, use at your own risk!")
)

try:
    bot.interact()
finally:
    for server in mcp_servers:
        try:
            server.close()
        except:
            pass
