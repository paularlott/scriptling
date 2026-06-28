package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
			instance: object.NewInstanceWithFields(GetOpenAIClientClass(), nil),
			wantError: "missing internal client reference",
		},
		{
			name: "nil client",
			instance: object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
					"_client": &object.ClientWrapper{Client: nil},
				}),
			wantError: "client is nil",
		},
		{
			name: "invalid client type",
			instance: object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
					"_client": &object.ClientWrapper{Client: "not a ClientInstance"},
				}),
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
			instance: object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
					"_client": &object.ClientWrapper{Client: nil},
				}),
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
	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

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

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

	result := modelsMethod(instance, ctx)
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
	}
}

// Test response methods error paths
func TestResponseMethodsErrors(t *testing.T) {
	ctx := context.Background()

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

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
			instance: object.NewInstanceWithFields(GetChatStreamClass(), nil),
			wantError: "missing internal stream reference",
		},
		{
			name: "nil stream",
			instance: object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
					"_stream": &object.ClientWrapper{Client: nil},
				}),
			wantError: "stream is nil",
		},
		{
			name: "invalid stream type",
			instance: object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
					"_stream": &object.ClientWrapper{Client: "not a ChatStreamInstance"},
				}),
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

	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{stream: nil},
			},
		})

	result := nextStreamMethod(instance, ctx)
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
	}
}

// Test completionStreamMethod message validation
func TestCompletionStreamMethodMessageValidation(t *testing.T) {
	ctx := context.Background()

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

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

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{Client: nil},
		})

	result := completionStreamMethod(instance, ctx, object.Kwargs{}, "gpt-4", []map[string]any{{"role": "user", "content": "Hello"}})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
	}
}

func TestCompletionMethodTimeoutKwarg(t *testing.T) {
	ctx := context.Background()
	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: timeoutMockClient{}},
			},
		})

	kwargs := object.NewKwargs(map[string]object.Object{
		"timeout": object.NewInteger(10),
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

	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{
					stream: openaiapi.NewChatStream(streamCtx, responseChan, errorChan),
					cancel: cancel,
				},
			},
		})

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

	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{
					stream: openaiapi.NewChatStream(streamCtx, responseChan, errorChan),
					cancel: cancel,
				},
			},
		})

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
	if errStr.StringValue() != context.Canceled.Error() {
		t.Fatalf("expected %q, got %q", context.Canceled.Error(), errStr.StringValue())
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

	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				Client: &ChatStreamInstance{
					stream: openaiapi.NewChatStream(streamCtx, responseChan, errorChan),
					cancel: cancel,
				},
			},
		})

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
	if str.StringValue() != "hello from tool test" {
		t.Fatalf("expected %q, got %q", "hello from tool test", str.StringValue())
	}
}

func TestClientCustomHeadersSentWithCompletion(t *testing.T) {
	var gotHeader string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotHeader = r.Header.Get("X-Scriptling-Test")
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		if err := json.Unmarshal(bodyBytes, &gotBody); err != nil {
			t.Fatalf("failed decoding request body: %v\n%s", err, string(bodyBytes))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "test-model",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": "ok",
				},
			}},
		})
	}))
	defer server.Close()

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	Register(p)
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	result, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1", headers={"X-Scriptling-Test": "custom-value"})
response = client.completion("test-model", "hello", extra_body={
    "thinking": {"type": "enabled", "clear_thinking": False}
})
response.choices[0].message.content
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	if str.StringValue() != "ok" {
		t.Fatalf("expected response content ok, got %q", str.StringValue())
	}
	if gotHeader != "custom-value" {
		t.Fatalf("expected custom header, got %q", gotHeader)
	}
	if _, ok := gotBody["extra_body"]; ok {
		t.Fatalf("extra_body should not be sent literally: %#v", gotBody)
	}
	thinking, ok := gotBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("expected merged thinking body, got %#v", gotBody)
	}
	if thinking["type"] != "enabled" || thinking["clear_thinking"] != false {
		t.Fatalf("unexpected thinking body: %#v", thinking)
	}
}

