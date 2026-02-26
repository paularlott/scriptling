package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/paularlott/scriptling/conversion"

	"github.com/paularlott/mcp/ai"
	"github.com/paularlott/scriptling/object"
)

// ClientInstance wraps an AI client for use in scriptling
type ClientInstance struct {
	client ai.Client
}

var (
	openaiClientClass     *object.Class
	openaiClientClassOnce sync.Once
)

// GetOpenAIClientClass returns the OpenAI Client class (thread-safe singleton)
func GetOpenAIClientClass() *object.Class {
	openaiClientClassOnce.Do(func() {
		openaiClientClass = buildOpenAIClientClass()
	})
	return openaiClientClass
}

// buildOpenAIClientClass builds the OpenAI Client class
func buildOpenAIClientClass() *object.Class {
	return object.NewClassBuilder("OpenAIClient").
		MethodWithHelp("completion", completionMethod, `completion(model, messages, **kwargs) - Create a chat completion

Creates a chat completion using this client's configuration.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (str or list): Either a string (user message) or a list of message dicts with "role" and "content" keys
  system_prompt (str, optional): System prompt to use when messages is a string
  tools (list, optional): List of tool schema dicts from ToolRegistry.build()
  temperature (float, optional): Sampling temperature (0.0-2.0)
  top_p (float, optional): Nucleus sampling threshold (0.0-1.0)
  max_tokens (int, optional): Maximum tokens to generate

Returns:
  dict: Response containing id, choices, usage, etc.

Examples:
  # String shorthand (simple user message)
  response = client.completion("gpt-4", "Hello!")
  print(response.choices[0].message.content)

  # String shorthand with system prompt
  response = client.completion("gpt-4", "What is 2+2?", system_prompt="You are a helpful math tutor")
  print(response.choices[0].message.content)

  # Full messages array
  response = client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
  print(response.choices[0].message.content)

With tools:
  tools = ai.ToolRegistry()
  tools.add("get_time", "Get current time", {}, lambda args: "12:00 PM")
  schemas = tools.build()
  response = client.completion("gpt-4", [{"role": "user", "content": "What time is it?"}], tools=schemas)`).
		MethodWithHelp("completion_stream", completionStreamMethod, `completion_stream(model, messages, **kwargs) - Create a streaming chat completion

Creates a streaming chat completion using this client's configuration.
Returns a ChatStream object that can be iterated over.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (str or list): Either a string (user message) or a list of message dicts with "role" and "content" keys
  system_prompt (str, optional): System prompt to use when messages is a string
  temperature (float, optional): Sampling temperature (0.0-2.0)
  top_p (float, optional): Nucleus sampling threshold (0.0-1.0)
  max_tokens (int, optional): Maximum tokens to generate

Returns:
  ChatStream: A stream object with a next() method

Examples:
  # String shorthand (simple user message)
  stream = client.completion_stream("gpt-4", "Hello!")
  while True:
    chunk = stream.next()
    if chunk is None:
      break
    if chunk.choices and len(chunk.choices) > 0:
      delta = chunk.choices[0].delta
      if delta.content:
        print(delta.content, end="")
  print()

  # String shorthand with system prompt
  stream = client.completion_stream("gpt-4", "Explain quantum physics", system_prompt="You are a physics professor")
  # ... iterate as above

  # Full messages array
  stream = client.completion_stream("gpt-4", [{"role": "user", "content": "Hello!"}])
  # ... iterate as above`).
		MethodWithHelp("models", modelsMethod, `models() - List available models

Lists all models available for this client configuration.

Returns:
  list: List of model dicts with id, created, owned_by, etc.

Example:
  models = client.models()
  for model in models:
    print(model.id)`).
		MethodWithHelp("response_create", responseCreateMethod, `response_create(model, input, **kwargs) - Create a Responses API response

Creates a response using the OpenAI Responses API (new structured API).

Parameters:
  model (str): Model identifier (e.g., "gpt-4o", "gpt-4")
  input (str or list): Either a string (user message content) or a list of input items (messages)
  system_prompt (str, optional): System prompt to use when input is a string
  background (bool, optional): If true, runs asynchronously and returns immediately with in_progress status

Returns:
  dict: Response object with id, status, output, usage, etc.

Examples:
  # String shorthand (simple user message)
  response = client.response_create("gpt-4o", "Hello!")
  print(response.output)

  # String shorthand with system prompt
  response = client.response_create("gpt-4o", "What is AI?", system_prompt="You are a helpful assistant")
  print(response.output)

  # Background processing
  response = client.response_create("gpt-4o", "What is AI?", background=True)
  print(response.status)  # "queued" or "in_progress"
  # Poll for completion
  while response.status in ["queued", "in_progress"]:
    response = client.response_get(response.id)
  print(response.status)  # "completed"

  # Full input array (Responses API format)
  response = client.response_create("gpt-4o", [
    {"type": "message", "role": "user", "content": "Hello!"}
  ])
  print(response.output)`).
		MethodWithHelp("response_get", responseGetMethod, `response_get(id) - Get a response by ID

Retrieves a previously created response by its ID.

Parameters:
  id (str): Response ID

Returns:
  dict: Response object with id, status, output, usage, etc.

Example:
  response = client.response_get("resp_123")
  print(response.status)`).
		MethodWithHelp("response_cancel", responseCancelMethod, `response_cancel(id) - Cancel a response

Cancels a currently in-progress response.

Parameters:
  id (str): Response ID to cancel

Returns:
  dict: Cancelled response object

Example:
  response = client.response_cancel("resp_123")`).
		MethodWithHelp("response_delete", responseDeleteMethod, `response_delete(id) - Delete a response

Deletes a response by ID, removing it from storage.

Parameters:
  id (str): Response ID to delete

Returns:
  None

Example:
  client.response_delete("resp_123")`).
		MethodWithHelp("response_stream", responseStreamMethod, `response_stream(model, input, **kwargs) - Stream a Responses API response

Streams a response using the OpenAI Responses API, returning a ResponseStream object.

Parameters:
  model (str): Model identifier (e.g., "gpt-4o", "gpt-4")
  input (str or list): Either a string (user message content) or a list of input items (messages)
  system_prompt (str, optional): System prompt to use when input is a string

Returns:
  ResponseStream: A stream object with a next() method that yields ResponseStreamEvent dicts

Event types:
  - "response.created"           - response object created
  - "response.output_item.added" - new output item started
  - "response.output_text.delta" - text delta (use event.delta field)
  - "response.output_text.done"  - text item complete
  - "response.completed"         - full response object available
  - "error"                      - stream error

Examples:
  stream = client.response_stream("gpt-4o", "Hello!")
  while True:
    event = stream.next()
    if event is None:
      break
    if event.type == "response.output_text.delta":
      print(event.delta, end="")
  print()

  # With system prompt
  stream = client.response_stream("gpt-4o", "Explain AI", system_prompt="You are a helpful assistant")`).
		MethodWithHelp("response_compact", responseCompactMethod, `response_compact(id) - Compact a response

Compacts a response by removing intermediate reasoning steps, returning a more concise version.

Parameters:
  id (str): Response ID to compact

Returns:
  dict: Compacted response object with reasoning removed

Example:
  response = client.response_compact("resp_123")
  print(response.output)  # Output without reasoning steps`).
		MethodWithHelp("embedding", embeddingMethod, `embedding(model, input) - Create an embedding

Creates an embedding vector for the given input text(s) using the specified model.

Parameters:
  model (str): Model identifier (e.g., "text-embedding-3-small", "text-embedding-3-large")
  input (str or list): Input text(s) to embed - can be a string or list of strings

Returns:
  dict: Response containing data (list of embeddings with index, embedding, object), model, and usage

Example:
  response = client.embedding("text-embedding-3-small", "Hello world")
  print(response.data[0].embedding)

  # Batch embedding
  response = client.embedding("text-embedding-3-small", ["Hello", "World"])
  for emb in response.data:
    print(emb.embedding)`).
		MethodWithHelp("ask", askMethod, `ask(model, messages, **kwargs) - Quick completion that returns text directly

Creates a chat completion and returns just the text content, with thinking blocks automatically removed.
This is a convenience method for simple queries where you don't need the full response object.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (str or list): Either a string (user message) or a list of message dicts with "role" and "content" keys
  system_prompt (str, optional): System prompt to use when messages is a string
  tools (list, optional): List of tool schema dicts from ToolRegistry.build()
  temperature (float, optional): Sampling temperature (0.0-2.0)
  top_p (float, optional): Nucleus sampling threshold (0.0-1.0)
  max_tokens (int, optional): Maximum tokens to generate

Returns:
  str: The response text with thinking blocks removed

Examples:
  # Simple query
  answer = client.ask("gpt-4", "What is 2+2?")
  print(answer)  # "4"

  # With system prompt
  answer = client.ask("gpt-4", "Explain quantum physics", system_prompt="You are a physics professor")
  print(answer)

  # Full messages array
  answer = client.ask("gpt-4", [{"role": "user", "content": "Hello!"}])
  print(answer)`).
		Build()
}

