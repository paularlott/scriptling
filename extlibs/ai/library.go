package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/paularlott/mcp"
	"github.com/paularlott/mcp/ai"
	"github.com/paularlott/mcp/ai/openai"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/evaliface"
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
			// Get optional max_tokens, temperature and top_p
			maxTokens := int(kwargs.MustGetInt("max_tokens", 0))
			var temperature *float64
			if kwargs.Has("temperature") {
				v := kwargs.MustGetFloat("temperature", 0)
				temperature = &v
			}
			var topP *float64
			if kwargs.Has("top_p") {
				v := kwargs.MustGetFloat("top_p", 0)
				topP = &v
			}

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
					TopP:                topP,
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
  top_p (float, optional): Default top_p (nucleus sampling) for all requests (0.0-1.0)
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

  print("Response:", result["content"])`).

		// text(response) - Get text content from response (without thinking blocks)
		FunctionWithHelp("text", func(ctx context.Context, responseObj object.Object) (object.Object, error) {
			// Convert response to Go type to access it
			responseGo := conversion.ToGo(responseObj)
			responseMap, ok := responseGo.(map[string]any)
			if !ok {
				return &object.String{Value: ""}, nil
			}

			// Extract content from response.choices[0].message.content
			content := ""
			if choices, ok := responseMap["choices"].([]any); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]any); ok {
					if message, ok := choice["message"].(map[string]any); ok {
						if msgContent, ok := message["content"].(string); ok {
							// Extract thinking and get clean content
							result := extractThinking(msgContent)
							if contentStr, ok := result["content"].(string); ok {
								content = contentStr
							}
						}
					}
				}
			}

			return &object.String{Value: content}, nil
		}, `text(response) - Get text content from response (without thinking blocks)

Extracts the text content from a completion response, automatically removing any thinking blocks.

Parameters:
  response (dict): Chat completion response from client.completion()

Returns:
  str: The response text with thinking blocks removed

