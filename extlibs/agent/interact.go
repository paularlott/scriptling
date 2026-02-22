package agent

const (
	InteractLibraryName = "scriptling.ai.agent.interact"
)

const InteractScript = `
import scriptling.console as console
import scriptling.ai.agent as agent_module
import scriptling.ai as ai

_OriginalAgent = agent_module.Agent

class Agent(_OriginalAgent):
    def interact(self):
        console.set_status("scriptling", self.model if self.model else "default")

        def cmd_clear(args):
            self.messages = []
            if self.system_prompt:
                self.messages.append({"role": "system", "content": self.system_prompt})
            console.clear_output()

        def cmd_model(args):
            if not args or args == "none":
                self.model = ""
                console.print("Model reset to default.")
            else:
                self.model = args
                console.print("Model set to: " + self.model)
            console.set_status("scriptling", self.model if self.model else "default")
            console.set_labels("", self.model if self.model else "Assistant", "")

        def cmd_history(args):
            for msg in self.messages:
                role = msg.get("role", "?")
                content = msg.get("content", "")
                if content:
                    console.print("[" + role + "] " + str(content)[:120])

        console.register_command("clear", "Clear conversation history and screen", cmd_clear)
        console.register_command("model", "Switch model (or 'none' for default)", cmd_model)
        console.register_command("history", "Show conversation history", cmd_history)

        def on_submit(user_input):
            cancelled = [False]
            def on_esc():
                cancelled[0] = True
                console.spinner_stop()
            console.on_escape(on_esc)

            # Add system prompt if first message
            if len(self.messages) == 0 and self.system_prompt:
                self.messages.append({"role": "system", "content": self.system_prompt})
            msg_index = len(self.messages)
            self.messages.append({"role": "user", "content": user_input})

            console.spinner_start("Thinking")

            # Agentic loop with streaming on final response
            for i in range(20):
                if cancelled[0]:
                    break

                # Non-streaming call for tool use iterations
                response = self.client.completion(self.model, self.messages, tools=self.tool_schemas)

                if not response.choices or len(response.choices) == 0:
                    break

                message = response.choices[0].message

                tool_calls = message.tool_calls if hasattr(message, "tool_calls") else None

                if not tool_calls or len(tool_calls) == 0:
                    # Final response — use streaming
                    console.spinner_stop()
                    if cancelled[0]:
                        break

                    stream = self.client.completion_stream(self.model, self.messages, tools=self.tool_schemas)
                    full_content = ""
                    console.stream_start()
                    while True:
                        if cancelled[0]:
                            break
                        chunk = stream.next()
                        if chunk is None:
                            break
                        if chunk.choices and len(chunk.choices) > 0:
                            delta = chunk.choices[0].delta
                            if delta.content:
                                console.stream_chunk(delta.content)
                                full_content = full_content + delta.content
                    console.stream_end()

                    if not cancelled[0] and stream.err() is None:
                        result = ai.extract_thinking(full_content)
                        self.messages.append({"role": "assistant", "content": result["content"]})
                    else:
                        self.messages = self.messages[:msg_index]
                    break

                # Execute tool calls
                import json
                tool_results = []
                for tool_call in tool_calls:
                    tool_func = tool_call.function
                    tool_name = tool_func.name
                    tool_args = json.loads(tool_func.arguments)
                    tool_id = tool_call.id

                    try:
                        handler = self.tools.get_handler(tool_name)
                        result = handler(tool_args)
                        tool_results.append({"role": "tool", "tool_call_id": tool_id, "content": str(result)})
                    except Exception as e:
                        tool_results.append({"role": "tool", "tool_call_id": tool_id, "content": "error: " + str(e)})

                self.messages.append({
                    "role": "assistant",
                    "content": message.content if message.content else "",
                    "tool_calls": tool_calls
                })
                for tr in tool_results:
                    self.messages.append(tr)

            if cancelled[0]:
                self.messages = self.messages[:msg_index]

        console.on_submit(on_submit)
        console.run()

agent_module.Agent = Agent
Agent
`

// RegisterInteract registers the interact library as a sub-library
func RegisterInteract(registrar interface{ RegisterScriptLibrary(string, string) error }) error {
	return registrar.RegisterScriptLibrary(InteractLibraryName, InteractScript)
}