// getClientInstance extracts the ClientInstance from an object.Instance
func getClientInstance(instance *object.Instance) (*ClientInstance, *object.Error) {
	wrapper, ok := object.GetClientField(instance, "_client")
	if !ok {
		return nil, &object.Error{Message: "OpenAIClient: missing internal client reference"}
	}
	if wrapper.Client == nil {
		return nil, &object.Error{Message: "OpenAIClient: client is nil"}
	}
	ci, ok := wrapper.Client.(*ClientInstance)
	if !ok {
		return nil, &object.Error{Message: "OpenAIClient: invalid internal client reference"}
	}
	return ci, nil
}

// completion method implementation
func completionMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, model string, messages any) object.Object {
	// Validate input and handle string shorthand BEFORE client check
	var messagesList []map[string]any

	// First, check if it's a string (string shorthand)
	if msgStr, ok := messages.(string); ok {
		// String shorthand: build messages array from string and optional system_prompt
		messagesList = []map[string]any{{"role": "user", "content": msgStr}}
		if kwargs.Has("system_prompt") {
			systemPrompt := kwargs.MustGetString("system_prompt", "")
			messagesList = append([]map[string]any{{"role": "system", "content": systemPrompt}}, messagesList...)
		}
	} else if msgList, ok := messages.([]map[string]any); ok {
		// Already properly typed messages array
		messagesList = msgList
		// Check for system_prompt kwarg - error if provided with array (ambiguous)
		if kwargs.Has("system_prompt") {
			return &object.Error{Message: "chat: system_prompt kwarg is only valid when passing a string, not a messages array"}
		}
	} else if msgSlice, ok := messages.([]any); ok {
		// Scriptling list comes as []any, need to convert each element to map[string]any
		messagesList = make([]map[string]any, 0, len(msgSlice))
		for i, item := range msgSlice {
			if msgMap, ok := item.(map[string]any); ok {
				messagesList = append(messagesList, msgMap)
			} else {
				return &object.Error{Message: fmt.Sprintf("chat: messages[%d] must be a dict", i)}
			}
		}
		// Check for system_prompt kwarg - error if provided with array (ambiguous)
		if kwargs.Has("system_prompt") {
			return &object.Error{Message: "chat: system_prompt kwarg is only valid when passing a string, not a messages array"}
		}
	} else if msgObj, ok := messages.(object.Object); ok {
		// Convert scriptling object to Go type (for cases where it's still an object)
		messagesGo := conversion.ToGo(msgObj)
		if msgList, ok := messagesGo.([]map[string]any); ok {
			// Successfully converted to messages array
			messagesList = msgList
			// Check for system_prompt kwarg - error if provided with array (ambiguous)
			if kwargs.Has("system_prompt") {
				return &object.Error{Message: "chat: system_prompt kwarg is only valid when passing a string, not a messages array"}
			}
		} else {
			return &object.Error{Message: "chat: messages must be a string or a list of message dicts"}
		}
	} else {
		return &object.Error{Message: "chat: messages must be a string or a list of message dicts"}
	}

	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "chat: no client configured"}
	}

	// Convert messages to ai.Message
	openaiMessages := make([]ai.Message, len(messagesList))
	for i, msg := range messagesList {
		omsg := ai.Message{}
		if role, ok := msg["role"].(string); ok {
			if role == "" {
				return &object.Error{Message: "chat: message role cannot be empty"}
			}
			omsg.Role = role
		} else {
			return &object.Error{Message: "chat: message missing required 'role' field"}
		}
		if content, ok := msg["content"]; ok {
			omsg.Content = content
		}
		if toolCallID, ok := msg["tool_call_id"].(string); ok {
			omsg.ToolCallID = toolCallID
		}
		// Extract tool_calls if present
		if toolCallsRaw, ok := msg["tool_calls"]; ok && toolCallsRaw != nil {
			// Convert to []any first
			var toolCallsList []any
			switch tc := toolCallsRaw.(type) {
			case []any:
				toolCallsList = tc
			case []map[string]any:
				toolCallsList = make([]any, len(tc))
				for i, v := range tc {
					toolCallsList[i] = v
				}
			case object.Object:
				// Convert scriptling object to Go
				tcGo := conversion.ToGo(tc)
				if tcSlice, ok := tcGo.([]any); ok {
					toolCallsList = tcSlice
				}
			}

			if len(toolCallsList) > 0 {
				toolCalls := make([]ai.ToolCall, 0, len(toolCallsList))
				for _, tcRaw := range toolCallsList {
					var tcMap map[string]any
					switch tcVal := tcRaw.(type) {
					case map[string]any:
						tcMap = tcVal
					case object.Object:
						if tcGo := conversion.ToGo(tcVal); tcGo != nil {
							if m, ok := tcGo.(map[string]any); ok {
								tcMap = m
							}
						}
					}

					if tcMap != nil {
						tc := ai.ToolCall{}
						if id, ok := tcMap["id"].(string); ok {
							tc.ID = id
						}
						if tcType, ok := tcMap["type"].(string); ok && tcType != "" {
							tc.Type = tcType
						} else {
							// Default to "function" if type is not specified or empty
							tc.Type = "function"
						}
						if fnRaw, ok := tcMap["function"]; ok && fnRaw != nil {
							var fnMap map[string]any
							switch fn := fnRaw.(type) {
							case map[string]any:
								fnMap = fn
							case object.Object:
								if fnGo := conversion.ToGo(fn); fnGo != nil {
									if m, ok := fnGo.(map[string]any); ok {
										fnMap = m
									}
								}
							}

							if fnMap != nil {
								if name, ok := fnMap["name"].(string); ok {
									tc.Function.Name = name
								}
								if args, ok := fnMap["arguments"]; ok {
									// Arguments can be string or map
									switch argsVal := args.(type) {
									case string:
										// Parse JSON string to map
										var argsMap map[string]any
										if err := json.Unmarshal([]byte(argsVal), &argsMap); err == nil {
											tc.Function.Arguments = argsMap
										}
									case map[string]any:
										tc.Function.Arguments = argsVal
									case object.Object:
										if argsGo := conversion.ToGo(argsVal); argsGo != nil {
											if m, ok := argsGo.(map[string]any); ok {
												tc.Function.Arguments = m
											}
										}
									}
								}
							}
						}
						toolCalls = append(toolCalls, tc)
					}
				}
				omsg.ToolCalls = toolCalls
			}
		}
		openaiMessages[i] = omsg
	}

	// Build request
	req := ai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
	}

	// Handle optional parameters (override client defaults)
	if kwargs.Has("temperature") {
		v := kwargs.MustGetFloat("temperature", 0)
		req.Temperature = &v
	}
	if kwargs.Has("top_p") {
		v := kwargs.MustGetFloat("top_p", 0)
		req.TopP = &v
	}
	if kwargs.Has("max_tokens") {
		req.MaxTokens = int(kwargs.MustGetInt("max_tokens", 0))
	}

	// Handle optional tools parameter
	if kwargs.Has("tools") {
		toolsObjs := kwargs.MustGetList("tools", nil)
		tools := make([]ai.Tool, 0, len(toolsObjs))
		for i, toolObj := range toolsObjs {
			// Convert dict to ai.Tool
			toolMap, err := toolObj.AsDict()
			if err != nil {
				return &object.Error{Message: fmt.Sprintf("tools[%d] must be a dict: %v", i, err)}
			}
			tool := ai.Tool{Type: "function"}
			if fnVal, ok := toolMap["function"]; ok && fnVal != nil {
				// Convert object.Object to Go map using ToGo
				fnGo := conversion.ToGo(fnVal)
				if fnMap, ok := fnGo.(map[string]any); ok {
					if name, ok := fnMap["name"].(string); ok {
						tool.Function.Name = name
					}
					if desc, ok := fnMap["description"].(string); ok {
						tool.Function.Description = desc
					}
					if params, ok := fnMap["parameters"].(map[string]any); ok {
						tool.Function.Parameters = params
					}
				}
			}
			tools = append(tools, tool)
		}
		req.Tools = tools
	}

	chatResp, chatErr := ci.client.ChatCompletion(ctx, req)
	if chatErr != nil {
		return &object.Error{Message: "chat completion failed: " + chatErr.Error()}
	}

	return conversion.FromGo(chatResp)
}

