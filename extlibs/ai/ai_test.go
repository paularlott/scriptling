package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	mcpai "github.com/paularlott/mcp/ai"
	openaiapi "github.com/paularlott/mcp/ai/openai"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

type timeoutMockClient struct{}
type toolArgsMockClient struct{}
type toolStreamMockClient struct{}
type thinkingTagStreamMockClient struct{}

func (timeoutMockClient) Provider() string                                         { return "mock" }
func (timeoutMockClient) SupportsCapability(string) bool                           { return false }
func (timeoutMockClient) GetModels(context.Context) (*mcpai.ModelsResponse, error) { return nil, nil }
func (timeoutMockClient) ChatCompletion(ctx context.Context, req mcpai.ChatCompletionRequest) (*mcpai.ChatCompletionResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (timeoutMockClient) StreamChatCompletion(ctx context.Context, req mcpai.ChatCompletionRequest) *mcpai.ChatStream {
	responseChan := make(chan openaiapi.ChatCompletionResponse)
	errorChan := make(chan error)
	go func() {
		<-ctx.Done()
		errorChan <- ctx.Err()
		close(errorChan)
		close(responseChan)
	}()
	return openaiapi.NewChatStream(ctx, responseChan, errorChan)
}
func (timeoutMockClient) CreateEmbedding(context.Context, mcpai.EmbeddingRequest) (*mcpai.EmbeddingResponse, error) {
	return nil, nil
}
func (timeoutMockClient) CreateResponse(context.Context, mcpai.CreateResponseRequest) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (timeoutMockClient) StreamResponse(context.Context, mcpai.CreateResponseRequest) *mcpai.ResponseStream {
	return nil
}
func (timeoutMockClient) GetResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (timeoutMockClient) CancelResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (timeoutMockClient) DeleteResponse(context.Context, string) error { return nil }
func (timeoutMockClient) CompactResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (timeoutMockClient) Close() error { return nil }

func (toolArgsMockClient) Provider() string                                         { return "mock" }
func (toolArgsMockClient) SupportsCapability(string) bool                           { return false }
func (toolArgsMockClient) GetModels(context.Context) (*mcpai.ModelsResponse, error) { return nil, nil }
func (toolArgsMockClient) ChatCompletion(ctx context.Context, req mcpai.ChatCompletionRequest) (*mcpai.ChatCompletionResponse, error) {
	return &mcpai.ChatCompletionResponse{
		Choices: []openaiapi.Choice{
			{
				Message: openaiapi.Message{
					Role:    "assistant",
					Content: "",
					ToolCalls: []openaiapi.ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: openaiapi.ToolCallFunction{
								Name: "echo_tool",
								Arguments: map[string]any{
									"message": "hello from tool test",
								},
							},
						},
					},
				},
			},
		},
	}, nil
}
func (toolArgsMockClient) StreamChatCompletion(ctx context.Context, req mcpai.ChatCompletionRequest) *mcpai.ChatStream {
	responseChan := make(chan openaiapi.ChatCompletionResponse)
	errorChan := make(chan error)
	close(responseChan)
	close(errorChan)
	return openaiapi.NewChatStream(ctx, responseChan, errorChan)
}
func (toolArgsMockClient) CreateEmbedding(context.Context, mcpai.EmbeddingRequest) (*mcpai.EmbeddingResponse, error) {
	return nil, nil
}
func (toolArgsMockClient) CreateResponse(context.Context, mcpai.CreateResponseRequest) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolArgsMockClient) StreamResponse(context.Context, mcpai.CreateResponseRequest) *mcpai.ResponseStream {
	return nil
}
func (toolArgsMockClient) GetResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolArgsMockClient) CancelResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolArgsMockClient) DeleteResponse(context.Context, string) error { return nil }
func (toolArgsMockClient) CompactResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolArgsMockClient) Close() error { return nil }

func (toolStreamMockClient) Provider() string               { return "mock" }
func (toolStreamMockClient) SupportsCapability(string) bool { return false }
func (toolStreamMockClient) GetModels(context.Context) (*mcpai.ModelsResponse, error) {
	return nil, nil
}
func (toolStreamMockClient) ChatCompletion(context.Context, mcpai.ChatCompletionRequest) (*mcpai.ChatCompletionResponse, error) {
	return &mcpai.ChatCompletionResponse{}, nil
}
func (toolStreamMockClient) StreamChatCompletion(ctx context.Context, req mcpai.ChatCompletionRequest) *mcpai.ChatStream {
	responseChan := make(chan openaiapi.ChatCompletionResponse, 2)
	errorChan := make(chan error, 1)
	go func() {
		defer close(responseChan)
		defer close(errorChan)
		responseChan <- openaiapi.ChatCompletionResponse{
			Choices: []openaiapi.Choice{
				{
					Delta: openaiapi.Delta{
						ReasoningContent: "Thinking about tools.",
						ToolCalls: []openaiapi.DeltaToolCall{
							{
								Index: 0,
								ID:    "call_stream_1",
								Type:  "function",
								Function: openaiapi.DeltaFunction{
									Name:      "echo_tool",
									Arguments: `{"message":"hello from streaming helper"}`,
								},
							},
						},
					},
				},
			},
		}
		responseChan <- openaiapi.ChatCompletionResponse{
			Choices: []openaiapi.Choice{
				{
					FinishReason: "tool_calls",
				},
			},
		}
	}()
	return openaiapi.NewChatStream(ctx, responseChan, errorChan)
}
func (toolStreamMockClient) CreateEmbedding(context.Context, mcpai.EmbeddingRequest) (*mcpai.EmbeddingResponse, error) {
	return nil, nil
}
func (toolStreamMockClient) CreateResponse(context.Context, mcpai.CreateResponseRequest) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolStreamMockClient) StreamResponse(context.Context, mcpai.CreateResponseRequest) *mcpai.ResponseStream {
	return nil
}
func (toolStreamMockClient) GetResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolStreamMockClient) CancelResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolStreamMockClient) DeleteResponse(context.Context, string) error { return nil }
func (toolStreamMockClient) CompactResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (toolStreamMockClient) Close() error { return nil }