Example:
  response = client.completion("gpt-4", "What is 2+2?")
  text = ai.text(response)
  print(text)  # "4"`).

		// thinking(response) - Get thinking blocks from response
		FunctionWithHelp("thinking", func(ctx context.Context, responseObj object.Object) (object.Object, error) {
			// Convert response to Go type to access it
			responseGo := conversion.ToGo(responseObj)
			responseMap, ok := responseGo.(map[string]any)
			if !ok {
				return &object.List{Elements: []object.Object{}}, nil
			}

			// Extract content from response.choices[0].message.content
			var content string
			if choices, ok := responseMap["choices"].([]any); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]any); ok {
					if message, ok := choice["message"].(map[string]any); ok {
						if msgContent, ok := message["content"].(string); ok {
							content = msgContent
						}
					}
				}
			}

			// Extract thinking blocks
			result := extractThinking(content)
			thinkingBlocks := result["thinking"]

			// Convert to list of strings
			list := make([]object.Object, 0)
			if thinkingList, ok := thinkingBlocks.([]any); ok {
				for _, block := range thinkingList {
					if blockStr, ok := block.(string); ok {
						list = append(list, &object.String{Value: blockStr})
					}
				}
			}

			return &object.List{Elements: list}, nil
		}, `thinking(response) - Get thinking blocks from response

Extracts thinking/reasoning blocks from a completion response.

Parameters:
  response (dict): Chat completion response from client.completion()

Returns:
  list: List of thinking block strings (empty if no thinking blocks)

Example:
  response = client.completion("gpt-4", "Explain step by step")
  thoughts = ai.thinking(response)
  for thought in thoughts:
      print("Reasoning:", thought)`).
		FunctionWithHelp("tool_calls", func(ctx context.Context, input object.Object) (object.Object, error) {
			return conversion.FromGo(extractToolCallsFromGo(conversion.ToGo(input))), nil
		}, `tool_calls(response_or_message) - Extract normalized tool calls

Extracts tool calls from a completion response, a message dict, or a tool call list
and returns them in a normalized format.

Parameters:
  response_or_message (dict or list): A completion response, message dict, or list of tool calls

Returns:
  list: Normalized tool call dicts with id, type, and function fields

Example:
  response = client.completion("gpt-4", [{"role": "user", "content": "Call a tool"}], tools=schemas)
  tool_calls = ai.tool_calls(response)
  for tool_call in tool_calls:
      print(tool_call["function"]["name"])`).
		FunctionWithHelp("execute_tool_calls", func(ctx context.Context, registryObj object.Object, toolCalls object.Object) (object.Object, error) {
			registry, ok := registryObj.(*object.Instance)
			if !ok {
				return &object.Error{Message: "execute_tool_calls: registry must be a ToolRegistry"}, nil
			}
			results, errObj := executeToolCalls(ctx, registry, extractToolCallsFromGo(conversion.ToGo(toolCalls)))
			if errObj != nil {
				return errObj, nil
			}
			return conversion.FromGo(results), nil
		}, `execute_tool_calls(registry, tool_calls) - Execute tool calls with a ToolRegistry

Executes normalized tool calls using handlers from a ToolRegistry and returns tool
result messages suitable for appending to a conversation.

Parameters:
  registry (ToolRegistry): Tool registry containing handlers
  tool_calls (list): Tool call dicts, typically from ai.tool_calls(...)

Returns:
  list: Tool result message dicts with role, tool_call_id, and content

Example:
  response = client.completion("gpt-4", messages, tools=schemas)
  tool_calls = ai.tool_calls(response)
  tool_results = ai.execute_tool_calls(tools, tool_calls)
  for result in tool_results:
      print(result["content"])`).
		FunctionWithHelp("collect_stream", func(ctx context.Context, kwargs object.Kwargs, streamObj object.Object) (object.Object, error) {
			stream, ok := streamObj.(*object.Instance)
			if !ok {
				return &object.Error{Message: "collect_stream: stream must be a ChatStream"}, nil
			}
			chunkTimeoutMs := kwargs.MustGetInt("chunk_timeout_ms", 0)
			firstChunkTimeoutMs := kwargs.MustGetInt("first_chunk_timeout_ms", 0)
			callback := kwargs.Get("on_event")
			result, errObj := collectStream(ctx, stream, chunkTimeoutMs, firstChunkTimeoutMs, callback)
			if errObj != nil {
				return errObj, nil
			}
			return conversion.FromGo(result), nil
		}, `collect_stream(stream, **kwargs) - Collect a chat stream into a single result

Consumes a ChatStream, aggregates reasoning, content, tool calls, and finish status,
and optionally emits events to a callback while chunks are processed.

Parameters:
  stream (ChatStream): Stream returned by client.completion_stream()
  chunk_timeout_ms (int, optional): Per-chunk timeout in milliseconds. Default: 0
  first_chunk_timeout_ms (int, optional): Timeout for the first chunk only (models may need time to load). Falls back to chunk_timeout_ms. Default: 0
  on_event (callable, optional): Callback invoked with event dicts during collection

Returns:
  dict: Aggregated result with content, reasoning, tool_calls, finish_reason, timed_out,
        assistant_message, and error (only present when timed_out is true)

Example:
  stream = client.completion_stream("gpt-4", messages, tools=schemas)
  result = ai.collect_stream(stream, first_chunk_timeout_ms=30000, chunk_timeout_ms=4000)
  print(result["content"])
  print(len(result["tool_calls"]))`).
		FunctionWithHelp("tool_round", func(ctx context.Context, kwargs object.Kwargs, clientObj object.Object, model string, messages object.Object, registryObj object.Object) (object.Object, error) {
			client, ok := clientObj.(*object.Instance)
			if !ok {
				return &object.Error{Message: "tool_round: client must be an OpenAIClient"}, nil
			}
			registry, ok := registryObj.(*object.Instance)
			if !ok {
				return &object.Error{Message: "tool_round: registry must be a ToolRegistry"}, nil
			}
			result, errObj := runToolRound(ctx, kwargs, client, model, messages, registry)
			if errObj != nil {
				return errObj, nil
			}
			return conversion.FromGo(result), nil
		}, `tool_round(client, model, messages, registry, **kwargs) - Run one tool-enabled completion round

Runs either a non-streaming or streaming completion with tools, extracts normalized tool
calls, executes them with a ToolRegistry, and returns the aggregated round state.

Parameters:
  client (OpenAIClient): AI client instance
  model (str): Model identifier
  messages (str or list): User message string or message list
  registry (ToolRegistry): Tool registry containing schemas and handlers
  stream (bool, optional): Use completion_stream() instead of completion(). Default: False
  chunk_timeout_ms (int, optional): Per-chunk timeout for streaming mode. Default: 0
  on_event (callable, optional): Streaming callback that receives event dicts
  system_prompt (str, optional): System prompt when messages is a string
  temperature (float, optional): Sampling temperature
  top_p (float, optional): Nucleus sampling threshold
  max_tokens (int, optional): Maximum tokens to generate
  timeout_ms (int, optional): Overall request timeout in milliseconds

Returns:
  dict: Round result with assistant_message, content, reasoning, tool_calls, tool_results,
        finish_reason, and timed_out. Non-streaming mode also includes response.

Example:
  result = ai.tool_round(client, "gpt-4", messages, tools, timeout_ms=30000)
  if len(result["tool_calls"]) > 0:
      messages.append(result["assistant_message"])
      for tool_result in result["tool_results"]:
          messages.append(tool_result)`).

		// estimate_tokens(request, response) - Estimate token counts for request and response
		FunctionWithHelp("estimate_tokens", func(ctx context.Context, requestObj object.Object, responseObj object.Object) (object.Object, error) {
			tc := openai.NewTokenCounter()

			// Estimate prompt tokens from request messages
			requestGo := conversion.ToGo(requestObj)
			if requestMap, ok := requestGo.(map[string]any); ok {
				if messagesRaw, ok := requestMap["messages"]; ok {
					requestGo = messagesRaw
				}
			}

			switch req := requestGo.(type) {
			case []any:
				maps := make([]map[string]any, 0, len(req))
				for _, item := range req {
					if m, ok := item.(map[string]any); ok {
						maps = append(maps, m)
					}
				}
				tc.AddPromptTokensFromMaps(maps)
			case string:
				tc.AddPromptTokensFromMessages([]ai.Message{{Role: "user", Content: req}})
			}

			// Estimate completion tokens from response
			responseGo := conversion.ToGo(responseObj)
			if responseMap, ok := responseGo.(map[string]any); ok {
				tc.AddCompletionTokensFromResponseMap(responseMap)
			}

			usage := tc.GetUsage()
			return conversion.FromGo(map[string]any{
				"prompt_tokens":     usage.PromptTokens,
				"completion_tokens": usage.CompletionTokens,
				"total_tokens":      usage.TotalTokens,
			}), nil
		}, `estimate_tokens(request, response) - Estimate token counts for messages and response

	Estimates the number of tokens in the request messages and response using
	a character-based heuristic (~4 characters per token). This provides a fast,
	reproducible approximation useful for cost estimation and context window management.

	Parameters:
	  request (str, list, or dict): The messages sent to the AI. Can be:
	    - A string (user message)
	    - A list of message dicts with "role" and "content" keys
	    - A completion request dict with a "messages" key
	  response (dict): The completion response from client.completion() or client.response_create()

	Returns:
	  dict: Token usage estimates with keys:
	    - prompt_tokens (int): Estimated tokens in the request messages
	    - completion_tokens (int): Estimated tokens in the response
	    - total_tokens (int): Sum of prompt and completion tokens

	Example:
	  client = ai.Client("", api_key="sk-...")
	  messages = [{"role": "user", "content": "Hello!"}]
	  response = client.completion("gpt-4", messages)
	  usage = ai.estimate_tokens(messages, response)
	  print(f"Prompt: {usage.prompt_tokens}, Completion: {usage.completion_tokens}")

	  # Also works with string shorthand
	  response = client.completion("gpt-4", "What is 2+2?")
	  usage = ai.estimate_tokens("What is 2+2?", response)
	  print(f"Total: {usage.total_tokens} tokens")`)

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