// models method implementation
func modelsMethod(self *object.Instance, ctx context.Context) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "models: no client configured"}
	}

	models, err := ci.client.GetModels(ctx)
	if err != nil {
		return &object.Error{Message: "failed to get models: " + err.Error()}
	}

	return conversion.FromGo(models)
}

// response_create method implementation
func responseCreateMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, model string, input any) object.Object {
	// Validate input and handle string shorthand BEFORE client check
	var inputList []any

	// First, check if it's a string (string shorthand)
	if inputStr, ok := input.(string); ok {
		// String shorthand: build input array from string and optional system_prompt
		inputList = []any{map[string]any{"type": "message", "role": "user", "content": inputStr}}
		if kwargs.Has("system_prompt") {
			systemPrompt := kwargs.MustGetString("system_prompt", "")
			inputList = append([]any{map[string]any{"type": "message", "role": "system", "content": systemPrompt}}, inputList...)
		}
	} else if inputSlice, ok := input.([]any); ok {
		// Scriptling list comes as []any, already the right type
		inputList = inputSlice
		// Check for system_prompt kwarg - error if provided with array (ambiguous)
		if kwargs.Has("system_prompt") {
			return &object.Error{Message: "response_create: system_prompt kwarg is only valid when passing a string, not an input array"}
		}
	} else if inputObj, ok := input.(object.Object); ok {
		// Convert scriptling object to Go type (for cases where it's still an object)
		inputGo := conversion.ToGo(inputObj)
		if inputSlice, ok := inputGo.([]any); ok {
			// Successfully converted to input array
			inputList = inputSlice
			// Check for system_prompt kwarg - error if provided with array (ambiguous)
			if kwargs.Has("system_prompt") {
				return &object.Error{Message: "response_create: system_prompt kwarg is only valid when passing a string, not an input array"}
			}
		} else {
			return &object.Error{Message: "response_create: input must be a string or a list of input items"}
		}
	} else {
		return &object.Error{Message: "response_create: input must be a string or a list of input items"}
	}

	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_create: no client configured"}
	}

	req := ai.CreateResponseRequest{
		Model: model,
		Input: inputList,
	}

	// Handle background parameter
	if kwargs.Has("background") {
		req.Background = kwargs.MustGetBool("background", false)
	}

	resp, err := ci.client.CreateResponse(ctx, req)
	if err != nil {
		return &object.Error{Message: "failed to create response: " + err.Error()}
	}

	return conversion.FromGo(resp)
}

