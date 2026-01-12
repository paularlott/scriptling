package ai

import (
	"context"
	"sync"

	"github.com/paularlott/mcp"
	"github.com/paularlott/mcp/openai"
	scriptlib "github.com/paularlott/scriptling"
	scriptlingmcp "github.com/paularlott/scriptling/mcp"
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
		MethodWithHelp("chat", chatMethod, `chat(model, messages...) - Create a chat completion

Creates a chat completion using this client's configuration.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (dict...): One or more message dicts with "role" and "content" keys

Returns:
  dict: Response containing id, choices, usage, etc.

Example:
  response = client.chat("gpt-4", {"role": "user", "content": "Hello!"})
  print(response.choices[0].message.content)`).

		MethodWithHelp("models", modelsMethod, `models() - List available models

Lists all models available for this client configuration.

Returns:
  list: List of model dicts with id, created, owned_by, etc.

Example:
  models = client.models()
  for model in models:
    print(model.id)`).

		MethodWithHelp("response_create", responseCreateMethod, `response_create(input, model="gpt-4o") - Create a Responses API response

Creates a response using the OpenAI Responses API (new structured API).

Parameters:
  input (list): Input items (messages)
  model (str, optional): Model identifier (default: "gpt-4o")

Returns:
  dict: Response object with id, status, output, usage, etc.

Example:
  response = client.response_create([
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

		MethodWithHelp("add_remote_server", addRemoteServerMethod, `add_remote_server(mcp_client) - Add a remote MCP server

Adds a remote MCP server that will be available to all AI calls via this client.
The prefix is derived from the MCP client instance.

Parameters:
  mcp_client (MCPClient): An MCP client instance

Example:
  mcp_client = mcp.new_client("https://api.example.com/mcp", "myprefix")
  ai_client.add_remote_server(mcp_client)`).

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

// chat method implementation
func chatMethod(ctx context.Context, self *object.Instance, model string, messages ...object.Object) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "chat: no client configured"}
	}

	// Convert messages from scriptling objects to openai.Message
	openaiMessages, err := convertMessagesToOpenAI(messages)
	if err != nil {
		return err
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
func modelsMethod(ctx context.Context, self *object.Instance) object.Object {
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
func responseCreateMethod(ctx context.Context, self *object.Instance, inputObj object.Object, args ...object.Object) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "response_create: no client configured"}
	}

	// Convert input from scriptling object to []any
	inputRaw := scriptlib.ToGo(inputObj)
	input, ok := inputRaw.([]any)
	if !ok {
		input = []any{inputRaw}
	}

	req := openai.CreateResponseRequest{
		Input: input,
	}

	if len(args) > 0 {
		if model, err := args[0].AsString(); err == nil {
			req.Model = model
		}
	}

	resp, err := ci.client.CreateResponse(ctx, req)
	if err != nil {
		return &object.Error{Message: "failed to create response: " + err.Error()}
	}

	return scriptlib.FromGo(resp)
}

// response_get method implementation
func responseGetMethod(ctx context.Context, self *object.Instance, id string) object.Object {
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
func responseCancelMethod(ctx context.Context, self *object.Instance, id string) object.Object {
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
func addRemoteServerMethod(ctx context.Context, self *object.Instance, mcpClientObj object.Object) object.Object {
	ci, cerr := getClientInstance(self)
	if cerr != nil {
		return cerr
	}

	if ci.client == nil {
		return &object.Error{Message: "add_remote_server: no client configured"}
	}

	// Extract the MCP client from the scriptling instance
	instance, ok := mcpClientObj.(*object.Instance)
	if !ok {
		return &object.Error{Message: "add_remote_server: argument must be an MCP client instance"}
	}

	wrapper, ok := object.GetClientField(instance, "_client")
	if !ok {
		return &object.Error{Message: "add_remote_server: argument must be an MCP client instance"}
	}

	// The wrapper.Client is *scriptlingmcp.ClientInstance which has a GetClient() method
	mcpClientInstance, ok := wrapper.Client.(*scriptlingmcp.ClientInstance)
	if !ok {
		return &object.Error{Message: "add_remote_server: invalid MCP client instance"}
	}

	// Get the underlying *mcp.Client from the wrapper
	mcpClient := mcpClientInstance.GetClient()
	ci.client.AddRemoteServer(mcpClient)

	return &object.Null{}
}

// remove_remote_server method implementation
func removeRemoteServerMethod(ctx context.Context, self *object.Instance, prefix string) object.Object {
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

// convertMessagesToOpenAI converts scriptling message objects to openai.Message format
func convertMessagesToOpenAI(messages []object.Object) ([]openai.Message, object.Object) {
	openaiMessages := make([]openai.Message, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		msg, ok := messages[i].(*object.Dict)
		if !ok {
			return nil, &object.Error{Message: "messages must be dicts"}
		}
		omsg := openai.Message{}
		for k, v := range msg.Pairs {
			switch k {
			case "role":
				if role, err := v.Value.AsString(); err == nil {
					omsg.Role = role
				}
			case "content":
				omsg.Content = scriptlib.ToGo(v.Value)
			case "tool_calls":
				// TODO: implement tool_calls conversion
			case "tool_call_id":
				if tcid, err := v.Value.AsString(); err == nil {
					omsg.ToolCallID = tcid
				}
			}
		}
		openaiMessages = append(openaiMessages, omsg)
	}
	return openaiMessages, nil
}

// getMCPAuth creates an mcp.AuthProvider from auth configuration
func getMCPAuth(authDict *object.Dict) mcp.AuthProvider {
	authType := ""
	var token string

	for k, v := range authDict.Pairs {
		switch k {
		case "type":
			authType, _ = v.Value.AsString()
		case "token":
			token, _ = v.Value.AsString()
		}
	}

	if authType == "bearer" && token != "" {
		return mcp.NewBearerTokenAuth(token)
	}

	return nil
}