type streamingThinkingState struct {
	inReasoning bool
	carry       string
}

func processStreamingThinkingDelta(delta string, state *streamingThinkingState, finalize bool) (string, string) {
	if state == nil || delta == "" && !finalize {
		return "", ""
	}

	startTags := []string{"<thinking>", "<think>", "<thought>"}
	endTags := []string{"</thinking>", "</think>", "</thought>"}

	input := state.carry + delta
	state.carry = ""

	contentOut := strings.Builder{}
	reasoningOut := strings.Builder{}

	for len(input) > 0 {
		lowerInput := strings.ToLower(input)

		if state.inReasoning {
			tagIndex, tagLen := findFirstStreamingTag(lowerInput, endTags)
			if tagIndex >= 0 {
				reasoningOut.WriteString(input[:tagIndex])
				input = input[tagIndex+tagLen:]
				state.inReasoning = false
				input = strings.TrimLeft(input, "\r\n\t ")
				continue
			}

			if !finalize {
				carryLen := streamingTagCarryLen(lowerInput, endTags)
				safeLen := len(input) - carryLen
				if safeLen > 0 {
					reasoningOut.WriteString(input[:safeLen])
					input = input[safeLen:]
				}
				state.carry = input
				input = ""
				continue
			}

			reasoningOut.WriteString(input)
			input = ""
			continue
		}

		tagIndex, tagLen := findFirstStreamingTag(lowerInput, startTags)
		if tagIndex >= 0 {
			contentOut.WriteString(input[:tagIndex])
			input = input[tagIndex+tagLen:]
			state.inReasoning = true
			input = strings.TrimLeft(input, "\r\n\t ")
			continue
		}

		if !finalize {
			carryLen := streamingTagCarryLen(lowerInput, startTags)
			safeLen := len(input) - carryLen
			if safeLen > 0 {
				contentOut.WriteString(input[:safeLen])
				input = input[safeLen:]
			}
			state.carry = input
			input = ""
			continue
		}

		contentOut.WriteString(input)
		input = ""
	}

	return contentOut.String(), reasoningOut.String()
}