func TestResponseCreateExtraBodyMerged(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed reading request body: %v", err)
		}
		if err := json.Unmarshal(bodyBytes, &gotBody); err != nil {
			t.Fatalf("failed decoding request body: %v\n%s", err, string(bodyBytes))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1,
			"model":   "test-model",
			"choices": []map[string]any{{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": "ok",
				},
			}},
		})
	}))
	defer server.Close()

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	Register(p)
	if err := p.SetVar("server_url", server.URL); err != nil {
		t.Fatalf("SetVar(server_url): %v", err)
	}

	result, err := p.Eval(`
import scriptling.ai as ai

client = ai.Client(server_url + "/v1", provider=ai.OPENAI)
response = client.response_create("test-model", "hello", extra_body={
    "thinking": {"type": "enabled", "clear_thinking": False}
})
response.output[0].content[0].text
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String, got %T", result)
	}
	if str.StringValue() != "ok" {
		t.Fatalf("expected response text ok, got %q", str.StringValue())
	}
	if _, ok := gotBody["extra_body"]; ok {
		t.Fatalf("extra_body should not be sent literally: %#v", gotBody)
	}
	thinking, ok := gotBody["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("expected merged thinking body, got %#v", gotBody)
	}
	if thinking["type"] != "enabled" || thinking["clear_thinking"] != false {
		t.Fatalf("unexpected thinking body: %#v", thinking)
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

func TestNormalizeToolArguments(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{"nil returns empty map", nil, map[string]any{}},
		{"empty string returns empty map", "", map[string]any{}},
		{"whitespace string returns empty map", "  ", map[string]any{}},
		{"valid JSON string parses to map", `{"key":"value"}`, map[string]any{"key": "value"}},
		{"invalid JSON string returns raw string", "not json", "not json"},
		{"map passes through", map[string]any{"a": 1}, map[string]any{"a": 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeToolArguments(tt.input)
			if !deepEqualAny(result, tt.expected) {
				t.Fatalf("normalizeToolArguments(%#v) = %#v, want %#v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected []map[string]any
	}{
		{
			"nil returns empty",
			nil,
			[]map[string]any{},
		},
		{
			"standard tool call with JSON string arguments",
			[]any{
				map[string]any{
					"id":   "call_abc",
					"type": "function",
					"function": map[string]any{
						"name":      "read_file",
						"arguments": `{"path":"/test.txt"}`,
					},
				},
			},
			[]map[string]any{
				{
					"id":   "call_abc",
					"type": "function",
					"function": map[string]any{
						"name":      "read_file",
						"arguments": map[string]any{"path": "/test.txt"},
					},
				},
			},
		},
		{
			"tool call with dict arguments passes through",
			[]any{
				map[string]any{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "greet",
						"arguments": map[string]any{"name": "Paul"},
					},
				},
			},
			[]map[string]any{
				{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "greet",
						"arguments": map[string]any{"name": "Paul"},
					},
				},
			},
		},
		{
			"nil arguments becomes empty map",
			[]any{
				map[string]any{
					"id":   "call_2",
					"type": "function",
					"function": map[string]any{
						"name":      "get_time",
						"arguments": nil,
					},
				},
			},
			[]map[string]any{
				{
					"id":   "call_2",
					"type": "function",
					"function": map[string]any{
						"name":      "get_time",
						"arguments": map[string]any{},
					},
				},
			},
		},
		{
			"missing id defaults to empty string",
			[]any{
				map[string]any{
					"type": "function",
					"function": map[string]any{
						"name":      "tool",
						"arguments": "{}",
					},
				},
			},
			[]map[string]any{
				{
					"id":   "",
					"type": "function",
					"function": map[string]any{
						"name":      "tool",
						"arguments": map[string]any{},
					},
				},
			},
		},
		{
			"missing type defaults to function",
			[]any{
				map[string]any{
					"id": "call_3",
					"function": map[string]any{
						"name":      "tool",
						"arguments": "{}",
					},
				},
			},
			[]map[string]any{
				{
					"id":   "call_3",
					"type": "function",
					"function": map[string]any{
						"name":      "tool",
						"arguments": map[string]any{},
					},
				},
			},
		},
		{
			"top-level name fallback when function.name missing",
			[]any{
				map[string]any{
					"id":   "call_4",
					"type": "function",
					"name": "fallback_tool",
					"function": map[string]any{
						"arguments": "{}",
					},
				},
			},
			[]map[string]any{
				{
					"id":   "call_4",
					"type": "function",
					"function": map[string]any{
						"name":      "fallback_tool",
						"arguments": map[string]any{},
					},
				},
			},
		},
		{
			"top-level arguments fallback when function.arguments missing",
			[]any{
				map[string]any{
					"id":   "call_5",
					"type": "function",
					"function": map[string]any{
						"name": "tool",
					},
					"arguments": `{"key":"val"}`,
				},
			},
			[]map[string]any{
				{
					"id":   "call_5",
					"type": "function",
					"function": map[string]any{
						"name":      "tool",
						"arguments": map[string]any{"key": "val"},
					},
				},
			},
		},
		{
			"index field preserved",
			[]any{
				map[string]any{
					"id":    "call_6",
					"type":  "function",
					"index": int64(0),
					"function": map[string]any{
						"name":      "tool",
						"arguments": "{}",
					},
				},
			},
			[]map[string]any{
				{
					"id":    "call_6",
					"type":  "function",
					"index": int64(0),
					"function": map[string]any{
						"name":      "tool",
						"arguments": map[string]any{},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeToolCalls(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("normalizeToolCalls returned %d items, want %d", len(result), len(tt.expected))
			}
			for i, got := range result {
				want := tt.expected[i]
				if !deepEqualAny(got, want) {
					t.Fatalf("item %d:\ngot:  %#v\nwant: %#v", i, got, want)
				}
			}
		})
	}
}

func TestExtractToolCallsFromGoFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"nil input returns empty", nil, 0},
		{"full response with choices[0].message.tool_calls", map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "test",
									"arguments": "{}",
								},
							},
						},
					},
				},
			},
		}, 1},
		{"message dict with tool_calls key", map[string]any{
			"tool_calls": []any{
				map[string]any{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "test",
						"arguments": "{}",
					},
				},
			},
		}, 1},
		{"delta dict with tool_calls", map[string]any{
			"choices": []any{
				map[string]any{
					"delta": map[string]any{
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "test",
									"arguments": "{}",
								},
							},
						},
					},
				},
			},
		}, 1},
		{"raw list of tool calls", []any{
			map[string]any{
				"id":   "call_1",
				"type": "function",
				"function": map[string]any{
					"name":      "test",
					"arguments": "{}",
				},
			},
		}, 1},
		{"[]map[string]any input", []map[string]any{
			{
				"id":   "call_1",
				"type": "function",
				"function": map[string]any{
					"name":      "test",
					"arguments": "{}",
				},
			},
		}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractToolCallsFromGo(tt.input)
			if len(result) != tt.expected {
				t.Fatalf("extractToolCallsFromGo returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestExecuteToolCallsViaScript(t *testing.T) {
	tests := []struct {
		name        string
		script      string
		expectError bool
		check       func(t *testing.T, result string)
	}{
		{
			"plain tool name",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {"msg": "string"}, lambda args: "got:" + args["msg"])
tool_calls = [{"id": "c1", "type": "function", "function": {"name": "echo", "arguments": {"msg": "hi"}}}]
results = ai.execute_tool_calls(tools, tool_calls)
results[0]["content"]
`,
			false,
			func(t *testing.T, result string) {
				if result != "got:hi" {
					t.Fatalf("expected 'got:hi', got %q", result)
				}
			},
		},
		{
			"{namespace:name} wrapper stripped",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {"msg": "string"}, lambda args: "ok")