func (thinkingTagStreamMockClient) Provider() string               { return "mock" }
func (thinkingTagStreamMockClient) SupportsCapability(string) bool { return false }
func (thinkingTagStreamMockClient) GetModels(context.Context) (*mcpai.ModelsResponse, error) {
	return nil, nil
}
func (thinkingTagStreamMockClient) ChatCompletion(context.Context, mcpai.ChatCompletionRequest) (*mcpai.ChatCompletionResponse, error) {
	return &mcpai.ChatCompletionResponse{}, nil
}
func (thinkingTagStreamMockClient) StreamChatCompletion(ctx context.Context, req mcpai.ChatCompletionRequest) *mcpai.ChatStream {
	responseChan := make(chan openaiapi.ChatCompletionResponse, 3)
	errorChan := make(chan error, 1)
	go func() {
		defer close(responseChan)
		defer close(errorChan)
		responseChan <- openaiapi.ChatCompletionResponse{
			Choices: []openaiapi.Choice{
				{
					Delta: openaiapi.Delta{
						Content: "<thinking>\nThe user asked to read the ",
					},
				},
			},
		}
		responseChan <- openaiapi.ChatCompletionResponse{
			Choices: []openaiapi.Choice{
				{
					Delta: openaiapi.Delta{
						Content: "`LICENSE.txt` file.\n</thinking>\n\nHere is the file content.",
					},
				},
			},
		}
		responseChan <- openaiapi.ChatCompletionResponse{
			Choices: []openaiapi.Choice{
				{
					FinishReason: "stop",
				},
			},
		}
	}()
	return openaiapi.NewChatStream(ctx, responseChan, errorChan)
}
func (thinkingTagStreamMockClient) CreateEmbedding(context.Context, mcpai.EmbeddingRequest) (*mcpai.EmbeddingResponse, error) {
	return nil, nil
}
func (thinkingTagStreamMockClient) CreateResponse(context.Context, mcpai.CreateResponseRequest) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (thinkingTagStreamMockClient) StreamResponse(context.Context, mcpai.CreateResponseRequest) *mcpai.ResponseStream {
	return nil
}
func (thinkingTagStreamMockClient) GetResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (thinkingTagStreamMockClient) CancelResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (thinkingTagStreamMockClient) DeleteResponse(context.Context, string) error { return nil }
func (thinkingTagStreamMockClient) CompactResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (thinkingTagStreamMockClient) Close() error { return nil }

func TestAILibraryConstants(t *testing.T) {
	if AILibraryName != "scriptling.ai" {
		t.Errorf("AILibraryName = %q, want %q", AILibraryName, "scriptling.ai")
	}

	if AILibraryDesc == "" {
		t.Error("AILibraryDesc should not be empty")
	}
}

func TestGetOpenAIClientClass(t *testing.T) {
	class := GetOpenAIClientClass()

	if class == nil {
		t.Error("GetOpenAIClientClass() returned nil")
	}

	if class.Name != "OpenAIClient" {
		t.Errorf("Class name = %q, want %q", class.Name, "OpenAIClient")
	}
}

func TestGetChatStreamClass(t *testing.T) {
	class := GetChatStreamClass()

	if class == nil {
		t.Error("GetChatStreamClass() returned nil")
	}

	if class.Name != "ChatStream" {
		t.Errorf("Class name = %q, want %q", class.Name, "ChatStream")
	}
}

func TestGetOpenAIClientClassSingleton(t *testing.T) {
	class1 := GetOpenAIClientClass()
	class2 := GetOpenAIClientClass()

	if class1 != class2 {
		t.Error("GetOpenAIClientClass() should return the same instance (singleton)")
	}
}

func TestGetChatStreamClassSingleton(t *testing.T) {
	class1 := GetChatStreamClass()
	class2 := GetChatStreamClass()

	if class1 != class2 {
		t.Error("GetChatStreamClass() should return the same instance (singleton)")
	}
}

func TestConvertMapsToOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		messages []map[string]any
		wantLen  int
	}{
		{
			name: "single message with role and content",
			messages: []map[string]any{
				{"role": "user", "content": "Hello"},
			},
			wantLen: 1,
		},
		{
			name: "multiple messages",
			messages: []map[string]any{
				{"role": "system", "content": "You are helpful"},
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there!"},
			},
			wantLen: 3,
		},
		{
			name: "message with tool_call_id",
			messages: []map[string]any{
				{"role": "tool", "content": "result", "tool_call_id": "call_123"},
			},
			wantLen: 1,
		},
		{
			name:     "empty messages",
			messages: []map[string]any{},
			wantLen:  0,
		},
		{
			name: "message with only role",
			messages: []map[string]any{
				{"role": "user"},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMapsToOpenAI(tt.messages)
			if len(result) != tt.wantLen {
				t.Errorf("convertMapsToOpenAI() length = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestConvertMapsToOpenAIValues(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Hello", "tool_call_id": "call_123"},
	}

	result := convertMapsToOpenAI(messages)

	if len(result) != 1 {
		t.Fatalf("convertMapsToOpenAI() length = %d, want 1", len(result))
	}

	msg := result[0]
	if msg.Role != "user" {
		t.Errorf("msg.Role = %q, want %q", msg.Role, "user")
	}

	if msg.Content != "Hello" {
		t.Errorf("msg.Content = %v, want %q", msg.Content, "Hello")
	}

	if msg.ToolCallID != "call_123" {
		t.Errorf("msg.ToolCallID = %q, want %q", msg.ToolCallID, "call_123")
	}
}

// Test getClientInstance error paths
func TestGetClientInstanceErrors(t *testing.T) {
	tests := []struct {
		name      string
		instance  *object.Instance
		wantError string
	}{
		{
			name:      "nil instance",
			instance:  nil,
			wantError: "",
		},
		{
			name: "missing _client field",
			instance: &object.Instance{
				Class:  GetOpenAIClientClass(),
				Fields: map[string]object.Object{},
			},
			wantError: "missing internal client reference",
		},
		{
			name: "nil client",
			instance: &object.Instance{
				Class: GetOpenAIClientClass(),
				Fields: map[string]object.Object{
					"_client": &object.ClientWrapper{Client: nil},
				},
			},
			wantError: "client is nil",
		},
		{
			name: "invalid client type",
			instance: &object.Instance{
				Class: GetOpenAIClientClass(),
				Fields: map[string]object.Object{
					"_client": &object.ClientWrapper{Client: "not a ClientInstance"},
				},
			},
			wantError: "invalid internal client reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.instance == nil {
				return // Skip nil instance test
			}
			_, err := getClientInstance(tt.instance)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if tt.wantError != "" {
				if err.Message == "" {
					t.Errorf("error message should not be empty")
				}
			}
		})
	}
}

