package mcp

import (
	"encoding/json"

	"github.com/paularlott/scriptling/conversion"

	"github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/object"
)

// DecodeToolResponse converts an MCP ToolResponse to a scriptling Object.
// Handles:
// - Single text content: returns parsed JSON or string
// - Multiple content blocks: returns list of decoded blocks
// - Structured content: returns decoded object
func DecodeToolResponse(response *mcp.ToolResponse) object.Object {
	if response == nil {
		return &object.Null{}
	}

	// Handle structured content (new format)
	if response.StructuredContent != nil {
		return conversion.FromGo(response.StructuredContent)
	}

	// Handle content blocks
	if len(response.Content) == 0 {
		return &object.Null{}
	}

	// Single content block
	if len(response.Content) == 1 {
		return DecodeToolContent(response.Content[0])
	}

	// Multiple content blocks
	elements := make([]object.Object, len(response.Content))
	for i, block := range response.Content {
		elements[i] = DecodeToolContent(block)
	}
	return &object.List{Elements: elements}
}

// DecodeToolContent converts a single ToolContent block to a scriptling Object.
func DecodeToolContent(content mcp.ToolContent) object.Object {
	switch content.Type {
	case "text":
		return decodeTextContent(content.Text)
	case "image", "audio":
		return conversion.FromGo(map[string]interface{}{
			"type":     content.Type,
			"data":     content.Data,
			"mimeType": content.MimeType,
		})
	case "resource":
		if content.Resource != nil {
			return conversion.FromGo(map[string]interface{}{
				"type":     content.Type,
				"uri":      content.Resource.URI,
				"text":     content.Resource.Text,
				"mimeType": content.Resource.MimeType,
			})
		}
		return conversion.FromGo(content)
	case "resource_link":
		if content.Resource != nil {
			return conversion.FromGo(map[string]interface{}{
				"type": content.Type,
				"uri":  content.Resource.URI,
				"text": content.Resource.Text,
			})
		}
		return conversion.FromGo(content)
	default:
		// Unknown type, return as-is
		return conversion.FromGo(content)
	}
}

// decodeTextContent decodes text content, parsing JSON if valid.
func decodeTextContent(text string) object.Object {
	// Try to parse as JSON
	var jsonValue interface{}
	if err := json.Unmarshal([]byte(text), &jsonValue); err == nil {
		return conversion.FromGo(jsonValue)
	}
	// Return as plain string
	return &object.String{Value: text}
}

// DictToMap converts a scriptling Dict to a Go map[string]interface{}.
// This is a convenience wrapper around conversion.ToGo for the common case
// of converting Dict arguments for tool calls.
func DictToMap(dict *object.Dict) map[string]interface{} {
	if dict == nil {
		return nil
	}

	result := make(map[string]interface{}, len(dict.Pairs))
	for _, pair := range dict.Pairs {
		result[pair.StringKey()] = conversion.ToGo(pair.Value)
	}
	return result
}