tool_calls = [{"id": "c1", "type": "function", "function": {"name": "{mcp:echo}", "arguments": {}}}]
results = ai.execute_tool_calls(tools, tool_calls)
results[0]["content"]
`,
			false,
			func(t *testing.T, result string) {
				if result != "ok" {
					t.Fatalf("expected 'ok', got %q", result)
				}
			},
		},
		{
			"function_name_ prefix stripped",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {"msg": "string"}, lambda args: "ok")
tool_calls = [{"id": "c1", "type": "function", "function": {"name": "function_name_echo", "arguments": {}}}]
results = ai.execute_tool_calls(tools, tool_calls)
results[0]["content"]
`,
			false,
			func(t *testing.T, result string) {
				if result != "ok" {
					t.Fatalf("expected 'ok', got %q", result)
				}
			},
		},
		{
			"{key} wrappers on argument keys stripped",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {"msg": "string"}, lambda args: "got:" + args["msg"])
tool_calls = [{"id": "c1", "type": "function", "function": {"name": "echo", "arguments": {"{msg}": "hello"}}}]
results = ai.execute_tool_calls(tools, tool_calls)
results[0]["content"]
`,
			false,
			func(t *testing.T, result string) {
				if result != "got:hello" {
					t.Fatalf("expected 'got:hello', got %q", result)
				}
			},
		},
		{
			"unknown tool returns error",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {"msg": "string"}, lambda args: "ok")
tool_calls = [{"id": "c1", "type": "function", "function": {"name": "unknown", "arguments": {}}}]
results = ai.execute_tool_calls(tools, tool_calls)
results[0]["content"]
`,
			true,
			nil,
		},
		{
			"tool_call_id preserved in result",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {}, lambda args: "ok")
tool_calls = [{"id": "call_xyz", "type": "function", "function": {"name": "echo", "arguments": {}}}]
results = ai.execute_tool_calls(tools, tool_calls)
results[0]["tool_call_id"]
`,
			false,
			func(t *testing.T, result string) {
				if result != "call_xyz" {
					t.Fatalf("expected 'call_xyz', got %q", result)
				}
			},
		},
		{
			"ai.tool_calls extracts from response then executes",
			`