// response_get method implementation
func responseGetMethod(self *object.Instance, ctx context.Context, id string) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_get: no client configured"}
	}

	resp, err := ci.client.GetResponse(ctx, id)
	if err != nil {
		return &object.Error{Message: "failed to get response: " + err.Error()}
	}

	return conversion.FromGo(resp)
}

// response_cancel method implementation
func responseCancelMethod(self *object.Instance, ctx context.Context, id string) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_cancel: no client configured"}
	}

	resp, err := ci.client.CancelResponse(ctx, id)
	if err != nil {
		return &object.Error{Message: "failed to cancel response: " + err.Error()}
	}

	return conversion.FromGo(resp)
}

// response_delete method implementation
func responseDeleteMethod(self *object.Instance, ctx context.Context, id string) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_delete: no client configured"}
	}

	err := ci.client.DeleteResponse(ctx, id)
	if err != nil {
		return &object.Error{Message: "failed to delete response: " + err.Error()}
	}

	return nil
}

// response_compact method implementation
func responseCompactMethod(self *object.Instance, ctx context.Context, id string) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_compact: no client configured"}
	}

	resp, err := ci.client.CompactResponse(ctx, id)
	if err != nil {
		return &object.Error{Message: "failed to compact response: " + err.Error()}
	}

	return conversion.FromGo(resp)
}

