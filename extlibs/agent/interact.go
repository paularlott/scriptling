package agent

const (
	InteractLibraryName = "scriptling.ai.agent.interact"
)

const InteractScript = `
import json
import scriptling.console as console
import scriptling.ai.agent as agent_module
import scriptling.ai as ai

_OriginalAgent = agent_module.Agent

class Agent(_OriginalAgent):
    def _is_error_result(self, value):
        return type(value) == "ERROR" or str(value).startswith("ERROR:")

    def _show_error(self, main, err):
        console.spinner_stop()
        main.add_message("[Error: " + str(err) + "]", label="Error", role="system")

    def _stream_reasoning_chunk(self, main, text, reasoning_state):
        if not text:
            return
        if not reasoning_state["open"]:
            main.stream_start(label="Thinking", role="thinking")
            reasoning_state["open"] = True
        main.stream_chunk(text)

    def _stream_content_chunk(self, main, text, reasoning_state, content_state):
        if not text:
            return
        if reasoning_state["open"]:
            main.stream_end()
            reasoning_state["open"] = False
        if not content_state["open"]:
            main.stream_start()
            content_state["open"] = True
        main.stream_chunk(text)

    def _tool_result_summary(self, tool_call, tool_result):
        summary = self._tool_summary(tool_call)
        content = ""
        if isinstance(tool_result, dict):
            content = str(tool_result.get("content", ""))
        else:
            content = str(tool_result)

        status = "done"
        lowered = content.lower()
        if lowered.startswith("error:") or lowered.startswith("[error"):
            status = "error"

        preview = content.strip()
        if len(preview) > 80:
            preview = preview[:77] + "..."

        if preview:
            return status + " " + summary + " -> " + preview
        return status + " " + summary

    def _tool_message(self, tool_call, tool_result):
        return "Calling " + self._tool_summary(tool_call) + "\n" + self._tool_result_summary(tool_call, tool_result)

    def interact(self, max_iterations=25):
        main = console.main_panel()
        console.set_status("scriptling", self.model if self.model else "default")

        def cmd_clear(args):
            self.messages = []
            if self.system_prompt:
                self.messages.append({"role": "system", "content": self.system_prompt})
            main.clear()

        def cmd_model(args):
            if not args or args == "none":
                self.model = ""
                main.add_message("Model reset to default.")
            else:
                self.model = args
                main.add_message("Model set to: " + self.model)
            console.set_status("scriptling", self.model if self.model else "default")
            console.set_labels("", self.model if self.model else "Assistant", "")

        def cmd_history(args):
            for msg in self.messages:
                role = msg.get("role", "?")
                content = msg.get("content", "")
                if content:
                    main.add_message("[" + role + "] " + str(content)[:120])

        console.register_command("clear", "Clear conversation history and screen", cmd_clear)
        console.register_command("model", "Switch model (or 'none' for default)", cmd_model)
        console.register_command("history", "Show conversation history", cmd_history)

        def on_submit(user_input):
            cancelled = [False]
            def on_esc():
                cancelled[0] = True
                console.spinner_stop()
            console.on_escape(on_esc)

            if len(self.messages) == 0 and self.system_prompt:
                self.messages.append({"role": "system", "content": self.system_prompt})
            msg_index = len(self.messages)
            self.messages.append({"role": "user", "content": user_input})

            console.spinner_start("Working")

            hit_limit = False
            for i in range(max_iterations):
                if cancelled[0]:
                    break

                # Auto-compact if conversation exceeds threshold
                if self._should_compact():
                    console.spinner_start("Compacting")
                    self._compact_messages()
                    console.spinner_start("Working")

                try:
                    stream = self.client.completion_stream(self.model, self.messages, **self._completion_kwargs())
                except Exception as e:
                    self._show_error(main, e)
                    self.messages = self.messages[:msg_index]
                    break

                reasoning_state = {"open": False}
                content_state = {"open": False}
                seen_tools = set()
                pending_tool_calls = []

                def on_event(event):
                    if cancelled[0]:
                        return
                    if event["type"] == "reasoning":
                        self._stream_reasoning_chunk(main, event["content"], reasoning_state)
                    elif event["type"] == "content":
                        self._stream_content_chunk(main, event["content"], reasoning_state, content_state)
                    elif event["type"] == "tool_call":
                        tc = event["tool_call"]
                        tc_id = tc.get("id", "")
                        fn = tc.get("function", {})
                        name = fn.get("name", "")
                        args_text = str(fn.get("arguments", "{}"))
                        seen_key = tc_id if tc_id else name + ":" + args_text
                        if name and seen_key not in seen_tools:
                            seen_tools.add(seen_key)
                            # Close any open streams before showing tool message
                            if reasoning_state["open"]:
                                main.stream_end()
                                reasoning_state["open"] = False
                            if content_state["open"]:
                                main.stream_end()
                                content_state["open"] = False
                            pending_tool_calls.append(tc)

                result = ai.collect_stream(stream, first_chunk_timeout=120, chunk_timeout=60, on_event=on_event)

                # Close any open streams
                if reasoning_state["open"]:
                    main.stream_end()
                if content_state["open"]:
                    main.stream_end()

                if cancelled[0]:
                    self.messages = self.messages[:msg_index]
                    break

                if result["timed_out"]:
                    self._show_error(main, "Streaming stalled before completion")
                    self.messages = self.messages[:msg_index]
                    break

                # Execute tool calls if present
                calls = result["tool_calls"]
                if len(calls) > 0:
                    console.spinner_start("Running tools")
                    tool_results = ai.execute_tool_calls(self.tools, calls)
                    if self._is_error_result(tool_results):
                        if len(calls) > 0:
                            main.add_message(self._tool_message(calls[0], {"content": str(tool_results)}), label="Tool", role="tool")
                        self._show_error(main, tool_results)
                        self.messages = self.messages[:msg_index]
                        break

                    for idx in range(min(len(calls), len(tool_results))):
                        tr = tool_results[idx]
                        main.add_message(self._tool_message(calls[idx], tr), label="Tool", role="tool")

                    self.messages.append(result["assistant_message"])
                    for tr in tool_results:
                        self.messages.append(tr)

                    console.spinner_start("Working")
                    if i == max_iterations - 1:
                        hit_limit = True
                    continue

                # No tool calls — final response
                console.spinner_stop()
                self.messages.append(result["assistant_message"])
                break

            if hit_limit and not cancelled[0]:
                console.spinner_stop()
                main.add_message("[Reached max iterations (" + str(max_iterations) + "). Type 'continue' or ask me to proceed.]", label="System")

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
