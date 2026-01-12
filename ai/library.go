package ai

import (
	"context"
	"fmt"
	"sync"

	"github.com/paularlott/mcp/openai"
	scriptlib "github.com/paularlott/scriptling"
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
			c := getClient()
			if c == nil {
				return &object.Error{Message: "ai.chat: no client configured - use ai.SetClient() or create a client from script"}
			}
			if len(args) < 2 {
				return &object.Error{Message: "chat requires at least 2 arguments: model, messages"}
			}

			model, err := args[0].AsString()
			if err != nil {
				return &object.Error{Message: "model must be a string: " + object.AsError(err)}
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
				return &object.Error{Message: "chat completion failed: " + chatErr.Error()}
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
		RawFunctionWithHelp("models", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			c := getClient()
			if c == nil {
				return &object.Error{Message: "ai.models: no client configured - use ai.SetClient() or create a client from script"}
			}

			resp, err := c.GetModels(ctx)
			if err != nil {
				return &object.Error{Message: "failed to get models: " + err.Error()}
			}

			return scriptlib.FromGo(resp.Data)
		}, `models() - List available models

Lists all models available for the current API configuration.

Returns:
  list: List of model dicts with id, created, owned_by, etc.

Example:
  models = ai.models()
  for model in models:
    print(model.id)`).

		// response_create(input, **kwargs) - Create a Responses API response
		RawFunctionWithHelp("response_create", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			c := getClient()
			if c == nil {
				return &object.Error{Message: "ai.response_create: no client configured - use ai.SetClient() or create a client from script"}
			}
			if len(args) < 1 {
				return &object.Error{Message: "response_create requires at least 1 argument: input"}
			}

			// Convert input from scriptling object to []any
			inputRaw := scriptlib.ToGo(args[0])
			input, ok := inputRaw.([]any)
			if !ok {
				// Wrap single item in array
				input = []any{inputRaw}
			}

			// Build request
			req := openai.CreateResponseRequest{
				Input: input,
			}

			// Get optional model parameter from kwargs (default to "gpt-4o")
			model := kwargs.MustGetString("model", "gpt-4o")
			if model != "" {
				req.Model = model
			}

			resp, err := c.CreateResponse(ctx, req)
			if err != nil {
				return &object.Error{Message: "failed to create response: " + err.Error()}
			}

			return scriptlib.FromGo(resp)
		}, `response_create(input, **kwargs) - Create a Responses API response

Creates a response using the OpenAI Responses API (new structured API).

Parameters:
  input (list): Input items (messages)
  model (str, optional): Model identifier (default: "gpt-4o")

Returns:
  dict: Response object with id, status, output, usage, etc.

Example:
  # Default model (gpt-4o)
  response = ai.response_create([
    {"type": "message", "role": "user", "content": "Hello!"}
  ])

  # Custom model
  response = ai.response_create([
    {"type": "message", "role": "user", "content": "Hello!"}
  ], model="gpt-4")
  print(response.output)`).

		// response_get(id) - Get a Responses API response by ID
		FunctionWithHelp("response_get", func(ctx context.Context, id string) (any, error) {
			c := getClient()
			if c == nil {
				return nil, fmt.Errorf("no client configured")
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
				return nil, fmt.Errorf("no client configured")
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
		FunctionWithHelp("new_client", func(kwargs object.Kwargs, apiKey string) (object.Object, error) {
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
