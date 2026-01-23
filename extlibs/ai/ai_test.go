package ai

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

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
		name     string
		instance *object.Instance
		model    string
		messages []map[string]any
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
			model:      "gpt-4",
			messages:   []map[string]any{{"role": "user", "content": "Hello"}},
			wantError: "client is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionMethod(tt.instance, ctx, tt.model, tt.messages)
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
		name     string
		messages []map[string]any
		wantError string
	}{
		{
			name:     "empty role",
			messages: []map[string]any{{"role": "", "content": "Hello"}},
			wantError: "role cannot be empty",
		},
		{
			name:     "missing role field",
			messages: []map[string]any{{"content": "Hello"}},
			wantError: "missing required 'role' field",
		},
		{
			name:     "non-string role",
			messages: []map[string]any{{"role": 123, "content": "Hello"}},
			wantError: "missing required 'role' field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionMethod(instance, ctx, "gpt-4", tt.messages)
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
		result := responseCreateMethod(instance, ctx, "gpt-4", []any{"test"})
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

// Test remote server methods
func TestRemoteServerMethods(t *testing.T) {
	ctx := context.Background()

	instance := &object.Instance{
		Class: GetOpenAIClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				Client: &ClientInstance{client: nil},
			},
		},
	}

	t.Run("add_remote_server with nil client", func(t *testing.T) {
		kwargs := object.NewKwargs(map[string]object.Object{
			"namespace": &object.String{Value: "test"},
			"bearer_token": &object.String{Value: "token"},
		})
		result := addRemoteServerMethod(instance, ctx, kwargs, "https://example.com")
		if result.Type() != object.ERROR_OBJ {
			t.Errorf("expected error, got %v", result.Type())
		}
	})

	t.Run("remove_remote_server with nil client", func(t *testing.T) {
		result := removeRemoteServerMethod(instance, ctx, "prefix")
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
		name     string
		messages []map[string]any
		wantError string
	}{
		{
			name:     "empty role",
			messages: []map[string]any{{"role": "", "content": "Hello"}},
			wantError: "role cannot be empty",
		},
		{
			name:     "missing role field",
			messages: []map[string]any{{"content": "Hello"}},
			wantError: "missing required 'role' field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := completionStreamMethod(instance, ctx, "gpt-4", tt.messages)
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

	result := completionStreamMethod(instance, ctx, "gpt-4", []map[string]any{{"role": "user", "content": "Hello"}})
	if result.Type() != object.ERROR_OBJ {
		t.Errorf("expected error, got %v", result.Type())
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

	// Check that expected functions exist
	expectedFuncs := []string{"completion", "models", "response_create", "response_get", "response_cancel", "new_client"}
	for _, name := range expectedFuncs {
		if _, ok := lib.Functions()[name]; !ok {
			t.Errorf("library missing function %q", name)
		}
	}
}

// mockRegistrar implements the RegisterLibrary interface
type mockRegistrar struct {
	libraryName string
	library     *object.Library
	called      bool
}

func (m *mockRegistrar) RegisterLibrary(name string, lib *object.Library) {
	m.libraryName = name
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

// Test getClient returns nil when no client is set
func TestGetClient(t *testing.T) {
	c := getClient()
	if c != nil {
		t.Errorf("getClient() = %v, want nil (no client set)", c)
	}
}

// Test OpenAIClientClass has expected methods
func TestOpenAIClientClassMethods(t *testing.T) {
	class := GetOpenAIClientClass()

	expectedMethods := []string{
		"completion", "completion_stream", "models",
		"response_create", "response_get", "response_cancel",
		"add_remote_server", "remove_remote_server",
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
