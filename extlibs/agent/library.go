package agent

const (
	AgentLibraryName = "scriptling.ai.agent"
)

const agentScript = `
import json
import scriptling.ai as ai

class Agent:
    def __init__(self, client, tools=None, system_prompt="", model=""):
        self.client = client
        self.tools = tools
        self.system_prompt = system_prompt
        self.model = model
        self.messages = []
        # Build and store tool schemas if tools provided
        self.tool_schemas = tools.build() if tools is not None else []

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
                    tool_results.append({
                        "role": "tool",
                        "tool_call_id": tool_id,
                        "content": str(result)
                    })
                except Exception as e:
                    tool_results.append({
                        "role": "tool",
                        "tool_call_id": tool_id,
                        "content": "error: " + str(e)
                    })

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
