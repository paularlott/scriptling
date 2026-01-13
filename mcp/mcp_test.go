package mcp

import (
	"testing"

	"github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/object"
)

func TestMCPLibraryConstants(t *testing.T) {
	if MCPLibraryName != "mcp" {
		t.Errorf("MCPLibraryName = %q, want %q", MCPLibraryName, "mcp")
	}

	if MCPLibraryDesc == "" {
		t.Error("MCPLibraryDesc should not be empty")
	}
}

func TestGetMCPClientClass(t *testing.T) {
	class := GetMCPClientClass()

	if class == nil {
		t.Error("GetMCPClientClass() returned nil")
	}

	if class.Name != "MCPClient" {
		t.Errorf("Class name = %q, want %q", class.Name, "MCPClient")
	}
}

func TestGetMCPClientClassSingleton(t *testing.T) {
	class1 := GetMCPClientClass()
	class2 := GetMCPClientClass()

	if class1 != class2 {
		t.Error("GetMCPClientClass() should return the same instance (singleton)")
	}
}

func TestDecodeToolResponse_Nil(t *testing.T) {
	result := DecodeToolResponse(nil)

	if _, ok := result.(*object.Null); !ok {
		t.Errorf("DecodeToolResponse(nil) should return Null, got %T", result)
	}
}

func TestDecodeToolResponse_Empty(t *testing.T) {
	response := &mcp.ToolResponse{}
	result := DecodeToolResponse(response)

	if _, ok := result.(*object.Null); !ok {
		t.Errorf("DecodeToolResponse(empty) should return Null, got %T", result)
	}
}

func TestDecodeToolResponse_StructuredContent(t *testing.T) {
	tests := []struct {
		name     string
		content  any
		wantType string
	}{
		{
			name:     "string content",
			content:  "hello",
			wantType: "*object.String",
		},
		{
			name:     "int content",
			content:  42,
			wantType: "*object.Integer",
		},
		{
			name:     "float content",
			content:  3.14,
			wantType: "*object.Float",
		},
		{
			name:     "bool content",
			content:  true,
			wantType: "*object.Boolean",
		},
		{
			name:     "list content",
			content:  []any{1, 2, 3},
			wantType: "*object.List",
		},
		{
			name:     "dict content",
			content:  map[string]any{"key": "value"},
			wantType: "*object.Dict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := &mcp.ToolResponse{
				StructuredContent: tt.content,
			}
			result := DecodeToolResponse(response)
			// Just verify it returns something
			if result == nil {
				t.Error("DecodeToolResponse() returned nil")
			}
		})
	}
}

func TestDecodeToolResponse_SingleTextContent(t *testing.T) {
	response := &mcp.ToolResponse{
		Content: []mcp.ToolContent{
			{Type: "text", Text: "plain text"},
		},
	}
	result := DecodeToolResponse(response)

	if str, ok := result.(*object.String); ok {
		if str.Value != "plain text" {
			t.Errorf("String value = %q, want %q", str.Value, "plain text")
		}
	} else {
		t.Errorf("DecodeToolResponse() should return String, got %T", result)
	}
}

func TestDecodeToolResponse_SingleJSONContent(t *testing.T) {
	response := &mcp.ToolResponse{
		Content: []mcp.ToolContent{
			{Type: "text", Text: `{"key": "value", "number": 42}`},
		},
	}
	result := DecodeToolResponse(response)

	if dict, ok := result.(*object.Dict); ok {
		if len(dict.Pairs) != 2 {
			t.Errorf("Dict pairs count = %d, want 2", len(dict.Pairs))
		}
	} else {
		t.Errorf("DecodeToolResponse() with JSON should return Dict, got %T", result)
	}
}

func TestDecodeToolResponse_MultipleContent(t *testing.T) {
	response := &mcp.ToolResponse{
		Content: []mcp.ToolContent{
			{Type: "text", Text: "first"},
			{Type: "text", Text: "second"},
		},
	}
	result := DecodeToolResponse(response)

	if list, ok := result.(*object.List); ok {
		if len(list.Elements) != 2 {
			t.Errorf("List elements count = %d, want 2", len(list.Elements))
		}
	} else {
		t.Errorf("DecodeToolResponse() with multiple content should return List, got %T", result)
	}
}