// embedding method implementation
func embeddingMethod(self *object.Instance, ctx context.Context, model string, input any) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "embedding: no client configured"}
	}

	req := ai.EmbeddingRequest{
		Model: model,
		Input: input,
	}

	resp, err := ci.client.CreateEmbedding(ctx, req)
	if err != nil {
		return &object.Error{Message: "failed to create embedding: " + err.Error()}
	}

	return conversion.FromGo(resp)
}

// ask method implementation - quick completion that returns text directly
func askMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, model string, messages any) object.Object {
	// Call the completion method first
	resp := completionMethod(self, ctx, kwargs, model, messages)

	// If there was an error, return it
	if errObj, ok := resp.(*object.Error); ok {
		return errObj
	}

	// Extract text from response (without thinking blocks)
	return extractTextFromResponse(resp)
}

// extractTextFromResponse extracts just the text content from a completion response
// with thinking blocks removed
func extractTextFromResponse(resp object.Object) object.Object {
	// Convert response to Go type
	respGo := conversion.ToGo(resp)
	responseMap, ok := respGo.(map[string]any)
	if !ok {
		return &object.String{Value: ""}
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

	return &object.String{Value: content}
}

// createClientInstance creates a new scriptling Instance wrapping an AI client
func createClientInstance(client ai.Client) *object.Instance {
	return &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				TypeName: "OpenAIClient",
				Client:   &ClientInstance{client: client},
			},
		},
	}
}

