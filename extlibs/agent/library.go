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

class Agent:
    def __init__(self, client, tools=None, system_prompt="", model="", memory=None, max_tokens=32000, compaction_threshold=80, request_timeout=300):
        self.client = client
        self.system_prompt = system_prompt
        self.model = model
        self.messages = []
        self.memory = memory
        self.max_tokens = max_tokens
        self.compaction_threshold = compaction_threshold
        self.request_timeout = request_timeout

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

            # Append memory instructions to system prompt
            self.system_prompt = system_prompt + _MEMORY_INSTRUCTIONS

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

    def _completion_kwargs(self, include_tools=True):
        kwargs = {}
        if include_tools and self.tool_schemas:
            kwargs["tools"] = self.tool_schemas
        if self.request_timeout and self.request_timeout > 0:
            kwargs["timeout"] = self.request_timeout
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

    def _tool_summary(self, tool_call):
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
                    if len(text) > 40:
                        text = text[:37] + "..."
                    parts.append(str(key) + "=" + text)
                return name + "(" + ", ".join(parts) + ")"
        except:
            pass

        return name

    def _compact_safe_suffix_start(self, conversation):
        if not conversation:
            return 0

        last = conversation[-1]
        last_role = last.get("role", "")

        if last_role == "user":
            return len(conversation) - 1

        if last_role == "tool":
            idx = len(conversation) - 1
            while idx >= 0 and conversation[idx].get("role", "") == "tool":
                idx = idx - 1
            if idx >= 0:
                msg = conversation[idx]
                if msg.get("role", "") == "assistant" and msg.get("tool_calls"):
                    return idx
            return len(conversation) - 1

        if last_role == "assistant" and last.get("tool_calls"):
            return len(conversation) - 1

        return max(0, len(conversation) - 1)

    def _summary_text_for_message(self, msg):
        role = msg.get("role", "?")
        content = str(msg.get("content", ""))

        if role == "assistant" and msg.get("tool_calls"):
            calls = self._normalise_tool_calls(msg.get("tool_calls", []))
            if len(calls) > 0:
                summaries = [self._tool_summary(tc) for tc in calls]
                return "[assistant tool calls]: " + ", ".join(summaries)

        if role == "tool":
            tool_text = content.strip()
            if len(tool_text) > 500:
                tool_text = tool_text[:497] + "..."
            if tool_text:
                return "[tool]: " + tool_text
            return ""

        if content:
            if len(content) > 1000:
                content = content[:997] + "..."
            return "[" + role + "]: " + content

        return ""

    def _should_compact(self):
        """Check if the conversation history should be compacted."""
        if self.max_tokens <= 0 or self.compaction_threshold <= 0:
            return False
        threshold_tokens = int(self.max_tokens * self.compaction_threshold / 100)
        usage = ai.estimate_tokens(self.messages, {})
        return usage.prompt_tokens >= threshold_tokens

    def _compact_messages(self):
        """Compact the conversation history by asking the AI to summarize it."""
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

        # Build a summary of the compactable part of the conversation only
        summary_parts = []
        for msg in summary_source:
            text = self._summary_text_for_message(msg)
            if text:
                summary_parts.append(text)

        if not summary_parts:
            return

        summary_prompt = "Summarize the following conversation concisely, preserving all key facts, decisions, code, and context needed to continue. Do not add commentary or filler:\n\n" + "\n".join(summary_parts)

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

        if not summary:
            return

        # Rebuild: system prompt + summary as context + protected live suffix
        new_messages = []
        if system_msg:
            new_messages.append(system_msg)
        new_messages.append({"role": "user", "content": "[Previous conversation summary]\n" + summary})
        new_messages.append({"role": "assistant", "content": "Understood. I have the context from our previous conversation and am ready to continue."})
        for msg in protected_suffix:
            new_messages.append(msg)

        self.messages = new_messages

    def _execute_tools(self, tool_calls):
        """Execute tool calls and return list of tool results."""
        tool_results = []
        for idx in range(len(tool_calls)):
            tool_call = self._normalise_tool_call(tool_calls[idx], idx)
            tool_func = tool_call["function"]
            tool_name = tool_func["name"]
            tool_args_str = tool_func["arguments"]
            tool_id = tool_call["id"]

            try:
                # Parse arguments
                tool_args = json.loads(tool_args_str)

                # Strip {function_name:...} wrapper from tool name if present
                if tool_name.startswith("{") and ":" in tool_name:
                    parts = tool_name.split(":", 1)
                    if len(parts) == 2 and parts[1].endswith("}"):
                        tool_name = parts[1][:-1]

                # Strip function_name_ from tool name if present
                if tool_name.startswith("function_name_"):
                    tool_name = tool_name[len("function_name_"):]

                # Strip {...} wrapper from argument keys if present (e.g., {name} -> name)
                cleaned_args = {}
                for key, value in tool_args.items():
                    clean_key = key
                    if clean_key.startswith("{") and clean_key.endswith("}"):
                        clean_key = clean_key[1:-1]
                    cleaned_args[clean_key] = value
                tool_args = cleaned_args

                # Get handler from tools
                if self.tools is None:
                    tool_results.append({
                        "role": "tool",
                        "tool_call_id": tool_id,
                        "content": "error: no tools configured"
                    })
                    continue

                handler = self.tools.get_handler(tool_name)
                result = handler(tool_args)
                # Auto-encode complex types to JSON
                if isinstance(result, (dict, list, tuple)):
                    content = json.dumps(result)
                else:
                    content = str(result)
                tool_results.append({
                    "role": "tool",
                    "tool_call_id": tool_id,
                    "content": content
                })
            except Exception as e:
                tool_results.append({
                    "role": "tool",
                    "tool_call_id": tool_id,
                    "content": "error: " + str(e)
                })

        return tool_results

    def trigger(self, message, max_iterations=1):
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
        last_response = None
        for i in range(max_iterations):
            # Auto-compact if conversation exceeds threshold
            if self._should_compact():
                self._compact_messages()

            # Call completion with tools
            response = self.client.completion(self.model, self.messages, **self._completion_kwargs())

            # Get message from response
            choices = self._field(response, "choices", [])
            if not choices or len(choices) == 0:
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
                break

            # Execute tool calls via the registry
            tool_results = ai.execute_tool_calls(self.tools, calls)
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

        # If we hit max iterations and last_response has no content, create a summary
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
