package ai

import (
	"context"
	"sync"

	"github.com/paularlott/mcp"
	"github.com/paularlott/mcp/openai"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// ClientInstance wraps an OpenAI client for use in scriptling
type ClientInstance struct {
	client *openai.Client
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
		MethodWithHelp("completion", completionMethod, `completion(model, messages...) - Create a chat completion

Creates a chat completion using this client's configuration.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (dict...): One or more message dicts with "role" and "content" keys

Returns:
  dict: Response containing id, choices, usage, etc.

Example:
  response = client.completion("gpt-4", {"role": "user", "content": "Hello!"})
  print(response.choices[0].message.content)`).

		MethodWithHelp("completion_stream", completionStreamMethod, `completion_stream(model, messages...) - Create a streaming chat completion

Creates a streaming chat completion using this client's configuration.
Returns a ChatStream object that can be iterated over.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (dict...): One or more message dicts with "role" and "content" keys

Returns:
  ChatStream: A stream object with a next() method

Example:
  stream = client.completion_stream("gpt-4", {"role": "user", "content": "Hello!"})
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

		MethodWithHelp("add_remote_server", addRemoteServerMethod, `add_remote_server(base_url, **kwargs) - Add a remote MCP server

Adds a remote MCP server that will be available to all AI calls via this client.

Parameters:
  base_url (str): URL of the MCP server
  namespace (str, optional): Namespace for tool names
  bearer_token (str, optional): Bearer token for authentication

Example:
  ai_client.add_remote_server("https://api.example.com/mcp", namespace="myprefix")
  ai_client.add_remote_server("https://api.example.com/mcp", namespace="myprefix", bearer_token="secret")`).

		MethodWithHelp("remove_remote_server", removeRemoteServerMethod, `remove_remote_server(prefix) - Remove a remote MCP server

Removes a previously added remote MCP server.

Parameters:
  prefix (str): Prefix of the server to remove

Example:
  client.remove_remote_server("knot")`).

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
func completionMethod(self *object.Instance, ctx context.Context, model string, messages ...map[string]any) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "chat: no client configured"}
	}

	// Convert messages to openai.Message
	openaiMessages := make([]openai.Message, len(messages))
	for i, msg := range messages {
		omsg := openai.Message{}
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

	chatResp, chatErr := ci.client.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
	})
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

	resp, err := ci.client.GetModels(ctx)
	if err != nil {
		return &object.Error{Message: "failed to get models: " + err.Error()}
	}

	return scriptlib.FromGo(resp.Data)
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

	req := openai.CreateResponseRequest{
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

// add_remote_server method implementation
func addRemoteServerMethod(self *object.Instance, ctx context.Context, kwargs object.Kwargs, baseURL string) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "add_remote_server: no client configured"}
	}

	// Get optional parameters from kwargs
	namespace := kwargs.MustGetString("namespace", "")
	bearerToken := kwargs.MustGetString("bearer_token", "")

	// Create auth provider if bearer token is provided
	var authProvider mcp.AuthProvider
	if bearerToken != "" {
		authProvider = mcp.NewBearerTokenAuth(bearerToken)
	}

	// Create the RemoteServerConfig and add it
	config := openai.RemoteServerConfig{
		BaseURL:    baseURL,
		Auth:       authProvider,
		Namespace:  namespace,
	}
	ci.client.AddRemoteServer(config)

	return &object.Null{}
}

// remove_remote_server method implementation
func removeRemoteServerMethod(self *object.Instance, ctx context.Context, prefix string) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "remove_remote_server: no client configured"}
	}

	ci.client.RemoveRemoteServer(prefix)

	return &object.Null{}
}

// createClientInstance creates a new scriptling Instance wrapping an OpenAI client
func createClientInstance(client *openai.Client) *object.Instance {
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

// ChatStreamInstance wraps an OpenAI chat stream for use in scriptling
type ChatStreamInstance struct {
	stream *openai.ChatStream
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
func completionStreamMethod(self *object.Instance, ctx context.Context, model string, messages ...map[string]any) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "completion_stream: no client configured"}
	}

	// Convert messages to openai.Message
	openaiMessages := make([]openai.Message, len(messages))
	for i, msg := range messages {
		omsg := openai.Message{}
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
	stream := ci.client.StreamChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: openaiMessages,
	})

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