// response_stream method implementation
func responseStreamMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, model string, input any) object.Object {
	var inputList []any

	if inputStr, ok := input.(string); ok {
		inputList = []any{map[string]any{"type": "message", "role": "user", "content": inputStr}}
		if kwargs.Has("system_prompt") {
			systemPrompt := kwargs.MustGetString("system_prompt", "")
			inputList = append([]any{map[string]any{"type": "message", "role": "system", "content": systemPrompt}}, inputList...)
		}
	} else if inputSlice, ok := input.([]any); ok {
		inputList = inputSlice
		if kwargs.Has("system_prompt") {
			return &object.Error{Message: "response_stream: system_prompt kwarg is only valid when passing a string, not an input array"}
		}
	} else if inputObj, ok := input.(object.Object); ok {
		inputGo := conversion.ToGo(inputObj)
		if inputSlice, ok := inputGo.([]any); ok {
			inputList = inputSlice
			if kwargs.Has("system_prompt") {
				return &object.Error{Message: "response_stream: system_prompt kwarg is only valid when passing a string, not an input array"}
			}
		} else {
			return &object.Error{Message: "response_stream: input must be a string or a list of input items"}
		}
	} else {
		return &object.Error{Message: "response_stream: input must be a string or a list of input items"}
	}

	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_stream: no client configured"}
	}

	stream := ci.client.StreamResponse(ctx, ai.CreateResponseRequest{
		Model: model,
		Input: inputList,
	})

	return &object.Instance{
		Class: GetResponseStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ResponseStream",
				Client:   &ResponseStreamInstance{stream: stream},
			},
		},
	}
}

// ResponseStreamInstance wraps an AI response stream for use in scriptling
type ResponseStreamInstance struct {
	stream *ai.ResponseStream
}

var (
	responseStreamClass     *object.Class
	responseStreamClassOnce sync.Once
)

// GetResponseStreamClass returns the ResponseStream class (thread-safe singleton)
func GetResponseStreamClass() *object.Class {
	responseStreamClassOnce.Do(func() {
		responseStreamClass = buildResponseStreamClass()
	})
	return responseStreamClass
}

func buildResponseStreamClass() *object.Class {
	return object.NewClassBuilder("ResponseStream").
		MethodWithHelp("next", nextResponseStreamMethod, `next() - Get the next event from the response stream

Advances to the next SSE event and returns it as a dict.

Returns:
  dict: The next event with 'type' and event-specific fields, or null if the stream is complete

Event types and fields:
  - "response.output_text.delta": {type, delta, item_id, output_index, content_index}
  - "response.output_text.done":  {type, text, item_id, output_index, content_index}
  - "response.completed":         {type, response} where response is the full ResponseObject
  - others: {type, ...}

Example:
  stream = client.response_stream("gpt-4o", "Hello!")
  while True:
    event = stream.next()
    if event is None:
      break
    if event.type == "response.output_text.delta":
      print(event.delta, end="")`).
		Build()
}

func getResponseStreamInstance(instance *object.Instance) (*ResponseStreamInstance, *object.Error) {
	wrapper, ok := object.GetClientField(instance, "_stream")
	if !ok {
		return nil, &object.Error{Message: "ResponseStream: missing internal stream reference"}
	}
	if wrapper.Client == nil {
		return nil, &object.Error{Message: "ResponseStream: stream is nil"}
	}
	si, ok := wrapper.Client.(*ResponseStreamInstance)
	if !ok {
		return nil, &object.Error{Message: "ResponseStream: invalid internal stream reference"}
	}
	return si, nil
}

func nextResponseStreamMethod(self *object.Instance, ctx context.Context) object.Object {
	si, cerr := getResponseStreamInstance(self)
	if cerr != nil {
		return cerr
	}

	if si.stream == nil {
		return &object.Error{Message: "next: stream is nil"}
	}

	if !si.stream.Next() {
		if err := si.stream.Err(); err != nil {
			return &object.Error{Message: "stream error: " + err.Error()}
		}
		return &object.Null{}
	}

	event := si.stream.Current()
	// Convert event to a map: type + parsed data fields
	result := map[string]any{"type": event.Type}
	if len(event.Data) > 0 {
		var fields map[string]any
		if err := json.Unmarshal(event.Data, &fields); err == nil {
			for k, v := range fields {
				if k != "type" {
					result[k] = v
				}
			}
		}
	}
	return conversion.FromGo(result)
}

// ChatStreamInstance wraps an AI chat stream for use in scriptling
type ChatStreamInstance struct {
	stream *ai.ChatStream
}

// GetChatStreamClass returns the ChatStream class (thread-safe singleton)
var (
	chatStreamClass     *object.Class
	chatStreamClassOnce sync.Once
)