// Test completionMethod error paths
func TestCompletionMethodErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		instance  *object.Instance
		model     string
		messages  []map[string]any
		wantError string
	}{
		{
			name: "nil client",
			instance: &object.Instance{
				Class: GetOpenAIClientClass(),
				Fields: map[string]object.Object{
					"_client": &object.ClientWrapper{Client: nil},
				},
			},
			model:     "gpt-4",
			messages:  []map[string]any{{"role": "user", "content": "Hello"}},
			wantError: "client is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionMethod(tt.instance, ctx, object.Kwargs{}, tt.model, tt.messages)
			if result.Type() != object.ERROR_OBJ {
				t.Errorf("expected error, got %v", result.Type())
			}
		})
	}
}

// Test completionMethod message validation
func TestCompletionMethodMessageValidation(t *testing.T) {
	ctx := context.Background()

	// Create an instance with a valid ClientInstance structure but nil client
	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	tests := []struct {
		name      string
		messages  []map[string]any
		wantError string
	}{
		{
			name:      "empty role",
			messages:  []map[string]any{{"role": "", "content": "Hello"}},
			wantError: "role cannot be empty",
		},
		{
			name:      "missing role field",
			messages:  []map[string]any{{"content": "Hello"}},
			wantError: "missing required 'role' field",
		},
		{
			name:      "non-string role",
			messages:  []map[string]any{{"role": 123, "content": "Hello"}},
			wantError: "missing required 'role' field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionMethod(instance, ctx, object.Kwargs{}, "gpt-4", tt.messages)
			if result.Type() != object.ERROR_OBJ {
				t.Errorf("expected error, got %v", result.Type())
			}
			err := result.(*object.Error)
			if err.Message == "" {
				t.Error("error message should not be empty")
			}
		})
	}
}

// Test modelsMethod error paths
func TestModelsMethodErrors(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	result := modelsMethod(instance, ctx)
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
	}
}

// Test response methods error paths
func TestResponseMethodsErrors(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	t.Run("response_create with nil client", func(t *testing.T) {
		result := responseCreateMethod(instance, ctx, object.Kwargs{}, "gpt-4", []any{"test"})
		if result.Type() != object.ERROR_OBJ {
			t.Errorf("expected error, got %v", result.Type())
		}
	})

	t.Run("response_get with nil client", func(t *testing.T) {
		result := responseGetMethod(instance, ctx, "resp_123")
		if result.Type() != object.ERROR_OBJ {
			t.Errorf("expected error, got %v", result.Type())
		}
	})

	t.Run("response_cancel with nil client", func(t *testing.T) {
		result := responseCancelMethod(instance, ctx, "resp_123")
		if result.Type() != object.ERROR_OBJ {
			t.Errorf("expected error, got %v", result.Type())
		}
	})
}

// Test getStreamInstance error paths
func TestGetStreamInstanceErrors(t *testing.T) {
	tests := []struct {
		name      string
		instance  *object.Instance
		wantError string
	}{
		{
			name: "missing _stream field",
			instance: &object.Instance{
				Class:  GetChatStreamClass(),
				Fields: map[string]object.Object{},
			},
			wantError: "missing internal stream reference",
		},
		{
			name: "nil stream",
			instance: &object.Instance{
				Class: GetChatStreamClass(),
				Fields: map[string]object.Object{
					"_stream": &object.ClientWrapper{Client: nil},
				},
			},
			wantError: "stream is nil",
		},
		{
			name: "invalid stream type",
			instance: &object.Instance{
				Class: GetChatStreamClass(),
				Fields: map[string]object.Object{
					"_stream": &object.ClientWrapper{Client: "not a ChatStreamInstance"},
				},
			},
			wantError: "invalid internal stream reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getStreamInstance(tt.instance)
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if tt.wantError != "" && err.Message == "" {
				t.Errorf("error message should not be empty")
			}
		})
	}
}

// Test nextStreamMethod error paths
func TestNextStreamMethodErrors(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{stream: nil},
			},
		},
	}

	result := nextStreamMethod(instance, ctx)
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
	}
}

// Test completionStreamMethod message validation
func TestCompletionStreamMethodMessageValidation(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	tests := []struct {
		name      string
		messages  []map[string]any
		wantError string
	}{
		{
			name:      "empty role",
			messages:  []map[string]any{{"role": "", "content": "Hello"}},
			wantError: "role cannot be empty",
		},
		{
			name:      "missing role field",
			messages:  []map[string]any{{"content": "Hello"}},
			wantError: "missing required 'role' field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionStreamMethod(instance, ctx, object.Kwargs{}, "gpt-4", tt.messages)
			if result.Type() != object.ERROR_OBJ {
				t.Errorf("expected error, got %v", result.Type())
			}
		})
	}
}

