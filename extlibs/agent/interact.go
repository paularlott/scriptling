package agent

const (
	InteractLibraryName = "scriptling.ai.agent.interact"
)

const InteractScript = `
import scriptling.console as console
import scriptling.ai.agent as agent_module
import re

# Create new Agent class that extends the original with interact
_OriginalAgent = agent_module.Agent

class Agent(_OriginalAgent):
    def interact(self):
        # ANSI colors
        ESC = chr(27)
        RESET = ESC + "[0m"
        BOLD = ESC + "[1m"
        DIM = ESC + "[2m"
        BLUE = ESC + "[34m"
        CYAN = ESC + "[36m"
        PURPLE = ESC + "[35m"
        GREEN = ESC + "[32m"
        GRAY = ESC + "[90m"
        
        separator = DIM + ("-" * 80) + RESET
        
        while True:
            print(separator)
            user_input = console.input(BOLD + BLUE + "❯" + RESET + " ").strip()
            print(separator)
            
            if not user_input:
                continue
            if user_input == "/q" or user_input == "exit":
                break
            if user_input == "/c":
                self.messages = []
                if self.system_prompt:
                    self.messages.append({"role": "system", "content": self.system_prompt})
                print(GREEN + "⏺ Cleared conversation" + RESET)
                continue
            
            # Trigger with max_iterations=20
            response = self.trigger(user_input, max_iterations=20)
            
            # Display response with thinking
            if response and hasattr(response, "content") and response.content:
                content = response.content
                
                # Extract and display thinking blocks in purple
                think_pattern = r'<think>(.*?)</think>'
                matches = re.findall(think_pattern, content, re.DOTALL)
                
                if matches:
                    for think in matches:
                        print()
                        print(PURPLE + think.strip() + RESET)
                    
                    # Remove think blocks from content
                    content = re.sub(think_pattern, '', content, flags=re.DOTALL).strip()
                
                # Format code blocks and inline code
                backtick = chr(96)
                content = re.sub(backtick + backtick + backtick + r'[a-z]*' + chr(10), GRAY, content)
                content = re.sub(backtick + backtick + backtick, RESET, content)
                # Use lambda to properly handle inline code formatting
                content = re.sub(backtick + r'([^' + backtick + r']+)' + backtick, lambda m: GRAY + m.group(1) + RESET + CYAN, content)
                
                # Display main content
                if content:
                    print()
                    print(CYAN + "⏺" + RESET + " " + content)
                print()

# Replace the Agent in the module
agent_module.Agent = Agent
Agent
`

// RegisterInteract registers the interact library as a sub-library
func RegisterInteract(registrar interface{ RegisterScriptLibrary(string, string) error }) error {
	return registrar.RegisterScriptLibrary(InteractLibraryName, InteractScript)
}