func GetChatStreamClass() *object.Class {
	chatStreamClassOnce.Do(func() {
		chatStreamClass = buildChatStreamClass()
	})
	return chatStreamClass
}

// buildChatStreamClass builds the ChatStream class
func buildChatStreamClass() *object.Class {
	return object.NewClassBuilder("ChatStream").
		MethodWithHelp("next", nextStreamMethod, `next() - Get the next chunk from the stream

Advances to the next response chunk and returns it.

Returns:
  dict: The next response chunk, or null if the stream is complete

Example:
  while True:
    chunk = stream.next()
    if chunk is None:
      break
    if chunk.choices and len(chunk.choices) > 0:
      delta = chunk.choices[0].delta
      if delta.content:
        print(delta.content, end="")`).
		MethodWithHelp("err", errStreamMethod, `err() - Get any error from the stream

Returns the error that caused the stream to stop, or None if no error.
A context.Canceled error indicates the stream was cancelled (e.g. user pressed Esc).

Returns:
  str: Error message, or None if no error`).
		Build()
}

// getStreamInstance extracts the ChatStreamInstance from an object.Instance
func getStreamInstance(instance *object.Instance) (*ChatStreamInstance, *object.Error) {
	wrapper, ok := object.GetClientField(instance, "_stream")
	if !ok {
		return nil, &object.Error{Message: "ChatStream: missing internal stream reference"}
	}
	if wrapper.Client == nil {
		return nil, &object.Error{Message: "ChatStream: stream is nil"}
	}
	si, ok := wrapper.Client.(*ChatStreamInstance)
	if !ok {
		return nil, &object.Error{Message: "ChatStream: invalid internal stream reference"}
	}
	return si, nil
}

// nextStream method implementation
func nextStreamMethod(self *object.Instance, ctx context.Context) object.Object {
	si, cerr := getStreamInstance(self)
	if cerr != nil {
		return cerr
	}

	if si.stream == nil {
		return &object.Error{Message: "next: stream is nil"}
	}

	// Advance to next chunk
	if !si.stream.Next() {
		return &object.Null{}
	}

	// Return current chunk
	current := si.stream.Current()
	return conversion.FromGo(current)
}

// errStream method implementation
func errStreamMethod(self *object.Instance, ctx context.Context) object.Object {
	si, cerr := getStreamInstance(self)
	if cerr != nil {
		return cerr
	}
	if si.stream == nil {
		return &object.Null{}
	}
	if err := si.stream.Err(); err != nil {
		return &object.String{Value: err.Error()}
	}
	return &object.Null{}
}