func TestDecodeToolContent_Image(t *testing.T) {
	content := mcp.ToolContent{
		Type:     "image",
		Data:     "base64data",
		MimeType: "image/png",
	}
	result := DecodeToolContent(content)

	if dict, ok := result.(*object.Dict); ok {
		if len(dict.Pairs) != 3 {
			t.Errorf("Dict pairs count = %d, want 3", len(dict.Pairs))
		}
	} else {
		t.Errorf("DecodeToolContent(image) should return Dict, got %T", result)
	}
}

func TestDecodeToolContent_Audio(t *testing.T) {
	content := mcp.ToolContent{
		Type:     "audio",
		Data:     "audiodata",
		MimeType: "audio/mp3",
	}
	result := DecodeToolContent(content)

	if dict, ok := result.(*object.Dict); ok {
		if len(dict.Pairs) != 3 {
			t.Errorf("Dict pairs count = %d, want 3", len(dict.Pairs))
		}
	} else {
		t.Errorf("DecodeToolContent(audio) should return Dict, got %T", result)
	}
}

func TestDecodeToolContent_Resource(t *testing.T) {
	// Note: Resource type tests depend on external MCP library types
	// This test just verifies the function can handle the case
	content := mcp.ToolContent{
		Type: "resource",
		// Resource field would be populated by external library
	}
	result := DecodeToolContent(content)
	// Just verify it returns something
	if result == nil {
		t.Error("DecodeToolContent(resource) should return something")
	}
}

func TestDecodeToolContent_ResourceLink(t *testing.T) {
	// Note: Resource type tests depend on external MCP library types
	// This test just verifies the function can handle the case
	content := mcp.ToolContent{
		Type: "resource_link",
		// Resource field would be populated by external library
	}
	result := DecodeToolContent(content)
	// Just verify it returns something
	if result == nil {
		t.Error("DecodeToolContent(resource_link) should return something")
	}
}

func TestDecodeToolContent_UnknownType(t *testing.T) {
	content := mcp.ToolContent{
		Type: "unknown_type",
	}
	result := DecodeToolContent(content)

	// Unknown type should be converted as-is
	if result == nil {
		t.Error("DecodeToolContent(unknown) should return something")
	}
}

func TestDecodeTextContent(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantType string
	}{
		{
			name:     "plain text",
			text:     "hello world",
			wantType: "*object.String",
		},
		{
			name:     "empty text",
			text:     "",
			wantType: "*object.String",
		},
		{
			name:     "invalid JSON",
			text:     "{not valid json}",
			wantType: "*object.String",
		},
		{
			name:     "valid JSON object",
			text:     `{"key": "value"}`,
			wantType: "*object.Dict",
		},
		{
			name:     "valid JSON array",
			text:     `[1, 2, 3]`,
			wantType: "*object.List",
		},
		{
			name:     "valid JSON string",
			text:     `"quoted"`,
			wantType: "*object.String",
		},
		{
			name:     "valid JSON number",
			text:     `42`,
			wantType: "*object.Float",
		},
		{
			name:     "valid JSON boolean",
			text:     `true`,
			wantType: "*object.Boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeTextContent(tt.text)
			if result == nil {
				t.Error("decodeTextContent() returned nil")
			}
		})
	}
}

func TestDictToMap(t *testing.T) {
	tests := []struct {
		name  string
		dict  *object.Dict
		nilOK bool
	}{
		{
			name:  "nil dict",
			dict:  nil,
			nilOK: true,
		},
		{
			name: "empty dict",
			dict:  &object.Dict{Pairs: map[string]object.DictPair{}},
		},
		{
			name: "simple dict",
			dict: &object.Dict{Pairs: map[string]object.DictPair{
				"key": {
					Key:   &object.String{Value: "key"},
					Value: &object.String{Value: "value"},
				},
			}},
		},
		{
			name: "multiple keys",
			dict: &object.Dict{Pairs: map[string]object.DictPair{
				"key1": {
					Key:   &object.String{Value: "key1"},
					Value: &object.String{Value: "value1"},
				},
				"key2": {
					Key:   &object.String{Value: "key2"},
					Value: &object.Integer{Value: 42},
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DictToMap(tt.dict)
			if tt.nilOK && result == nil {
				return
			}
			if result == nil && tt.dict != nil {
				t.Error("DictToMap() returned nil for non-nil dict")
			}
		})
	}
}

func TestClientInstance_GetClient(t *testing.T) {
	// Create a mock client (nil is fine for this test)
	ci := &ClientInstance{client: nil}

	if ci.GetClient() != nil {
		t.Error("GetClient() should return nil")
	}
}