import scriptling.ai as ai
tools = ai.ToolRegistry()
tools.add("echo", "Echo", {"msg": "string"}, lambda args: "got:" + args["msg"])
response = {"choices": [{"message": {"tool_calls": [{"id": "c1", "type": "function", "function": {"name": "echo", "arguments": "{\"msg\": \"world\"}"}}], "content": ""}, "finish_reason": "tool_calls"}]}
calls = ai.tool_calls(response)
results = ai.execute_tool_calls(tools, calls)
results[0]["content"]
`,
			false,
			func(t *testing.T, result string) {
				if result != "got:world" {
					t.Fatalf("expected 'got:world', got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := scriptlib.New()
			stdlib.RegisterAll(p)
			Register(p)

			result, err := p.Eval(tt.script)
			if tt.expectError {
				if err == nil {
					if _, ok := result.(*object.Error); !ok {
						t.Fatal("expected error, got success")
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			if errObj, ok := result.(*object.Error); ok {
				t.Fatalf("Eval returned error: %s", errObj.Message)
			}
			str, ok := result.(*object.String)
			if !ok {
				t.Fatalf("expected String, got %T: %#v", result, result)
			}
			tt.check(t, str.StringValue())
		})
	}
}

func deepEqualAny(a, b any) bool {
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

func TestCollectStreamAggregatesToolCalls(t *testing.T) {
	stream := toolStreamMockClient{}.StreamChatCompletion(context.Background(), mcpai.ChatCompletionRequest{})
	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client: &ChatStreamInstance{
					stream: stream,
				},
			},
		})

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
	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client: &ChatStreamInstance{
					stream: stream,
				},
			},
		})

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

// Test createClientInstance creates a valid instance
func TestCreateClientInstance(t *testing.T) {
	instance := createClientInstance(nil)

	if instance == nil {
		t.Fatal("createClientInstance() returned nil")
	}

	if instance.Class != GetOpenAIClientClass() {
		t.Errorf("instance.Class = %v, want %v", instance.Class, GetOpenAIClientClass())
	}

	if instance.FieldCount() != 1 {
		t.Errorf("instance.Fields length = %d, want 1", instance.FieldCount())
	}

	clientWrapper, ok := instance.Field("_client").(*object.ClientWrapper)
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
	expectedFuncs := []string{"Client", "extract_thinking", "text", "thinking", "tool_calls", "execute_tool_calls", "collect_stream", "estimate_tokens"}
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
	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				TypeName: "OpenAIClient",
				Client:   &ClientInstance{client: nil},
			},
		})

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
	instance := object.NewInstanceWithFields(GetChatStreamClass(), map[string]object.Object{
			"_stream": &object.ClientWrapper{
				TypeName: "ChatStream",
				Client:   &ChatStreamInstance{stream: nil},
			},
		})

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

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

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
				"system_prompt": object.NewString("You are a helpful math tutor"),
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
				"system_prompt": object.NewString("You are helpful"),
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

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

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
				"system_prompt": object.NewString("You are a physics professor"),
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
				"system_prompt": object.NewString("You are helpful"),
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

	instance := object.NewInstanceWithFields(GetOpenAIClientClass(), map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		})

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
				"system_prompt": object.NewString("You are a helpful assistant"),
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
				"system_prompt": object.NewString("You are helpful"),
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

	t.Run("request only with omitted response", func(t *testing.T) {
		request := conversion.FromGo([]any{
			map[string]any{"role": "user", "content": "Estimate this request before sending."},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		usage := conversion.ToGo(result).(map[string]any)
		promptTokens, _ := usage["prompt_tokens"].(int64)
		completionTokens, _ := usage["completion_tokens"].(int64)
		totalTokens, _ := usage["total_tokens"].(int64)
		if promptTokens == 0 {
			t.Error("prompt_tokens should be > 0 for request-only estimate")
		}
		if completionTokens != 0 {
			t.Errorf("completion_tokens = %d, want 0", completionTokens)
		}
		if totalTokens != promptTokens {
			t.Errorf("total_tokens = %d, want prompt_tokens %d", totalTokens, promptTokens)
		}
	})

	t.Run("request only with None response", func(t *testing.T) {
		request := conversion.FromGo([]any{
			map[string]any{"role": "user", "content": "Estimate this request before sending."},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), request, &object.Null{})
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		usage := conversion.ToGo(result).(map[string]any)
		promptTokens, _ := usage["prompt_tokens"].(int64)
		completionTokens, _ := usage["completion_tokens"].(int64)
		if promptTokens == 0 {
			t.Error("prompt_tokens should be > 0 for request-only estimate")
		}
		if completionTokens != 0 {
			t.Errorf("completion_tokens = %d, want 0", completionTokens)
		}
	})

	t.Run("response only with None request", func(t *testing.T) {
		response := conversion.FromGo(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": "Only count this response.",
					},
				},
			},
		})

		result := fn.Fn(context.Background(), object.NewKwargs(nil), &object.Null{}, response)
		if errObj, ok := result.(*object.Error); ok {
			t.Fatalf("unexpected error: %s", errObj.Message)
		}

		usage := conversion.ToGo(result).(map[string]any)
		promptTokens, _ := usage["prompt_tokens"].(int64)
		completionTokens, _ := usage["completion_tokens"].(int64)
		totalTokens, _ := usage["total_tokens"].(int64)
		if promptTokens != 0 {
			t.Errorf("prompt_tokens = %d, want 0", promptTokens)
		}
		if completionTokens == 0 {
			t.Error("completion_tokens should be > 0 for response-only estimate")
		}
		if totalTokens != completionTokens {
			t.Errorf("total_tokens = %d, want completion_tokens %d", totalTokens, completionTokens)
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

// ---------------------------------------------------------------------------
// Parallel helpers
// ---------------------------------------------------------------------------

// echoMockClient returns a completion whose content is the user message text.
type echoMockClient struct{}

func (echoMockClient) Provider() string                                         { return "mock" }
func (echoMockClient) SupportsCapability(string) bool                           { return false }
func (echoMockClient) GetModels(context.Context) (*mcpai.ModelsResponse, error) { return nil, nil }
func (echoMockClient) ChatCompletion(_ context.Context, req mcpai.ChatCompletionRequest) (*mcpai.ChatCompletionResponse, error) {
	content := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if s, ok := msg.Content.(string); ok {
				content = s
			}
		}
	}
	return &mcpai.ChatCompletionResponse{
		Choices: []openaiapi.Choice{
			{Message: openaiapi.Message{Role: "assistant", Content: content}},
		},
	}, nil
}
func (echoMockClient) StreamChatCompletion(ctx context.Context, _ mcpai.ChatCompletionRequest) *mcpai.ChatStream {
	ch := make(chan openaiapi.ChatCompletionResponse)
	ec := make(chan error)
	close(ch)
	close(ec)
	return openaiapi.NewChatStream(ctx, ch, ec)
}
func (echoMockClient) CreateEmbedding(context.Context, mcpai.EmbeddingRequest) (*mcpai.EmbeddingResponse, error) {
	return nil, nil
}
func (echoMockClient) CreateResponse(context.Context, mcpai.CreateResponseRequest) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (echoMockClient) StreamResponse(context.Context, mcpai.CreateResponseRequest) *mcpai.ResponseStream {
	return nil
}
func (echoMockClient) GetResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (echoMockClient) CancelResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (echoMockClient) DeleteResponse(context.Context, string) error { return nil }
func (echoMockClient) CompactResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (echoMockClient) Close() error { return nil }

// rateLimitMockClient returns a rate-limit retry response for the first call,
// then a normal echo response for all subsequent calls.
type rateLimitMockClient struct {
	called int32
}

func (r *rateLimitMockClient) Provider() string                                         { return "mock" }
func (r *rateLimitMockClient) SupportsCapability(string) bool                           { return false }
func (r *rateLimitMockClient) GetModels(context.Context) (*mcpai.ModelsResponse, error) { return nil, nil }
func (r *rateLimitMockClient) ChatCompletion(_ context.Context, req mcpai.ChatCompletionRequest) (*mcpai.ChatCompletionResponse, error) {
	content := ""
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			if s, ok := msg.Content.(string); ok {
				content = s
			}
		}
	}
	resp := &mcpai.ChatCompletionResponse{
		Choices: []openaiapi.Choice{
			{Message: openaiapi.Message{Role: "assistant", Content: content}},
		},
	}
	// First call signals a rate limit so adaptive backoff kicks in.
	if atomic.AddInt32(&r.called, 1) == 1 {
		resp.Retry = &openaiapi.RetryMetadata{
			Attempts:     1,
			RateLimitHit: true,
			TotalBackoff: 0,
		}
	}
	return resp, nil
}
func (r *rateLimitMockClient) StreamChatCompletion(ctx context.Context, _ mcpai.ChatCompletionRequest) *mcpai.ChatStream {
	ch := make(chan openaiapi.ChatCompletionResponse)
	ec := make(chan error)
	close(ch)
	close(ec)
	return openaiapi.NewChatStream(ctx, ch, ec)
}
func (r *rateLimitMockClient) CreateEmbedding(context.Context, mcpai.EmbeddingRequest) (*mcpai.EmbeddingResponse, error) {
	return nil, nil
}
func (r *rateLimitMockClient) CreateResponse(context.Context, mcpai.CreateResponseRequest) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (r *rateLimitMockClient) StreamResponse(context.Context, mcpai.CreateResponseRequest) *mcpai.ResponseStream {
	return nil
}
func (r *rateLimitMockClient) GetResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (r *rateLimitMockClient) CancelResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (r *rateLimitMockClient) DeleteResponse(context.Context, string) error { return nil }
func (r *rateLimitMockClient) CompactResponse(context.Context, string) (*mcpai.ResponseObject, error) {
	return nil, nil
}
func (r *rateLimitMockClient) Close() error { return nil }

func newParallelInstance(client mcpai.Client) *object.Instance {
	return createClientInstance(client)
}

// ---------------------------------------------------------------------------
// completion_parallel tests
// ---------------------------------------------------------------------------

func TestCompletionParallelEmpty(t *testing.T) {
	inst := newParallelInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(3)})

	result := completionParallelMethod(inst, ctx, kwargs, "gpt-4", &object.List{Elements: []object.Object{}})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 0 {
		t.Errorf("expected empty list, got %d elements", len(list.Elements))
	}
}

func TestCompletionParallelInvalidInput(t *testing.T) {
	inst := newParallelInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := completionParallelMethod(inst, ctx, kwargs, "gpt-4", 42)
	if result.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error, got %T", result)
	}
	if !contains(result.(*object.Error).Message, "completion_parallel") {
		t.Errorf("error should mention completion_parallel, got: %s", result.(*object.Error).Message)
	}
}

func TestCompletionParallelPreservesOrder(t *testing.T) {
	inst := newParallelInstance(echoMockClient{})
	ctx := context.Background()
	questions := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	items := make([]object.Object, len(questions))
	for i, q := range questions {
		items[i] = object.NewString(q)
	}
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(3)})

	result := completionParallelMethod(inst, ctx, kwargs, "gpt-4",
		&object.List{Elements: items})

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != len(questions) {
		t.Fatalf("expected %d results, got %d", len(questions), len(list.Elements))
	}
	for i, q := range questions {
		respGo := conversion.ToGo(list.Elements[i])
		respMap, ok := respGo.(map[string]any)
		if !ok {
			t.Errorf("[%d] result is not a map", i)
			continue
		}
		choices, _ := respMap["choices"].([]any)
		if len(choices) == 0 {
			t.Errorf("[%d] no choices", i)
			continue
		}
		msg, _ := choices[0].(map[string]any)["message"].(map[string]any)
		if msg["content"] != q {
			t.Errorf("[%d] expected content %q, got %q", i, q, msg["content"])
		}
	}
}

func TestCompletionParallelContextCancelled(t *testing.T) {
	// Use a mock that blocks until the context is done so that the cancellation
	// path in acquireSlot is actually exercised.
	inst := newParallelInstance(timeoutMockClient{})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	items := []object.Object{
		object.NewString("q1"),
		object.NewString("q2"),
		object.NewString("q3"),
	}
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(1)})

	// Should not hang; returns when context is cancelled.
	result := completionParallelMethod(inst, ctx, kwargs, "gpt-4",
		&object.List{Elements: items})

	if result.Type() == object.ERROR_OBJ {
		// Top-level error is fine (e.g. from the first item timing out)
		return
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	// All results should be present (some may be errors from cancellation).
	if len(list.Elements) != len(items) {
		t.Errorf("expected %d results, got %d", len(items), len(list.Elements))
	}
}

func TestCompletionParallelRateLimitAdaptive(t *testing.T) {
	// rateLimitMockClient reports a rate-limit on the first call.
	// runParallel should halve the slot count and apply a backoff, but still
	// complete all items successfully.
	rl := &rateLimitMockClient{}
	inst := newParallelInstance(rl)
	ctx := context.Background()

	n := 4
	items := make([]object.Object, n)
	for i := range items {
		items[i] = object.NewString(fmt.Sprintf("item%d", i))
	}
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(2)})

	result := completionParallelMethod(inst, ctx, kwargs, "gpt-4",
		&object.List{Elements: items})

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != n {
		t.Fatalf("expected %d results, got %d", n, len(list.Elements))
	}
	// All results should be non-nil and non-error.
	for i, elem := range list.Elements {
		if elem == nil || elem.Type() == object.ERROR_OBJ {
			t.Errorf("[%d] unexpected nil or error result", i)
		}
	}
}

// ---------------------------------------------------------------------------
// ask_parallel tests
// ---------------------------------------------------------------------------

func TestAskParallelEmpty(t *testing.T) {
	inst := newParallelInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	result := askParallelMethod(inst, ctx, kwargs, "gpt-4", &object.List{Elements: []object.Object{}})
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 0 {
		t.Errorf("expected empty list, got %d elements", len(list.Elements))
	}
}

func TestAskParallelReturnsTextNotMap(t *testing.T) {
	inst := newParallelInstance(echoMockClient{})
	ctx := context.Background()
	questions := []string{"hello", "world"}

	items := make([]object.Object, len(questions))
	for i, q := range questions {
		items[i] = object.NewString(q)
	}
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(2)})

	result := askParallelMethod(inst, ctx, kwargs, "gpt-4",
		&object.List{Elements: items})

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != len(questions) {
		t.Fatalf("expected %d results, got %d", len(questions), len(list.Elements))
	}
	for i, q := range questions {
		str, ok := list.Elements[i].(*object.String)
		if !ok {
			t.Errorf("[%d] expected *object.String, got %T", i, list.Elements[i])
			continue
		}
		if str.Inspect() != q {
			t.Errorf("[%d] expected %q, got %q", i, q, str.Inspect())
		}
	}
}

func TestAskParallelPreservesOrderUnderConcurrency(t *testing.T) {
	inst := newParallelInstance(echoMockClient{})
	ctx := context.Background()

	n := 20
	items := make([]object.Object, n)
	expected := make([]string, n)
	for i := range items {
		expected[i] = fmt.Sprintf("question-%d", i)
		items[i] = object.NewString(expected[i])
	}
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(10)})

	result := askParallelMethod(inst, ctx, kwargs, "gpt-4",
		&object.List{Elements: items})

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	for i, want := range expected {
		str, ok := list.Elements[i].(*object.String)
		if !ok {
			t.Errorf("[%d] expected *object.String, got %T", i, list.Elements[i])
			continue
		}
		if str.Inspect() != want {
			t.Errorf("[%d] expected %q, got %q", i, want, str.Inspect())
		}
	}
}

func TestAskParallelAdaptiveBackoffPreservesRetryMetadata(t *testing.T) {
	// ask_parallel must surface rate-limit retry info to runParallel so that
	// adaptive backoff can fire. Verify all items complete (not that the limit
	// was halved, which is an internal detail) and results are strings.
	rl := &rateLimitMockClient{}
	inst := newParallelInstance(rl)
	ctx := context.Background()

	n := 4
	items := make([]object.Object, n)
	for i := range items {
		items[i] = object.NewString(fmt.Sprintf("q%d", i))
	}
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(2)})

	result := askParallelMethod(inst, ctx, kwargs, "gpt-4",
		&object.List{Elements: items})

	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != n {
		t.Fatalf("expected %d results, got %d", n, len(list.Elements))
	}
	for i, elem := range list.Elements {
		if _, ok := elem.(*object.String); !ok {
			t.Errorf("[%d] expected *object.String (text), got %T", i, elem)
		}
	}
}

// ---------------------------------------------------------------------------
// Pipeline tests
// ---------------------------------------------------------------------------

func newPipelineClientInstance(client mcpai.Client) *object.Instance {
	return createClientInstance(client)
}

func TestPipelineCompleteEmpty(t *testing.T) {
	inst := newPipelineClientInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(2)})

	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4")
	if pipe.Type() == object.ERROR_OBJ {
		t.Fatalf("pipelineMethod returned error: %v", pipe.(*object.Error).Message)
	}

	result := completeMethod(pipe.(*object.Instance), ctx)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 0 {
		t.Errorf("expected empty list, got %d elements", len(list.Elements))
	}
}

func TestPipelineCompleteTwiceErrors(t *testing.T) {
	inst := newPipelineClientInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4").(*object.Instance)
	completeMethod(pipe, ctx) // first call
	result := completeMethod(pipe, ctx)
	if result.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error on second complete(), got %T", result)
	}
}

func TestPipelineAddAfterCompleteErrors(t *testing.T) {
	inst := newPipelineClientInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4").(*object.Instance)
	completeMethod(pipe, ctx)
	result := addMethod(pipe, ctx, "late message")
	if result.Type() != object.ERROR_OBJ {
		t.Fatalf("expected error on add() after complete(), got %T", result)
	}
}

func TestPipelinePreservesOrder(t *testing.T) {
	inst := newPipelineClientInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(4)})

	questions := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4").(*object.Instance)
	for _, q := range questions {
		if r := addMethod(pipe, ctx, q); r.Type() == object.ERROR_OBJ {
			t.Fatalf("add() returned error: %v", r.(*object.Error).Message)
		}
	}
	result := completeMethod(pipe, ctx)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != len(questions) {
		t.Fatalf("expected %d results, got %d", len(questions), len(list.Elements))
	}
	for i, q := range questions {
		respGo := conversion.ToGo(list.Elements[i])
		respMap, ok := respGo.(map[string]any)
		if !ok {
			t.Errorf("[%d] result is not a map", i)
			continue
		}
		choices, _ := respMap["choices"].([]any)
		if len(choices) == 0 {
			t.Errorf("[%d] no choices", i)
			continue
		}
		msg, _ := choices[0].(map[string]any)["message"].(map[string]any)
		if msg["content"] != q {
			t.Errorf("[%d] expected %q, got %q", i, q, msg["content"])
		}
	}
}

func TestPipelineAskModeReturnsStrings(t *testing.T) {
	inst := newPipelineClientInstance(echoMockClient{})
	ctx := context.Background()
	kwargs := object.NewKwargs(map[string]object.Object{
		"max_parallel": object.NewInteger(3),
		"ask":          object.NewBoolean(true),
	})

	questions := []string{"hello", "world", "foo"}
	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4").(*object.Instance)
	for _, q := range questions {
		addMethod(pipe, ctx, q)
	}
	result := completeMethod(pipe, ctx)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	for i, q := range questions {
		s, ok := list.Elements[i].(*object.String)
		if !ok {
			t.Errorf("[%d] expected *object.String, got %T", i, list.Elements[i])
			continue
		}
		if s.Inspect() != q {
			t.Errorf("[%d] expected %q, got %q", i, q, s.Inspect())
		}
	}
}

func TestPipelineContextCancellation(t *testing.T) {
	inst := newPipelineClientInstance(timeoutMockClient{})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(1)})
	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4").(*object.Instance)
	for i := 0; i < 3; i++ {
		addMethod(pipe, ctx, fmt.Sprintf("q%d", i))
	}

	result := completeMethod(pipe, ctx)
	if result.Type() == object.ERROR_OBJ {
		return // top-level error is acceptable
	}
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 3 {
		t.Errorf("expected 3 results, got %d", len(list.Elements))
	}
}

func TestPipelineRateLimitAdaptive(t *testing.T) {
	rl := &rateLimitMockClient{}
	inst := newPipelineClientInstance(rl)
	ctx := context.Background()
	kwargs := object.NewKwargs(map[string]object.Object{"max_parallel": object.NewInteger(2)})

	pipe := pipelineMethod(inst, ctx, kwargs, "gpt-4").(*object.Instance)
	for i := 0; i < 4; i++ {
		addMethod(pipe, ctx, fmt.Sprintf("item%d", i))
	}
	result := completeMethod(pipe, ctx)
	list, ok := result.(*object.List)
	if !ok {
		t.Fatalf("expected List, got %T", result)
	}
	if len(list.Elements) != 4 {
		t.Fatalf("expected 4 results, got %d", len(list.Elements))
	}
	for i, elem := range list.Elements {
		if elem == nil || elem.Type() == object.ERROR_OBJ {
			t.Errorf("[%d] unexpected nil or error result", i)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	lib := buildLibrary()
	fn, ok := lib.Functions()["cosine_similarity"]
	if !ok {
		t.Fatal("cosine_similarity function not found in library")
	}

	tests := []struct {
		name      string
		a         object.Object
		b         object.Object
		want      float64
		wantError bool
		errMsg    string
	}{
		{
			name: "identical vectors",
			a:    conversion.FromGo([]any{1.0, 2.0, 3.0}),
			b:    conversion.FromGo([]any{1.0, 2.0, 3.0}),
			want: 1.0,
		},
		{
			name: "opposite vectors",
			a:    conversion.FromGo([]any{1.0, 0.0, 0.0}),
			b:    conversion.FromGo([]any{-1.0, 0.0, 0.0}),
			want: -1.0,
		},
		{
			name: "orthogonal vectors",
			a:    conversion.FromGo([]any{1.0, 0.0}),
			b:    conversion.FromGo([]any{0.0, 1.0}),
			want: 0.0,
		},
		{
			name: "similar vectors",
			a:    conversion.FromGo([]any{1.0, 2.0, 3.0}),
			b:    conversion.FromGo([]any{1.1, 2.1, 2.9}),
			want: 0.999,
		},
		{
			name: "zero vector returns zero",
			a:    conversion.FromGo([]any{0.0, 0.0, 0.0}),
			b:    conversion.FromGo([]any{1.0, 2.0, 3.0}),
			want: 0.0,
		},
		{
			name: "single element",
			a:    conversion.FromGo([]any{5.0}),
			b:    conversion.FromGo([]any{5.0}),
			want: 1.0,
		},
		{
			name: "integer lists",
			a:    conversion.FromGo([]any{1, 0, 0}),
			b:    conversion.FromGo([]any{0, 1, 0}),
			want: 0.0,
		},
		{
			name:      "mismatched lengths",
			a:         conversion.FromGo([]any{1.0, 2.0}),
			b:         conversion.FromGo([]any{1.0}),
			wantError: true,
			errMsg:    "same length",
		},
		{
			name:      "empty vectors",
			a:         conversion.FromGo([]any{}),
			b:         conversion.FromGo([]any{}),
			wantError: true,
			errMsg:    "empty",
		},
		{
			name:      "non-list argument",
			a:         object.NewString("hello"),
			b:         conversion.FromGo([]any{1.0}),
			wantError: true,
			errMsg:    "expected a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fn.Fn(context.Background(), object.NewKwargs(nil), tt.a, tt.b)

			if tt.wantError {
				if errObj, ok := result.(*object.Error); ok {
					if tt.errMsg != "" && !strings.Contains(errObj.Message, tt.errMsg) {
						t.Errorf("error message %q does not contain %q", errObj.Message, tt.errMsg)
					}
					return
				}
				t.Fatalf("expected error, got %T: %v", result, result)
			}

			if errObj, ok := result.(*object.Error); ok {
				t.Fatalf("unexpected error: %s", errObj.Message)
			}

			f, ok := result.(*object.Float)
			if !ok {
				t.Fatalf("expected Float, got %T", result)
			}
			if math.Abs(f.FloatValue()-tt.want) > 0.001 {
				t.Errorf("cosine_similarity = %f, want %f", f.FloatValue(), tt.want)
			}
		})
	}
}
