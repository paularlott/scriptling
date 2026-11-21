package agent

const (
	AgentLibraryName = "scriptling.ai.agent"
)

const agentScript = `
import json
import scriptling.ai as ai

_MEMORY_INSTRUCTIONS = """

## Memory

You have persistent memory across conversations via the following tools:
- memory_remember(content, type, importance) — store a fact, preference, event or note
- memory_recall(query, limit, type) — search memory by keyword; omit query to get recent context
- memory_forget(id) — remove a memory by its ID

Guidelines:
- At the start of each conversation, call memory_recall() with no query to load recent context.
- Store one fact per memory — do not combine multiple subjects into a single memory_remember() call.
- Keep memory content concise: a single clear sentence, no padding or filler.
- Be proactive: if information comes up in conversation that could be useful in a future session, store it without waiting to be asked. When in doubt, store it.
- Store technical details, product names, configurations, project context, decisions made, and anything the user might ask about again later.
- When the user shares personal information, preferences, or important facts, store them immediately with memory_remember().
- Use type="preference" for how the user likes things done. Use type="fact" for objective information. Use type="event" for things that happened. Use type="note" for general notes.
- Use importance=0.9 for critical facts (names, keys, deadlines) and importance=0.5 for general notes.
- Before answering questions that may benefit from past context, call memory_recall(query) first.
- Do not mention the memory tools to the user unless asked — use them silently.
"""

_MCP_SKILLS_INSTRUCTIONS = """

## Skills

Skills from the connected MCP servers are listed below. To use a skill, call the get_skill tool with the skill's URI, read the instructions it returns and follow them.
"""

_SUMMARY_MARKER = "[Previous conversation summary]"

_SUMMARY_PROMPT = """Summarise the conversation below so the work can continue from the summary alone: the original messages are about to be discarded. Include:
- what the user asked for, and any constraints or preferences they gave
- what has been done so far, including files changed and commands run
- what was learned: concrete facts with file paths, line numbers, names, values and error messages, quoted exactly where they will be needed again
- what is still to be done
Be specific rather than brief: anything left out will have to be looked up again. Do not add commentary.

"""

_LIMIT_PROMPT = "You have reached the limit on tool calls for this request, so no more tools can be called. Answer now from what you have found so far, and say plainly what is unfinished or unverified."

_REPEAT_NOTE = "\n\n[Note: this exact call has returned this same result {count} times. Repeating it will not change anything: try a different approach, or give your answer.]"

# The share of the compaction threshold that the most recent messages may
# occupy and still be kept word for word when the history is compacted.
_KEEP_RECENT = 0.5

# _make_mcp_tool_handler returns a handler that forwards a tool call to an MCP
# server. A factory (rather than a lambda in the loop) binds server and tool
# name at creation time; default arguments and closures over loop variables
# are evaluated at call time, so they would late-bind to the last iteration.
def _make_mcp_tool_handler(server, name):
    def handler(args):
        return server.call_tool(name, args)
    return handler

# _make_get_skill_handler returns the handler for the get_skill tool. It routes
# a namespaced skill URI (skill://<namespace>/...) to the owning server by its
# first path segment, strips the namespace, and reads the original URI from
# that server. Errors come back as tool-result strings so the model can react
# instead of the loop aborting.
def _make_get_skill_handler(servers):
    def handler(args):
        uri = args.get("uri", "")
        if uri.startswith("skill://"):
            parts = uri[len("skill://"):].split("/", 1)
            if len(parts) == 2 and parts[0] in servers:
                try:
                    return _mcp_resource_text(servers[parts[0]].read_resource("skill://" + parts[1]))
                except Exception as e:
                    return "Error reading skill: " + str(e)
        return "Error: unknown skill URI, use one of the skill URIs listed in the system prompt"
    return handler

def _mcp_resource_text(content):
    if isinstance(content, list):
        texts = []
        for item in content:
            text = item.get("text", "") if isinstance(item, dict) else str(item)
            if text:
                texts.append(str(text))
        return "\n".join(texts)
    if isinstance(content, dict):
        text = content.get("text", "")
        if text:
            return str(text)
        return str(content)
    return str(content)

class Agent:
    def __init__(self, client, tools=None, system_prompt="", model="", memory=None, max_tokens=32000, compaction_threshold=80, request_timeout=300, extra_body=None, mcp_servers=None):
        self.client = client
        self.system_prompt = system_prompt
        self.model = model
        self.messages = []
        self.memory = memory
        self.max_tokens = max_tokens
        self.compaction_threshold = compaction_threshold
        self.request_timeout = request_timeout
        self.extra_body = extra_body
        self.mcp_servers = mcp_servers if mcp_servers is not None else []
        # Why the last trigger() stopped: "done" (the model answered),
        # "max_iterations" (the tool-call limit was reached and the model was
        # asked for a final answer) or "no_response".
        self.stop_reason = ""

        # Wire MCP servers into the tool registry and system prompt: every
        # remote tool is registered under its namespaced name (the client's
        # namespace plus the tool name), and each server's skills are listed
        # in the system prompt with namespace-qualified URIs served by a
        # get_skill tool. Servers must each have a distinct namespace: it is
        # both the tool-name prefix and the key get_skill routes by.
        if len(self.mcp_servers) > 0:
            if tools is None:
                tools = ai.ToolRegistry()

            skill_servers = {}
            seen_namespaces = {}
            skill_lines = []
            for server in self.mcp_servers:
                namespace = getattr(server, "namespace") if hasattr(server, "namespace") else ""
                if not namespace:
                    raise ValueError("mcp_servers: every MCP client must be created with a namespace, e.g. mcp.Client(url, namespace=\"shop\")")
                if namespace in seen_namespaces:
                    raise ValueError("mcp_servers: duplicate namespace \"" + namespace + "\"; give each MCP server its own namespace")
                seen_namespaces[namespace] = True
                skill_servers[namespace] = server

                for tool in server.tools():
                    schema = tool.get("inputSchema")
                    if schema is None:
                        schema = {"type": "object", "properties": {}}
                    tools.add_schema(tool.name, tool.get("description", ""), schema, _make_mcp_tool_handler(server, tool.name))

                # A server without the Skills extension simply contributes no
                # skills; its tools still work.
                try:
                    skills = server.skills()
                except:
                    skills = []
                for skill in skills:
                    fm = skill.get("frontmatter")
                    if fm is None:
                        fm = {}
                    skill_name = fm.get("name", "")
                    if not skill_name or not skill.uri.startswith("skill://"):
                        continue
                    skill_lines.append("- " + namespace + "/" + skill_name + ": " + fm.get("description", "") + " (skill://" + namespace + "/" + skill.uri[len("skill://"):] + ")")

            # Sort so the prompt is byte-identical across runs to help prompt caches.
            skill_lines = sorted(skill_lines)

            if len(skill_lines) > 0:
                # Register get_skill only when the name is free: a caller
                # that registered their own get_skill keeps it.
                has_get_skill = True
                try:
                    tools.get_handler("get_skill")
                except:
                    has_get_skill = False
                if not has_get_skill:
                    tools.add(
                        "get_skill",
                        "Fetch an MCP skill's instructions by its skill:// URI, as listed in the system prompt",
                        {"uri": "string"},
                        _make_get_skill_handler(skill_servers)
                    )
                self.system_prompt = self.system_prompt + _MCP_SKILLS_INSTRUCTIONS + "\n".join(skill_lines)

        # Wire memory tools and augment system prompt if a memory object was provided
        if memory is not None:
            if tools is None:
                tools = ai.ToolRegistry()
            tools.add(
                "memory_remember",
                "Store a memory for later recall. Use for facts, preferences, events or notes about the user or conversation.",
                {"content": "string", "type": "string?", "importance": "number?"},
                lambda args: memory.remember(args["content"], type=args.get("type", "note"), importance=float(args.get("importance", 0.5)))
            )
            tools.add(
                "memory_recall",
                "Search stored memories by keyword. Returns relevant memories ranked by relevance. Use before answering questions that may benefit from past context.",
                {"query": "string?", "limit": "number?", "type": "string?"},
                lambda args: memory.recall(args.get("query", ""), limit=int(args.get("limit", 10)), type=args.get("type", ""))
            )
            tools.add(
                "memory_forget",
                "Remove a specific memory by its ID.",
                {"id": "string"},
                lambda args: memory.forget(args["id"])
            )

            # Append memory instructions to the system prompt. Append to
            # self.system_prompt (which may already carry the MCP skills
            # section), never rebuild from the raw parameter, or earlier
            # sections would be dropped.
            self.system_prompt = self.system_prompt + _MEMORY_INSTRUCTIONS

            # Pre-load preferences into system prompt so the LLM has immediate context
            preferences = memory.recall("", limit=50, type="preference")
            if preferences and len(preferences) > 0:
                pref_lines = ["\n## Remembered Preferences"]
                for p in preferences:
                    pref_lines.append("- " + p["content"])
                self.system_prompt = self.system_prompt + "\n".join(pref_lines)

        self.tools = tools
        # Build and store tool schemas if tools provided
        self.tool_schemas = tools.build() if tools is not None else []
        # The tool schemas are sent with every request, so they count against
        # the context budget alongside the messages.
        self._tool_schema_tokens = int(len(json.dumps(self.tool_schemas)) / 4) if self.tool_schemas else 0

    def _completion_kwargs(self, include_tools=True):
        kwargs = {}
        if include_tools and self.tool_schemas:
            kwargs["tools"] = self.tool_schemas
        if self.request_timeout and self.request_timeout > 0:
            kwargs["timeout"] = self.request_timeout
        if self.extra_body:
            kwargs["extra_body"] = self.extra_body
        return kwargs

    def _field(self, value, name, default=None):
        if value is None:
            return default
        if hasattr(value, name):
            result = getattr(value, name)
            if result is not None:
                return result
        if isinstance(value, dict):
            return value.get(name, default)
        return default

    def _is_error_result(self, value):
        return type(value) == "ERROR" or str(value).startswith("ERROR:")

    def _normalise_tool_call(self, tool_call, index=0):
        function = self._field(tool_call, "function", {})
        arguments = self._field(function, "arguments", "{}")
        if arguments is None or arguments == "":
            arguments = "{}"
        elif not isinstance(arguments, str):
            arguments = json.dumps(arguments)

        tool_name = self._field(function, "name", "")
        tool_type = self._field(tool_call, "type", "function")
        tool_id = self._field(tool_call, "id", "")
        if not tool_id:
            tool_id = "call_" + str(index)

        return {
            "id": tool_id,
            "type": tool_type,
            "function": {
                "name": tool_name,
                "arguments": arguments
            }
        }

    def _normalise_tool_calls(self, tool_calls):
        result = []
        if not tool_calls:
            return result
        for idx in range(len(tool_calls)):
            result.append(self._normalise_tool_call(tool_calls[idx], idx))
        return result

    def _tool_summary(self, tool_call, arg_limit=40):
        function = self._field(tool_call, "function", {})
        name = self._field(function, "name", "tool")
        arguments = self._field(function, "arguments", "{}")
        if not isinstance(arguments, str):
            arguments = json.dumps(arguments)

        try:
            parsed = json.loads(arguments)
            if isinstance(parsed, dict) and len(parsed) > 0:
                parts = []
                for key, value in parsed.items():
                    text = str(value)
                    if len(text) > arg_limit:
                        text = text[:arg_limit - 3] + "..."
                    parts.append(str(key) + "=" + text)
                return name + "(" + ", ".join(parts) + ")"
        except:
            pass

        return name

    def _clip(self, text, limit, hint=""):
        """Shorten text to about limit characters, keeping the start and the end."""
        if limit <= 0 or len(text) <= limit:
            return text
        head = int(limit * 2 / 3)
        tail = limit - head
        return text[:head] + "\n[... " + str(len(text) - limit) + " characters omitted" + hint + " ...]\n" + text[len(text) - tail:]

    def _estimate(self, messages):
        return ai.estimate_tokens(messages, {}).prompt_tokens

    def _threshold_tokens(self):
        return int(self.max_tokens * self.compaction_threshold / 100)

    def _run_tools(self, calls, seen):
        """Execute tool calls and return their result messages.

        A result too large for the context budget is cut down, keeping its
        start and end, so one oversized output cannot crowd out the rest of
        the conversation. seen carries, across the iterations of one request,
        the last result of each distinct call: a call repeated with the same
        arguments and the same result is flagged to the model, which
        otherwise can spend every remaining iteration on it.
        """
        tool_results = ai.execute_tool_calls(self.tools, calls)
        if self._is_error_result(tool_results):
            return tool_results

        # A single result may take up to a quarter of the budget (about four
        # characters per token, so max_tokens characters).
        limit = max(400, self.max_tokens) if self.max_tokens > 0 else 0

        for idx in range(len(tool_results)):
            result = tool_results[idx]
            content = self._clip(str(result.get("content", "")), limit, hint=": the output was too long, ask for a narrower range or a more specific search to see the rest")

            if idx < len(calls):
                function = self._field(calls[idx], "function", {})
                key = str(self._field(function, "name", "")) + "\n" + str(self._field(function, "arguments", ""))
                previous = seen.get(key)
                if previous is not None and previous[0] == content:
                    previous[1] = previous[1] + 1
                else:
                    previous = [content, 1]
                    seen[key] = previous
                if previous[1] >= 3:
                    content = content + _REPEAT_NOTE.replace("{count}", str(previous[1]))

            result["content"] = content
        return tool_results

    def _message_units(self, conversation):
        """Split a conversation into [start, end) index pairs that must stay together.

        An assistant message that calls tools and the tool results answering
        it form one unit: an API rejects a history holding one without the
        other. Every other message is a unit on its own.
        """
        units = []
        idx = 0
        while idx < len(conversation):
            msg = conversation[idx]
            end = idx + 1
            if msg.get("role", "") == "assistant" and msg.get("tool_calls"):
                while end < len(conversation) and conversation[end].get("role", "") == "tool":
                    end = end + 1
            units.append([idx, end])
            idx = end
        return units

    def _compact_safe_suffix_start(self, conversation):
        """Index of the first message kept word for word when compacting.

        The most recent messages stay as they are, up to _KEEP_RECENT of the
        compaction threshold: they are what the model is working from right
        now, and summarising them makes it fetch the same things again. The
        last unit is always kept, whatever its size.
        """
        units = self._message_units(conversation)
        if len(units) == 0:
            return 0

        budget = int(self._threshold_tokens() * _KEEP_RECENT)
        start = units[-1][0]
        used = self._estimate(conversation[start:])
        idx = len(units) - 2
        while idx >= 0:
            unit = units[idx]
            cost = self._estimate(conversation[unit[0]:unit[1]])
            if used + cost > budget:
                break
            used = used + cost
            start = unit[0]
            idx = idx - 1
        return start

    def _is_summary_message(self, msg):
        content = msg.get("content", "")
        return msg.get("role", "") == "user" and isinstance(content, str) and content.startswith(_SUMMARY_MARKER)

    def _summary_text_for_message(self, msg):
        role = msg.get("role", "?")
        content = msg.get("content", "")
        if content is None:
            content = ""
        content = str(content)

        # An earlier summary is carried over whole: it is all that is left of
        # everything before it.
        if self._is_summary_message(msg):
            return content

        if role == "tool":
            tool_text = self._clip(content.strip(), 2000)
            if tool_text:
                return "[tool result]: " + tool_text
            return ""

        parts = []
        if content:
            parts.append("[" + role + "]: " + self._clip(content, 4000))

        if role == "assistant" and msg.get("tool_calls"):
            calls = self._normalise_tool_calls(msg.get("tool_calls", []))
            if len(calls) > 0:
                summaries = [self._tool_summary(tc, arg_limit=300) for tc in calls]
                parts.append("[assistant tool calls]: " + ", ".join(summaries))

        return "\n".join(parts)

    def _should_compact(self):
        """Check if the conversation history should be compacted."""
        if self.max_tokens <= 0 or self.compaction_threshold <= 0:
            return False
        # The tool schemas use part of the budget, but never more than half
        # of it for this purpose: a large tool set must not make every
        # request look over the threshold.
        threshold_tokens = self._threshold_tokens()
        threshold_tokens = max(threshold_tokens - self._tool_schema_tokens, int(threshold_tokens / 2))
        return self._estimate(self.messages) >= threshold_tokens

    def _compact_messages(self):
        """Compact the conversation history by asking the AI to summarize the older part of it."""
        if len(self.messages) <= 3:
            return

        # Separate system prompt from conversation
        system_msg = None
        conversation = []
        for msg in self.messages:
            if msg.get("role") == "system" and system_msg is None:
                system_msg = msg
            else:
                conversation.append(msg)

        if len(conversation) <= 1:
            return

        safe_suffix_start = self._compact_safe_suffix_start(conversation)
        summary_source = conversation[:safe_suffix_start]
        protected_suffix = conversation[safe_suffix_start:]

        if len(summary_source) == 0:
            return

        # Nothing but an earlier summary (and its acknowledgement) to fold
        # in: summarising a summary again loses detail and frees nothing.
        if len(summary_source) <= 2 and self._is_summary_message(summary_source[0]):
            return

        # Build a summary of the compactable part of the conversation only
        summary_parts = []
        for msg in summary_source:
            text = self._summary_text_for_message(msg)
            if text:
                summary_parts.append(text)

        if not summary_parts:
            return

        summary_prompt = _SUMMARY_PROMPT + "\n".join(summary_parts)

        # Ask the AI to summarize
        summary_messages = []
        if system_msg:
            summary_messages.append(system_msg)
        summary_messages.append({"role": "user", "content": summary_prompt})

        response = self.client.completion(self.model, summary_messages, **self._completion_kwargs(include_tools=False))
        summary = ""
        choices = self._field(response, "choices", [])
        if choices and len(choices) > 0:
            msg = self._field(choices[0], "message", {})
            summary = self._field(msg, "content", "")
        if summary:
            summary = ai.extract_thinking(summary)["content"]

        if not summary:
            return

        # The request being worked on must survive word for word: a summary
        # of the instructions is not the instructions. When the latest user
        # message is in the part being summarised, repeat it after the summary.
        current_request = ""
        for msg in summary_source:
            if msg.get("role", "") == "user" and not self._is_summary_message(msg):
                content = msg.get("content", "")
                if isinstance(content, str):
                    current_request = content
        for msg in protected_suffix:
            if msg.get("role", "") == "user":
                current_request = ""
        summary_text = _SUMMARY_MARKER + "\n" + summary
        if current_request:
            summary_text = summary_text + "\n\n[The user's most recent request, word for word]\n" + current_request

        # Rebuild: system prompt + summary as context + protected live suffix
        new_messages = []
        if system_msg:
            new_messages.append(system_msg)
        new_messages.append({"role": "user", "content": summary_text})
        # The acknowledgement keeps user and assistant turns alternating; it
        # is left out when the kept messages already open with the assistant.
        if len(protected_suffix) == 0 or protected_suffix[0].get("role", "") != "assistant":
            new_messages.append({"role": "assistant", "content": "Understood. I have the context from our previous conversation and am ready to continue."})
        for msg in protected_suffix:
            new_messages.append(msg)

        self.messages = new_messages

    def _final_answer(self):
        """Ask the model for an answer, with no tools, after the tool-call limit.

        Returns the assistant message, or None when the request fails or
        comes back empty, leaving the history as it was.
        """
        self.messages.append({"role": "user", "content": _LIMIT_PROMPT})
        try:
            response = self.client.completion(self.model, self.messages, **self._completion_kwargs(include_tools=False))
        except Exception as e:
            self.messages.pop()
            return None

        choices = self._field(response, "choices", [])
        message = self._field(choices[0], "message", {}) if choices and len(choices) > 0 else None
        content = self._field(message, "content", "")
        if content:
            content = ai.extract_thinking(content)["content"]
        if not content:
            self.messages.pop()
            return None

        if isinstance(message, dict):
            message["content"] = content
        else:
            message.content = content
        self.messages.append({"role": "assistant", "content": content})
        return message

    def trigger(self, message, max_iterations=25):
        # Convert message to dict if string
        if type(message) == type(""):
            msg_dict = {"role": "user", "content": message}
        else:
            msg_dict = message

        # Add system prompt if first message
        if len(self.messages) == 0 and self.system_prompt:
            self.messages.append({"role": "system", "content": self.system_prompt})

        # Add user message
        self.messages.append(msg_dict)

        # Agentic loop
        self.stop_reason = ""
        last_response = None
        seen_calls = {}
        tools_pending = False
        for i in range(max_iterations):
            # Auto-compact if conversation exceeds threshold
            if self._should_compact():
                self._compact_messages()

            # Call completion with tools
            response = self.client.completion(self.model, self.messages, **self._completion_kwargs())

            # Get message from response
            choices = self._field(response, "choices", [])
            if not choices or len(choices) == 0:
                self.stop_reason = "no_response"
                tools_pending = False
                break

            choice = choices[0]
            message = self._field(choice, "message", {})
            last_response = message

            # Strip thinking blocks from content
            message_content = self._field(message, "content", "")
            if message_content:
                result = ai.extract_thinking(message_content)
                if isinstance(message, dict):
                    message["content"] = result["content"]
                else:
                    message.content = result["content"]

            # Extract normalized tool calls
            calls = ai.tool_calls(response)

            if len(calls) == 0:
                # No tool calls — add assistant message and break
                self.messages.append({"role": "assistant", "content": self._field(message, "content", "")})
                self.stop_reason = "done"
                tools_pending = False
                break

            # Execute tool calls via the registry
            tool_results = self._run_tools(calls, seen_calls)
            if self._is_error_result(tool_results):
                return tool_results

            # Add assistant message with tool calls
            self.messages.append({
                "role": "assistant",
                "content": self._field(message, "content", ""),
                "tool_calls": calls
            })

            # Add tool results
            for tr in tool_results:
                self.messages.append(tr)
            tools_pending = True

        # The loop ran out of iterations with tool results the model has not
        # answered from yet: ask it for an answer rather than handing the
        # caller raw tool output.
        if tools_pending:
            self.stop_reason = "max_iterations"
            final = self._final_answer()
            if final is not None:
                return final

        # If that produced nothing and last_response has no content, fall back to the last tool results
        if last_response and (not self._field(last_response, "content", "")):
            # Collect the last tool results
            tool_result_contents = []
            for msg in reversed(self.messages):
                if msg.get("role") == "tool":
                    tool_result_contents.append(msg.get("content", ""))
                elif msg.get("role") == "assistant":
                    break

            if tool_result_contents:
                # Reverse to get correct order
                tool_result_contents.reverse()
                # Create a response with the tool results
                class SummaryMessage:
                    def __init__(self, content):
                        self.content = content
                        self.role = "assistant"
                return SummaryMessage(" ".join(tool_result_contents))

        return last_response

    def get_messages(self):
        return self.messages

    def set_messages(self, messages):
        self.messages = messages

Agent
`

// Register registers the agent library
func Register(registrar interface{ RegisterScriptLibrary(string, string) error }) error {
	return registrar.RegisterScriptLibrary(AgentLibraryName, agentScript)
}
