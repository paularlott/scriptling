package mcp

import (
	"strings"

	"github.com/paularlott/scriptling/conversion"

	"github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/object"
)

// DecodeToolResponse converts an MCP ToolResponse to a scriptling Object.
// Handles:
//   - Single text content: returns parsed JSON or string
//   - Multiple content blocks: returns list of decoded blocks
//   - Structured content: returns decoded object
//   - isError set (a tool execution error): returns an *object.Error carrying
//     the tool's text, so the caller raises it in the script just as a
//     JSON-RPC error from the server is raised. The remote tool's failure is
//     never handed to the script as if it were a successful result.
func DecodeToolResponse(response *mcp.ToolResponse) object.Object {
	if response == nil {
		return &object.Null{}
	}

	if response.IsError {
		return ToolResponseError(response)
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

// ToolErrorPrefix starts the message of the error a script sees when a tool
// reports a tool execution error (isError).
const ToolErrorPrefix = "MCP tool error: "

// ToolResponseErrorText is the text a tool execution error result carries:
// its text content blocks joined by newlines, or a generic message when it
// has none.
func ToolResponseErrorText(response *mcp.ToolResponse) string {
	var parts []string
	if response != nil {
		for _, c := range response.Content {
			if c.Type == "text" && c.Text != "" {
				parts = append(parts, c.Text)
			}
		}
	}
	if len(parts) == 0 {
		return "the tool reported an error"
	}
	return strings.Join(parts, "\n")
}

// ToolResponseError is the script-level error for a tool execution error
// result.
func ToolResponseError(response *mcp.ToolResponse) *object.Error {
	return &object.Error{Message: ToolErrorPrefix + ToolResponseErrorText(response)}
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
	// Try to parse as JSON, keeping Python's number typing: whole numbers
	// stay integers (MustParseJSON) instead of the all-float64 decoding of
	// encoding/json, so {"items": 3} reads back as 3, not 3.0.
	if parsed, err := conversion.ParseJSON(text); err == nil {
		return parsed
	}
	// Return as plain string
	return object.NewString(text)
}

// DictToMap converts a scriptling Dict to a Go map[string]interface{}.
// This is a convenience wrapper around conversion.ToGo for the common case
// of converting Dict arguments for tool calls.
func DictToMap(dict *object.Dict) map[string]interface{} {
	if dict == nil {
		return nil
	}
	return conversion.ToGo(dict).(map[string]interface{})
}