// Test completionStreamMethod with nil client
func TestCompletionStreamMethodNilClient(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{Client: nil},
		},
	}

	result := completionStreamMethod(instance, ctx, object.Kwargs{}, "gpt-4", []map[string]any{{"role": "user", "content": "Hello"}})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
	}
}

func TestCompletionMethodTimeoutKwarg(t *testing.T) {
	ctx := context.Background()
	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: timeoutMockClient{}},
			},
		},
	}

	kwargs := object.NewKwargs(map[string]object.Object{
		"timeout_ms": &object.Integer{Value: 10},
	})
	result := completionMethod(instance, ctx, kwargs, "gpt-4", []map[string]any{{"role": "user", "content": "Hello"}})
	if result.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error, got %v", result.Type())
	}
	if !strings.Contains(result.Inspect(), "deadline exceeded") {
		t.Fatalf("expected deadline exceeded error, got %s", result.Inspect())
	}
}

func TestNextTimeoutSuppressesFallbackCancelError(t *testing.T) {
	ctx := context.Background()
	streamCtx, cancel := context.WithCancel(ctx)
	responseChan := make(chan openaiapi.ChatCompletionResponse)
	errorChan := make(chan error)
	go func() {
		<-streamCtx.Done()
		errorChan <- streamCtx.Err()
		close(errorChan)
		close(responseChan)
	}()

	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{
					stream: openaiapi.NewChatStream(streamCtx, responseChan, errorChan),
					cancel: cancel,
				},
			},
		},
	}

	result := nextTimeoutStreamMethod(instance, ctx, 5)
	if result.Type() != object.DICT_OBJ {
		t.Fatalf("expected dict timeout marker, got %v", result.Type())
	}

	time.Sleep(20 * time.Millisecond)

	errResult := errStreamMethod(instance, ctx)
	if errResult.Type() != object.NULL_OBJ {
		t.Fatalf("expected null after internal timeout cancellation, got %v (%s)", errResult.Type(), errResult.Inspect())
	}
}

func TestNextTimeoutCancelsStreamOnCallerCancellation(t *testing.T) {
	parentCtx := context.Background()
	streamCtx, cancel := context.WithCancel(parentCtx)
	responseChan := make(chan openaiapi.ChatCompletionResponse)
	errorChan := make(chan error)
	go func() {
		<-streamCtx.Done()
		errorChan <- streamCtx.Err()
		close(errorChan)
		close(responseChan)
	}()

	callCtx, callCancel := context.WithCancel(parentCtx)
	callCancel()

	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{
					stream: openaiapi.NewChatStream(streamCtx, responseChan, errorChan),
					cancel: cancel,
				},
			},
		},
	}

	result := nextTimeoutStreamMethod(instance, callCtx, 100)
	if result.Type() != object.NULL_OBJ {
		t.Fatalf("expected null on caller cancellation, got %v", result.Type())
	}

	time.Sleep(20 * time.Millisecond)

	errResult := errStreamMethod(instance, parentCtx)
	errStr, ok := errResult.(*object.String)
	if !ok {
		t.Fatalf("expected String error after caller cancellation, got %T", errResult)
	}
	if errStr.Value != context.Canceled.Error() {
		t.Fatalf("expected %q, got %q", context.Canceled.Error(), errStr.Value)
	}
}

func TestNextTimeoutCallerCancellationDoesNotWedgeSubsequentNext(t *testing.T) {
	parentCtx := context.Background()
	streamCtx, cancel := context.WithCancel(parentCtx)
	responseChan := make(chan openaiapi.ChatCompletionResponse)
	errorChan := make(chan error)
	go func() {
		<-streamCtx.Done()
		errorChan <- streamCtx.Err()
		close(errorChan)
		close(responseChan)
	}()

	callCtx, callCancel := context.WithCancel(parentCtx)
	callCancel()

	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{
					stream: openaiapi.NewChatStream(streamCtx, responseChan, errorChan),
					cancel: cancel,
				},
			},
		},
	}

	result := nextTimeoutStreamMethod(instance, callCtx, 100)
	if result.Type() != object.NULL_OBJ {
		t.Fatalf("expected null on caller cancellation, got %v", result.Type())
	}

	time.Sleep(20 * time.Millisecond)

	nextResult := nextStreamMethod(instance, parentCtx)
	if nextResult.Type() != object.NULL_OBJ {
		t.Fatalf("expected null from subsequent next() after cancellation, got %v", nextResult.Type())
	}
}

