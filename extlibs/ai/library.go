package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/paularlott/mcp"
	"github.com/paularlott/mcp/openai"
	"github.com/paularlott/scriptling/extlibs/ai/tools"
	"github.com/paularlott/scriptling/object"
)

const (
	AILibraryName = "scriptling.ai"
	AILibraryDesc = "AI and LLM functions for interacting with OpenAI-compatible APIs"
)

var (
	client      *openai.Client
	clientMutex sync.RWMutex
	library     *object.Library
	libraryOnce sync.Once
)

// WrapClient wraps an OpenAI client as a scriptling Object that can be
// passed into a script via SetObjectVar. This allows multiple clients
// to be used simultaneously.
func WrapClient(c *openai.Client) object.Object {
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

	builder.

		// completion(model, messages) - Create a chat completion
		FunctionWithHelp("completion", func(ctx context.Context, model string, messages []map[string]any) (any, error) {
			c := getClient()
			if c == nil {
				return nil, fmt.Errorf("ai.completion: no client configured - use ai.SetClient() or create a client from script")
			}

			// Convert messages to openai.Message
			openaiMessages := convertMapsToOpenAI(messages)

			chatResp, chatErr := c.ChatCompletion(ctx, openai.ChatCompletionRequest{
				Model:    model,
				Messages: openaiMessages,
			})
			if chatErr != nil {
				return nil, fmt.Errorf("chat completion failed: %s", chatErr.Error())
			}

			return chatResp, nil
		}, `completion(model, messages) - Create a chat completion

Creates a chat completion using the specified model and messages.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (list): List of message dicts with "role" and "content" keys

Returns:
  dict: Response containing id, choices, usage, etc.

Example:
  response = ai.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
  print(response.choices[0].message.content)`).

		// models() - List available models
		FunctionWithHelp("models", func(ctx context.Context) (any, error) {
			c := getClient()
			if c == nil {
				return nil, fmt.Errorf("ai.models: no client configured - use ai.SetClient() or create a client from script")
			}

			resp, err := c.GetModels(ctx)
			if err != nil {
				return nil, err
			}

			return resp.Data, nil
		}, `models() - List available models

Lists all models available for the current API configuration.

Returns:
  list: List of model dicts with id, created, owned_by, etc.

Example:
  models = ai.models()
  for model in models:
    print(model.id)`).

		// response_create(model, input) - Create a Responses API response
		FunctionWithHelp("response_create", func(ctx context.Context, model string, input []any) (any, error) {
			c := getClient()
			if c == nil {
				return nil, fmt.Errorf("ai.response_create: no client configured - use ai.SetClient() or create a client from script")
			}

			// Build request
			req := openai.CreateResponseRequest{
				Model: model,
				Input: input,
			}

			resp, createErr := c.CreateResponse(ctx, req)
			if createErr != nil {
				return nil, fmt.Errorf("failed to create response: %s", createErr.Error())
			}

			return resp, nil
		}, `response_create(model, input) - Create a Responses API response

Creates a response using the OpenAI Responses API (new structured API).

Parameters:
  model (str): Model identifier (e.g., "gpt-4o", "gpt-4")
  input (list): Input items (messages)

Returns:
  dict: Response object with id, status, output, usage, etc.

Example:
  response = ai.response_create("gpt-4", [
    {"type": "message", "role": "user", "content": "Hello!"}
  ])
  print(response.output)`).

		// response_get(id) - Get a Responses API response by ID
		FunctionWithHelp("response_get", func(ctx context.Context, id string) (any, error) {
			c := getClient()
			if c == nil {
				return nil, fmt.Errorf("ai.response_get: no client configured - use ai.SetClient() or create a client from script")
			}
			return c.GetResponse(ctx, id)
		}, `response_get(id) - Get a response by ID

Retrieves a previously created response by its ID.

Parameters:
  id (str): Response ID

Returns:
  dict: Response object with id, status, output, usage, etc.

Example:
  response = ai.response_get("resp_123")
  print(response.status)`).

		// response_cancel(id) - Cancel a Responses API response
		FunctionWithHelp("response_cancel", func(ctx context.Context, id string) (any, error) {
			c := getClient()
			if c == nil {
				return nil, fmt.Errorf("ai.response_cancel: no client configured - use ai.SetClient() or create a client from script")
			}
			return c.CancelResponse(ctx, id)
		}, `response_cancel(id) - Cancel a response

Cancels a currently in-progress response.

Parameters:
  id (str): Response ID to cancel

Returns:
  dict: Cancelled response object

Example:
  response = ai.response_cancel("resp_123")`).

		// new_client(base_url, **kwargs) - Create a new AI client
		FunctionWithHelp("new_client", func(ctx context.Context, kwargs object.Kwargs, baseURL string) (object.Object, error) {
			// Get optional service from kwargs, default to "openai"
			service := kwargs.MustGetString("service", "openai")
			// Get optional api_key from kwargs
			apiKey := kwargs.MustGetString("api_key", "")

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

			switch service {
			case "openai":
				config := openai.Config{
					APIKey:             apiKey,
					BaseURL:            baseURL,
					RemoteServerConfigs: remoteServerConfigs,
				}

				client, err := openai.New(config)
				if err != nil {
					return nil, err
				}

				return createClientInstance(client), nil
			default:
				return nil, fmt.Errorf("unsupported service: %s", service)
			}
		}, `new_client(base_url, **kwargs) - Create a new AI client

Creates a new AI client instance for making API calls to supported services.

Parameters:
  base_url (str): Base URL of the API (defaults to https://api.openai.com/v1 if empty)
  service (str, optional): Service type ("openai" by default)
  api_key (str, optional): API key for authentication
  remote_servers (list, optional): List of remote MCP server configs, each a dict with:
    - base_url (str, required): URL of the MCP server
    - namespace (str, optional): Namespace prefix for tools
    - bearer_token (str, optional): Bearer token for authentication

Returns:
  AIClient: A client instance with methods for API calls

Example:
  # OpenAI API (default service)
  client = ai.new_client("", api_key="sk-...")
  response = client.completion("gpt-4", {"role": "user", "content": "Hello!"})

  # LM Studio / Local LLM
  client = ai.new_client("http://127.0.0.1:1234/v1")

  # With MCP servers
  client = ai.new_client("http://127.0.0.1:1234/v1", remote_servers=[
      {"base_url": "http://127.0.0.1:8080/mcp", "namespace": "scriptling"},
      {"base_url": "https://api.example.com/mcp", "namespace": "search", "bearer_token": "secret"},
  ])

  # Future: Other services
  client = ai.new_client("https://api.anthropic.com", service="anthropic", api_key="...")`).

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
  response = ai.completion(...)
  result = ai.extract_thinking(response.choices[0].message.content)

  for thought in result["thinking"]:
      print("Thinking:", thought)

  print("Response:", result["content"])`)

	return builder.Build()
}

// getClient returns the current client (thread-safe)
func getClient() *openai.Client {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	return client
}

// convertMapsToOpenAI converts Go map messages to openai.Message format
func convertMapsToOpenAI(messages []map[string]any) []openai.Message {
	openaiMessages := make([]openai.Message, 0, len(messages))
	for _, msg := range messages {
		omsg := openai.Message{}
		if role, ok := msg["role"].(string); ok {
			omsg.Role = role
		}
		if content, ok := msg["content"]; ok {
			omsg.Content = content
		}
		if tcid, ok := msg["tool_call_id"].(string); ok {
			omsg.ToolCallID = tcid
		}
		openaiMessages = append(openaiMessages, omsg)
	}
	return openaiMessages
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
