package ai

import (
	"context"
	"fmt"
	"sync"

	"github.com/paularlott/mcp/openai"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const (
	AILibraryName = "ai"
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
func Register(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(AILibraryName, library)
}

// buildLibrary builds the AI library
func buildLibrary() *object.Library {
	return object.NewLibraryBuilder(AILibraryName, AILibraryDesc).

		// chat(model, messages...) - Create a chat completion
		RawFunctionWithHelp("chat", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}
			c := getClient()
			if c == nil {
				return errors.NewError("ai.chat: no client configured - use ai.SetClient() or create a client from script")
			}

			model, err := args[0].AsString()
			if err != nil {
				return errors.ParameterError("model", err)
			}

			// Convert messages from scriptling objects to openai.Message
			messages, convertErr := convertMessagesToOpenAI(args[1:])
			if convertErr != nil {
				return convertErr
			}

			chatResp, chatErr := c.ChatCompletion(ctx, openai.ChatCompletionRequest{
				Model:    model,
				Messages: messages,
			})
			if chatErr != nil {
				return errors.NewError("chat completion failed: %s", chatErr.Error())
			}

			return scriptlib.FromGo(chatResp)
		}, `chat(model, messages...) - Create a chat completion

Creates a chat completion using the specified model and messages.

Parameters:
  model (str): Model identifier (e.g., "gpt-4", "gpt-3.5-turbo")
  messages (dict...): One or more message dicts with "role" and "content" keys

Returns:
  dict: Response containing id, choices, usage, etc.

Example:
  response = ai.chat("gpt-4", {"role": "user", "content": "Hello!"})
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
		RawFunctionWithHelp("response_create", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 2); err != nil {
				return err
			}
			c := getClient()
			if c == nil {
				return errors.NewError("ai.response_create: no client configured - use ai.SetClient() or create a client from script")
			}

			model, err := args[0].AsString()
			if err != nil {
				return errors.ParameterError("model", err)
			}

			// Convert input from scriptling object to []any
			inputRaw := scriptlib.ToGo(args[1])
			input, ok := inputRaw.([]any)
			if !ok {
				// Wrap single item in array
				input = []any{inputRaw}
			}

			// Build request
			req := openai.CreateResponseRequest{
				Model: model,
				Input: input,
			}

			resp, createErr := c.CreateResponse(ctx, req)
			if createErr != nil {
				return errors.NewError("failed to create response: %s", createErr.Error())
			}

			return scriptlib.FromGo(resp)
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

		// new_client(api_key, **kwargs) - Create a new OpenAI client
		FunctionWithHelp("new_client", func(ctx context.Context, kwargs object.Kwargs, apiKey string) (object.Object, error) {
			// Get optional base_url from kwargs
			baseURL := kwargs.MustGetString("base_url", "")

			config := openai.Config{
				APIKey:  apiKey,
				BaseURL: baseURL,
			}

			client, err := openai.New(config)
			if err != nil {
				return nil, err
			}

			return createClientInstance(client), nil
		}, `new_client(api_key, **kwargs) - Create a new OpenAI client

Creates a new OpenAI client instance for making API calls.

Parameters:
  api_key (str): OpenAI API key
  base_url (str, optional): Custom base URL (defaults to https://api.openai.com/v1)

Returns:
  OpenAIClient: A client instance with methods for API calls

Example:
  # Default OpenAI API
  client = ai.new_client("sk-...")

  # Custom base URL (e.g., LM Studio, local LLM)
  client = ai.new_client("lm-studio", base_url="http://127.0.0.1:1234/v1")
  response = client.chat("gpt-4", {"role": "user", "content": "Hello!"})`).

		Build()
}

// getClient returns the current client (thread-safe)
func getClient() *openai.Client {
	clientMutex.RLock()
	defer clientMutex.RUnlock()
	return client
}

// convertMessagesToOpenAI converts scriptling message objects to openai.Message format
func convertMessagesToOpenAI(messages []object.Object) ([]openai.Message, object.Object) {
	openaiMessages := make([]openai.Message, 0, len(messages))
	for i := 0; i < len(messages); i++ {
		msg, ok := messages[i].(*object.Dict)
		if !ok {
			return nil, errors.NewError("messages must be dicts")
		}
		omsg := openai.Message{}
		for k, v := range msg.Pairs {
			switch k {
			case "role":
				if role, err := v.Value.AsString(); err == nil {
					if role == "" {
						return nil, errors.NewError("message role cannot be empty")
					}
					omsg.Role = role
				} else {
					return nil, errors.ParameterError("role", err)
				}
			case "content":
				omsg.Content = scriptlib.ToGo(v.Value)
			case "tool_calls":
				// tool_calls are handled automatically by the MCP OpenAI client
				// Scripts don't need to send tool_calls - they're in assistant responses
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
