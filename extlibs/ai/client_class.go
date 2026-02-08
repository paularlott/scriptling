package ai

import (
	"context"
	"fmt"
	"sync"

	"github.com/paularlott/mcp/ai"
	scriptlib "github.com/paularlott/scriptling"
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
  messages (list): List of message dicts with "role" and "content" keys
  tools (list, optional): List of tool schema dicts from ToolRegistry.build()
  temperature (float, optional): Sampling temperature (0.0-2.0)
  max_tokens (int, optional): Maximum tokens to generate

Returns:
  dict: Response containing id, choices, usage, etc.

Example:
  response = client.completion("gpt-4", [{"role": "user", "content": "Hello!"}])
  print(response.choices[0].message.content)

With tools:
  tools = ai.ToolRegistry()
  tools.add("get_time", "Get current time", {}, lambda args: "12:00 PM")
  schemas = tools.build()
  response = client.completion("gpt-4", [{"role": "user", "content": "What time is it?"}], tools=schemas)`).

		MethodWithHelp("completion_stream", completionStreamMethod, `completion_stream(model, messages) - Create a streaming chat completion

Creates a streaming chat completion using this client's configuration.
Returns a ChatStream object that can be iterated over.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (list): List of message dicts with "role" and "content" keys

Returns:
  ChatStream: A stream object with a next() method

Example:
  stream = client.completion_stream("gpt-4", [{"role": "user", "content": "Hello!"}])
  while True:
    chunk = stream.next()
    if chunk is None:
      break
    if chunk.choices and len(chunk.choices) > 0:
      delta = chunk.choices[0].delta
      if delta.content:
        print(delta.content, end="")
  print()`).

		MethodWithHelp("models", modelsMethod, `models() - List available models

Lists all models available for this client configuration.

Returns:
  list: List of model dicts with id, created, owned_by, etc.

Example:
  models = client.models()
  for model in models:
    print(model.id)`).

		MethodWithHelp("response_create", responseCreateMethod, `response_create(model, input) - Create a Responses API response

Creates a response using the OpenAI Responses API (new structured API).

Parameters:
  model (str): Model identifier (e.g., "gpt-4o", "gpt-4")
  input (list): Input items (messages)

Returns:
  dict: Response object with id, status, output, usage, etc.

Example:
  response = client.response_create("gpt-4", [
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
func completionMethod(self *object.Instance, ctx context.Context, model string, messages []map[string]any, kwargs object.Kwargs) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "chat: no client configured"}
	}

	// Convert messages to ai.Message
	openaiMessages := make([]ai.Message, len(messages))
	for i, msg := range messages {
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
		openaiMessages[i] = omsg
	}

	// Build request
	req := ai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
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
				fnGo := scriptlib.ToGo(fnVal)
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

	return scriptlib.FromGo(chatResp)
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

	models, err := ci.client.ListModels(ctx)
	if err != nil {
		return &object.Error{Message: "failed to get models: " + err.Error()}
	}

	return scriptlib.FromGo(models)
}

// response_create method implementation
func responseCreateMethod(self *object.Instance, ctx context.Context, model string, input []any) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_create: no client configured"}
	}

	req := ai.CreateResponseRequest{
		Model: model,
		Input: input,
	}

	resp, err := ci.client.CreateResponse(ctx, req)
	if err != nil {
		return &object.Error{Message: "failed to create response: " + err.Error()}
	}

	return scriptlib.FromGo(resp)
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

	return scriptlib.FromGo(resp)
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

	return scriptlib.FromGo(resp)
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

	return scriptlib.FromGo(resp)
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
		// Stream is done, check for error
		if err := si.stream.Err(); err != nil {
			return &object.Error{Message: "stream error: " + err.Error()}
		}
		return &object.Null{}
	}

	// Return current chunk
	current := si.stream.Current()
	return scriptlib.FromGo(current)
}

// completion_stream method implementation
func completionStreamMethod(self *object.Instance, ctx context.Context, model string, messages []map[string]any) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "completion_stream: no client configured"}
	}

	// Convert messages to ai.Message
	openaiMessages := make([]ai.Message, len(messages))
	for i, msg := range messages {
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
		openaiMessages[i] = omsg
	}

	// Create streaming request
	stream, err := ci.client.StreamChatCompletion(ctx, ai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
	})
	if err != nil {
		return &object.Error{Message: "completion_stream: " + err.Error()}
	}

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