func findFirstStreamingTag(input string, tags []string) (int, int) {
	firstIndex := -1
	firstLen := 0
	for _, tag := range tags {
		if idx := strings.Index(input, tag); idx >= 0 && (firstIndex < 0 || idx < firstIndex) {
			firstIndex = idx
			firstLen = len(tag)
		}
	}
	return firstIndex, firstLen
}

func streamingTagCarryLen(input string, tags []string) int {
	maxCarry := 0
	for _, tag := range tags {
		maxPrefix := len(tag) - 1
		if maxPrefix > len(input) {
			maxPrefix = len(input)
		}
		for prefixLen := maxPrefix; prefixLen > 0; prefixLen-- {
			if strings.HasSuffix(input, tag[:prefixLen]) {
				if prefixLen > maxCarry {
					maxCarry = prefixLen
				}
				break
			}
		}
	}
	return maxCarry
}

func extractToolCallsFromGo(input any) []map[string]any {
	switch v := input.(type) {
	case nil:
		return []map[string]any{}
	case map[string]any:
		if choicesRaw, ok := v["choices"].([]any); ok && len(choicesRaw) > 0 {
			if choice, ok := choicesRaw[0].(map[string]any); ok {
				if message, ok := choice["message"].(map[string]any); ok {
					return extractToolCallsFromGo(message)
				}
				if delta, ok := choice["delta"].(map[string]any); ok {
					return extractToolCallsFromGo(delta)
				}
			}
		}
		if toolCallsRaw, ok := v["tool_calls"]; ok {
			return normalizeToolCalls(toolCallsRaw)
		}
	case []any:
		return normalizeToolCalls(v)
	case []map[string]any:
		items := make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
		return normalizeToolCalls(items)
	}

	return []map[string]any{}
}