// completion_stream method implementation
func completionStreamMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, model string, messages any) object.Object {
	// Validate input and handle string shorthand BEFORE client check
	var messagesList []map[string]any

	// First, check if it's a string (string shorthand)
	if msgStr, ok := messages.(string); ok {
		// String shorthand: build messages array from string and optional system_prompt
		messagesList = []map[string]any{{"role": "user", "content": msgStr}}
		if kwargs.Has("system_prompt") {
			systemPrompt := kwargs.MustGetString("system_prompt", "")
			messagesList = append([]map[string]any{{"role": "system", "content": systemPrompt}}, messagesList...)
		}
	} else if msgList, ok := messages.([]map[string]any); ok {
		// Already properly typed messages array
		messagesList = msgList
		// Check for system_prompt kwarg - error if provided with array (ambiguous)
		if kwargs.Has("system_prompt") {
			return &object.Error{Message: "completion_stream: system_prompt kwarg is only valid when passing a string, not a messages array"}
		}
	} else if msgSlice, ok := messages.([]any); ok {
		// Scriptling list comes as []any, need to convert each element to map[string]any
		messagesList = make([]map[string]any, 0, len(msgSlice))
		for i, item := range msgSlice {
			if msgMap, ok := item.(map[string]any); ok {
				messagesList = append(messagesList, msgMap)
			} else {
				return &object.Error{Message: fmt.Sprintf("completion_stream: messages[%d] must be a dict", i)}
			}
		}
		// Check for system_prompt kwarg - error if provided with array (ambiguous)
		if kwargs.Has("system_prompt") {
			return &object.Error{Message: "completion_stream: system_prompt kwarg is only valid when passing a string, not a messages array"}
		}
	} else if msgObj, ok := messages.(object.Object); ok {
		// Convert scriptling object to Go type (for cases where it's still an object)
		messagesGo := conversion.ToGo(msgObj)
		if msgList, ok := messagesGo.([]map[string]any); ok {
			// Successfully converted to messages array
			messagesList = msgList
			// Check for system_prompt kwarg - error if provided with array (ambiguous)
			if kwargs.Has("system_prompt") {
				return &object.Error{Message: "completion_stream: system_prompt kwarg is only valid when passing a string, not a messages array"}
			}
		} else {
			return &object.Error{Message: "completion_stream: messages must be a string or a list of message dicts"}
		}
	} else {
		return &object.Error{Message: "completion_stream: messages must be a string or a list of message dicts"}
	}

	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "completion_stream: no client configured"}
	}

	// Convert messages to ai.Message
	openaiMessages := make([]ai.Message, len(messagesList))
	for i, msg := range messagesList {
		omsg := ai.Message{}
		if role, ok := msg["role"].(string); ok {
			if role == "" {
				return &object.Error{Message: "completion_stream: message role cannot be empty"}
			}
			omsg.Role = role
		} else {
			return &object.Error{Message: "completion_stream: message missing required 'role' field"}
		}
		if content, ok := msg["content"]; ok {
			omsg.Content = content
		}
		if toolCallID, ok := msg["tool_call_id"].(string); ok {
			omsg.ToolCallID = toolCallID
		}
		// Extract tool_calls if present
		if toolCallsRaw, ok := msg["tool_calls"]; ok && toolCallsRaw != nil {
			// Convert to []any first
			var toolCallsList []any
			switch tc := toolCallsRaw.(type) {
			case []any:
				toolCallsList = tc
			case []map[string]any:
				toolCallsList = make([]any, len(tc))
				for i, v := range tc {
					toolCallsList[i] = v
				}
			case object.Object:
				// Convert scriptling object to Go
				tcGo := conversion.ToGo(tc)
				if tcSlice, ok := tcGo.([]any); ok {
					toolCallsList = tcSlice
				}
			}

			if len(toolCallsList) > 0 {
				toolCalls := make([]ai.ToolCall, 0, len(toolCallsList))
				for _, tcRaw := range toolCallsList {
					var tcMap map[string]any
					switch tcVal := tcRaw.(type) {
					case map[string]any:
						tcMap = tcVal
					case object.Object:
						if tcGo := conversion.ToGo(tcVal); tcGo != nil {
							if m, ok := tcGo.(map[string]any); ok {
								tcMap = m
							}
						}
					}

					if tcMap != nil {
						tc := ai.ToolCall{}
						if id, ok := tcMap["id"].(string); ok {
							tc.ID = id
						}
						if tcType, ok := tcMap["type"].(string); ok && tcType != "" {
							tc.Type = tcType
						} else {
							// Default to "function" if type is not specified or empty
							tc.Type = "function"
						}
						if fnRaw, ok := tcMap["function"]; ok && fnRaw != nil {
							var fnMap map[string]any
							switch fn := fnRaw.(type) {
							case map[string]any:
								fnMap = fn
							case object.Object:
								if fnGo := conversion.ToGo(fn); fnGo != nil {
									if m, ok := fnGo.(map[string]any); ok {
										fnMap = m
									}
								}
							}

							if fnMap != nil {
								if name, ok := fnMap["name"].(string); ok {
									tc.Function.Name = name
								}
								if args, ok := fnMap["arguments"]; ok {
									// Arguments can be string or map
									switch argsVal := args.(type) {
									case string:
										// Parse JSON string to map
										var argsMap map[string]any
										if err := json.Unmarshal([]byte(argsVal), &argsMap); err == nil {
											tc.Function.Arguments = argsMap
										}
									case map[string]any:
										tc.Function.Arguments = argsVal
									case object.Object:
										if argsGo := conversion.ToGo(argsVal); argsGo != nil {
											if m, ok := argsGo.(map[string]any); ok {
												tc.Function.Arguments = m
											}
										}
									}
								}
							}
						}
						toolCalls = append(toolCalls, tc)
					}
				}
				omsg.ToolCalls = toolCalls
			}
		}
		openaiMessages[i] = omsg
	}

	// Create streaming request
	streamReq := ai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
	}

	// Handle optional parameters (override client defaults)
	if kwargs.Has("temperature") {
		v := kwargs.MustGetFloat("temperature", 0)
		streamReq.Temperature = &v
	}
	if kwargs.Has("top_p") {
		v := kwargs.MustGetFloat("top_p", 0)
		streamReq.TopP = &v
	}
	if kwargs.Has("max_tokens") {
		streamReq.MaxTokens = int(kwargs.MustGetInt("max_tokens", 0))
	}

	stream := ci.client.StreamChatCompletion(ctx, streamReq)

	// Wrap stream in instance
	return &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client:   &ChatStreamInstance{stream: stream},
			},
		},
	}
}
