package mcp

import (
	"encoding/json"

	mcplib "github.com/paularlott/mcp"
	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// ParseToolSearchResults extracts tool list from tool_search response.
// Returns a scriptling List containing tool dicts with name, description, input_schema.
// The tool_search response is expected to be a ToolResponse with JSON text content
// containing an array of tool definitions.
func ParseToolSearchResults(response *mcplib.ToolResponse) (*object.List, error) {
	if response == nil {
		return &object.List{Elements: []object.Object{}}, nil
	}

	// Extract JSON from the response
	var toolsJSON string

	// Check structured content first
	if response.StructuredContent != nil {
		// Try to convert structured content to JSON string
		jsonBytes, err := json.Marshal(response.StructuredContent)
		if err == nil {
			toolsJSON = string(jsonBytes)
		}
	}

	// Fall back to content blocks
	if toolsJSON == "" && len(response.Content) > 0 {
		// Get the first content block
		content := response.Content[0]
		if content.Type == "text" && content.Text != "" {
			toolsJSON = content.Text
		}
	}

	// If no JSON found, return empty list
	if toolsJSON == "" {
		return &object.List{Elements: []object.Object{}}, nil
	}

	return parseToolSearchJSON(toolsJSON)
}

// ParseToolSearchResultsFromText parses tool search results from a JSON string.
// This is a convenience function when you already have the JSON text extracted.
func ParseToolSearchResultsFromText(text string) (*object.List, error) {
	return parseToolSearchJSON(text)
}

// parseToolSearchJSON parses the JSON string containing tool search results.
func parseToolSearchJSON(toolsJSON string) (*object.List, error) {
	// Parse the JSON array of tools
	var tools []map[string]interface{}
	if err := json.Unmarshal([]byte(toolsJSON), &tools); err != nil {
		// Try parsing as a single object with tools array
		var result map[string]interface{}
		if err2 := json.Unmarshal([]byte(toolsJSON), &result); err2 == nil {
			if toolsArray, ok := result["tools"].([]interface{}); ok {
				// Convert toolsArray to []map[string]interface{}
				tools = make([]map[string]interface{}, 0, len(toolsArray))
				for _, t := range toolsArray {
					if toolMap, ok := t.(map[string]interface{}); ok {
						tools = append(tools, toolMap)
					}
				}
			} else {
				// Return error if we can't find tools
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Convert to scriptling objects, same format as list_tools
	toolList := make([]object.Object, 0, len(tools))
	for _, tool := range tools {
		toolDict := &object.Dict{
			Pairs: map[string]object.DictPair{},
		}
		for k, v := range tool {
			toolDict.Pairs[k] = object.DictPair{
				Key:   &object.String{Value: k},
				Value: scriptlib.FromGo(v),
			}
		}
		toolList = append(toolList, toolDict)
	}

	return &object.List{Elements: toolList}, nil
}