func normalizeToolCalls(raw any) []map[string]any {
	var items []any

	switch v := raw.(type) {
	case nil:
		return []map[string]any{}
	case []any:
		items = v
	case []map[string]any:
		items = make([]any, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
	default:
		return []map[string]any{}
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		tcMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		function := map[string]any{}
		if fnRaw, ok := tcMap["function"].(map[string]any); ok {
			function = copyMap(fnRaw)
		}

		if function["name"] == nil {
			if name, ok := tcMap["name"].(string); ok && name != "" {
				function["name"] = name
			}
		}

		arguments := function["arguments"]
		if arguments == nil {
			if args, ok := tcMap["arguments"]; ok {
				arguments = args
			}
		}
		function["arguments"] = normalizeToolArguments(arguments)

		normalized := map[string]any{
			"id":       stringValue(tcMap["id"]),
			"type":     stringOrDefault(tcMap["type"], "function"),
			"function": function,
		}
		if index, ok := integerValue(tcMap["index"]); ok {
			normalized["index"] = index
		}

		result = append(result, normalized)
	}

	return result
}

func executeToolCalls(ctx context.Context, registry *object.Instance, toolCalls []map[string]any) ([]map[string]any, *object.Error) {
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return nil, &object.Error{Message: "execute_tool_calls: evaluator not available"}
	}

	env := toolEnvFromContext(ctx)
	results := make([]map[string]any, 0, len(toolCalls))

	for _, toolCall := range toolCalls {
		function, _ := toolCall["function"].(map[string]any)
		name := stringValue(function["name"])
		if name == "" {
			return nil, &object.Error{Message: "execute_tool_calls: tool call missing function.name"}
		}

		// Strip {namespace:name} wrapper from tool name
		if strings.HasPrefix(name, "{") {
			if idx := strings.Index(name, ":"); idx >= 0 {
				remainder := name[idx+1:]
				if strings.HasSuffix(remainder, "}") {
					name = remainder[:len(remainder)-1]
				}
			}
		}
		// Strip function_name_ prefix from tool name
		name = strings.TrimPrefix(name, "function_name_")

		handler, errObj := tools.GetHandlerObject(registry, name)
		if errObj != nil {
			return nil, errObj
		}

		argsMap, _ := normalizeToolArguments(function["arguments"]).(map[string]any)
		// Strip {...} wrappers from argument keys (some models emit {key} instead of key)
		for key, value := range argsMap {
			if strings.HasPrefix(key, "{") && strings.HasSuffix(key, "}") && len(key) > 2 {
				delete(argsMap, key)
				argsMap[key[1:len(key)-1]] = value
			}
		}
		callArg := conversion.FromGo(argsMap)
		resultObj := eval.CallObjectFunction(ctx, handler, []object.Object{callArg}, nil, env)

		if errObj, ok := resultObj.(*object.Error); ok {
			return nil, errObj
		}
		if exObj, ok := resultObj.(*object.Exception); ok {
			return nil, &object.Error{Message: exObj.Message}
		}

		results = append(results, map[string]any{
			"role":         "tool",
			"tool_call_id": stringValue(toolCall["id"]),
			"content":      stringifyToolResult(resultObj),
		})
	}

	return results, nil
}

