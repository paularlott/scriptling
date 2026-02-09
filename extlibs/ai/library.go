package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/paularlott/mcp"
	"github.com/paularlott/mcp/ai"
	"github.com/paularlott/mcp/ai/openai"
	"github.com/paularlott/scriptling/extlibs/ai/tools"
	"github.com/paularlott/scriptling/object"
)

const (
	AILibraryName = "scriptling.ai"
	AILibraryDesc = "AI and LLM functions for interacting with multiple AI provider APIs"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

// WrapClient wraps an AI client as a scriptling Object that can be
// passed into a script via SetObjectVar. This allows multiple clients
// to be used simultaneously.
func WrapClient(c ai.Client) object.Object {
	return createClientInstance(c)
}

// Register registers the ai library with the given registrar
// First call builds the library, subsequent calls just register it
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

// buildLibrary builds the AI library
func buildLibrary() *object.Library {
	builder := object.NewLibraryBuilder(AILibraryName, AILibraryDesc)

	// Add ToolRegistry class
	builder.Constant("ToolRegistry", tools.GetRegistryClass())

	// Provider constants
	builder.Constant("OPENAI", string(ai.ProviderOpenAI))
	builder.Constant("CLAUDE", string(ai.ProviderClaude))
	builder.Constant("GEMINI", string(ai.ProviderGemini))
	builder.Constant("OLLAMA", string(ai.ProviderOllama))
	builder.Constant("ZAI", string(ai.ProviderZAi))
	builder.Constant("MISTRAL", string(ai.ProviderMistral))

	builder.
		// Client(base_url, **kwargs) - Create a new AI client
		FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, baseURL string) (object.Object, error) {
			// Get optional provider from kwargs, default to "openai"
			provider := kwargs.MustGetString("provider", "openai")
			// Get optional api_key from kwargs
			apiKey := kwargs.MustGetString("api_key", "")
			// Get optional max_tokens and temperature
			maxTokens := int(kwargs.MustGetInt("max_tokens", 0))
			temperature := float32(kwargs.MustGetFloat("temperature", 0))

			// Parse remote_servers if provided
			var remoteServerConfigs []openai.RemoteServerConfig
			if kwargs.Has("remote_servers") {
				remoteServersObjs := kwargs.MustGetList("remote_servers", nil)
				remoteServerConfigs = make([]openai.RemoteServerConfig, 0, len(remoteServersObjs))
				for i, serverObj := range remoteServersObjs {
					// Convert Object to map[string]Object
					serverMap, err := serverObj.AsDict()
					if err != nil {
						return nil, fmt.Errorf("remote_servers[%d] must be a dict: %v", i, err)
					}
					baseURLVal, ok := serverMap["base_url"]
					if !ok || baseURLVal == nil {
						return nil, fmt.Errorf("remote_servers[%d] must have a 'base_url'", i)
					}
					baseURLStr, err := baseURLVal.AsString()
					if err != nil {
						return nil, fmt.Errorf("remote_servers[%d].base_url must be a string: %v", i, err)
					}

					var namespace string
					if nsVal, ok := serverMap["namespace"]; ok && nsVal != nil {
						namespace, _ = nsVal.AsString()
					}

					config := openai.RemoteServerConfig{
						BaseURL:   baseURLStr,
						Namespace: namespace,
					}

					if tokenVal, ok := serverMap["bearer_token"]; ok && tokenVal != nil {
						bearerToken, _ := tokenVal.AsString()
						if bearerToken != "" {
							config.Auth = mcp.NewBearerTokenAuth(bearerToken)
						}
					}

					remoteServerConfigs = append(remoteServerConfigs, config)
				}
			}

			// Map provider string to provider
			var providerType ai.Provider
			switch provider {
			case "openai":
				providerType = ai.ProviderOpenAI
			case "claude":
				providerType = ai.ProviderClaude
			case "gemini":
				providerType = ai.ProviderGemini
			case "ollama":
				providerType = ai.ProviderOllama
			case "zai":
				providerType = ai.ProviderZAi
			case "mistral":
				providerType = ai.ProviderMistral
			default:
				return nil, fmt.Errorf("unsupported provider: %s", provider)
			}

			client, err := ai.NewClient(ai.Config{
				Provider: providerType,
				Config: openai.Config{
					APIKey:              apiKey,
					BaseURL:             baseURL,
					RemoteServerConfigs: remoteServerConfigs,
					MaxTokens:           maxTokens,
					Temperature:         temperature,
				},
			})
			if err != nil {
				return nil, err
			}

			return createClientInstance(client), nil
		}, `Client(base_url, **kwargs) - Create a new AI client

Creates a new AI client instance for making API calls to supported services.

Parameters:
  base_url (str): Base URL of the API (defaults to https://api.openai.com/v1 if empty)
  provider (str, optional): Provider type (defaults to ai.OPENAI). Use constants: ai.OPENAI, ai.CLAUDE, ai.GEMINI, ai.OLLAMA, ai.ZAI, ai.MISTRAL
  api_key (str, optional): API key for authentication
  max_tokens (int, optional): Default max_tokens for all requests (Claude defaults to 4096 if not set)
  temperature (float, optional): Default temperature for all requests (0.0-2.0)
  remote_servers (list, optional): List of remote MCP server configs, each a dict with:
    - base_url (str, required): URL of the MCP server
    - namespace (str, optional): Namespace prefix for tools
    - bearer_token (str, optional): Bearer token for authentication

Returns:
  AIClient: A client instance with methods for API calls

Example:
  # OpenAI API (default service)
  client = ai.Client("", api_key="sk-...", max_tokens=2048, temperature=0.7)
  response = client.completion("gpt-4", {"role": "user", "content": "Hello!"})

  # LM Studio / Local LLM
  client = ai.Client("http://127.0.0.1:1234/v1")

  # Claude (max_tokens defaults to 4096 if not specified)
  client = ai.Client("https://api.anthropic.com", provider=ai.CLAUDE, api_key="sk-ant-...")

  # With MCP servers
  client = ai.Client("http://127.0.0.1:1234/v1", remote_servers=[
      {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
      {"base_url": "https://api.example.com/mcp", "namespace": "search", "bearer_token": "secret"},
  ])`).

		// extract_thinking(text) - Extract thinking blocks from AI response
		FunctionWithHelp("extract_thinking", func(ctx context.Context, text string) (map[string]any, error) {
			return extractThinking(text), nil
		}, `extract_thinking(text) - Extract thinking blocks from AI response

Extracts thinking/reasoning blocks from AI model responses and returns
both the extracted thinking and the cleaned content.

Supports multiple formats:
  - XML-style: <think>...</think>, <thinking>...</thinking>
  - Markdown code blocks: `+"```thinking\\n...\\n```"+`
  - OpenAI <Thought>...</Thought> style

Parameters:
  text (str): The AI response text to process

Returns:
  dict: Contains 'thinking' (list of extracted blocks) and 'content' (cleaned text)

Example:
  client = ai.Client("", api_key="sk-...")
  response = client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
  result = ai.extract_thinking(response.choices[0].message.content)

  for thought in result["thinking"]:
      print("Thinking:", thought)

  print("Response:", result["content"])`)

	return builder.Build()
}