func TestCompletionToolCallArgumentsSupportDictGet(t *testing.T) {
	p := scriptlib.New()
	stdlib.RegisterAll(p)
	Register(p)

	if err := p.SetObjectVar("ai_client", WrapClient(toolArgsMockClient{})); err != nil {
		t.Fatalf("SetObjectVar(ai_client): %v", err)
	}

	result, err := p.Eval(`
response = ai_client.completion("gpt-4", "hello")
tool_calls = response.choices[0].message.tool_calls
tool_calls[0].function.arguments.get("message", "missing")
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	if str.Value != "hello from tool test" {
		t.Fatalf("expected %q, got %q", "hello from tool test", str.Value)
	}
}

func TestExtractToolCallsFromGo(t *testing.T) {
	response, err := toolArgsMockClient{}.ChatCompletion(context.Background(), mcpai.ChatCompletionRequest{})
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	toolCalls := extractToolCallsFromGo(chatCompletionResponseToGoMap(response))
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if got := toolCalls[0]["function"].(map[string]any)["name"]; got != "echo_tool" {
		t.Fatalf("expected tool name echo_tool, got %#v", got)
	}
	args, ok := toolCalls[0]["function"].(map[string]any)["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("expected normalized arguments dict")
	}
	if args["message"] != "hello from tool test" {
		t.Fatalf("expected message argument, got %#v", args["message"])
	}
}

func TestCollectStreamAggregatesToolCalls(t *testing.T) {
	stream := toolStreamMockClient{}.StreamChatCompletion(context.Background(), mcpai.ChatCompletionRequest{})
	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client: &ChatStreamInstance{
					stream: stream,
				},
			},
		},
	}

	result, errObj := collectStream(context.Background(), instance, 100, 0, nil)
	if errObj != nil {
		t.Fatalf("collectStream returned error: %s", errObj.Message)
	}
	if result["reasoning"] != "Thinking about tools." {
		t.Fatalf("unexpected reasoning: %#v", result["reasoning"])
	}
	if result["finish_reason"] != "tool_calls" {
		t.Fatalf("unexpected finish_reason: %#v", result["finish_reason"])
	}
	toolCalls, ok := result["tool_calls"].([]map[string]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected one aggregated tool call, got %#v", result["tool_calls"])
	}
	args, ok := toolCalls[0]["function"].(map[string]any)["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("expected tool arguments dict")
	}
	if args["message"] != "hello from streaming helper" {
		t.Fatalf("unexpected streamed message: %#v", args["message"])
	}
}

func TestCollectStreamExtractsThinkingTagsFromContentDeltas(t *testing.T) {
	stream := thinkingTagStreamMockClient{}.StreamChatCompletion(context.Background(), mcpai.ChatCompletionRequest{})
	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client: &ChatStreamInstance{
					stream: stream,
				},
			},
		},
	}

	result, errObj := collectStream(context.Background(), instance, 100, 0, nil)
	if errObj != nil {
		t.Fatalf("collectStream returned error: %s", errObj.Message)
	}
	if result["reasoning"] != "The user asked to read the `LICENSE.txt` file." {
		t.Fatalf("unexpected reasoning: %#v", result["reasoning"])
	}
	if result["content"] != "Here is the file content." {
		t.Fatalf("unexpected content: %#v", result["content"])
	}

	assistantMessage, ok := result["assistant_message"].(map[string]any)
	if !ok {
		t.Fatalf("assistant_message should be a map, got %T", result["assistant_message"])
	}
	if assistantMessage["content"] != "<thinking>\nThe user asked to read the `LICENSE.txt` file.\n</thinking>\n\nHere is the file content." {
		t.Fatalf("unexpected assistant message content: %#v", assistantMessage["content"])
	}
}

func TestAIToolHelperScriptAPI(t *testing.T) {
	t.Run("tool_round executes non-streaming tool calls", func(t *testing.T) {
		p := scriptlib.New()
		stdlib.RegisterAll(p)
		Register(p)

		if err := p.SetObjectVar("ai_client", WrapClient(toolArgsMockClient{})); err != nil {
			t.Fatalf("SetObjectVar(ai_client): %v", err)
		}

		result, err := p.Eval(`
import scriptling.ai as ai

tools = ai.ToolRegistry()
tools.add("echo_tool", "Echo a message", {"message": "string"}, lambda args: "handled:" + args.get("message", "missing"))

round = ai.tool_round(ai_client, "gpt-4", "hello", tools)
round["tool_results"][0]["content"]
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		str, ok := result.(*object.String)
		if !ok {
			t.Fatalf("expected String, got %T", result)
		}
		if str.Value != "handled:hello from tool test" {
			t.Fatalf("expected tool result, got %q", str.Value)
		}
	})

	t.Run("tool_round executes streaming tool calls", func(t *testing.T) {
		p := scriptlib.New()
		stdlib.RegisterAll(p)
		Register(p)

		if err := p.SetObjectVar("ai_client", WrapClient(toolStreamMockClient{})); err != nil {
			t.Fatalf("SetObjectVar(ai_client): %v", err)
		}

		result, err := p.Eval(`
import scriptling.ai as ai

tools = ai.ToolRegistry()
tools.add("echo_tool", "Echo a message", {"message": "string"}, lambda args: "streamed:" + args.get("message", "missing"))

round = ai.tool_round(ai_client, "gpt-4", "hello", tools, stream=True, chunk_timeout_ms=100)
round["reasoning"] + "|" + round["tool_results"][0]["content"]
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}

		str, ok := result.(*object.String)
		if !ok {
			t.Fatalf("expected String, got %T", result)
		}
		if str.Value != "Thinking about tools.|streamed:hello from streaming helper" {
			t.Fatalf("unexpected streaming tool round output: %q", str.Value)
		}
	})
}

// Test createClientInstance creates a valid instance
func TestCreateClientInstance(t *testing.T) {
	instance := createClientInstance(nil)

	if instance == nil {
		t.Fatal("createClientInstance() returned nil")
	}

	if instance.Class != GetOpenAIClientClass() {
		t.Errorf("instance.Class = %v, want %v", instance.Class, GetOpenAIClientClass())
	}

	if len(instance.Fields) != 1 {
		t.Errorf("instance.Fields length = %d, want 1", len(instance.Fields))
	}

	clientWrapper, ok := instance.Fields["_client"].(*object.ClientWrapper)
	if !ok {
		t.Error("_client field is not a ClientWrapper")
	}

	if clientWrapper.TypeName != "OpenAIClient" {
		t.Errorf("ClientWrapper.TypeName = %q, want %q", clientWrapper.TypeName, "OpenAIClient")
	}

	if clientWrapper.Client == nil {
		t.Error("ClientWrapper.Client should not be nil")
	}

	_, ok = clientWrapper.Client.(*ClientInstance)
	if !ok {
		t.Error("ClientWrapper.Client is not a *ClientInstance")
	}
}

// Test WrapClient creates a valid instance
func TestWrapClient(t *testing.T) {
	result := WrapClient(nil)

	if result == nil {
		t.Fatal("WrapClient() returned nil")
	}

	instance, ok := result.(*object.Instance)
	if !ok {
		t.Fatalf("WrapClient() returned %T, want *object.Instance", result)
	}

	if instance.Class != GetOpenAIClientClass() {
		t.Errorf("instance.Class = %v, want %v", instance.Class, GetOpenAIClientClass())
	}
}

// Test buildLibrary creates a valid library
func TestBuildLibrary(t *testing.T) {
	lib := buildLibrary()

	if lib == nil {
		t.Fatal("buildLibrary() returned nil")
	}

	if lib.Description() != AILibraryDesc {
		t.Errorf("library.Description() = %q, want %q", lib.Description(), AILibraryDesc)
	}

	// Check that expected functions exist (only library-level functions)
	expectedFuncs := []string{"Client", "extract_thinking", "text", "thinking", "tool_calls", "execute_tool_calls", "collect_stream", "tool_round", "estimate_tokens"}
	for _, name := range expectedFuncs {
		if _, ok := lib.Functions()[name]; !ok {
			t.Errorf("library missing function %q", name)
		}
	}

	// Check that ToolRegistry constant exists
	if _, ok := lib.Constants()["ToolRegistry"]; !ok {
		t.Error("library missing ToolRegistry constant")
	}
}

// mockRegistrar implements the RegisterLibrary interface
type mockRegistrar struct {
	libraryName string
	library     *object.Library
	called      bool
}

func (m *mockRegistrar) RegisterLibrary(lib *object.Library) {
	m.libraryName = lib.Name()
	m.library = lib
	m.called = true
}

// Test Register function (basic smoke test)
func TestRegister(t *testing.T) {
	// Create a mock registrar
	registrar := &mockRegistrar{}

	// Register should call the RegisterLibrary method
	Register(registrar)

	if !registrar.called {
		t.Error("Register did not call RegisterLibrary")
	}

	if registrar.libraryName != AILibraryName {
		t.Errorf("registered name = %q, want %q", registrar.libraryName, AILibraryName)
	}

	if registrar.library == nil {
		t.Error("library was not registered")
	}

	if registrar.library.Description() != AILibraryDesc {
		t.Errorf("registered library description = %q, want %q", registrar.library.Description(), AILibraryDesc)
	}
}

// Test OpenAIClientClass has expected methods
func TestOpenAIClientClassMethods(t *testing.T) {
	class := GetOpenAIClientClass()

	expectedMethods := []string{
		"completion", "completion_stream", "models",
		"response_create", "response_stream", "response_get", "response_cancel",
		"embedding", "ask",
	}

	for _, methodName := range expectedMethods {
		if _, ok := class.Methods[methodName]; !ok {
			t.Errorf("OpenAIClient class missing method %q", methodName)
		}
	}
}

// Test ChatStreamClass has expected methods
func TestChatStreamClassMethods(t *testing.T) {
	class := GetChatStreamClass()

	expectedMethods := []string{"next"}

	for _, methodName := range expectedMethods {
		if _, ok := class.Methods[methodName]; !ok {
			t.Errorf("ChatStream class missing method %q", methodName)
		}
	}
}

// Test getClientInstance with valid client instance
func TestGetClientInstanceValid(t *testing.T) {
	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				TypeName: "OpenAIClient",
				Client:   &ClientInstance{client: nil},
			},
		},
	}

	ci, err := getClientInstance(instance)
	if err != nil {
		t.Fatalf("getClientInstance() error = %v", err)
	}

	if ci == nil {
		t.Error("getClientInstance() returned nil ClientInstance")
	}

	if ci.client != nil {
		t.Errorf("ClientInstance.client = %v, want nil", ci.client)
	}
}

// Test getStreamInstance with valid stream instance
func TestGetStreamInstanceValid(t *testing.T) {
	instance := &object.Instance{
		Class: GetChatStreamClass(),
		Fields: map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client:   &ChatStreamInstance{stream: nil},
			},
		},
	}

	si, err := getStreamInstance(instance)
	if err != nil {
		t.Fatalf("getStreamInstance() error = %v", err)
	}

	if si == nil {
		t.Error("getStreamInstance() returned nil ChatStreamInstance")
	}

	if si.stream != nil {
		t.Errorf("ChatStreamInstance.stream = %v, want nil", si.stream)
	}
}

// Test completionMethod with string shorthand
func TestCompletionMethodStringShorthand(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	tests := []struct {
		name          string
		messages      any
		kwargs        object.Kwargs
		expectedError string
	}{
		{
			name:          "string without system_prompt",
			messages:      "Hello, AI!",
			kwargs:        object.Kwargs{},
			expectedError: "no client configured",
		},
		{
			name:     "string with system_prompt",
			messages: "What is 2+2?",
			kwargs: object.NewKwargs(map[string]object.Object{
				"system_prompt": &object.String{Value: "You are a helpful math tutor"},
			}),
			expectedError: "no client configured",
		},
		{
			name:          "array still works",
			messages:      []map[string]any{{"role": "user", "content": "Hello!"}},
			kwargs:        object.Kwargs{},
			expectedError: "no client configured",
		},
		{
			name:     "system_prompt with array should error",
			messages: []map[string]any{{"role": "user", "content": "Hello!"}},
			kwargs: object.NewKwargs(map[string]object.Object{
				"system_prompt": &object.String{Value: "You are helpful"},
			}),
			expectedError: "system_prompt kwarg is only valid when passing a string",
		},
		{
			name:          "invalid type for messages",
			messages:      123,
			kwargs:        object.Kwargs{},
			expectedError: "must be a string or a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionMethod(instance, ctx, tt.kwargs, "gpt-4", tt.messages)
			if result.Type() != object.ERROR_OBJ {
				t.Errorf("expected error, got %v", result.Type())
				return
			}
			err := result.(*object.Error)
			if err.Message == "" {
				t.Error("error message should not be empty")
			}
			if tt.expectedError != "" && !contains(err.Message, tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Message)
			}
		})
	}
}

// Test completionStreamMethod with string shorthand
func TestCompletionStreamMethodStringShorthand(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	tests := []struct {
		name          string
		messages      any
		kwargs        object.Kwargs
		expectedError string
	}{
		{
			name:          "string without system_prompt",
			messages:      "Hello, AI!",
			kwargs:        object.Kwargs{},
			expectedError: "no client configured",
		},
		{
			name:     "string with system_prompt",
			messages: "Explain quantum physics",
			kwargs: object.NewKwargs(map[string]object.Object{
				"system_prompt": &object.String{Value: "You are a physics professor"},
			}),
			expectedError: "no client configured",
		},
		{
			name:          "array still works",
			messages:      []map[string]any{{"role": "user", "content": "Hello!"}},
			kwargs:        object.Kwargs{},
			expectedError: "no client configured",
		},
		{
			name:     "system_prompt with array should error",
			messages: []map[string]any{{"role": "user", "content": "Hello!"}},
			kwargs: object.NewKwargs(map[string]object.Object{
				"system_prompt": &object.String{Value: "You are helpful"},
			}),
			expectedError: "system_prompt kwarg is only valid when passing a string",
		},
		{
			name:          "invalid type for messages",
			messages:      123,
			kwargs:        object.Kwargs{},
			expectedError: "must be a string or a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionStreamMethod(instance, ctx, tt.kwargs, "gpt-4", tt.messages)
			if result.Type() != object.ERROR_OBJ {
				t.Errorf("expected error, got %v", result.Type())
				return
			}
			err := result.(*object.Error)
			if err.Message == "" {
				t.Error("error message should not be empty")
			}
			if tt.expectedError != "" && !contains(err.Message, tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Message)
			}
		})
	}
}

// Test responseCreateMethod with string shorthand
func TestResponseCreateMethodStringShorthand(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	tests := []struct {
		name          string
		input         any
		kwargs        object.Kwargs
		expectedError string
	}{
		{
			name:          "string without system_prompt",
			input:         "Hello, AI!",
			kwargs:        object.Kwargs{},
			expectedError: "no client configured",
		},
		{
			name:  "string with system_prompt",
			input: "What is AI?",
			kwargs: object.NewKwargs(map[string]object.Object{
				"system_prompt": &object.String{Value: "You are a helpful assistant"},
			}),
			expectedError: "no client configured",
		},
		{
			name:          "array still works",
			input:         []any{map[string]any{"type": "message", "role": "user", "content": "Hello!"}},
			kwargs:        object.Kwargs{},
			expectedError: "no client configured",
		},
		{
			name:  "system_prompt with array should error",
			input: []any{map[string]any{"type": "message", "role": "user", "content": "Hello!"}},
			kwargs: object.NewKwargs(map[string]object.Object{
				"system_prompt": &object.String{Value: "You are helpful"},
			}),
			expectedError: "system_prompt kwarg is only valid when passing a string",
		},
		{
			name:          "invalid type for input",
			input:         123,
			kwargs:        object.Kwargs{},
			expectedError: "must be a string or a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := responseCreateMethod(instance, ctx, tt.kwargs, "gpt-4", tt.input)
			if result.Type() != object.ERROR_OBJ {
				t.Errorf("expected error, got %v", result.Type())
				return
			}
			err := result.(*object.Error)
			if err.Message == "" {
				t.Error("error message should not be empty")
			}
			if tt.expectedError != "" && !contains(err.Message, tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, err.Message)
			}
		})
	}
}

// Helper function for string containment
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestExtractThinking(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedBlocks  int
		expectedContent string
	}{
		{
			name:            "no thinking blocks",
			input:           "Hello, how can I help you?",
			expectedBlocks:  0,
			expectedContent: "Hello, how can I help you?",
		},
		{
			name:            "XML think block",
			input:           "<think>Let me think about this...</think>\nHere's my response.",
			expectedBlocks:  1,
			expectedContent: "Here's my response.",
		},
		{
			name:            "XML thinking block",
			input:           "<thinking>Analyzing the question</thinking>\n\nThe answer is 42.",
			expectedBlocks:  1,
			expectedContent: "The answer is 42.",
		},
		{
			name:            "multiple think blocks",
			input:           "<think>First thought</think>\nResponse one.\n<think>Second thought</think>\nResponse two.",
			expectedBlocks:  2,
			expectedContent: "Response one.\n\nResponse two.",
		},
		{
			name:            "Thought block (OpenAI style)",
			input:           "<Thought>Reasoning here</Thought>Final answer.",
			expectedBlocks:  1,
			expectedContent: "Final answer.",
		},
		{
			name:            "markdown code block",
			input:           "```thinking\nMy internal reasoning\n```\n\nActual response here.",
			expectedBlocks:  1,
			expectedContent: "Actual response here.",
		},
		{
			name:            "mixed formats",
			input:           "<think>First</think>\n```thinking\nSecond\n```\nFinal answer",
			expectedBlocks:  2,
			expectedContent: "Final answer",
		},
		{
			name:            "multiline think block",
			input:           "<think>\nLine 1\nLine 2\nLine 3\n</think>\n\nResponse",
			expectedBlocks:  1,
			expectedContent: "Response",
		},
		{
			name:            "case insensitive",
			input:           "<THINK>Caps thinking</THINK>Response here",
			expectedBlocks:  1,
			expectedContent: "Response here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractThinking(tt.input)

			thinking, ok := result["thinking"].([]any)
			if !ok {
				t.Fatalf("thinking is not []any, got %T", result["thinking"])
			}

			if len(thinking) != tt.expectedBlocks {
				t.Errorf("got %d thinking blocks, want %d", len(thinking), tt.expectedBlocks)
			}

			content, ok := result["content"].(string)
			if !ok {
				t.Fatalf("content is not string, got %T", result["content"])
			}

			if content != tt.expectedContent {
				t.Errorf("content = %q, want %q", content, tt.expectedContent)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	lib := buildLibrary()
	fn, ok := lib.Functions()["estimate_tokens"]
	if !ok {
		t.Fatal("estimate_tokens function not found in library")
	}

	t.Run("string request with chat completion response", func(t *testing.T) {
		request := conversion.FromGo("What is 2+2?")
		response := conversion.FromGo(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "The answer is 4.",
					},
				},
			},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		resultGo := conversion.ToGo(result)
		usage, ok := resultGo.(map[string]any)
		if !ok {
			t.Fatalf("result is %T, want map[string]any", resultGo)
		}

		promptTokens, _ := usage["prompt_tokens"].(int64)
		completionTokens, _ := usage["completion_tokens"].(int64)
		totalTokens, _ := usage["total_tokens"].(int64)

		if promptTokens == 0 {
			t.Error("prompt_tokens should be > 0")
		}
		if completionTokens == 0 {
			t.Error("completion_tokens should be > 0")
		}
		if totalTokens != promptTokens+completionTokens {
			t.Errorf("total_tokens (%d) != prompt (%d) + completion (%d)",
				totalTokens, promptTokens, completionTokens)
		}
	})

	t.Run("list of message maps request", func(t *testing.T) {
		request := conversion.FromGo([]any{
			map[string]any{"role": "system", "content": "You are helpful."},
			map[string]any{"role": "user", "content": "Hello!"},
		})
		response := conversion.FromGo(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hi there!",
					},
				},
			},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		resultGo := conversion.ToGo(result)
		usage, ok := resultGo.(map[string]any)
		if !ok {
			t.Fatalf("result is %T, want map[string]any", resultGo)
		}

		promptTokens, _ := usage["prompt_tokens"].(int64)
		if promptTokens == 0 {
			t.Error("prompt_tokens should be > 0 for messages")
		}
	})

	t.Run("request dict with messages key", func(t *testing.T) {
		request := conversion.FromGo(map[string]any{
			"messages": []any{
				map[string]any{"role": "user", "content": "What is AI?"},
			},
		})
		response := conversion.FromGo(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "AI is a field of computer science.",
					},
				},
			},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		resultGo := conversion.ToGo(result)
		usage, ok := resultGo.(map[string]any)
		if !ok {
			t.Fatalf("result is %T, want map[string]any", resultGo)
		}

		promptTokens, _ := usage["prompt_tokens"].(int64)
		if promptTokens == 0 {
			t.Error("prompt_tokens should be > 0 for request dict")
		}
	})

	t.Run("response with tool calls", func(t *testing.T) {
		request := conversion.FromGo("What's the weather?")
		response := conversion.FromGo(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "get_weather",
									"arguments": `{"city": "Paris"}`,
								},
							},
						},
					},
				},
			},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		resultGo := conversion.ToGo(result)
		usage, ok := resultGo.(map[string]any)
		if !ok {
			t.Fatalf("result is %T, want map[string]any", resultGo)
		}

		completionTokens, _ := usage["completion_tokens"].(int64)
		if completionTokens == 0 {
			t.Error("completion_tokens should be > 0 with tool calls")
		}
	})

	t.Run("responses API format", func(t *testing.T) {
		request := conversion.FromGo("Explain AI")
		response := conversion.FromGo(map[string]any{
			"output": []any{
				map[string]any{
					"type": "message",
					"content": []any{
						map[string]any{
							"type": "output_text",
							"text": "AI stands for artificial intelligence.",
						},
					},
				},
			},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		resultGo := conversion.ToGo(result)
		usage, ok := resultGo.(map[string]any)
		if !ok {
			t.Fatalf("result is %T, want map[string]any", resultGo)
		}

		completionTokens, _ := usage["completion_tokens"].(int64)
		if completionTokens == 0 {
			t.Error("completion_tokens should be > 0 for Responses API format")
		}
	})

	t.Run("empty response", func(t *testing.T) {
		request := conversion.FromGo("Hello")
		response := conversion.FromGo(map[string]any{})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		resultGo := conversion.ToGo(result)
		usage, ok := resultGo.(map[string]any)
		if !ok {
			t.Fatalf("result is %T, want map[string]any", resultGo)
		}

		promptTokens, _ := usage["prompt_tokens"].(int64)
		completionTokens, _ := usage["completion_tokens"].(int64)
		if promptTokens == 0 {
			t.Error("prompt_tokens should be > 0 even with empty response")
		}
		if completionTokens != 0 {
			t.Error("completion_tokens should be 0 for empty response")
		}
	})

	t.Run("consistency - same input gives same output", func(t *testing.T) {
		request := conversion.FromGo("Hello world, this is a test of token estimation.")
		response := conversion.FromGo(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "This is the response from the AI model.",
					},
				},
			},
		})

		result1 := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)
		result2 := fn.Fn(context.Background(), object.NewKwargs(nil), request, response)

		usage1 := conversion.ToGo(result1).(map[string]any)
		usage2 := conversion.ToGo(result2).(map[string]any)

		for _, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
			if usage1[key] != usage2[key] {
				t.Errorf("inconsistent results for %s: %v vs %v", key, usage1[key], usage2[key])
			}
		}
	})
}