func collectStream(ctx context.Context, stream *object.Instance, chunkTimeoutMs int64, firstChunkTimeoutMs int64, callback object.Object) (map[string]any, *object.Error) {
	reasoning := strings.Builder{}
	content := strings.Builder{}
	thinkingState := &streamingThinkingState{}
	toolCallsByIndex := map[int]map[string]any{}
	toolCallOrder := []int{}
	finishReason := ""
	timedOut := false
	firstChunk := true

	for {
		var chunkObj object.Object
		if chunkTimeoutMs > 0 || firstChunkTimeoutMs > 0 {
			timeout := chunkTimeoutMs
			if firstChunk && firstChunkTimeoutMs > 0 {
				timeout = firstChunkTimeoutMs
			}
			if timeout > 0 {
				chunkObj = nextTimeoutStreamMethod(stream, ctx, timeout)
			} else {
				chunkObj = nextStreamMethod(stream, ctx)
			}
		} else {
			chunkObj = nextStreamMethod(stream, ctx)
		}
		firstChunk = false

		if errObj, ok := chunkObj.(*object.Error); ok {
			return nil, errObj
		}
		if _, ok := chunkObj.(*object.Null); ok {
			break
		}

		chunkGo := conversion.ToGo(chunkObj)
		chunkMap, ok := chunkGo.(map[string]any)
		if !ok {
			continue
		}
		if timedOutValue, ok := chunkMap["timed_out"].(bool); ok && timedOutValue {
			timedOut = true
			break
		}

		emitCollectorEvent(ctx, callback, map[string]any{
			"type":  "chunk",
			"chunk": chunkMap,
		})

		choicesRaw, ok := chunkMap["choices"].([]any)
		if !ok {
			continue
		}
		for _, choiceRaw := range choicesRaw {
			choice, ok := choiceRaw.(map[string]any)
			if !ok {
				continue
			}

			if finishReason == "" {
				finishReason = stringValue(choice["finish_reason"])
			}

			delta, _ := choice["delta"].(map[string]any)
			if delta == nil {
				continue
			}

			reasoningDelta := stringValue(delta["reasoning_content"])
			if reasoningDelta == "" {
				reasoningDelta = stringValue(delta["reasoning"])
			}
			if reasoningDelta != "" {
				reasoning.WriteString(reasoningDelta)
				emitCollectorEvent(ctx, callback, map[string]any{
					"type":    "reasoning",
					"content": reasoningDelta,
				})
			}

			contentDelta := stringValue(delta["content"])
			if contentDelta != "" {
				contentDeltaOut, reasoningDeltaOut := processStreamingThinkingDelta(contentDelta, thinkingState, false)
				if reasoningDeltaOut != "" {
					reasoning.WriteString(reasoningDeltaOut)
					emitCollectorEvent(ctx, callback, map[string]any{
						"type":    "reasoning",
						"content": reasoningDeltaOut,
					})
				}
				if contentDeltaOut != "" {
					content.WriteString(contentDeltaOut)
					emitCollectorEvent(ctx, callback, map[string]any{
						"type":    "content",
						"content": contentDeltaOut,
					})
				}
			}

			for _, toolCall := range normalizeToolCalls(delta["tool_calls"]) {
				index, ok := integerValue(toolCall["index"])
				if !ok {
					index = len(toolCallOrder)
				}
				existing, found := toolCallsByIndex[index]
				if !found {
					existing = map[string]any{
						"id":       "",
						"type":     "function",
						"function": map[string]any{},
						"index":    index,
					}
					toolCallsByIndex[index] = existing
					toolCallOrder = append(toolCallOrder, index)
				}
				mergeToolCall(existing, toolCall)
				emitCollectorEvent(ctx, callback, map[string]any{
					"type":      "tool_call",
					"tool_call": copyMap(existing),
				})
			}
		}
	}

	if errValue := errStreamMethod(stream, ctx); errValue != nil {
		if _, ok := errValue.(*object.Null); !ok {
			if errObj, ok := errValue.(*object.Error); ok {
				return nil, errObj
			}
			if errString, ok := errValue.(*object.String); ok {
				return nil, &object.Error{Message: errString.Value}
			}
		}
	}

	finalContent, finalReasoning := processStreamingThinkingDelta("", thinkingState, true)
	if finalReasoning != "" {
		reasoning.WriteString(finalReasoning)
	}
	if finalContent != "" {
		content.WriteString(finalContent)
	}

	toolCalls := make([]map[string]any, 0, len(toolCallOrder))
	for _, index := range toolCallOrder {
		toolCall := toolCallsByIndex[index]
		if toolCall == nil {
			continue
		}
		toolCall["function"] = normalizeToolFunction(toolCall["function"])
		delete(toolCall, "index")
		toolCalls = append(toolCalls, toolCall)
	}

	// On timeout, discard partial tool calls to avoid executing incomplete data
	if timedOut {
		toolCalls = []map[string]any{}
	}

	reasoningText := strings.TrimSpace(reasoning.String())
	result := map[string]any{
		"content":           content.String(),
		"reasoning":         reasoningText,
		"tool_calls":        toolCalls,
		"finish_reason":     finishReason,
		"timed_out":         timedOut,
		"assistant_message": buildAssistantMessage(content.String(), reasoningText, toolCalls),
	}
	if timedOut {
		result["error"] = "stream timed out"
	}

	return result, nil
}

