#!/usr/bin/env scriptling
"""
new_scriptlingcoder - Console-connected agent prototype for debugging tool calls.

WARNING: This example executes AI-generated code and shell commands.
It may modify or delete files. Use at your own risk!
"""

import glob
import os
import re
import subprocess

import scriptling.ai as ai
import scriptling.ai.agent.interact as interact
import scriptling.console as console


# Configuration from environment
BASE_URL = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:1234/v1")
MODEL = os.getenv("OPENAI_MODEL", "mistralai/ministral-3-3b")
API_KEY = os.getenv("OPENAI_API_KEY", "")


def read_file(args):
    try:
        content = os.read_file(args["path"])
    except Exception as err:
        return "error: " + str(err)
    lines = content.split("\n")
    offset = args.get("offset", 0)
    limit = args.get("limit", 200)
    result = []
    for idx in range(limit):
        line_num = offset + idx
        if line_num < len(lines):
            result.append(lines[line_num])
    if offset + limit < len(lines):
        result.append("... truncated, " + str(len(lines) - (offset + limit)) + " more lines remain")
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

    try:
        os.write_file(args["path"], replacement)
        return "ok"
    except Exception as err:
        return "error: " + str(err)


def glob_files(args):
    root = args.get("path", ".")
    files = glob.glob(args["pat"], root)
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
    try:
        result = subprocess.run(args["cmd"], capture_output=True, shell=True, text=True)
        output = result.stdout
        if result.stderr:
            output = output + result.stderr if output else result.stderr
        return output if output else "(empty)"
    except Exception as err:
        return "error: " + str(err)


def build_tools():
    tools = ai.ToolRegistry()
    tools.add("read", "Read plain file text. Defaults to 200 lines unless limit is provided.", {"path": "string", "offset": "integer?", "limit": "integer?"}, read_file)
    tools.add("write", "Write content to file", {"path": "string", "content": "string"}, write_file)
    tools.add("edit", "Replace old with new in file", {"path": "string", "old": "string", "new": "string", "all": "boolean?"}, edit_file)
    tools.add("glob", "Find files by pattern, sorted by mtime", {"pat": "string", "path": "string?"}, glob_files)
    tools.add("grep", "Search files for regex pattern", {"pat": "string", "path": "string?"}, grep_files)
    tools.add("bash", "Run shell command", {"cmd": "string"}, run_bash)
    return tools


client = ai.Client(BASE_URL, api_key=API_KEY)
tools = build_tools()

bot = interact.Agent(
    client,
    tools=tools,
    system_prompt="Concise coding assistant. cwd: " + os.getcwd() + ". Use tools when you need to inspect files, edit files, search the repo, or run shell commands.",
    model=MODEL
)

main = console.main_panel()
main.add_message(
    console.styled(console.PRIMARY, "ScriptlingCoder") + " - type your coding requests.\n" +
    console.styled(console.DIM, "Type '/exit' to quit.") + "\n\n" +
    console.styled(console.DIM, "Model: ") + MODEL + "\n" +
    console.styled(console.DIM, "Base URL: ") + BASE_URL + "\n\n" +
    "Available tools: read, write, edit, glob, grep, bash.\n" +
    "Use `read` with `offset` and `limit` to inspect larger files in chunks.\n\n" +
    console.styled(console.SECONDARY, "WARNING: This example executes AI-generated code and shell commands, use at your own risk!")
)

bot.interact()
