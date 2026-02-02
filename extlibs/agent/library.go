package agent

import (
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

const (
	AgentLibraryName = "scriptling.agent"
)

// Register registers the agent library
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(buildLibrary())
}

func buildLibrary() *object.Library {
	// The agent is implemented entirely in scriptling code
	// We just register the Agent class from the script
	script := `
import json

class Agent:
    def __init__(self, client, tools=None, system_prompt="", model=""):
        self.client = client
        self.tools = tools
        self.system_prompt = system_prompt
        self.model = model
        self.messages = []
        
        # Set tools on client if provided
        if tools is not None:
            schemas = tools.build()
            client.set_tools(schemas)
    
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
            # Call completion
            if self.model:
                response = self.client.completion(self.model, self.messages)
            else:
                response = self.client.completion(self.messages)
            
            # Get message from response
            if not response.choices or len(response.choices) == 0:
                break
            
            choice = response.choices[0]
            message = choice.message
            last_response = message
            
            # Strip thinking blocks from content for non-interactive use
            if message.content:
                import re
                message.content = re.sub(r'<think>.*?</think>', '', message.content, flags=re.DOTALL).strip()
            
            # Check for tool calls
            tool_calls = message.tool_calls if hasattr(message, "tool_calls") else None
            
            if not tool_calls or len(tool_calls) == 0:
                # Add assistant message and break
                self.messages.append({"role": "assistant", "content": message.content})
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
                "content": message.content,
                "tool_calls": tool_calls
            })
            
            # Add tool results
            for tr in tool_results:
                self.messages.append(tr)
        
        return last_response
    
    def get_messages(self):
        return self.messages
    
    def set_messages(self, messages):
        self.messages = messages

Agent
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	result, err := p.Eval(script)
	if err != nil {
		panic("Failed to create Agent class: " + err.Error())
	}

	builder := object.NewLibraryBuilder(AgentLibraryName, "Agentic AI loop for tool execution")
	builder.Constant("Agent", result)
	return builder.Build()
}