func runToolRound(ctx context.Context, kwargs object.Kwargs, client *object.Instance, model string, messages object.Object, registry *object.Instance) (map[string]any, *object.Error) {
	schemasObj := tools.BuildSchemasObject(registry, ctx)
	if errObj, ok := schemasObj.(*object.Error); ok {
		return nil, errObj
	}

	streaming := kwargs.MustGetBool("stream", false)
	chunkTimeoutMs := kwargs.MustGetInt("chunk_timeout_ms", 0)
	callback := kwargs.Get("on_event")

	requestKwargs := filterCompletionKwargs(kwargs)
	requestKwargs.Kwargs["tools"] = schemasObj

	result := map[string]any{
		"tool_results": []any{},
		"tool_calls":   []any{},
	}

	if streaming {
		streamObj := completionStreamMethod(client, ctx, requestKwargs, model, conversion.ToGo(messages))
		if errObj, ok := streamObj.(*object.Error); ok {
			return nil, errObj
		}
		streamInstance, ok := streamObj.(*object.Instance)
		if !ok {
			return nil, &object.Error{Message: "tool_round: completion_stream did not return a ChatStream"}
		}

		streamResult, errObj := collectStream(ctx, streamInstance, chunkTimeoutMs, chunkTimeoutMs, callback)
		if errObj != nil {
			return nil, errObj
		}
		for k, v := range streamResult {
			result[k] = v
		}
	} else {
		responseObj := completionMethod(client, ctx, requestKwargs, model, conversion.ToGo(messages))
		if errObj, ok := responseObj.(*object.Error); ok {
			return nil, errObj
		}

		responseGo := conversion.ToGo(responseObj)
		responseMap, _ := responseGo.(map[string]any)
		message := extractAssistantMessage(responseMap)
		toolCalls := extractToolCallsFromGo(message)
		contentText := stringValue(message["content"])
		reasoningText := ""
		if contentText != "" {
			if extracted := extractThinking(contentText); extracted != nil {
				contentText = stringValue(extracted["content"])
				reasoningText = strings.Join(stringSlice(extracted["thinking"]), "\n\n")
			}
		}

		result["response"] = responseMap
		result["assistant_message"] = normalizeAssistantMessage(message)
		result["content"] = contentText
		result["reasoning"] = reasoningText
		result["tool_calls"] = toolCalls
		result["finish_reason"] = extractFinishReason(responseMap)
		result["timed_out"] = false
	}

	toolCalls := extractToolCallsFromGo(result["tool_calls"])
	toolResults, errObj := executeToolCalls(ctx, registry, toolCalls)
	if errObj != nil {
		return nil, errObj
	}
	result["tool_calls"] = toolCalls
	result["tool_results"] = toolResults

	return result, nil
}

func extractAssistantMessage(response map[string]any) map[string]any {
	if response == nil {
		return map[string]any{"role": "assistant", "content": ""}
	}
	if choicesRaw, ok := response["choices"].([]any); ok && len(choicesRaw) > 0 {
		if choice, ok := choicesRaw[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				return normalizeAssistantMessage(message)
			}
		}
	}
	return map[string]any{"role": "assistant", "content": ""}
}

func normalizeAssistantMessage(message map[string]any) map[string]any {
	if message == nil {
		return map[string]any{"role": "assistant", "content": ""}
	}
	normalized := map[string]any{
		"role":    stringOrDefault(message["role"], "assistant"),
		"content": stringValue(message["content"]),
	}
	if toolCalls := extractToolCallsFromGo(message); len(toolCalls) > 0 {
		normalized["tool_calls"] = toolCalls
	}
	if toolCallID := stringValue(message["tool_call_id"]); toolCallID != "" {
		normalized["tool_call_id"] = toolCallID
	}
	return normalized
}

