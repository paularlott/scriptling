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
    def __init__(self, client, tools=None, system_prompt="", model="", memory=None):
        self.client = client
        self.system_prompt = system_prompt
        self.model = model
        self.messages = []
        self.memory = memory

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

    def _execute_tools(self, tool_calls):
        """Execute tool calls and return list of tool results."""
        tool_results = []
        for tool_call in tool_calls:
            tool_func = tool_call.function
            tool_name = tool_func.name
            tool_args_str = tool_func.arguments
            tool_id = tool_call.id

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

            try:
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
            # Call completion with tools
            response = self.client.completion(self.model, self.messages, tools=self.tool_schemas)

            # Get message from response
            if not response.choices or len(response.choices) == 0:
                break

            choice = response.choices[0]
            message = choice.message
            last_response = message

            # Strip thinking blocks from content using extract_thinking
            if message.content:
                result = ai.extract_thinking(message.content)
                message.content = result["content"]

            # Check for tool calls
            tool_calls = message.tool_calls if hasattr(message, "tool_calls") else None

            if not tool_calls or len(tool_calls) == 0:
                # Add assistant message and break
                self.messages.append({"role": "assistant", "content": message.content})
                last_response = message
                break

            # Execute tool calls
            tool_results = self._execute_tools(tool_calls)

            # Add assistant message with tool calls
            self.messages.append({
                "role": "assistant",
                "content": message.content if message.content else "",
                "tool_calls": tool_calls
            })

            # Add tool results
            for tr in tool_results:
                self.messages.append(tr)

        # If we hit max iterations and last_response has no content, create a summary
        if last_response and (not last_response.content or last_response.content == ""):
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