// convertMapsToOpenAI converts Go map messages to ai.Message format
func convertMapsToOpenAI(messages []map[string]any) []ai.Message {
	aiMessages := make([]ai.Message, 0, len(messages))
	for _, msg := range messages {
		omsg := ai.Message{}
		if role, ok := msg["role"].(string); ok {
			omsg.Role = role
		}
		if content, ok := msg["content"]; ok {
			omsg.Content = content
		}
		if tcid, ok := msg["tool_call_id"].(string); ok {
			omsg.ToolCallID = tcid
		}
		aiMessages = append(aiMessages, omsg)
	}
	return aiMessages
}

// extractThinking extracts thinking/reasoning blocks from AI responses
// and returns both the extracted blocks and the cleaned content.
// Supports multiple formats used by various AI models.
func extractThinking(text string) map[string]any {
	var thinkingBlocks []any
	content := text

	// Define patterns for different thinking block formats
	// Each pattern captures the content inside the thinking block
	patterns := []struct {
		regex *regexp.Regexp
		name  string
	}{
		// XML-style: <think>...</think>
		{regexp.MustCompile(`(?is)<think>(.*?)</think>`), "think"},
		// XML-style: <thinking>...</thinking>
		{regexp.MustCompile(`(?is)<thinking>(.*?)</thinking>`), "thinking"},
		// OpenAI Thought format: <Thought>...</Thought>
		{regexp.MustCompile(`(?is)<Thought>(.*?)</Thought>`), "Thought"},
		// Markdown code block: ```thinking\n...\n```
		{regexp.MustCompile("(?is)```thinking\\s*\\n(.*?)\\n?```"), "md-thinking"},
		// Markdown code block: ```thought\n...\n```
		{regexp.MustCompile("(?is)```thought\\s*\\n(.*?)\\n?```"), "md-thought"},
		// Claude-style: <antThinking>...</antThinking>
		{regexp.MustCompile(`(?is)<antThinking>(.*?)</antThinking>`), "antThinking"},
	}

	// Extract all thinking blocks
	for _, p := range patterns {
		matches := p.regex.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				thought := strings.TrimSpace(match[1])
				if thought != "" {
					thinkingBlocks = append(thinkingBlocks, thought)
				}
			}
		}
		// Remove the matched blocks from content
		content = p.regex.ReplaceAllString(content, "")
	}

	// Clean up the content - remove extra whitespace
	content = strings.TrimSpace(content)
	// Collapse multiple newlines into at most two
	multiNewline := regexp.MustCompile(`\n{3,}`)
	content = multiNewline.ReplaceAllString(content, "\n\n")

	return map[string]any{
		"thinking": thinkingBlocks,
		"content":  content,
	}
}