func extractFinishReason(response map[string]any) string {
	if response == nil {
		return ""
	}
	if choicesRaw, ok := response["choices"].([]any); ok && len(choicesRaw) > 0 {
		if choice, ok := choicesRaw[0].(map[string]any); ok {
			return stringValue(choice["finish_reason"])
		}
	}
	return ""
}

func buildAssistantMessage(content string, reasoning string, toolCalls []map[string]any) map[string]any {
	messageContent := strings.TrimSpace(content)
	if reasoning != "" {
		if messageContent != "" {
			messageContent = "<thinking>\n" + reasoning + "\n</thinking>\n\n" + messageContent
		} else {
			messageContent = "<thinking>\n" + reasoning + "\n</thinking>"
		}
	}

	message := map[string]any{
		"role":    "assistant",
		"content": messageContent,
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	return message
}

func mergeToolCall(dst map[string]any, src map[string]any) {
	if id := stringValue(src["id"]); id != "" {
		dst["id"] = id
	}
	if tcType := stringValue(src["type"]); tcType != "" {
		dst["type"] = tcType
	}

	dstFunction, _ := dst["function"].(map[string]any)
	if dstFunction == nil {
		dstFunction = map[string]any{}
		dst["function"] = dstFunction
	}
	srcFunction, _ := src["function"].(map[string]any)
	if srcFunction == nil {
		return
	}
	if name := stringValue(srcFunction["name"]); name != "" {
		dstFunction["name"] = name
	}

	currentArgs := dstFunction["arguments"]
	nextArgs := srcFunction["arguments"]
	switch existing := currentArgs.(type) {
	case string:
		dstFunction["arguments"] = existing + stringValue(nextArgs)
	case nil:
		dstFunction["arguments"] = nextArgs
	default:
		if nextString := stringValue(nextArgs); nextString != "" {
			dstFunction["arguments"] = nextString
		}
	}
}

func normalizeToolFunction(raw any) map[string]any {
	fn, _ := raw.(map[string]any)
	if fn == nil {
		fn = map[string]any{}
	}
	return map[string]any{
		"name":      stringValue(fn["name"]),
		"arguments": normalizeToolArguments(fn["arguments"]),
	}
}

func normalizeToolArguments(raw any) any {
	switch v := raw.(type) {
	case nil:
		return map[string]any{}
	case map[string]any:
		return v
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return map[string]any{}
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			return parsed
		}
		return trimmed
	default:
		return v
	}
}

func stringifyToolResult(result object.Object) string {
	goValue := conversion.ToGo(result)
	switch v := goValue.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any, []any:
		bytes, err := json.Marshal(v)
		if err == nil {
			return string(bytes)
		}
	}

	if coerced, errObj := result.CoerceString(); errObj == nil {
		return coerced
	}
	return result.Inspect()
}

func emitCollectorEvent(ctx context.Context, callback object.Object, event map[string]any) {
	if callback == nil {
		return
	}
	eval := evaliface.FromContext(ctx)
	if eval == nil {
		return
	}
	result := eval.CallObjectFunction(ctx, callback, []object.Object{conversion.FromGo(event)}, nil, toolEnvFromContext(ctx))
	if _, ok := result.(*object.Error); ok {
		return
	}
}

func toolEnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok && env != nil {
		return env
	}
	return object.NewEnvironment()
}

func filterCompletionKwargs(kwargs object.Kwargs) object.Kwargs {
	filtered := object.NewKwargs(map[string]object.Object{})
	for key, value := range kwargs.Kwargs {
		switch key {
		case "stream", "chunk_timeout_ms", "on_event":
			continue
		default:
			filtered.Kwargs[key] = value
		}
	}
	return filtered
}

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func stringOrDefault(value any, defaultValue string) string {
	if s := stringValue(value); s != "" {
		return s
	}
	return defaultValue
}

func integerValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func stringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok && s != "" {
			result = append(result, s)
		}
	}
	return result
}

func copyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
